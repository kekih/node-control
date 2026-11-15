package native

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"node-control/internal/client"
)

// AgentAPI provides HTTP methods to communicate with the active Node Agent
type AgentAPI struct {
	httpClient *http.Client
	store      *client.ServerStore
}

func NewAgentAPI(store *client.ServerStore) *AgentAPI {
	return &AgentAPI{
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		store: store,
	}
}

func (a *AgentAPI) getActiveServer() (*client.ServerConfig, error) {
	return a.store.GetActive()
}

func (a *AgentAPI) doRequest(method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	srv, err := a.getActiveServer()
	if err != nil {
		return nil, err
	}

	targetURL := strings.TrimRight(srv.URL, "/") + endpoint
	req, err := http.NewRequest(method, targetURL, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if srv.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+srv.Secret)
	}

	return a.httpClient.Do(req)
}

// Health check
type HealthInfo struct {
	Status    string    `json:"status"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Uptime    string    `json:"uptime"`
	StartTime time.Time `json:"start_time"`
	GoVersion string    `json:"go_version"`
	Version   string    `json:"version"`
}

func (a *AgentAPI) GetHealth() (*HealthInfo, int64, error) {
	start := time.Now()
	resp, err := a.doRequest(http.MethodGet, "/api/health", nil, "")
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("status %d: %s", resp.StatusCode, string(b))
	}

	ping := time.Since(start).Milliseconds()
	var info HealthInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, ping, err
	}
	return &info, ping, nil
}

// Files
type FileItem struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type DirectoryListing struct {
	CurrentPath string     `json:"current_path"`
	Items       []FileItem `json:"items"`
}

func (a *AgentAPI) ListFiles(path string) (*DirectoryListing, error) {
	endpoint := "/api/files?path=" + url.QueryEscape(path)
	resp, err := a.doRequest(http.MethodGet, endpoint, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: %s", string(b))
	}

	var res DirectoryListing
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (a *AgentAPI) Mkdir(path string) error {
	payload, _ := json.Marshal(map[string]string{"path": path})
	resp, err := a.doRequest(http.MethodPost, "/api/files/mkdir", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) DeleteFile(path string) error {
	payload, _ := json.Marshal(map[string]string{"path": path})
	resp, err := a.doRequest(http.MethodPost, "/api/files/delete", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) UploadFile(localPath, remoteDir string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("path", remoteDir)
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	resp, err := a.doRequest(http.MethodPost, "/api/files/upload", body, writer.FormDataContentType())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) DownloadFile(remotePath, localDestPath string) error {
	endpoint := "/api/files/download?path=" + url.QueryEscape(remotePath)
	resp, err := a.doRequest(http.MethodGet, endpoint, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}

	out, err := os.Create(localDestPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (a *AgentAPI) ShareFile(remotePath string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"path": remotePath})
	resp, err := a.doRequest(http.MethodPost, "/api/files/share", bytes.NewReader(payload), "application/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Link  string `json:"link"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.Link, nil
}

// Databases
type DBInfo struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // "postgres", "mysql", "kv"
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *AgentAPI) ListDatabases() ([]DBInfo, error) {
	resp, err := a.doRequest(http.MethodGet, "/api/db/list", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []DBInfo
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

func (a *AgentAPI) CreateDatabase(dbType, name, connStr string) error {
	payload, _ := json.Marshal(map[string]string{
		"type":       dbType,
		"name":       name,
		"connection": connStr,
	})
	resp, err := a.doRequest(http.MethodPost, "/api/db/create", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) DropDatabase(id string) error {
	payload, _ := json.Marshal(map[string]string{"id": id})
	resp, err := a.doRequest(http.MethodPost, "/api/db/drop", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Error   string          `json:"error,omitempty"`
}

func (a *AgentAPI) QueryDatabase(id, query string) (*QueryResult, error) {
	payload, _ := json.Marshal(map[string]string{
		"id":    id,
		"query": query,
	})
	resp, err := a.doRequest(http.MethodPost, "/api/db/query", bytes.NewReader(payload), "application/json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (a *AgentAPI) KVRead(base, path string) (string, error) {
	endpoint := fmt.Sprintf("/api/db/kv/read?base=%s&path=%s", url.QueryEscape(base), url.QueryEscape(path))
	resp, err := a.doRequest(http.MethodGet, endpoint, nil, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Value string `json:"value"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.Value, nil
}

func (a *AgentAPI) KVWrite(base, path, val string, merge, force bool) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"base":  base,
		"path":  path,
		"value": val,
		"merge": merge,
		"force": force,
	})
	resp, err := a.doRequest(http.MethodPost, "/api/db/kv/write", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

// Scripts
type ScriptInfo struct {
	ID        string    `json:"id"`
	Language  string    `json:"language"` // "python", "node", "go"
	Code      string    `json:"code"`
	IsRunning bool      `json:"is_running"`
	Pid       int       `json:"pid"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (a *AgentAPI) ListScripts() ([]ScriptInfo, error) {
	resp, err := a.doRequest(http.MethodGet, "/api/scripts/list", nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []ScriptInfo
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

func (a *AgentAPI) GetScript(id string) (*ScriptInfo, error) {
	resp, err := a.doRequest(http.MethodGet, "/api/scripts/get?id="+url.QueryEscape(id), nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var s ScriptInfo
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (a *AgentAPI) SaveScript(id, lang, code string) error {
	payload, _ := json.Marshal(map[string]string{
		"id":       id,
		"language": lang,
		"code":     code,
	})
	resp, err := a.doRequest(http.MethodPost, "/api/scripts/save", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) RunScript(id, lang, code string) error {
	payload, _ := json.Marshal(map[string]string{
		"id":       id,
		"language": lang,
		"code":     code,
	})
	resp, err := a.doRequest(http.MethodPost, "/api/scripts/run", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) StopScript(id string) error {
	payload, _ := json.Marshal(map[string]string{"id": id})
	resp, err := a.doRequest(http.MethodPost, "/api/scripts/stop", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(b))
	}
	return nil
}

func (a *AgentAPI) KillScript(id string) error {
	payload, _ := json.Marshal(map[string]string{"id": id})
	resp, err := a.doRequest(http.MethodPost, "/api/scripts/kill", bytes.NewReader(payload), "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (a *AgentAPI) GetScriptLogs(id string) (string, bool, error) {
	resp, err := a.doRequest(http.MethodGet, "/api/scripts/logs?id="+url.QueryEscape(id), nil, "")
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	var res struct {
		Logs      string `json:"logs"`
		IsRunning bool   `json:"is_running"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", false, err
	}
	return res.Logs, res.IsRunning, nil
}
