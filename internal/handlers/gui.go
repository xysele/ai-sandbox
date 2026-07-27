package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ai-sandbox-go/internal/config"
)

type ScreenshotRequest struct {
	Path string `json:"path"`
}

type ClickRequest struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button"`
	Double bool   `json:"double"`
}

type KeyboardRequest struct {
	Text string `json:"text"`
	Key  string `json:"key"`
}

type MoveMouseRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Screenshot 截取整个虚拟屏幕，返回 base64 PNG。
func Screenshot(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}

		var req ScreenshotRequest
		// 允许空 body：agent 常直接 POST 不带参数。
		if r.ContentLength > 0 {
			_ = parseJSON(r, &req)
		}

		outPath := req.Path
		if outPath == "" {
			outPath = filepath.Join(cfg.ScreenshotDir, fmt.Sprintf("shot_%d.png", time.Now().UnixMilli()))
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
			return
		}
		// 覆盖旧文件，否则 scrot 会因目标已存在而失败。
		_ = os.Remove(outPath)

		// scrot 优先，失败回退 imagemagick 的 import。
		res := runArgv([]string{"scrot", "--overwrite", outPath}, "", nil, 15)
		if !res.Success {
			res = runArgv([]string{"import", "-window", "root", outPath}, "", nil, 15)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError,
				failure("screenshot failed: "+res.Stderr, nil))
			return
		}

		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"path":      outPath,
			"mime_type": "image/png",
			"base64":    base64.StdEncoding.EncodeToString(data),
			"size":      len(data),
		}))
	}
}

// Click 移动鼠标到坐标并点击。button 支持 left/middle/right，
// double=true 时双击。
func Click(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}
		var req ClickRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}

		btn, ok := map[string]string{"left": "1", "middle": "2", "right": "3"}[req.Button]
		if !ok {
			if req.Button != "" {
				respondJSON(w, http.StatusBadRequest,
					failure("button must be left, middle or right", nil))
				return
			}
			btn = "1"
		}

		argv := []string{"xdotool", "mousemove", strconv.Itoa(req.X), strconv.Itoa(req.Y), "click"}
		if req.Double {
			argv = append(argv, "--repeat", "2")
		}
		argv = append(argv, btn)

		respondCmd(w, runArgv(argv, "", nil, 15))
	}
}

// TypeText 把文本作为键盘输入送入当前焦点窗口。文本走 argv 传递，
// 不经 shell，因此引号、$、反引号都按字面输入。
func TypeText(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}
		var req KeyboardRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}
		if req.Text == "" {
			respondJSON(w, http.StatusBadRequest, failure("text is empty", nil))
			return
		}
		respondCmd(w, runArgv([]string{"xdotool", "type", "--", req.Text}, "", nil, 30))
	}
}

// PressKey 发送单个按键或组合键，如 Return、ctrl+c、alt+Tab。
func PressKey(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}
		var req KeyboardRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}
		if req.Key == "" {
			respondJSON(w, http.StatusBadRequest, failure("key is empty", nil))
			return
		}
		respondCmd(w, runArgv([]string{"xdotool", "key", "--", req.Key}, "", nil, 15))
	}
}

// MoveMouse 只移动光标，不点击。用于触发 hover 效果。
func MoveMouse(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}
		var req MoveMouseRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}
		respondCmd(w, runArgv(
			[]string{"xdotool", "mousemove", strconv.Itoa(req.X), strconv.Itoa(req.Y)},
			"", nil, 15))
	}
}

// xserverReady 检查 X server 的 unix socket 是否存在。
// DISPLAY 形如 ":99" 或 ":99.0"，socket 路径只取屏幕号前的部分。
func xserverReady(display string) bool {
	num := strings.SplitN(strings.TrimPrefix(display, ":"), ".", 2)[0]
	_, err := os.Stat(filepath.Join("/tmp/.X11-unix", "X"+num))
	return err == nil
}

// hasTool 用 exec.LookPath 语义判断工具是否可用，比起 shell 调 which
// 少一次进程启动。
func hasTool(name string) bool {
	return runArgv([]string{"sh", "-c", "command -v " + name}, "", nil, 5).Success
}
