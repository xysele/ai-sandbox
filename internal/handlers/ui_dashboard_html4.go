package handlers

// dashboardHTML4 是 JavaScript 剩余函数
const dashboardHTML4 = `
let taskLogInterval = null;
async function viewTaskLogs(taskId, isRunning) {
  showModal('taskLogsModal');
  const output = document.getElementById('taskLogsOutput');
  output.textContent = '加载中...';
  if (taskLogInterval) {
    clearInterval(taskLogInterval);
    taskLogInterval = null;
  }
  async function fetchLogs() {
    try {
      const data = await apiCall('/task/status?task_id=' + taskId + '&stream=true', 'GET');
      if (data.success) {
        let content = '';
        if (data.status === 'running' || data.status === 'pending') {
          content = 'Status: ' + data.status + '\n\n';
          content += 'STDOUT (partial):\n' + (data.current_stdout || '(empty)') + '\n\n';
          content += 'STDERR (partial):\n' + (data.current_stderr || '(empty)');
        } else {
          content = 'Status: ' + data.status + '\n\n';
          if (data.result) {
            content += 'Exit Code: ' + data.result.exit_code + '\n\n';
            content += 'STDOUT:\n' + data.result.stdout + '\n\n';
            content += 'STDERR:\n' + data.result.stderr;
          }
          if (taskLogInterval) {
            clearInterval(taskLogInterval);
            taskLogInterval = null;
          }
        }
        output.textContent = content;
      } else {
        output.textContent = 'Error: ' + (data.error || 'Unknown error');
      }
    } catch (e) {
      output.textContent = 'Error: ' + e.message;
    }
  }
  await fetchLogs();
  if (isRunning) {
    taskLogInterval = setInterval(fetchLogs, 2000);
  }
}

async function cancelTask(taskId) {
  if (!confirm('确定要取消任务 ' + taskId + ' 吗？')) return;
  try {
    const data = await apiCall('/task/cancel', 'POST', { task_id: taskId });
    if (data.success) {
      alert('任务已取消');
      loadTasks();
    } else {
      alert('取消失败: ' + (data.error || 'Unknown error'));
    }
  } catch (e) {
    alert('Error: ' + e.message);
  }
}

function showModal(id) {
  document.getElementById(id).classList.add('active');
}

function closeModal(id) {
  document.getElementById(id).classList.remove('active');
  if (id === 'taskLogsModal' && taskLogInterval) {
    clearInterval(taskLogInterval);
    taskLogInterval = null;
  }
}

function showExecModal() { showModal('execModal'); }
function showScreenshotModal() { showModal('screenshotModal'); }
function showReadFileModal() { showModal('readFileModal'); }
function showWriteFileModal() { showModal('writeFileModal'); }

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

document.addEventListener('DOMContentLoaded', function() {
  loadSystemInfo();
  loadTasks();
  setInterval(() => {
    loadSystemInfo();
    loadTasks();
  }, 5000);
});

document.addEventListener('click', function(e) {
  if (e.target.classList.contains('modal')) {
    e.target.classList.remove('active');
    if (taskLogInterval) {
      clearInterval(taskLogInterval);
      taskLogInterval = null;
    }
  }
});
</script>
</body>
</html>
`
