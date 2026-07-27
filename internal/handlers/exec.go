package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

type ExecRequest struct {
	Command    string            `json:"command"`
	Cwd        string            `json:"cwd"`
	Env        map[string]string `json:"env"`
	Timeout    int               `json:"timeout"`
	Background bool              `json:"background"`
}

type RunCodeRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Timeout  int    `json:"timeout"`
}

// ExecShell 执行一行 shell 命令。走 sh -lc，所以管道、重定向、&& 都可用。
func ExecShell(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req ExecRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if strings.TrimSpace(req.Command) == "" {
		respondJSON(w, http.StatusBadRequest, failure("command is empty", nil))
		return
	}

	// 后台模式：立刻返回 pid，输出重定向到日志文件。适合起长时服务
	// （比如 http.server）。需要拿结果的长任务应该用 /task/create。
	if req.Background {
		const logFile = "/tmp/sandbox_bg.log"
		res := runArgv([]string{
			"sh", "-c",
			fmt.Sprintf("nohup sh -lc \"$SANDBOX_BG_CMD\" >>%s 2>&1 & echo $!", logFile),
		}, req.Cwd, map[string]string{"SANDBOX_BG_CMD": req.Command}, 10)

		pid := strings.TrimSpace(res.Stdout)
		if pid == "" {
			respondJSON(w, http.StatusInternalServerError,
				failure("background launch failed: "+res.Stderr, nil))
			return
		}
		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"pid":      pid,
			"log_file": logFile,
		}))
		return
	}

	res := runShell(req.Command, req.Cwd, req.Env, req.Timeout)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   res.Success,
		"stdout":    res.Stdout,
		"stderr":    res.Stderr,
		"exit_code": res.ExitCode,
	})
}

// runCodeSpec 描述一种语言怎么跑：解释器命令 + 临时文件后缀。
// 代码写进临时文件再执行，不拼进命令行，因此代码里的引号是安全的。
var runCodeSpecs = map[string]struct {
	argv []string
	ext  string
}{
	"python": {argv: []string{"python3"}, ext: ".py"},
	"bash":   {argv: []string{"bash"}, ext: ".sh"},
	"node":   {argv: []string{"node"}, ext: ".js"},
	"go":     {argv: []string{"go", "run"}, ext: ".go"},
}

// RunCode 执行一段代码。相比 /exec 的好处是代码经临时文件传递，
// 多行代码和任意引号都不会被 shell 二次解释。
func RunCode(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req RunCodeRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Code == "" {
		respondJSON(w, http.StatusBadRequest, failure("code is empty", nil))
		return
	}
	if req.Language == "" {
		req.Language = "python"
	}

	spec, ok := runCodeSpecs[strings.ToLower(req.Language)]
	if !ok {
		supported := make([]string, 0, len(runCodeSpecs))
		for k := range runCodeSpecs {
			supported = append(supported, k)
		}
		respondJSON(w, http.StatusBadRequest, failure(
			"unsupported language: "+req.Language,
			map[string]interface{}{"supported": supported},
		))
		return
	}

	tmp, err := writeTempFile(req.Code, spec.ext)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(tmp)

	// 解释器不存在时给出可执行的修复建议，而不是让 agent 看到裸的
	// "executable file not found"。
	if _, err := exec.LookPath(spec.argv[0]); err != nil {
		respondJSON(w, http.StatusBadRequest, failure(
			fmt.Sprintf("interpreter %q not installed in sandbox", spec.argv[0]),
			map[string]interface{}{
				"hint": fmt.Sprintf("install it first, e.g. POST /exec {\"command\": \"apt-get install -y %s\"}", spec.argv[0]),
			},
		))
		return
	}

	res := runArgv(append(spec.argv, tmp), "", nil, req.Timeout)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   res.Success,
		"stdout":    res.Stdout,
		"stderr":    res.Stderr,
		"exit_code": res.ExitCode,
		"language":  req.Language,
	})
}
