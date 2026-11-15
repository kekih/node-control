package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScriptInfo holds metadata of a script
type ScriptInfo struct {
	ID        string    `json:"id"`
	Language  string    `json:"language"`
	FilePath  string    `json:"file_path"`
	Status    string    `json:"status"` // "stopped", "running", "installing_deps", "error"
	UpdatedAt time.Time `json:"updated_at"`
	PID       int       `json:"pid,omitempty"`
}

// RunningJob represents an active or past script execution
type RunningJob struct {
	ID         string
	Language   string
	Cmd        *exec.Cmd
	InstallCmd *exec.Cmd
	Cancel     context.CancelFunc
	Status     string
	StartTime  time.Time
	LogBuffer  *bytes.Buffer
	Listeners  []chan string
	mu         sync.Mutex
}

func (j *RunningJob) Write(p []byte) (n int, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.LogBuffer.Write(p)
	msg := string(p)
	for _, ch := range j.Listeners {
		select {
		case ch <- msg:
		default:
		}
	}
	return len(p), nil
}

func (j *RunningJob) broadcastStatus(status string) {
	j.mu.Lock()
	j.Status = status
	payload, _ := json.Marshal(map[string]interface{}{
		"type":   "status",
		"status": status,
		"id":     j.ID,
	})
	msg := string(payload)
	for _, ch := range j.Listeners {
		select {
		case ch <- msg:
		default:
		}
	}
	j.mu.Unlock()
}

func (j *RunningJob) AddListener(ch chan string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Listeners = append(j.Listeners, ch)
}

func (j *RunningJob) RemoveListener(ch chan string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, c := range j.Listeners {
		if c == ch {
			j.Listeners = append(j.Listeners[:i], j.Listeners[i+1:]...)
			break
		}
	}
}

// ScriptManager manages scripts and running processes
type ScriptManager struct {
	baseDir string
	jobs    map[string]*RunningJob
	mu      sync.Mutex
}

func NewScriptManager(baseDir string) *ScriptManager {
	dir := filepath.Join(baseDir, "scripts_storage")
	_ = os.MkdirAll(dir, 0755)
	return &ScriptManager{
		baseDir: dir,
		jobs:    make(map[string]*RunningJob),
	}
}

func (sm *ScriptManager) getScriptDir(id string) string {
	dir := filepath.Join(sm.baseDir, id)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func (sm *ScriptManager) getScriptFile(id, lang string) string {
	ext := ".py"
	switch strings.ToLower(lang) {
	case "nodejs", "node", "javascript", "js":
		ext = ".js"
	case "golang", "go":
		ext = ".go"
	}
	return filepath.Join(sm.getScriptDir(id), "main"+ext)
}

// SaveScript saves script source code to disk
func (sm *ScriptManager) SaveScript(id, lang, content string) (string, error) {
	dir := sm.getScriptDir(id)
	filePath := sm.getScriptFile(id, lang)
	
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", err
	}

	// Save historical snapshot
	if len(strings.TrimSpace(content)) > 0 {
		histDir := filepath.Join(dir, "history")
		_ = os.MkdirAll(histDir, 0755)
		snapName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102_150405"), id, filepath.Ext(filePath))
		_ = os.WriteFile(filepath.Join(histDir, snapName), []byte(content), 0644)
	}

	// If golang, ensure go.mod exists
	if strings.ToLower(lang) == "golang" || strings.ToLower(lang) == "go" {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); os.IsNotExist(err) {
			modContent := fmt.Sprintf("module script_%s\n\ngo 1.22\n", id)
			_ = os.WriteFile(modPath, []byte(modContent), 0644)
		}
	}

	return filePath, nil
}

// DeleteScript removes script directory and cancels running job
func (sm *ScriptManager) DeleteScript(id string) error {
	sm.mu.Lock()
	if job, ok := sm.jobs[id]; ok {
		if job.Cancel != nil {
			job.Cancel()
		}
		if job.Cmd != nil && job.Cmd.Process != nil {
			_ = job.Cmd.Process.Kill()
		}
		delete(sm.jobs, id)
	}
	sm.mu.Unlock()

	dir := filepath.Join(sm.baseDir, id)
	return os.RemoveAll(dir)
}

// GetScriptHistory returns list of snapshots for script
func (sm *ScriptManager) GetScriptHistory(id string) ([]ScriptHistoryItem, error) {
	histDir := filepath.Join(sm.getScriptDir(id), "history")
	var items []ScriptHistoryItem
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return items, nil
	}
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.IsDir() {
			fi, err := e.Info()
			if err == nil {
				items = append(items, ScriptHistoryItem{
					Timestamp: fi.ModTime(),
					FileName:  e.Name(),
					Size:      fi.Size(),
				})
			}
		}
	}
	return items, nil
}

// GetScriptHistoryContent loads the content of a historic snapshot
func (sm *ScriptManager) GetScriptHistoryContent(id, fileName string) (string, error) {
	if strings.Contains(fileName, "..") || strings.ContainsAny(fileName, "/\\") {
		return "", fmt.Errorf("недопустимое имя файла")
	}
	fp := filepath.Join(sm.getScriptDir(id), "history", fileName)
	data, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetScript retrieves script code and metadata
func (sm *ScriptManager) GetScript(id, reqLang string) (string, string, string, error) {
	dir := sm.getScriptDir(id)

	var files []string
	switch strings.ToLower(reqLang) {
	case "nodejs", "node", "javascript", "js":
		files = []string{"main.js"}
	case "golang", "go":
		files = []string{"main.go"}
	case "python", "py":
		files = []string{"main.py"}
	default:
		files = []string{"main.py", "main.js", "main.go"}
	}

	for _, fn := range files {
		fp := filepath.Join(dir, fn)
		if data, err := os.ReadFile(fp); err == nil {
			lang := "python"
			if fn == "main.js" {
				lang = "nodejs"
			} else if fn == "main.go" {
				lang = "golang"
			}
			return id, lang, string(data), nil
		}
	}

	if reqLang != "" {
		targetLang := strings.ToLower(reqLang)
		if targetLang == "js" {
			targetLang = "nodejs"
		} else if targetLang == "go" {
			targetLang = "golang"
		} else if targetLang == "py" {
			targetLang = "python"
		}
		return id, targetLang, "", os.ErrNotExist
	}

	return id, "python", "", os.ErrNotExist
}

// ListScripts lists all script IDs
func (sm *ScriptManager) ListScripts() []ScriptInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var list []ScriptInfo
	entries, err := os.ReadDir(sm.baseDir)
	if err != nil {
		return list
	}

	for _, entry := range entries {
		if entry.IsDir() {
			id := entry.Name()
			status := "stopped"
			var pid int
			if job, exists := sm.jobs[id]; exists {
				status = job.Status
				if job.Cmd != nil && job.Cmd.Process != nil {
					pid = job.Cmd.Process.Pid
				}
			}

			// check file
			dir := filepath.Join(sm.baseDir, id)
			for _, fn := range []string{"main.py", "main.js", "main.go"} {
				fp := filepath.Join(dir, fn)
				if fi, err := os.Stat(fp); err == nil {
					lang := "python"
					if fn == "main.js" {
						lang = "nodejs"
					} else if fn == "main.go" {
						lang = "golang"
					}
					list = append(list, ScriptInfo{
						ID:        id,
						Language:  lang,
						FilePath:  fp,
						Status:    status,
						UpdatedAt: fi.ModTime(),
						PID:       pid,
					})
					break
				}
			}
		}
	}
	return list
}

// RunScript launches the script with auto dependency management
func (sm *ScriptManager) RunScript(id, lang, content string, autoDeps bool) error {
	sm.mu.Lock()
	if existing, ok := sm.jobs[id]; ok {
		// If job has a process, check if it's really alive
		isAlive := false
		if (existing.Status == "running" || existing.Status == "starting" || existing.Status == "installing_deps") && existing.Cmd != nil && existing.Cmd.Process != nil {
			isAlive = isProcessAlive(existing.Cmd.Process)
		}
		if isAlive {
			sm.mu.Unlock()
			return fmt.Errorf("скрипт '%s' уже выполняется (PID %d)", id, existing.Cmd.Process.Pid)
		}
		// Stale or dead, cleanup
		if existing.Cancel != nil {
			existing.Cancel()
		}
		if existing.Cmd != nil {
			killProcessTree(existing.Cmd)
		}
		existing.Status = "stopped"
	}

	filePath, err := sm.SaveScript(id, lang, content)
	if err != nil {
		sm.mu.Unlock()
		return fmt.Errorf("ошибка сохранения скрипта: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &RunningJob{
		ID:        id,
		Language:  lang,
		Cancel:    cancel,
		Status:    "starting",
		StartTime: time.Now(),
		LogBuffer: new(bytes.Buffer),
	}
	sm.jobs[id] = job
	sm.mu.Unlock()

	go func() {
		defer func() {
			cancel()
			job.broadcastStatus("stopped")
		}()

		workDir := filepath.Dir(filePath)
		fmt.Fprintf(job, "[Node Agent] 🚀 Запуск скрипта '%s' [%s] в %s\n", id, lang, time.Now().Format("15:04:05"))
		job.broadcastStatus("starting")

		if autoDeps {
			job.broadcastStatus("installing_deps")
			fmt.Fprintf(job, "[Node Agent] 🔍 Анализ зависимостей...\n")

			depCtx, depCancel := context.WithTimeout(ctx, 45*time.Second)
			_ = EnsureDependencies(depCtx, lang, content, workDir, job, func(c *exec.Cmd) {
				job.mu.Lock()
				job.InstallCmd = c
				job.mu.Unlock()
			})
			depCancel()

			job.mu.Lock()
			job.InstallCmd = nil
			job.mu.Unlock()
		}

		var cmd *exec.Cmd
		switch strings.ToLower(lang) {
		case "python", "py":
			pyBin := findExecutable("python3", "python", "/usr/bin/python3", "/usr/bin/python", "/usr/local/bin/python3")
			if pyBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: Python не найден в системе!\n")
				fmt.Fprintf(job, "  👉 Установите python на сервере:\n")
				fmt.Fprintf(job, "     • Linux: sudo apt update && sudo apt install -y python3 python3-pip\n")
				fmt.Fprintf(job, "     • Windows: скачайте с python.org\n")
				job.broadcastStatus("error")
				return
			}
			cmd = exec.CommandContext(ctx, pyBin, "-u", filePath)
			cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
		case "nodejs", "node", "javascript", "js":
			nodeBin := findExecutable("node", "nodejs", "/usr/bin/node", "/usr/bin/nodejs", "/usr/local/bin/node")
			if nodeBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: Node.js не найден в системе!\n")
				fmt.Fprintf(job, "  👉 Установите nodejs на сервере:\n")
				fmt.Fprintf(job, "     • Linux: sudo apt update && sudo apt install -y nodejs npm\n")
				job.broadcastStatus("error")
				return
			}
			cmd = exec.CommandContext(ctx, nodeBin, filePath)
		case "golang", "go":
			goBin := findExecutable("go", "/usr/bin/go", "/usr/local/go/bin/go")
			if goBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: Golang не найден в системе!\n")
				fmt.Fprintf(job, "  👉 Установите go на сервере:\n")
				fmt.Fprintf(job, "     • Linux: sudo apt update && sudo apt install -y golang-go\n")
				job.broadcastStatus("error")
				return
			}
			cmd = exec.CommandContext(ctx, goBin, "run", filepath.Base(filePath))
		default:
			fmt.Fprintf(job, "[Node Agent] ❌ Неподдерживаемый язык: %s\n", lang)
			job.broadcastStatus("error")
			return
		}

		setProcessGroup(cmd)
		cmd.Dir = workDir
		cmd.Stdout = job
		cmd.Stderr = job

		job.mu.Lock()
		job.Cmd = cmd
		job.mu.Unlock()

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(job, "[Node Agent] ❌ Ошибка запуска процесса: %v\n", err)
			job.broadcastStatus("error")
			return
		}

		job.broadcastStatus("running")
		fmt.Fprintf(job, "[Node Agent] 🟢 Процесс запущен (PID %d)\n", cmd.Process.Pid)

		exitErr := cmd.Wait()
		if exitErr != nil {
			fmt.Fprintf(job, "[Node Agent] 🛑 Процесс завершился: %v\n", exitErr)
		} else {
			fmt.Fprintf(job, "[Node Agent] 🏁 Процесс успешно завершен (код 0)\n")
		}
	}()

	return nil
}

// StopScript stops a running script gracefully
func (sm *ScriptManager) StopScript(id string) error {
	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	if !ok || job == nil {
		return nil
	}

	fmt.Fprintf(job, "\n[Node Agent] ⚠️ Остановка процесса по запросу пользователя...\n")
	if job.Cancel != nil {
		job.Cancel()
	}

	if job.InstallCmd != nil && job.InstallCmd.Process != nil {
		killProcessTree(job.InstallCmd)
	}

	if job.Cmd != nil && job.Cmd.Process != nil {
		killProcessTree(job.Cmd)
	}

	job.broadcastStatus("stopped")
	return nil
}

// KillScript performs critical force kill (SIGKILL) on process and all child processes
func (sm *ScriptManager) KillScript(id string) error {
	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	if !ok || job == nil {
		return nil
	}

	fmt.Fprintf(job, "\n[Node Agent] ⚡ КРИТИЧЕСКАЯ ОСТАНОВКА: Процесс и все дочерние задачи принудительно убиты (SIGKILL)\n")

	if job.Cancel != nil {
		job.Cancel()
	}

	if job.InstallCmd != nil && job.InstallCmd.Process != nil {
		killProcessTree(job.InstallCmd)
	}

	if job.Cmd != nil && job.Cmd.Process != nil {
		killProcessTree(job.Cmd)
	}

	job.broadcastStatus("stopped")
	return nil
}

// GetLogs returns buffered logs for a script
func (sm *ScriptManager) GetLogs(id string) string {
	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	if !ok {
		return "Логи не найдены для данного скрипта."
	}

	job.mu.Lock()
	defer job.mu.Unlock()
	return job.LogBuffer.String()
}

// StreamLogs handles SSE log streaming
func (sm *ScriptManager) StreamLogs(w http.ResponseWriter, r *http.Request, id string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	if !ok {
		fmt.Fprintf(w, "data: %s\n\n", "Скрипт еще не запускался")
		flusher.Flush()
		return
	}

	// Send current buffer first
	job.mu.Lock()
	initialLogs := job.LogBuffer.String()
	job.mu.Unlock()

	fmt.Fprintf(w, "event: init\ndata: %s\n\n", strings.ReplaceAll(initialLogs, "\n", "\\n"))
	flusher.Flush()

	ch := make(chan string, 100)
	job.AddListener(ch)
	defer job.RemoveListener(ch)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			payload, _ := json.Marshal(map[string]string{"log": msg, "status": job.Status})
			fmt.Fprintf(w, "data: %s\n\n", string(payload))
			flusher.Flush()
		}
	}
}

// StreamLogsWS handles real-time WebSocket log streaming
func (sm *ScriptManager) StreamLogsWS(ws *WSConn, id string) {
	defer ws.Close()

	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	var initialLogs string
	status := "stopped"
	if ok {
		job.mu.Lock()
		initialLogs = job.LogBuffer.String()
		status = job.Status
		job.mu.Unlock()
	}

	initPayload, _ := json.Marshal(map[string]interface{}{
		"type":   "init",
		"logs":   initialLogs,
		"status": status,
		"id":     id,
	})
	if err := ws.WriteText(string(initPayload)); err != nil {
		return
	}

	if !ok {
		// Keep alive waiting for client or future start
		ws.ReadLoop()
		return
	}

	ch := make(chan string, 100)
	job.AddListener(ch)
	defer job.RemoveListener(ch)

	// Run ReadLoop in background to handle ping/pong and close
	go ws.ReadLoop()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			job.mu.Lock()
			curStatus := job.Status
			job.mu.Unlock()

			payload, _ := json.Marshal(map[string]interface{}{
				"type":   "log",
				"log":    msg,
				"status": curStatus,
				"id":     id,
			})
			if err := ws.WriteText(string(payload)); err != nil {
				return
			}
		case <-ticker.C:
			job.mu.Lock()
			curStatus := job.Status
			job.mu.Unlock()

			payload, _ := json.Marshal(map[string]interface{}{
				"type":   "status",
				"status": curStatus,
				"id":     id,
			})
			if err := ws.WriteText(string(payload)); err != nil {
				return
			}
		}
	}
}
