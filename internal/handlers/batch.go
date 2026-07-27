package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

const maxBatchOps = 50 // 单次批量操作最多执行的子操作数

type batchRequest struct {
	Operations  []batchOperation `json:"operations"`
	StopOnError bool             `json:"stop_on_error"`
}

type batchOperation struct {
	Endpoint string                 `json:"endpoint"`
	Body     map[string]interface{} `json:"body"`
}

// Batch 批量执行多个 API 操作，减少 HTTP 往返次数。
//
// 适用场景：agent 需要连续执行多个操作（写文件→执行→读结果），一次请求完成。
// 每个子操作复用现有 handler，认证在批量端点级别完成，子操作不重复鉴权。
func Batch(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePOST(w, r) {
			return
		}

		var req batchRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}

		if len(req.Operations) == 0 {
			respondJSON(w, http.StatusBadRequest, failure("operations array is empty", nil))
			return
		}
		if len(req.Operations) > maxBatchOps {
			respondJSON(w, http.StatusBadRequest, failure(
				"too many operations",
				map[string]interface{}{"max": maxBatchOps, "requested": len(req.Operations)},
			))
			return
		}

		results := make([]map[string]interface{}, 0, len(req.Operations))
		executed := 0
		failed := 0

		for i, op := range req.Operations {
			if op.Endpoint == "" {
				results = append(results, failure("endpoint is empty", map[string]interface{}{"index": i}))
				failed++
				if req.StopOnError {
					break
				}
				continue
			}

			// 批量端点本身不能递归调用，避免无限嵌套
			if op.Endpoint == "/batch" {
				results = append(results, failure("recursive batch not allowed", map[string]interface{}{"index": i}))
				failed++
				if req.StopOnError {
					break
				}
				continue
			}

			result := executeSingleOperation(mux, op.Endpoint, op.Body)
			results = append(results, result)
			executed++

			// 检查是否成功
			if success, ok := result["success"].(bool); ok && !success {
				failed++
				if req.StopOnError {
					break
				}
			}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":  failed == 0,
			"results":  results,
			"executed": executed,
			"failed":   failed,
			"total":    len(req.Operations),
		})
	}
}

// executeSingleOperation 调用指定端点的 handler 并捕获响应。
func executeSingleOperation(mux *http.ServeMux, endpoint string, body map[string]interface{}) map[string]interface{} {
	// 构造内部请求
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return failure("invalid body for "+endpoint, map[string]interface{}{"error": err.Error()})
	}

	req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// 用 ResponseRecorder 捕获响应
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// 解析响应 JSON
	var result map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		// handler 返回了非 JSON（不应该发生，因为所有 handler 都用 respondJSON）
		return failure("handler returned non-JSON", map[string]interface{}{
			"endpoint":    endpoint,
			"status_code": rec.Code,
			"body":        rec.Body.String(),
		})
	}

	// 附加 HTTP 状态码供调试
	result["_status_code"] = rec.Code
	return result
}
