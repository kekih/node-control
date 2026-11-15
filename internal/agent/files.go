package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// FileManager manages server files and shareable links
type FileManager struct {
	rootDir    string
	shareLinks map[string]*ShareLink
	mu         sync.RWMutex
	sharePort  string
	publicHost string
}

func NewFileManager(rootDir, sharePort, publicHost string) *FileManager {
	if rootDir == "" {
		rootDir, _ = os.Getwd()
	}
	if sharePort == "" {
		sharePort = "9091"
	}
	_ = os.MkdirAll(rootDir, 0755)
	return &FileManager{
		rootDir:    rootDir,
		shareLinks: make(map[string]*ShareLink),
		sharePort:  sharePort,
		publicHost: publicHost,
	}
}

// resolveSafePath verifies target is within allowed root or absolute system path
func (fm *FileManager) resolveSafePath(userPath string) (string, error) {
	if userPath == "" || userPath == "." {
		return fm.rootDir, nil
	}
	// Normalize Windows slashes if server is running on Linux
	if runtime.GOOS != "windows" {
		userPath = strings.ReplaceAll(userPath, "\\", "/")
	} else {
		userPath = filepath.FromSlash(userPath)
	}

	if filepath.IsAbs(userPath) || strings.HasPrefix(userPath, "/") {
		return filepath.Clean(userPath), nil
	}

	return filepath.Join(fm.rootDir, filepath.Clean(userPath)), nil
}

// ListDirectory returns items in directory
func (fm *FileManager) ListDirectory(targetPath string) (*FilesListResponse, error) {
	fullPath, err := fm.resolveSafePath(targetPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	var items []FileItem
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		itemPath := filepath.Join(fullPath, entry.Name())
		if runtime.GOOS != "windows" {
			itemPath = filepath.ToSlash(itemPath)
		}
		items = append(items, FileItem{
			Name:    entry.Name(),
			Path:    itemPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Ext:     filepath.Ext(entry.Name()),
		})
	}

	parent := filepath.Dir(fullPath)
	if runtime.GOOS != "windows" {
		fullPath = filepath.ToSlash(fullPath)
		parent = filepath.ToSlash(parent)
	}
	return &FilesListResponse{
		CurrentPath: fullPath,
		ParentPath:  parent,
		Items:       items,
	}, nil
}

// CreateDirectory creates folder
func (fm *FileManager) CreateDirectory(targetPath string) error {
	fullPath, err := fm.resolveSafePath(targetPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(fullPath, 0755)
}

// DeleteItem removes a file or folder
func (fm *FileManager) DeleteItem(targetPath string) error {
	fullPath, err := fm.resolveSafePath(targetPath)
	if err != nil {
		return err
	}
	return os.RemoveAll(fullPath)
}

// ReadFile reads file contents as string
func (fm *FileManager) ReadFile(targetPath string) (string, error) {
	fullPath, err := fm.resolveSafePath(targetPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes file contents
func (fm *FileManager) WriteFile(targetPath, content string) error {
	fullPath, err := fm.resolveSafePath(targetPath)
	if err != nil {
		return err
	}
	dir := filepath.Dir(fullPath)
	_ = os.MkdirAll(dir, 0755)
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// CreateShareLink generates a secure token for sharing a file
func (fm *FileManager) CreateShareLink(filePath string) (*ShareLink, error) {
	fullPath, err := fm.resolveSafePath(filePath)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("нельзя поделиться всей папкой, выберите файл")
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(bytes)

	host := fm.publicHost
	if host == "" {
		host = DetectPublicIP()
	}
	if host == "" {
		h, err := os.Hostname()
		if err == nil && h != "" {
			host = h
		} else {
			host = "localhost"
		}
	}

	port := fm.sharePort
	if port == "" {
		port = "9091"
	}

	shareURL := fmt.Sprintf("http://%s:%s/share/%s", host, port, token)

	link := &ShareLink{
		Token:      token,
		FilePath:   fullPath,
		FileName:   fi.Name(),
		CreatedAt:  time.Now(),
		Downloads:  0,
		SharePort:  port,
		PublicHost: host,
		ShareURL:   shareURL,
	}

	fm.mu.Lock()
	fm.shareLinks[token] = link
	fm.mu.Unlock()

	return link, nil
}

// GetShareLink retrieves link by token
func (fm *FileManager) GetShareLink(token string) (*ShareLink, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	link, ok := fm.shareLinks[token]
	return link, ok
}

// ListShareLinks returns all active shares
func (fm *FileManager) ListShareLinks() []*ShareLink {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	var list []*ShareLink
	for _, l := range fm.shareLinks {
		list = append(list, l)
	}
	return list
}

// RevokeShare removes a share token
func (fm *FileManager) RevokeShare(token string) {
	fm.mu.Lock()
	delete(fm.shareLinks, token)
	fm.mu.Unlock()
}

// HandleUpload handles multipart file uploads
func (fm *FileManager) HandleUpload(r *http.Request) (string, error) {
	err := r.ParseMultipartForm(100 << 20) // 100MB
	if err != nil {
		return "", err
	}

	destDir := r.FormValue("dir")
	fullDir, err := fm.resolveSafePath(destDir)
	if err != nil {
		return "", err
	}
	_ = os.MkdirAll(fullDir, 0755)

	file, header, err := r.FormFile("file")
	if err != nil {
		return "", err
	}
	defer file.Close()

	destPath := filepath.Join(fullDir, header.Filename)
	out, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	return destPath, err
}

// HandlePublicShareDownload serves shared file without requiring authentication
func (fm *FileManager) HandlePublicShareDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// URL pattern: /share/{token} or /{token}
	token := strings.TrimPrefix(r.URL.Path, "/share/")
	token = strings.TrimPrefix(token, "/")
	token = strings.TrimSpace(token)

	fm.mu.Lock()
	link, ok := fm.shareLinks[token]
	if ok {
		link.Downloads++
	}
	fm.mu.Unlock()

	if !ok || link == nil {
		http.Error(w, "Ссылка недействительна или срок действия истек", http.StatusNotFound)
		return
	}

	file, err := os.Open(link.FilePath)
	if err != nil {
		http.Error(w, "Файл не найден на сервере", http.StatusNotFound)
		return
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		http.Error(w, "Ошибка чтения информации о файле", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fi.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), file)
}

// NewShareRouter returns an http.Handler for the dedicated share port
// (serves ONLY shared file downloads, no admin endpoints, no Tailscale restriction)
func (fm *FileManager) NewShareRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/share/", fm.HandlePublicShareDownload)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Node Share Server</title><style>body{background:#0b0f19;color:#f0f4f8;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;padding:20px;box-sizing:border-box} .card{background:#161d2d;border:1px solid rgba(255,255,255,0.08);border-radius:16px;padding:36px;max-width:440px;text-align:center;box-shadow:0 16px 36px rgba(0,0,0,0.5)} h2{margin:0 0 12px;color:#00c2ff;font-size:24px} p{color:#8ea0be;font-size:14px;line-height:1.6;margin:0 0 20px} .badge{display:inline-block;padding:4px 12px;background:rgba(0,194,255,0.1);color:#00c2ff;border-radius:20px;font-size:12px;font-weight:600;border:1px solid rgba(0,194,255,0.25)}</style></head><body><div class="card"><div style="font-size:40px;margin-bottom:12px;">📁</div><h2>Node File Share</h2><div class="badge">Публичный файловый сервер</div><p style="margin-top:16px">Этот порт предназначен исключительно для публичной раздачи файлов по прямым ссылкам.</p></div></body></html>`))
			return
		}
		token := strings.TrimPrefix(r.URL.Path, "/")
		if token != "" && !strings.Contains(token, "/") {
			r.URL.Path = "/share/" + token
			fm.HandlePublicShareDownload(w, r)
			return
		}
		http.NotFound(w, r)
	})
	return mux
}
