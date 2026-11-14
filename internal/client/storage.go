package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ServerConfig represents a remote VPS Node agent target
type ServerConfig struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`        // e.g. "http://1.2.3.4:9090"
	Secret     string    `json:"secret"`     // secret token for auth
	PublicHost string    `json:"public_host,omitempty"` // optional public IP or domain for share links
	SharePort  string    `json:"share_port,omitempty"`  // optional dedicated share port
	IsActive   bool      `json:"is_active"`  // whether currently selected
	Status     string    `json:"status"`     // "online", "offline", "checking"
	PingMs     int64     `json:"ping_ms"`
	LastSeen   time.Time `json:"last_seen"`
}

// ServerStore manages persisted servers
type ServerStore struct {
	filePath string
	servers  []ServerConfig
	mu       sync.RWMutex
}

func NewServerStore(baseDir string) *ServerStore {
	fp := filepath.Join(baseDir, "servers.json")
	store := &ServerStore{
		filePath: fp,
		servers:  make([]ServerConfig, 0),
	}
	_ = store.load()
	// Ensure at least one default localhost server if empty
	if len(store.servers) == 0 {
		store.servers = append(store.servers, ServerConfig{
			ID:       "srv_default",
			Name:     "VPS 1 (Локальный узел)",
			URL:      "http://127.0.0.1:9090",
			Secret:   "",
			IsActive: true,
			Status:   "unknown",
		})
		_ = store.save()
	}
	return store
}

func (s *ServerStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.servers)
}

func (s *ServerStore) save() error {
	data, err := json.MarshalIndent(s.servers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}

// List returns all servers
func (s *ServerStore) List() []ServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]ServerConfig, len(s.servers))
	copy(res, s.servers)
	return res
}

// GetActive returns active server
func (s *ServerStore) GetActive() (*ServerConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, srv := range s.servers {
		if srv.IsActive {
			sc := srv
			return &sc, nil
		}
	}
	if len(s.servers) > 0 {
		sc := s.servers[0]
		return &sc, nil
	}
	return nil, fmt.Errorf("нет настроенных серверов")
}

// AddServer adds a new server
func (s *ServerStore) AddServer(name, url, secret, publicHost, sharePort string) (*ServerConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("srv_%d", time.Now().UnixMilli())
	if name == "" {
		name = fmt.Sprintf("Сервер %d", len(s.servers)+1)
	}

	newSrv := ServerConfig{
		ID:         id,
		Name:       name,
		URL:        url,
		Secret:     secret,
		PublicHost: publicHost,
		SharePort:  sharePort,
		IsActive:   len(s.servers) == 0,
		Status:     "checking",
	}

	s.servers = append(s.servers, newSrv)
	_ = s.save()
	return &newSrv, nil
}

// SetActive marks a server as active
func (s *ServerStore) SetActive(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	for i := range s.servers {
		if s.servers[i].ID == id {
			s.servers[i].IsActive = true
			found = true
		} else {
			s.servers[i].IsActive = false
		}
	}
	if !found {
		return fmt.Errorf("сервер не найден")
	}
	return s.save()
}

// DeleteServer removes a server
func (s *ServerStore) DeleteServer(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var newServers []ServerConfig
	var wasActive bool
	for _, srv := range s.servers {
		if srv.ID == id {
			if srv.IsActive {
				wasActive = true
			}
			continue
		}
		newServers = append(newServers, srv)
	}

	if wasActive && len(newServers) > 0 {
		newServers[0].IsActive = true
	}

	s.servers = newServers
	return s.save()
}

// UpdateStatus updates server status and ping
func (s *ServerStore) UpdateStatus(id, status string, pingMs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.servers {
		if s.servers[i].ID == id {
			s.servers[i].Status = status
			s.servers[i].PingMs = pingMs
			s.servers[i].LastSeen = time.Now()
			break
		}
	}
}
