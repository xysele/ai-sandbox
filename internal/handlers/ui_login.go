package handlers

// loginFormHTML 是登录页面的 HTML。
const loginFormHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>AI Sandbox Gateway - Login</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: linear-gradient(135deg, #1e222e 0%, #0f1117 100%);
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #e5e7eb;
}
.login-container {
  background: #171a23;
  padding: 3rem 2.5rem;
  border-radius: 8px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.5);
  width: 100%;
  max-width: 400px;
}
h1 {
  font-size: 1.75rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
  color: #fbbf24;
}
.subtitle {
  color: #9ca3af;
  font-size: 0.875rem;
  margin-bottom: 2rem;
}
label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  margin-bottom: 0.5rem;
  color: #d1d5db;
}
input[type="password"] {
  width: 100%;
  padding: 0.75rem;
  background: #0f1117;
  border: 1px solid #374151;
  border-radius: 6px;
  color: #e5e7eb;
  font-size: 1rem;
  transition: border-color 0.2s;
}
input[type="password"]:focus {
  outline: none;
  border-color: #fbbf24;
}
button {
  width: 100%;
  padding: 0.75rem;
  margin-top: 1.5rem;
  background: #fbbf24;
  border: none;
  border-radius: 6px;
  color: #0f1117;
  font-size: 1rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}
button:hover {
  background: #f59e0b;
}
.footer {
  margin-top: 2rem;
  text-align: center;
  font-size: 0.75rem;
  color: #6b7280;
}
</style>
</head>
<body>
<div class="login-container">
  <h1>AI Sandbox Gateway</h1>
  <p class="subtitle">请输入 GATEWAY_TOKEN 以访问管理界面</p>
  <form method="POST" action="/ui/auth">
    <label for="token">Gateway Token</label>
    <input type="password" id="token" name="token" required autofocus>
    <button type="submit">登录</button>
  </form>
  <div class="footer">
    AI Sandbox Gateway v1.0
  </div>
</div>
</body>
</html>
`
