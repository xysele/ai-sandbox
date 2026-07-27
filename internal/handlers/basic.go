package handlers

import (
	"net/http"

	"ai-sandbox-go/internal/config"
)

// Root 是无需鉴权的存活探针，只回一个固定标识，不泄露任何环境信息。
// "/" 在 ServeMux 里是前缀兜底，所有未注册路径都会落到这里，因此必须显式
// 判断路径并返回 404——否则 agent 把端点名拼错时会收到 200，误以为调用成功。
func Root(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		respondJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   "no such endpoint: " + r.URL.Path,
		})
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"service": "ai-sandbox-go",
		"ok":      true,
	})
}

// Health 报告服务与各子系统状态，是 agent 开工前的第一个调用。
// 无需鉴权，因此不返回 token、路径、环境变量等敏感内容。
func Health(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gui := xserverReady(cfg.Display)
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"ok":      true,
			"display": cfg.Display,
			// gui_ready 为 false 时，/screenshot 等 GUI 接口一定会失败，
			// agent 应据此跳过 GUI 路线而不是逐个试错。
			"gui_ready": gui && hasTool("xdotool"),
			"runtimes":  detectRuntimes(),
		})
	}
}

// GuiStatus 给出 GUI 子系统的逐项明细，用于 gui_ready 为 false 时定位
// 是 X server 没起来还是某个工具没装。
func GuiStatus(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		xserver := xserverReady(cfg.Display)
		tools := map[string]bool{}
		for _, t := range []string{"xdotool", "scrot", "import", "fluxbox"} {
			tools[t] = hasTool(t)
		}
		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"display":   cfg.Display,
			"xserver":   xserver,
			"tools":     tools,
			"gui_ready": xserver && tools["xdotool"],
		}))
	}
}

// detectRuntimes 探测 /run_code 支持的语言在当前镜像里是否真的可用。
// 镜像可裁剪，所以这里实测而不是写死列表。
func detectRuntimes() map[string]bool {
	out := map[string]bool{}
	for lang, spec := range runCodeSpecs {
		out[lang] = hasTool(spec.argv[0])
	}
	return out
}
