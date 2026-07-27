package handlers

import (
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPFetchRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Timeout int               `json:"timeout"`
}

// maxFetchBytes 限制单次抓取的响应体大小。响应要 base64/JSON 编码后
// 整体返回给 agent，不设上限的话一个大文件 URL 就能打满内存。
const maxFetchBytes = 8 << 20 // 8 MiB

// HTTPFetch 代沙箱发起一次 HTTP 请求并把状态码、响应头、响应体一起返回。
//
// 存在的理由：agent 完全可以用 /exec 跑 curl，但要拿到结构化的响应头和
// 状态码就得自己解析 curl 的输出，容易出错。这里直接给结构化结果。
func HTTPFetch(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req HTTPFetchRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, failure("url is empty", nil))
		return
	}
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(strings.ToUpper(req.Method), req.URL, bodyReader)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, failure(err.Error(), nil))
		return
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: time.Duration(req.Timeout) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, failure(err.Error(), nil))
		return
	}
	defer resp.Body.Close()

	// 多读 1 字节以判断是否被截断
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	truncated := len(body) > maxFetchBytes
	if truncated {
		body = body[:maxFetchBytes]
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     headers,
		"body":        string(body),
		"size":        len(body),
		"truncated":   truncated,
	}))
}
