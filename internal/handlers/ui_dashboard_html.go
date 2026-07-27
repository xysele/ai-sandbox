package handlers

// dashboardHTML 是管理界面的完整 HTML（使用原始字符串字面量）。
const dashboardHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>AI Sandbox Gateway - Dashboard</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: #0f1117;
  color: #e5e7eb;
  line-height: 1.6;
}
.header {
  background: #171a23;
  padding: 1rem 2rem;
  border-bottom: 1px solid #374151;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header h1 {
  font-size: 1.5rem;
  font-weight: 600;
  color: #fbbf24;
}
.header .logout {
  padding: 0.5rem 1rem;
  background: #374151;
  border: none;
  border-radius: 6px;
  color: #e5e7eb;
  cursor: pointer;
  font-size: 0.875rem;
  text-decoration: none;
}
.header .logout:hover {
  background: #4b5563;
}
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 2rem;
}
.section {
  background: #171a23;
  border-radius: 8px;
  padding: 1.5rem;
  margin-bottom: 1.5rem;
  border: 1px solid #1e222e;
}
.section-title {
  font-size: 1.25rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: #fbbf24;
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-top: 1rem;
}
.stat-item {
  background: #1e222e;
  padding: 1rem;
  border-radius: 6px;
}
.stat-label {
  font-size: 0.875rem;
  color: #9ca3af;
  margin-bottom: 0.25rem;
}
.stat-value {
  font-size: 1.5rem;
  font-weight: 600;
  color: #fbbf24;
}
.button-group {
  display: flex;
  gap: 0.75rem;
  flex-wrap: wrap;
  margin-top: 1rem;
}
button, .button {
  padding: 0.75rem 1.25rem;
  background: #fbbf24;
  border: none;
  border-radius: 6px;
  color: #0f1117;
  font-size: 0.875rem;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}
button:hover, .button:hover {
  background: #f59e0b;
}
button.secondary {
  background: #374151;
  color: #e5e7eb;
}
button.secondary:hover {
  background: #4b5563;
}
button.danger {
  background: #ef4444;
  color: white;
}
button.danger:hover {
  background: #dc2626;
}
input[type="text"], textarea {
  width: 100%;
  padding: 0.75rem;
  background: #0f1117;
  border: 1px solid #374151;
  border-radius: 6px;
  color: #e5e7eb;
  font-size: 0.875rem;
  font-family: inherit;
}
input[type="text"]:focus, textarea:focus {
  outline: none;
  border-color: #fbbf24;
}
textarea {
  min-height: 120px;
  font-family: 'Courier New', monospace;
  resize: vertical;
}
label {
  display: block;
  font-size: 0.875rem;
  font-weight: 500;
  margin-bottom: 0.5rem;
  color: #d1d5db;
}
.form-group {
  margin-bottom: 1rem;
}
.task-list {
  margin-top: 1rem;
}
.task-item {
  background: #1e222e;
  padding: 1rem;
  border-radius: 6px;
  margin-bottom: 0.75rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.task-info {
  flex: 1;
}
.task-id {
  font-weight: 600;
  color: #fbbf24;
  font-size: 0.875rem;
}
.task-command {
  color: #9ca3af;
  font-size: 0.875rem;
  margin-top: 0.25rem;
  font-family: 'Courier New', monospace;
}
.task-status {
  padding: 0.25rem 0.75rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  margin-right: 0.5rem;
}
.task-status.running {
  background: #3b82f6;
  color: white;
}
.task-status.completed {
  background: #10b981;
  color: white;
}
.task-status.failed {
  background: #ef4444;
  color: white;
}
.task-status.cancelled {
  background: #6b7280;
  color: white;
}
.task-actions {
  display: flex;
  gap: 0.5rem;
}
.task-actions button {
  padding: 0.5rem 0.75rem;
  font-size: 0.75rem;
}
.modal {
  display: none;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.8);
  align-items: center;
  justify-content: center;
  z-index: 1000;
}
.modal.active {
  display: flex;
}
.modal-content {
  background: #171a23;
  border-radius: 8px;
  padding: 2rem;
  max-width: 800px;
  width: 90%;
  max-height: 80vh;
  overflow-y: auto;
  border: 1px solid #374151;
}
.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}
.modal-title {
  font-size: 1.25rem;
  font-weight: 600;
  color: #fbbf24;
}
.modal-close {
  background: none;
  border: none;
  color: #9ca3af;
  font-size: 1.5rem;
  cursor: pointer;
  padding: 0;
  line-height: 1;
}
.modal-close:hover {
  color: #e5e7eb;
}
.output-box {
  background: #0f1117;
  border: 1px solid #374151;
  border-radius: 6px;
  padding: 1rem;
  font-family: 'Courier New', monospace;
  font-size: 0.875rem;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 400px;
  overflow-y: auto;
  margin-top: 1rem;
}
.loading {
  color: #9ca3af;
  font-style: italic;
}
.empty-state {
  text-align: center;
  padding: 2rem;
  color: #6b7280;
}
</style>
</head>
<body>
`
