package handlers

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// maxReadBytes 限制单次 read_file / download 的大小。超过这个体积的文件
// 应该让 agent 用 offset/limit 分段读，或者在沙箱内先处理再取结果。
const maxReadBytes = 32 << 20 // 32 MiB

type pathRequest struct {
	Path string `json:"path"`
}

type readFileRequest struct {
	Path   string `json:"path"`
	Offset *int   `json:"offset"`
	Limit  *int   `json:"limit"`
}

type writeFileRequest struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	ContentB64 string `json:"content_b64"`
	Append     bool   `json:"append"`
}

type listDirRequest struct {
	Path       string `json:"path"`
	ShowHidden bool   `json:"show_hidden"`
}

type searchRequest struct {
	Pattern       string `json:"pattern"`
	Path          string `json:"path"`
	Glob          string `json:"glob"`
	AfterContext  *int   `json:"after_context"`
	BeforeContext *int   `json:"before_context"`
	MaxResults    int    `json:"max_results"`
}

// ReadFile 读取文件内容。offset/limit 以行为单位，便于 agent 分段读大文件。
func ReadFile(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req readFileRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" {
		respondJSON(w, http.StatusBadRequest, failure("path is empty", nil))
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		status, msg := statError(err, req.Path)
		respondJSON(w, status, failure(msg, map[string]interface{}{"path": req.Path}))
		return
	}
	if info.IsDir() {
		respondJSON(w, http.StatusBadRequest, failure("is a directory: "+req.Path, map[string]interface{}{"path": req.Path}))
		return
	}
	if info.Size() > maxReadBytes {
		respondJSON(w, http.StatusRequestEntityTooLarge, failure(
			"file too large; read it in chunks with offset/limit or process it in-sandbox",
			map[string]interface{}{"path": req.Path, "size": info.Size(), "max": maxReadBytes},
		))
		return
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	content := string(data)
	truncated := false
	if req.Offset != nil || req.Limit != nil {
		content, truncated = sliceLines(content, req.Offset, req.Limit)
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path":         req.Path,
		"content":      content,
		"size":         info.Size(),
		"more_lines":   truncated,
	}))
}

// WriteFile 写文件。支持 append 追加与 content_b64 二进制写入。
func WriteFile(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req writeFileRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" {
		respondJSON(w, http.StatusBadRequest, failure("path is empty", nil))
		return
	}

	if err := os.MkdirAll(filepath.Dir(req.Path), 0o755); err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	data := []byte(req.Content)
	if req.ContentB64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(req.ContentB64)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid base64: "+err.Error(), map[string]interface{}{"path": req.Path}))
			return
		}
		data = decoded
	}

	var err error
	if req.Append {
		var f *os.File
		f, err = os.OpenFile(req.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, err = f.Write(data)
			cerr := f.Close()
			if err == nil {
				err = cerr
			}
		}
	} else {
		err = os.WriteFile(req.Path, data, 0o644)
	}
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path":          req.Path,
		"bytes_written": len(data),
	}))
}

// DeleteFile 删除文件或目录（递归）。
func DeleteFile(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req pathRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" {
		respondJSON(w, http.StatusBadRequest, failure("path is empty", nil))
		return
	}
	// 拒绝删除根目录这类明显是失误的请求。
	if cleaned := filepath.Clean(req.Path); cleaned == "/" || cleaned == "." {
		respondJSON(w, http.StatusBadRequest, failure("refusing to delete "+cleaned, map[string]interface{}{"path": req.Path}))
		return
	}
	if err := os.RemoveAll(req.Path); err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}
	respondJSON(w, http.StatusOK, success(map[string]interface{}{"path": req.Path}))
}

// ListDir 列出目录内容。
func ListDir(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req listDirRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" {
		req.Path = "."
	}

	dirEntries, err := os.ReadDir(req.Path)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		respondJSON(w, status, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	entries := make([]map[string]interface{}, 0, len(dirEntries))
	for _, e := range dirEntries {
		if !req.ShowHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		item := map[string]interface{}{"name": e.Name(), "type": "file", "size": int64(0)}
		if e.IsDir() {
			item["type"] = "dir"
		} else if info, err := e.Info(); err == nil {
			item["size"] = info.Size()
			item["modified"] = info.ModTime().Unix()
		}
		entries = append(entries, item)
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path":    req.Path,
		"entries": entries,
		"count":   len(entries),
	}))
}

// SearchFiles 用 grep -rn 递归搜索文件内容。
func SearchFiles(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req searchRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Pattern == "" {
		respondJSON(w, http.StatusBadRequest, failure("pattern is empty", nil))
		return
	}
	if req.Path == "" {
		req.Path = "."
	}
	if req.MaxResults <= 0 {
		req.MaxResults = 200
	}

	// argv 形式传参：pattern 里的引号、$、反引号都是字面量，不会被 shell 解释。
	argv := []string{"grep", "-rn", "--color=never", "-m", strconv.Itoa(req.MaxResults)}
	if req.BeforeContext != nil && *req.BeforeContext > 0 {
		argv = append(argv, "-B", strconv.Itoa(*req.BeforeContext))
	}
	if req.AfterContext != nil && *req.AfterContext > 0 {
		argv = append(argv, "-A", strconv.Itoa(*req.AfterContext))
	}
	if req.Glob != "" {
		argv = append(argv, "--include", req.Glob)
	}
	// -e 和 -- 确保以 - 开头的 pattern / path 不被当成选项。
	argv = append(argv, "-e", req.Pattern, "--", req.Path)

	res := runArgv(argv, "", nil, 60)

	// grep 退出码：0 有匹配，1 无匹配（不是错误），>1 才是真错误。
	if res.ExitCode > 1 {
		respondJSON(w, http.StatusOK, failure(res.Stderr, map[string]interface{}{"path": req.Path}))
		return
	}

	matchCount := 0
	if res.Stdout != "" {
		matchCount = len(strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n"))
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path":        req.Path,
		"matches":     res.Stdout,
		"match_lines": matchCount,
	}))
}

// Upload 通过 base64 上传文件。语义与 write_file + content_b64 相同，
// 保留为独立端点是为了兼容按 upload/download 命名的 agent 工具定义。
func Upload(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req struct {
		Path       string `json:"path"`
		ContentB64 string `json:"content_b64"`
	}
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" || req.ContentB64 == "" {
		respondJSON(w, http.StatusBadRequest, failure("path and content_b64 are required", nil))
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.ContentB64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid base64: "+err.Error(), nil))
		return
	}
	if err := os.MkdirAll(filepath.Dir(req.Path), 0o755); err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}
	if err := os.WriteFile(req.Path, data, 0o644); err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path": req.Path,
		"size": len(data),
	}))
}

// Download 以 base64 取回文件内容。
func Download(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}
	var req pathRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Path == "" {
		respondJSON(w, http.StatusBadRequest, failure("path is empty", nil))
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		respondJSON(w, status, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}
	if !info.Mode().IsRegular() {
		respondJSON(w, http.StatusBadRequest, failure("not a regular file: "+req.Path, map[string]interface{}{"path": req.Path}))
		return
	}
	if info.Size() > maxReadBytes {
		respondJSON(w, http.StatusRequestEntityTooLarge, failure(
			"file too large; compress it in-sandbox via /exec first",
			map[string]interface{}{"path": req.Path, "size": info.Size(), "max": maxReadBytes},
		))
		return
	}

	data, err := os.ReadFile(req.Path)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), map[string]interface{}{"path": req.Path}))
		return
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"path":        req.Path,
		"content_b64": base64.StdEncoding.EncodeToString(data),
		"size":        len(data),
	}))
}

// sliceLines 按行切片，第二个返回值表示后面还有未返回的行。
func sliceLines(text string, offset, limit *int) (string, bool) {
	lines := strings.Split(text, "\n")
	start := 0
	if offset != nil && *offset > 0 {
		start = *offset
	}
	if start >= len(lines) {
		return "", false
	}
	end := len(lines)
	if limit != nil && *limit > 0 && start+*limit < end {
		end = start + *limit
	}
	return strings.Join(lines[start:end], "\n"), end < len(lines)
}

const (
	maxBatchFiles         = 100      // 单次批量写入最多文件数
	maxBatchTotalSize     = 16 << 20 // 所有文件内容总和上限 16 MiB
)

type writeFilesRequest struct {
	Files []struct {
		Path       string `json:"path"`
		Content    string `json:"content"`
		ContentB64 string `json:"content_b64"`
	} `json:"files"`
}

// WriteFiles 批量写入多个文件，适合 agent 生成多文件项目。
//
// 实现为半事务性：如果中途失败，已写入的文件会尝试回滚删除。
// 但如果回滚失败（磁盘满、权限变化），可能留下部分文件。
func WriteFiles(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req writeFilesRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}

	if len(req.Files) == 0 {
		respondJSON(w, http.StatusBadRequest, failure("files array is empty", nil))
		return
	}
	if len(req.Files) > maxBatchFiles {
		respondJSON(w, http.StatusBadRequest, failure(
			"too many files",
			map[string]interface{}{"max": maxBatchFiles, "requested": len(req.Files)},
		))
		return
	}

	// 预先计算总大小，避免写到一半才发现超限
	totalSize := 0
	for i, f := range req.Files {
		if f.Path == "" {
			respondJSON(w, http.StatusBadRequest, failure(
				"path is empty",
				map[string]interface{}{"index": i},
			))
			return
		}
		size := len(f.Content)
		if f.ContentB64 != "" {
			size = len(f.ContentB64) * 3 / 4 // base64 解码后约为原长度的 3/4
		}
		totalSize += size
	}
	if totalSize > maxBatchTotalSize {
		respondJSON(w, http.StatusRequestEntityTooLarge, failure(
			"total content size exceeds limit",
			map[string]interface{}{"total": totalSize, "max": maxBatchTotalSize},
		))
		return
	}

	// 先创建所有父目录
	dirs := make(map[string]bool)
	for _, f := range req.Files {
		dirs[filepath.Dir(f.Path)] = true
	}
	for dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			respondJSON(w, http.StatusInternalServerError, failure(
				"cannot create directory: "+err.Error(),
				map[string]interface{}{"dir": dir},
			))
			return
		}
	}

	// 写入所有文件，记录成功的路径用于回滚
	results := make([]map[string]interface{}, 0, len(req.Files))
	written := []string{}

	for _, f := range req.Files {
		data := []byte(f.Content)
		if f.ContentB64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(f.ContentB64)
			if err != nil {
				// 回滚已写入的文件
				rollbackFiles(written)
				respondJSON(w, http.StatusBadRequest, failure(
					"invalid base64 in file: "+f.Path,
					map[string]interface{}{"error": err.Error()},
				))
				return
			}
			data = decoded
		}

		if err := os.WriteFile(f.Path, data, 0o644); err != nil {
			// 回滚已写入的文件
			rollbackFiles(written)
			respondJSON(w, http.StatusInternalServerError, failure(
				"failed to write file: "+err.Error(),
				map[string]interface{}{"path": f.Path},
			))
			return
		}

		written = append(written, f.Path)
		results = append(results, map[string]interface{}{
			"path":  f.Path,
			"bytes": len(data),
		})
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"files_written": len(written),
		"results":       results,
	}))
}

// rollbackFiles 删除批量写入失败后已创建的文件。
func rollbackFiles(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p) // 尽力而为，失败也不报错
	}
}

