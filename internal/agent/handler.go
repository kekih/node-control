package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// AgentHandler holds all agent sub-managers and handles routing
type AgentHandler struct {
	secretKey  string
	startTime  time.Time
	files      *FileManager
	db         *DBManager
	scripts    *ScriptManager
	mux        *http.ServeMux
	version    string
}

func NewAgentHandler(secretKey, workDir string) *AgentHandler {
	h := &AgentHandler{
		secretKey: secretKey,
		startTime: time.Now(),
		files:     NewFileManager(workDir),
		db:        NewDBManager(),
		scripts:   NewScriptManager(workDir),
		mux:       http.NewServeMux(),
		version:   "1.0.0",
	}
	h.registerRoutes()
	return h
}

// ServeHTTP delegates to mux with CORS and Auth
func (h *AgentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for web clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Secret-Token")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Public routes
	if r.URL.Path == "/api/health" || strings.HasPrefix(r.URL.Path, "/share/") {
		h.mux.ServeHTTP(w, r)
		return
	}

	// Auth check if secret is configured
	if h.secretKey != "" {
		token := r.Header.Get("Authorization")
		token = strings.TrimPrefix(token, "Bearer ")
		if token == "" {
			token = r.Header.Get("X-Secret-Token")
		}
		if token == "" {
			token = r.URL.Query().Get("secret")
		}

		if token != h.secretKey {
			http.Error(w, `{"error":"Неверный или отсутствующий ключ авторизации (Secret Key)"}`, http.StatusUnauthorized)
			return
		}
	}

	h.mux.ServeHTTP(w, r)
}

func (h *AgentHandler) registerRoutes() {
	// Health
	h.mux.HandleFunc("/api/health", h.handleHealth)

	// Public file share
	h.mux.HandleFunc("/share/", h.files.HandlePublicShareDownload)

	// Files API
	h.mux.HandleFunc("/api/files", h.handleFilesList)
	h.mux.HandleFunc("/api/files/mkdir", h.handleFilesMkdir)
	h.mux.HandleFunc("/api/files/upload", h.handleFilesUpload)
	h.mux.HandleFunc("/api/files/download", h.handleFilesDownload)
	h.mux.HandleFunc("/api/files/view", h.handleFilesView)
	h.mux.HandleFunc("/api/files/save", h.handleFilesSave)
	h.mux.HandleFunc("/api/files/delete", h.handleFilesDelete)
	h.mux.HandleFunc("/api/files/share", h.handleFilesShare)
	h.mux.HandleFunc("/api/files/shared", h.handleFilesSharedList)
	h.mux.HandleFunc("/api/files/unshare", h.handleFilesUnshare)

	// Database API
	h.mux.HandleFunc("/api/db/test", h.handleDBTest)
	h.mux.HandleFunc("/api/db/list", h.handleDBList)
	h.mux.HandleFunc("/api/db/create", h.handleDBCreate)
	h.mux.HandleFunc("/api/db/drop", h.handleDBDrop)
	h.mux.HandleFunc("/api/db/tables", h.handleDBTables)
	h.mux.HandleFunc("/api/db/query", h.handleDBQuery)
	h.mux.HandleFunc("/api/db/kv/read", h.handleDBKVRead)
	h.mux.HandleFunc("/api/db/kv/write", h.handleDBKVWrite)
	h.mux.HandleFunc("/api/db/kv/delete", h.handleDBKVDelete)

	// Scripts API
	h.mux.HandleFunc("/api/scripts/list", h.handleScriptsList)
	h.mux.HandleFunc("/api/scripts/get", h.handleScriptsGet)
	h.mux.HandleFunc("/api/scripts/save", h.handleScriptsSave)
	h.mux.HandleFunc("/api/scripts/run", h.handleScriptsRun)
	h.mux.HandleFunc("/api/scripts/stop", h.handleScriptsStop)
	h.mux.HandleFunc("/api/scripts/logs", h.handleScriptsLogs)
	h.mux.HandleFunc("/api/scripts/logs/stream", h.handleScriptsLogsStream)
}

// Helpers
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Health Handler
func (h *AgentHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	uptime := time.Since(h.startTime).Truncate(time.Second).String()
	res := HealthResponse{
		Status:    "online",
		Hostname:  hostname,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Uptime:    uptime,
		StartTime: h.startTime,
		GoVersion: runtime.Version(),
		Version:   h.version,
	}
	writeJSON(w, http.StatusOK, res)
}

// Files Handlers
func (h *AgentHandler) handleFilesList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	res, err := h.files.ListDirectory(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AgentHandler) handleFilesMkdir(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}
	if err := h.files.CreateDirectory(body.Path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Папка успешно создана"})
}

func (h *AgentHandler) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	savedPath, err := h.files.HandleUpload(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Файл успешно загружен", "path": savedPath})
}

func (h *AgentHandler) handleFilesDownload(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	fullPath, err := h.files.resolveSafePath(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fi, err := os.Stat(fullPath)
	if err != nil || fi.IsDir() {
		writeError(w, http.StatusNotFound, "Файл не найден")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fi.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, fullPath)
}

func (h *AgentHandler) handleFilesView(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	content, err := h.files.ReadFile(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": target, "content": content})
}

func (h *AgentHandler) handleFilesSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}
	if err := h.files.WriteFile(body.Path, body.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Файл успешно сохранен"})
}

func (h *AgentHandler) handleFilesDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}
	if err := h.files.DeleteItem(body.Path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Успешно удалено"})
}

func (h *AgentHandler) handleFilesShare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}
	link, err := h.files.CreateShareLink(body.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, link)
}

func (h *AgentHandler) handleFilesSharedList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.files.ListShareLinks())
}

func (h *AgentHandler) handleFilesUnshare(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.files.RevokeShare(body.Token)
	writeJSON(w, http.StatusOK, map[string]string{"message": "Доступ отозван"})
}

// Database Handlers
func (h *AgentHandler) handleDBTest(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.TestConnection(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Подключение успешно установлено!"})
}

func (h *AgentHandler) handleDBList(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dbs, err := h.db.ListDatabases(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dbs)
}

func (h *AgentHandler) handleDBCreate(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.CreateDatabase(req, req.DBName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("База данных '%s' успешно создана", req.DBName)})
}

func (h *AgentHandler) handleDBDrop(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.DropDatabase(req, req.DBName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("База данных '%s' удалена", req.DBName)})
}

func (h *AgentHandler) handleDBTables(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tables, err := h.db.ListTables(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

func (h *AgentHandler) handleDBQuery(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.db.ExecuteQuery(req, req.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AgentHandler) handleDBKVRead(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	val, err := h.db.KVRead(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"value": val})
}

func (h *AgentHandler) handleDBKVWrite(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.KVWrite(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Значение успешно записано"})
}

func (h *AgentHandler) handleDBKVDelete(w http.ResponseWriter, r *http.Request) {
	var req DBRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.db.KVDelete(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Значение удалено"})
}

// Scripts Handlers
func (h *AgentHandler) handleScriptsList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.scripts.ListScripts())
}

func (h *AgentHandler) handleScriptsGet(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = "main"
	}
	sid, lang, content, err := h.scripts.GetScript(id)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":       id,
			"language": "python",
			"content":  "# Введите код скрипта\nprint('Hello from Node Agent!')\n",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"id":       sid,
		"language": lang,
		"content":  content,
	})
}

func (h *AgentHandler) handleScriptsSave(w http.ResponseWriter, r *http.Request) {
	var req ScriptRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ID == "" {
		req.ID = "main"
	}
	if req.Language == "" {
		req.Language = "python"
	}
	path, err := h.scripts.SaveScript(req.ID, req.Language, req.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Скрипт сохранен", "path": path})
}

func (h *AgentHandler) handleScriptsRun(w http.ResponseWriter, r *http.Request) {
	var req ScriptRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ID == "" {
		req.ID = "main"
	}
	if req.Language == "" {
		req.Language = "python"
	}
	if err := h.scripts.RunScript(req.ID, req.Language, req.Content, req.AutoDeps); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Скрипт запущен", "id": req.ID})
}

func (h *AgentHandler) handleScriptsStop(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.ID == "" {
		body.ID = "main"
	}
	if err := h.scripts.StopScript(body.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Скрипт остановлен", "id": body.ID})
}

func (h *AgentHandler) handleScriptsLogs(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = "main"
	}
	logs := h.scripts.GetLogs(id)
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "logs": logs})
}

func (h *AgentHandler) handleScriptsLogsStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = "main"
	}
	h.scripts.StreamLogs(w, r, id)
}
