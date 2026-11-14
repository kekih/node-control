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
	ID        string
	Language  string
	Cmd       *exec.Cmd
	Cancel    context.CancelFunc
	Status    string
	StartTime time.Time
	LogBuffer *bytes.Buffer
	Listeners []chan string
	mu        sync.Mutex
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

// GetScript retrieves script code and metadata
func (sm *ScriptManager) GetScript(id string) (string, string, string, error) {
	dir := sm.getScriptDir(id)
	files := []string{"main.py", "main.js", "main.go"}
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
	if existing, ok := sm.jobs[id]; ok && existing.Status == "running" {
		sm.mu.Unlock()
		return fmt.Errorf("скрипт '%s' уже запущен", id)
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
		defer cancel()
		workDir := filepath.Dir(filePath)
		fmt.Fprintf(job, "[Node Agent] 🚀 Запуск скрипта '%s' [%s] в %s\n", id, lang, time.Now().Format("15:04:05"))

		if autoDeps {
			job.mu.Lock()
			job.Status = "installing_deps"
			job.mu.Unlock()
			fmt.Fprintf(job, "[Node Agent] 🔍 Анализ зависимостей...\n")
			_ = EnsureDependencies(ctx, lang, content, workDir, job)
		}

		var cmd *exec.Cmd
		switch strings.ToLower(lang) {
		case "python", "py":
			pyBin := findExecutable("python3", "python")
			if pyBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: python не установлен на сервере!\n")
				job.mu.Lock()
				job.Status = "error"
				job.mu.Unlock()
				return
			}
			cmd = exec.CommandContext(ctx, pyBin, "-u", filePath) // -u for unbuffered stdout
		case "nodejs", "node", "javascript", "js":
			nodeBin := findExecutable("node")
			if nodeBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: nodejs не установлен на сервере!\n")
				job.mu.Lock()
				job.Status = "error"
				job.mu.Unlock()
				return
			}
			cmd = exec.CommandContext(ctx, nodeBin, filePath)
		case "golang", "go":
			goBin := findExecutable("go")
			if goBin == "" {
				fmt.Fprintf(job, "[Node Agent] ❌ Ошибка: golang не установлен на сервере!\n")
				job.mu.Lock()
				job.Status = "error"
				job.mu.Unlock()
				return
			}
			cmd = exec.CommandContext(ctx, goBin, "run", filepath.Base(filePath))
		default:
			fmt.Fprintf(job, "[Node Agent] ❌ Неподдерживаемый язык: %s\n", lang)
			job.mu.Lock()
			job.Status = "error"
			job.mu.Unlock()
			return
		}

		cmd.Dir = workDir
		cmd.Stdout = job
		cmd.Stderr = job

		job.mu.Lock()
		job.Cmd = cmd
		job.Status = "running"
		job.mu.Unlock()

		if err := cmd.Start(); err != nil {
			fmt.Fprintf(job, "[Node Agent] ❌ Ошибка запуска процесса: %v\n", err)
			job.mu.Lock()
			job.Status = "error"
			job.mu.Unlock()
			return
		}

		fmt.Fprintf(job, "[Node Agent] 🟢 Процесс запущен (PID %d)\n", cmd.Process.Pid)

		exitErr := cmd.Wait()
		job.mu.Lock()
		if exitErr != nil {
			fmt.Fprintf(job, "[Node Agent] 🛑 Процесс завершился с ошибкой: %v\n", exitErr)
			job.Status = "stopped"
		} else {
			fmt.Fprintf(job, "[Node Agent] 🏁 Процесс успешно завершен (код 0)\n")
			job.Status = "stopped"
		}
		job.mu.Unlock()
	}()

	return nil
}

// StopScript stops a running script
func (sm *ScriptManager) StopScript(id string) error {
	sm.mu.Lock()
	job, ok := sm.jobs[id]
	sm.mu.Unlock()

	if !ok || job.Status != "running" && job.Status != "starting" && job.Status != "installing_deps" {
		return fmt.Errorf("скрипт '%s' не запущен", id)
	}

	fmt.Fprintf(job, "\n[Node Agent] ⚠️ Остановка процесса по запросу пользователя...\n")
	job.Cancel()
	if job.Cmd != nil && job.Cmd.Process != nil {
		_ = job.Cmd.Process.Kill()
	}

	job.mu.Lock()
	job.Status = "stopped"
	job.mu.Unlock()
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
