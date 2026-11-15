package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"node-control/internal/updater"
)

// ClientManager handles client-side server list, proxying and health checks
type ClientManager struct {
	store      *ServerStore
	httpClient *http.Client
}

func NewClientManager(store *ServerStore) *ClientManager {
	cm := &ClientManager{
		store: store,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	// Start background health monitor
	go cm.monitorServers()
	return cm
}

func (cm *ClientManager) monitorServers() {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	// Initial check
	cm.checkAllServers()

	for range ticker.C {
		cm.checkAllServers()
	}
}

func (cm *ClientManager) checkAllServers() {
	servers := cm.store.List()
	for _, srv := range servers {
		go func(sc ServerConfig) {
			start := time.Now()
			targetURL := strings.TrimRight(sc.URL, "/") + "/api/health"
			req, err := http.NewRequest(http.MethodGet, targetURL, nil)
			if err != nil {
				cm.store.UpdateStatus(sc.ID, "offline", 0)
				return
			}
			if sc.Secret != "" {
				req.Header.Set("Authorization", "Bearer "+sc.Secret)
			}

			resp, err := cm.httpClient.Do(req)
			if err != nil || resp.StatusCode != http.StatusOK {
				cm.store.UpdateStatus(sc.ID, "offline", 0)
				return
			}
			_ = resp.Body.Close()
			ping := time.Since(start).Milliseconds()
			cm.store.UpdateStatus(sc.ID, "online", ping)
		}(srv)
	}
}

// RegisterRoutes sets up client server API endpoints
func (cm *ClientManager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/client/servers", cm.handleServers)
	mux.HandleFunc("/api/client/servers/active", cm.handleSetActive)
	mux.HandleFunc("/api/client/ping", cm.handlePing)
	mux.HandleFunc("/api/client/update/check", cm.handleUpdateCheck)
	mux.HandleFunc("/api/client/update/apply", cm.handleUpdateApply)
	mux.HandleFunc("/api/proxy/", cm.handleProxy)
}

func (cm *ClientManager) handleServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch r.Method {
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(cm.store.List())
	case http.MethodPost:
		var body struct {
			Name       string `json:"name"`
			URL        string `json:"url"`
			Secret     string `json:"secret"`
			PublicHost string `json:"public_host"`
			SharePort  string `json:"share_port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"Неверный запрос"}`, http.StatusBadRequest)
			return
		}
		if body.URL == "" {
			http.Error(w, `{"error":"URL сервера обязателен"}`, http.StatusBadRequest)
			return
		}
		srv, err := cm.store.AddServer(body.Name, body.URL, body.Secret, body.PublicHost, body.SharePort)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(srv)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, `{"error":"ID сервера обязателен"}`, http.StatusBadRequest)
			return
		}
		if err := cm.store.DeleteServer(id); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Сервер удален"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (cm *ClientManager) handleSetActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"Неверный запрос"}`, http.StatusBadRequest)
		return
	}
	if err := cm.store.SetActive(body.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "Активный сервер изменен"})
}

func (cm *ClientManager) handlePing(w http.ResponseWriter, r *http.Request) {
	active, err := cm.store.GetActive()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}
	cm.checkAllServers()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(active)
}

func (cm *ClientManager) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	info, err := updater.CheckGitHubUpdate(updater.CurrentVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(info)
}

func (cm *ClientManager) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var body struct {
		AssetURL string `json:"asset_url"`
		Restart  bool   `json:"restart"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	if body.AssetURL == "" {
		info, err := updater.CheckGitHubUpdate(updater.CurrentVersion)
		if err == nil && info.ClientAssetURL != "" {
			body.AssetURL = info.ClientAssetURL
		}
	}

	if body.AssetURL == "" {
		http.Error(w, `{"error":"URL обновления не найден"}`, http.StatusBadRequest)
		return
	}

	if err := updater.ApplyClientUpdate(body.AssetURL); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Ошибка обновления: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Обновление успешно установлено! Пожалуйста, перезапустите приложение.",
	})

	if body.Restart {
		go func() {
			time.Sleep(1 * time.Second)
			_ = updater.RestartSelf()
			os.Exit(0)
		}()
	}
}

// handleProxy transparently forwards request to the currently selected active server
func (cm *ClientManager) handleProxy(w http.ResponseWriter, r *http.Request) {
	active, err := cm.store.GetActive()
	if err != nil {
		http.Error(w, `{"error":"Нет активного сервера"}`, http.StatusServiceUnavailable)
		return
	}

	targetURL, err := url.Parse(active.URL)
	if err != nil {
		http.Error(w, `{"error":"Неверный URL активного сервера"}`, http.StatusInternalServerError)
		return
	}

	// Map /api/proxy/* to /* on the remote agent
	proxyPath := strings.TrimPrefix(r.URL.Path, "/api/proxy")
	if !strings.HasPrefix(proxyPath, "/") {
		proxyPath = "/" + proxyPath
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host
		req.URL.Path = proxyPath
		if r.URL.RawQuery != "" {
			req.URL.RawQuery = r.URL.RawQuery
		}
		if active.Secret != "" {
			req.Header.Set("Authorization", "Bearer "+active.Secret)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, fmt.Sprintf(`{"error":"Не удалось связаться с сервером %s: %s"}`, active.Name, err.Error()))
	}

	proxy.ServeHTTP(w, r)
}
