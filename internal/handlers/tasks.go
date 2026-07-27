package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 任务表容量与保留策略。魔搭创空间容器内存有限，任务表必须有上界，
// 否则长时间运行的 agent 会把已完成任务累积到 OOM。
const (
	maxTasks        = 200            // 表内最多保留的任务数
	taskRetention   = 30 * time.Minute // 已结束任务保留时长
	defaultTaskTTL  = 300            // 任务默认超时（秒）
)

// 任务状态取值。
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Task 表示一个异步任务。所有字段的读写都必须持 tasksMutex，
// 因为执行 goroutine 和 HTTP handler 会并发访问。
type Task struct {
	ID        string
	Command   string
	Status    string
	Result    *CmdResult
	StartTime time.Time
	EndTime   time.Time

	// CurrentStdout/Stderr 保存任务运行中的实时输出（流式日志用）
	CurrentStdout string
	CurrentStderr string

	// cancel 取消正在执行的进程。用 context 而非 channel：
	// channel 方案下如果任务还没进入 select，取消信号会丢失。
	cancel context.CancelFunc
}

var (
	tasks      = make(map[string]*Task)
	tasksMutex sync.RWMutex
	taskIDSeq  int
)

type taskCreateRequest struct {
	Command string            `json:"command"`
	Cwd     string            `json:"cwd"`
	Env     map[string]string `json:"env"`
	Timeout int               `json:"timeout"`
}

type taskIDRequest struct {
	TaskID string `json:"task_id"`
}

// TaskCreate 提交一个后台任务，立即返回 task_id。
// 适用于耗时超过 HTTP 网关超时（创空间约 60s）的命令。
func TaskCreate(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req taskCreateRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.Command == "" {
		respondJSON(w, http.StatusBadRequest, failure("command is empty", nil))
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultTaskTTL
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)

	tasksMutex.Lock()
	reapTasksLocked()
	if len(tasks) >= maxTasks {
		tasksMutex.Unlock()
		cancel()
		respondJSON(w, http.StatusServiceUnavailable, failure(
			"task table full, retry after existing tasks finish", nil))
		return
	}
	taskIDSeq++
	taskID := "task_" + strconv.Itoa(taskIDSeq)
	task := &Task{
		ID:        taskID,
		Command:   req.Command,
		Status:    StatusPending,
		StartTime: time.Now(),
		cancel:    cancel,
	}
	tasks[taskID] = task
	tasksMutex.Unlock()

	go runAsyncTask(ctx, task, req.Cwd, req.Env, req.Timeout)

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"task_id": taskID,
		"status":  StatusPending,
	}))
}

// TaskStatus 查询任务状态。GET 用 ?task_id=xxx，POST 用 JSON body。
// 支持 stream=true 参数返回任务运行中的实时输出。
func TaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, failure("method not allowed", nil))
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" && r.Method == http.MethodPost {
		var req taskIDRequest
		if err := parseJSON(r, &req); err != nil {
			respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
			return
		}
		taskID = req.TaskID
	}
	if taskID == "" {
		respondJSON(w, http.StatusBadRequest, failure("task_id is required", nil))
		return
	}

	stream := r.URL.Query().Get("stream") == "true"

	tasksMutex.RLock()
	task, exists := tasks[taskID]
	var snapshot map[string]interface{}
	if exists {
		snapshot = map[string]interface{}{
			"task_id":    task.ID,
			"command":    task.Command,
			"status":     task.Status,
			"start_time": task.StartTime.Unix(),
		}
		if isFinished(task.Status) {
			snapshot["end_time"] = task.EndTime.Unix()
			snapshot["duration"] = task.EndTime.Sub(task.StartTime).Seconds()
			if task.Result != nil {
				snapshot["result"] = task.Result
			}
		} else if stream {
			// 流式模式：返回当前已捕获的输出（任务还在运行）
			snapshot["partial"] = true
			snapshot["current_stdout"] = task.CurrentStdout
			snapshot["current_stderr"] = task.CurrentStderr
		}
	}
	tasksMutex.RUnlock()

	if !exists {
		respondJSON(w, http.StatusNotFound, failure("task not found: "+taskID, nil))
		return
	}
	respondJSON(w, http.StatusOK, success(snapshot))
}

// TaskCancel 终止一个进行中的任务。
func TaskCancel(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) {
		return
	}

	var req taskIDRequest
	if err := parseJSON(r, &req); err != nil {
		respondJSON(w, http.StatusBadRequest, failure("invalid JSON", nil))
		return
	}
	if req.TaskID == "" {
		respondJSON(w, http.StatusBadRequest, failure("task_id is required", nil))
		return
	}

	tasksMutex.Lock()
	task, exists := tasks[req.TaskID]
	alreadyDone := exists && isFinished(task.Status)
	var cancelFn context.CancelFunc
	if exists && !alreadyDone {
		cancelFn = task.cancel
	}
	tasksMutex.Unlock()

	switch {
	case !exists:
		respondJSON(w, http.StatusNotFound, failure("task not found: "+req.TaskID, nil))
	case alreadyDone:
		respondJSON(w, http.StatusBadRequest, failure("task already finished", nil))
	default:
		cancelFn()
		respondJSON(w, http.StatusOK, success(map[string]interface{}{
			"task_id": req.TaskID,
			"status":  "cancelling",
		}))
	}
}

// TaskList 列出所有任务（含已结束但未被回收的）。
func TaskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, failure("method not allowed", nil))
		return
	}

	tasksMutex.RLock()
	list := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		item := map[string]interface{}{
			"task_id":    task.ID,
			"command":    task.Command,
			"status":     task.Status,
			"start_time": task.StartTime.Unix(),
		}
		if isFinished(task.Status) {
			item["end_time"] = task.EndTime.Unix()
		}
		list = append(list, item)
	}
	tasksMutex.RUnlock()

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"tasks": list,
		"count": len(list),
	}))
}

func runAsyncTask(ctx context.Context, task *Task, cwd string, env map[string]string, timeout int) {
	defer task.cancel() // 释放 context，避免 vet 报 lostcancel

	tasksMutex.Lock()
	task.Status = StatusRunning
	tasksMutex.Unlock()

	// 使用 streamingCappedBuffer 支持流式读取
	stdout := &streamingCappedBuffer{limit: maxOutputBytes, task: task, isStdout: true}
	stderr := &streamingCappedBuffer{limit: maxOutputBytes, task: task, isStdout: false}

	res := runArgvCtxWithBuffers(ctx, []string{"sh", "-lc", task.Command}, cwd, env, timeout, stdout, stderr)

	tasksMutex.Lock()
	task.EndTime = time.Now()
	task.Result = &res
	switch {
	case ctx.Err() == context.Canceled:
		task.Status = StatusCancelled
	case res.Success:
		task.Status = StatusCompleted
	default:
		task.Status = StatusFailed
	}
	tasksMutex.Unlock()
}

// streamingCappedBuffer 在写入时同时更新 Task 的 CurrentStdout/Stderr
type streamingCappedBuffer struct {
	buf      bytes.Buffer
	limit    int
	overflow bool
	task     *Task
	isStdout bool
}

func (s *streamingCappedBuffer) Write(p []byte) (int, error) {
	remaining := s.limit - s.buf.Len()
	if remaining <= 0 {
		s.overflow = true
		return len(p), nil
	}
	if len(p) > remaining {
		s.buf.Write(p[:remaining])
		s.overflow = true
		s.updateTask()
		return len(p), nil
	}
	n, err := s.buf.Write(p)
	s.updateTask()
	return n, err
}

func (s *streamingCappedBuffer) String() string {
	if s.overflow {
		return s.buf.String() + fmt.Sprintf("\n[output truncated at %d bytes]", s.limit)
	}
	return s.buf.String()
}

func (s *streamingCappedBuffer) updateTask() {
	tasksMutex.Lock()
	defer tasksMutex.Unlock()
	if s.isStdout {
		s.task.CurrentStdout = s.buf.String()
	} else {
		s.task.CurrentStderr = s.buf.String()
	}
}

// runArgvCtxWithBuffers 与 runArgvCtx 相同，但允许传入自定义的 stdout/stderr buffer
func runArgvCtxWithBuffers(ctx context.Context, argv []string, cwd string, env map[string]string, timeout int, stdout, stderr io.Writer) CmdResult {
	if len(argv) == 0 {
		return CmdResult{Stderr: "empty command", ExitCode: 1}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	res := CmdResult{
		Success: err == nil,
	}

	// 从 buffer 获取最终输出
	if sb, ok := stdout.(*streamingCappedBuffer); ok {
		res.Stdout = sb.String()
	} else if cb, ok := stdout.(*cappedBuffer); ok {
		res.Stdout = cb.String()
	}
	if sb, ok := stderr.(*streamingCappedBuffer); ok {
		res.Stderr = sb.String()
	} else if cb, ok := stderr.(*cappedBuffer); ok {
		res.Stderr = cb.String()
	}

	if err != nil {
		switch {
		case errors.Is(ctx.Err(), context.DeadlineExceeded):
			res.ExitCode = 124
			res.Stderr += fmt.Sprintf("\n[timeout after %ds]", timeout)
		case errors.Is(ctx.Err(), context.Canceled):
			res.ExitCode = 143
			res.Stderr += "\n[cancelled]"
		default:
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				res.ExitCode = exitErr.ExitCode()
			} else {
				res.ExitCode = 1
				res.Stderr += "\n" + err.Error()
			}
		}
	}
	return res
}

func isFinished(status string) bool {
	return status == StatusCompleted || status == StatusFailed || status == StatusCancelled
}

// reapTasksLocked 清理超过保留期的已结束任务。调用方必须已持写锁。
func reapTasksLocked() {
	cutoff := time.Now().Add(-taskRetention)
	for id, task := range tasks {
		if isFinished(task.Status) && task.EndTime.Before(cutoff) {
			delete(tasks, id)
		}
	}
}

// SystemInfo 返回运行时与资源信息，用于 agent 决定是否还能接新任务。
func SystemInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, failure("method not allowed", nil))
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	tasksMutex.RLock()
	active := 0
	for _, task := range tasks {
		if task.Status == StatusPending || task.Status == StatusRunning {
			active++
		}
	}
	total := len(tasks)
	tasksMutex.RUnlock()

	// 容器内的 CPU/内存上限来自 cgroup，runtime 读不到，用 shell 补齐。
	loadAvg := runShell("cat /proc/loadavg 2>/dev/null | cut -d' ' -f1-3", "", nil, 5)
	memInfo := runShell("free -m 2>/dev/null | awk 'NR==2{print $2\" \"$3\" \"$4}'", "", nil, 5)
	diskInfo := runShell("df -h / 2>/dev/null | awk 'NR==2{print $2\" \"$3\" \"$4\" \"$5}'", "", nil, 5)

	respondJSON(w, http.StatusOK, success(map[string]interface{}{
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"go_version":   runtime.Version(),
		"cpu_count":    runtime.NumCPU(),
		"goroutines":   runtime.NumGoroutine(),
		"heap_alloc":   m.Alloc,
		"heap_sys":     m.Sys,
		"active_tasks": active,
		"total_tasks":  total,
		"max_tasks":    maxTasks,
		"loadavg":      strings.TrimSpace(loadAvg.Stdout),
		"memory_mb":    strings.TrimSpace(memInfo.Stdout),  // total used free
		"disk":         strings.TrimSpace(diskInfo.Stdout), // size used avail use%
	}))
}
