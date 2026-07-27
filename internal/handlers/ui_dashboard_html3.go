package handlers

// dashboardHTML3 是 JavaScript 代码部分
const dashboardHTML3 = `
<script>
async function apiCall(endpoint, method, body) {
  method = method || 'POST';
  const options = { method: method, headers: { 'Content-Type': 'application/json' } };
  if (body) options.body = JSON.stringify(body);
  const response = await fetch(endpoint, options);
  return await response.json();
}

async function loadSystemInfo() {
  try {
    const data = await apiCall('/system/info', 'GET');
    if (data.success) {
      document.getElementById('activeTasks').textContent = data.active_tasks || 0;
      document.getElementById('totalTasks').textContent = data.total_tasks || 0;
      document.getElementById('cpuCount').textContent = data.cpu_count || '-';
      const memParts = (data.memory_mb || '').split(' ');
      if (memParts.length >= 2) {
        document.getElementById('memoryUsage').textContent = memParts[1] + ' MB';
      }
    }
  } catch (e) {
    console.error('Failed to load system info:', e);
  }
}

async function loadTasks() {
  try {
    const data = await apiCall('/task/list', 'GET');
    const container = document.getElementById('taskList');
    if (!data.success || !data.tasks || data.tasks.length === 0) {
      container.innerHTML = '<div class="empty-state">暂无任务</div>';
      return;
    }
    container.innerHTML = data.tasks.map(task => {
      const statusClass = task.status;
      const isRunning = task.status === 'running' || task.status === 'pending';
      const cmd = escapeHtml(task.command.substring(0, 80));
      const cmdSuffix = task.command.length > 80 ? '...' : '';
      return '<div class="task-item"><div class="task-info"><div class="task-id">' + task.task_id +
        '</div><div class="task-command">' + cmd + cmdSuffix + '</div></div><div class="task-actions">' +
        '<span class="task-status ' + statusClass + '">' + task.status + '</span>' +
        '<button class="secondary" onclick="viewTaskLogs(\'' + task.task_id + '\',' + isRunning + ')">查看日志</button>' +
        (isRunning ? '<button class="danger" onclick="cancelTask(\'' + task.task_id + '\')">取消</button>' : '') +
        '</div></div>';
    }).join('');
  } catch (e) {
    console.error('Failed to load tasks:', e);
    document.getElementById('taskList').innerHTML = '<div class="empty-state">加载失败</div>';
  }
}

async function executeCommand() {
  const command = document.getElementById('execCommand').value.trim();
  if (!command) { alert('请输入命令'); return; }
  const output = document.getElementById('execOutput');
  output.style.display = 'block';
  output.textContent = '执行中...';
  try {
    const data = await apiCall('/exec', 'POST', { command: command, timeout: 60 });
    if (data.success) {
      output.textContent = 'STDOUT:\n' + data.stdout + '\n\nSTDERR:\n' + data.stderr;
    } else {
      output.textContent = 'Error: ' + (data.error || 'Unknown error');
    }
  } catch (e) {
    output.textContent = 'Error: ' + e.message;
  }
}

async function takeScreenshot() {
  const output = document.getElementById('screenshotOutput');
  output.innerHTML = '<div class="loading">截图中...</div>';
  try {
    const data = await apiCall('/screenshot', 'POST', {});
    if (data.success && data.base64) {
      output.innerHTML = '<img src="data:image/png;base64,' + data.base64 + '" style="max-width:100%; border-radius:6px;">';
    } else {
      output.innerHTML = '<div style="color:#ef4444;">截图失败: ' + (data.error || 'Unknown error') + '</div>';
    }
  } catch (e) {
    output.innerHTML = '<div style="color:#ef4444;">Error: ' + e.message + '</div>';
  }
}

async function readFile() {
  const path = document.getElementById('readFilePath').value.trim();
  if (!path) { alert('请输入文件路径'); return; }
  const output = document.getElementById('readFileOutput');
  output.style.display = 'block';
  output.textContent = '读取中...';
  try {
    const data = await apiCall('/read_file', 'POST', { path: path });
    if (data.success) {
      output.textContent = data.content;
    } else {
      output.textContent = 'Error: ' + (data.error || 'Unknown error');
    }
  } catch (e) {
    output.textContent = 'Error: ' + e.message;
  }
}

async function writeFile() {
  const path = document.getElementById('writeFilePath').value.trim();
  const content = document.getElementById('writeFileContent').value;
  if (!path) { alert('请输入文件路径'); return; }
  const output = document.getElementById('writeFileOutput');
  output.style.display = 'block';
  output.textContent = '写入中...';
  try {
    const data = await apiCall('/write_file', 'POST', { path: path, content: content });
    if (data.success) {
      output.textContent = '写入成功: ' + data.bytes_written + ' 字节';
    } else {
      output.textContent = 'Error: ' + (data.error || 'Unknown error');
    }
  } catch (e) {
    output.textContent = 'Error: ' + e.message;
  }
}
`
