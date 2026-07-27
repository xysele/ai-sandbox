package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// maxOutputBytes 限制单次命令捕获的 stdout/stderr 大小，防止 agent 误跑
// `cat 大文件` 或死循环输出把网关内存打满。
const maxOutputBytes = 4 << 20 // 4 MiB

// CmdResult 是所有命令执行的统一返回结构。用具体类型而非 map，
// 调用方读字段时不需要类型断言，避免 panic。
type CmdResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Success  bool   `json:"success"`
}

func success(data map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{"success": true}
	for k, v := range data {
		result[k] = v
	}
	return result
}

func failure(message string, extra map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"success": false,
		"error":   message,
	}
	for k, v := range extra {
		result[k] = v
	}
	return result
}

// respondJSON 先把 JSON 编码进内存缓冲再写出。如果直接 Encode 到
// ResponseWriter，编码中途失败时状态码已经发出去了，没法再改成 500。
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"success":false,"error":"failed to encode response"}`)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func parseJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// requirePOST 统一处理方法校验，返回 false 表示已经写过响应了。
func requirePOST(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, failure("method not allowed", nil))
		return false
	}
	return true
}

// cappedBuffer 在写入超过上限后丢弃后续内容，但不报错，
// 这样长输出的命令仍能正常结束并拿到前 4 MiB。
type cappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	remaining := c.limit - c.buf.Len()
	if remaining <= 0 {
		c.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		c.buf.Write(p[:remaining])
		c.overflow = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string {
	if c.overflow {
		return c.buf.String() + fmt.Sprintf("\n[output truncated at %d bytes]", c.limit)
	}
	return c.buf.String()
}

// runArgv 直接以参数数组启动进程，不经过 shell。所有把用户输入拼进
// 命令行的地方都必须走这里，否则文本里的引号和反引号会变成命令注入。
func runArgv(argv []string, cwd string, env map[string]string, timeout int) CmdResult {
	if timeout <= 0 {
		timeout = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	return runArgvCtx(ctx, argv, cwd, env, timeout)
}

// runArgvCtx 与 runArgv 相同，但由调用方提供 context，用于异步任务
// 这种需要外部取消的场景。timeout 只用于超时提示文案。
func runArgvCtx(ctx context.Context, argv []string, cwd string, env map[string]string, timeout int) CmdResult {
	if len(argv) == 0 {
		return CmdResult{Stderr: "empty command", ExitCode: 1}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdout := &cappedBuffer{limit: maxOutputBytes}
	stderr := &cappedBuffer{limit: maxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	res := CmdResult{
		Stdout:  stdout.String(),
		Stderr:  stderr.String(),
		Success: err == nil,
	}
	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			res.ExitCode = 124
			res.Stderr += fmt.Sprintf("\n[timeout after %ds]", timeout)
		case errors.Is(ctx.Err(), context.Canceled):
			res.ExitCode = 143
			res.Stderr += "\n[cancelled]"
		default:
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				res.ExitCode = exitErr.ExitCode()
			} else {
				res.ExitCode = 1
				res.Stderr += "\n" + err.Error()
			}
		}
	}
	return res
}

// trimOut 截断字符串用于任务列表这类概要展示，避免把整段输出塞进列表响应。
func trimOut(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// runShell 通过 sh -lc 执行一整行命令，用于 /exec 这种明确要 shell
// 语义（管道、重定向、&&）的场景。
func runShell(command, cwd string, env map[string]string, timeout int) CmdResult {
	return runArgv([]string{"sh", "-lc", command}, cwd, env, timeout)
}

// respondCmd 把命令结果直接作为响应体返回。GUI 类接口都用它，
// success 字段取自命令自身的退出码，agent 不需要再猜。
func respondCmd(w http.ResponseWriter, res CmdResult) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   res.Success,
		"stdout":    res.Stdout,
		"stderr":    res.Stderr,
		"exit_code": res.ExitCode,
	})
}

// writeTempFile 把代码写进带指定后缀的临时文件，返回路径。
// /run_code 用它而不是 `python3 -c`：-c 要把代码拼进命令行，
// 多行代码和内嵌引号会被 shell 二次解释而破坏。后缀有意义，
// go run 之类的工具靠它判断语言。
func writeTempFile(content, ext string) (string, error) {
	f, err := os.CreateTemp("", "sandbox_*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func removeFile(path string) {
	_ = os.Remove(path)
}

// statError 把 os.Stat/os.Open 的错误翻译成稳定的 HTTP 状态与措辞。
// 直接回传 err.Error() 会把宿主的 syscall 文案（"no such file or
// directory"）和内部路径细节暴露给调用方，agent 也难以做条件判断。
func statError(err error, path string) (int, string) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return http.StatusNotFound, "not found: " + path
	case errors.Is(err, os.ErrPermission):
		return http.StatusForbidden, "permission denied: " + path
	default:
		return http.StatusInternalServerError, "cannot access: " + path
	}
}
