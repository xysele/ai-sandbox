package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

// 浏览器自动化端点，基于 Playwright + Node.js。
// 每个操作启动独立的浏览器实例，避免状态泄漏。

type browserNavigateRequest struct {
	URL     string `json:"url"`
	Timeout int    `json:"timeout"`
}

type browserScreenshotRequest struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	FullPage bool   `json:"full_page"`
	Timeout  int    `json:"timeout"`
}

type browserClickRequest struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	Timeout  int    `json:"timeout"`
}

type browserTypeRequest struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	Text     string `json:"text"`
	Timeout  int    `json:"timeout"`
}

type browserEvaluateRequest struct {
	URL     string `json:"url"`
	Script  string `json:"script"`
	Timeout int    `json:"timeout"`
}

type browserWaitForRequest struct {
	URL      string `json:"url"`
	Selector string `json:"selector"`
	Timeout  int    `json:"timeout"`
}

// BrowserStatus 检查 Playwright 是否可用。
func BrowserStatus(w http.ResponseWriter, r *http.Request) {
	// 检查 playwright-chromium 是否安装
	checkScript := `
const { chromium } = require('playwright-chromium');
console.log(JSON.stringify({installed: true, version: chromium.name()}));
`
	scriptPath, err := writeTempFile(checkScript, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	res := runArgv([]string{"node", scriptPath}, "", nil, 10)

	if !res.Success {
		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"playwright_installed": false,
			"error":                res.Stderr,
			"hint":                 "Run: npm install -g playwright-chromium && playwright install chromium --with-deps",
		}))
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(res.Stdout), &result); err != nil {
		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"playwright_installed": false,
			"error":                "cannot parse version info",
		}))
		return
	}

	result["playwright_installed"] = true
	respondJSON(w, http.StatusOK, success(result))
}

// BrowserScreenshot 截取网页截图。
func BrowserScreenshot(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserScreenshotRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, failure("url is required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});

  let screenshot;
  if (process.env.SELECTOR) {
    const element = await page.$(process.env.SELECTOR);
    if (!element) throw new Error('Selector not found: ' + process.env.SELECTOR);
    screenshot = await element.screenshot();
  } else {
    screenshot = await page.screenshot({fullPage: process.env.FULL_PAGE === 'true'});
  }

  await browser.close();
  console.log(JSON.stringify({
    success: true,
    base64: screenshot.toString('base64'),
    size: screenshot.length
  }));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL": req.URL,
		"SELECTOR":   req.Selector,
		"FULL_PAGE":  fmt.Sprintf("%t", req.FullPage),
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// BrowserClick 点击网页元素。
func BrowserClick(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserClickRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" || req.Selector == "" {
		respondJSON(w, http.StatusBadRequest, failure("url and selector are required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});
  await page.click(process.env.SELECTOR);
  await browser.close();
  console.log(JSON.stringify({success: true}));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL": req.URL,
		"SELECTOR":   req.Selector,
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// BrowserType 在输入框中输入文本。
func BrowserType(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserTypeRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" || req.Selector == "" || req.Text == "" {
		respondJSON(w, http.StatusBadRequest, failure("url, selector and text are required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});
  await page.fill(process.env.SELECTOR, process.env.TEXT);
  await browser.close();
  console.log(JSON.stringify({success: true}));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL": req.URL,
		"SELECTOR":   req.Selector,
		"TEXT":       req.Text,
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// BrowserEvaluate 在页面上下文中执行 JavaScript。
func BrowserEvaluate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserEvaluateRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" || req.Script == "" {
		respondJSON(w, http.StatusBadRequest, failure("url and script are required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// 用 base64 传递脚本避免引号问题
	scriptB64 := base64.StdEncoding.EncodeToString([]byte(req.Script))

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});

  const userScript = Buffer.from(process.env.USER_SCRIPT_B64, 'base64').toString('utf-8');
  const result = await page.evaluate(userScript);

  await browser.close();
  console.log(JSON.stringify({success: true, result: result}));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL":      req.URL,
		"USER_SCRIPT_B64": scriptB64,
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// BrowserWaitFor 等待选择器出现。
func BrowserWaitFor(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserWaitForRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" || req.Selector == "" {
		respondJSON(w, http.StatusBadRequest, failure("url and selector are required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});
  await page.waitForSelector(process.env.SELECTOR, {timeout: %d});
  await browser.close();
  console.log(JSON.stringify({success: true}));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL": req.URL,
		"SELECTOR":   req.Selector,
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// BrowserNavigate 导航到 URL 并等待加载完成。
func BrowserNavigate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req browserNavigateRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, failure("url is required", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	script := fmt.Sprintf(`
const { chromium } = require('playwright-chromium');
(async () => {
  const browser = await chromium.launch({headless: true, args: ['--no-sandbox']});
  const page = await browser.newPage();
  const response = await page.goto(process.env.TARGET_URL, {waitUntil: 'networkidle', timeout: %d});
  await browser.close();
  console.log(JSON.stringify({
    success: true,
    status: response.status(),
    url: response.url()
  }));
})().catch(e => {
  console.log(JSON.stringify({success: false, error: e.message}));
  process.exit(1);
});
`, req.Timeout*1000)

	scriptPath, err := writeTempFile(script, ".js")
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(err.Error(), nil))
		return
	}
	defer removeFile(scriptPath)

	env := map[string]string{
		"TARGET_URL": req.URL,
	}

	res := runArgv([]string{"node", scriptPath}, "", env, req.Timeout+10)
	respondBrowserResult(w, res)
}

// respondBrowserResult 解析 Playwright 脚本的 JSON 输出并返回。
func respondBrowserResult(w http.ResponseWriter, res CmdResult) {
	if !res.Success {
		respondJSON(w, http.StatusInternalServerError, failure(
			"browser operation failed",
			map[string]interface{}{"stderr": res.Stderr},
		))
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(res.Stdout), &result); err != nil {
		respondJSON(w, http.StatusInternalServerError, failure(
			"cannot parse browser result",
			map[string]interface{}{"stdout": res.Stdout, "error": err.Error()},
		))
		return
	}

	if success, ok := result["success"].(bool); ok && !success {
		respondJSON(w, http.StatusOK, failure(
			result["error"].(string),
			result,
		))
		return
	}

	respondJSON(w, http.StatusOK, success(result))
}

// randomHex 生成随机十六进制字符串（用于临时文件名）。
func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
