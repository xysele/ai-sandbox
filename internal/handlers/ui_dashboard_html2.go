package handlers

// dashboardHTML2 继续 HTML 结构部分
const dashboardHTML2 = `
<div class="header">
  <h1>AI Sandbox Gateway</h1>
  <a href="/ui/logout" class="logout">登出</a>
</div>

<div class="container">
  <div class="section">
    <h2 class="section-title">系统状态</h2>
    <div class="stat-grid" id="systemStats">
      <div class="stat-item">
        <div class="stat-label">活跃任务</div>
        <div class="stat-value" id="activeTasks">-</div>
      </div>
      <div class="stat-item">
        <div class="stat-label">总任务数</div>
        <div class="stat-value" id="totalTasks">-</div>
      </div>
      <div class="stat-item">
        <div class="stat-label">CPU 核心</div>
        <div class="stat-value" id="cpuCount">-</div>
      </div>
      <div class="stat-item">
        <div class="stat-label">内存使用</div>
        <div class="stat-value" id="memoryUsage">-</div>
      </div>
    </div>
  </div>

  <div class="section">
    <h2 class="section-title">快速操作</h2>
    <div class="button-group">
      <button onclick="showExecModal()">执行命令</button>
      <button onclick="showScreenshotModal()">截图</button>
      <button onclick="showReadFileModal()">读取文件</button>
      <button onclick="showWriteFileModal()">写入文件</button>
    </div>
  </div>

  <div class="section">
    <h2 class="section-title">任务列表</h2>
    <div id="taskList" class="task-list">
      <div class="loading">加载中...</div>
    </div>
  </div>
</div>

<div id="execModal" class="modal">
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">执行命令</h3>
      <button class="modal-close" onclick="closeModal('execModal')">&times;</button>
    </div>
    <div class="form-group">
      <label for="execCommand">命令</label>
      <textarea id="execCommand" placeholder="ls -la"></textarea>
    </div>
    <button onclick="executeCommand()">执行</button>
    <div id="execOutput" class="output-box" style="display:none;"></div>
  </div>
</div>

<div id="screenshotModal" class="modal">
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">屏幕截图</h3>
      <button class="modal-close" onclick="closeModal('screenshotModal')">&times;</button>
    </div>
    <button onclick="takeScreenshot()">截图</button>
    <div id="screenshotOutput" style="margin-top:1rem;"></div>
  </div>
</div>

<div id="readFileModal" class="modal">
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">读取文件</h3>
      <button class="modal-close" onclick="closeModal('readFileModal')">&times;</button>
    </div>
    <div class="form-group">
      <label for="readFilePath">文件路径</label>
      <input type="text" id="readFilePath" placeholder="/tmp/example.txt">
    </div>
    <button onclick="readFile()">读取</button>
    <div id="readFileOutput" class="output-box" style="display:none;"></div>
  </div>
</div>

<div id="writeFileModal" class="modal">
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">写入文件</h3>
      <button class="modal-close" onclick="closeModal('writeFileModal')">&times;</button>
    </div>
    <div class="form-group">
      <label for="writeFilePath">文件路径</label>
      <input type="text" id="writeFilePath" placeholder="/tmp/example.txt">
    </div>
    <div class="form-group">
      <label for="writeFileContent">文件内容</label>
      <textarea id="writeFileContent" placeholder="文件内容..."></textarea>
    </div>
    <button onclick="writeFile()">写入</button>
    <div id="writeFileOutput" class="output-box" style="display:none;"></div>
  </div>
</div>

<div id="taskLogsModal" class="modal">
  <div class="modal-content">
    <div class="modal-header">
      <h3 class="modal-title">任务日志</h3>
      <button class="modal-close" onclick="closeModal('taskLogsModal')">&times;</button>
    </div>
    <div id="taskLogsOutput" class="output-box">加载中...</div>
  </div>
</div>
`
