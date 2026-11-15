package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	CurrentVersion = "1.0.4"
	GitHubRepo     = "kekih/node-control"
	GitHubAPIURL   = "https://api.github.com/repos/kekih/node-control/releases/latest"
)

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	PublishedAt time.Time     `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Assets      []GitHubAsset `json:"assets"`
}

type CheckUpdateResponse struct {
	UpdateAvailable bool          `json:"update_available"`
	HasUpdate       bool          `json:"has_update"`
	CurrentVersion  string        `json:"current_version"`
	LatestVersion   string        `json:"latest_version"`
	Version         string        `json:"version"`
	Title           string        `json:"title"`
	ReleaseNotes    string        `json:"release_notes"`
	PublishedAt     string        `json:"published_at"`
	ReleaseURL      string        `json:"release_url"`
	Assets          []GitHubAsset `json:"assets"`
	ClientAssetURL  string        `json:"client_asset_url,omitempty"`
}

func CheckGitHubUpdate(currentVer string) (*CheckUpdateResponse, error) {
	if currentVer == "" {
		currentVer = CurrentVersion
	}

	req, err := http.NewRequest("GET", GitHubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Node-Control-Client/"+currentVer)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка проверки обновлений GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API статус %d: %s", resp.StatusCode, string(b))
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	latestVer := strings.TrimPrefix(rel.TagName, "v")
	cleanCurrent := strings.TrimPrefix(currentVer, "v")

	isNewer := isVersionNewer(latestVer, cleanCurrent)

	// Determine matching asset for current client OS
	clientAssetName := "node-client.exe"
	if runtime.GOOS == "linux" {
		clientAssetName = "node-client-linux-amd64"
	}

	var clientAssetURL string
	for _, asset := range rel.Assets {
		if asset.Name == clientAssetName {
			clientAssetURL = asset.BrowserDownloadURL
			break
		}
	}

	return &CheckUpdateResponse{
		UpdateAvailable: isNewer,
		HasUpdate:       isNewer,
		CurrentVersion:  cleanCurrent,
		LatestVersion:   latestVer,
		Version:         latestVer,
		Title:           rel.Name,
		ReleaseNotes:    rel.Body,
		PublishedAt:     rel.PublishedAt.Format("02.01.2006 15:04"),
		ReleaseURL:      rel.HTMLURL,
		Assets:          rel.Assets,
		ClientAssetURL:  clientAssetURL,
	}, nil
}

// Compare semantic versions (e.g. 1.0.4 vs 1.0.3)
func isVersionNewer(latest, current string) bool {
	lParts := parseVersionParts(latest)
	cParts := parseVersionParts(current)

	maxLen := len(lParts)
	if len(cParts) > maxLen {
		maxLen = len(cParts)
	}

	for i := 0; i < maxLen; i++ {
		l := 0
		if i < len(lParts) {
			l = lParts[i]
		}
		c := 0
		if i < len(cParts) {
			c = cParts[i]
		}
		if l > c {
			return true
		} else if l < c {
			return false
		}
	}
	return false
}

func parseVersionParts(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	res := make([]int, 0, len(parts))
	for _, p := range parts {
		// Extract only digits
		var numStr string
		for _, r := range p {
			if r >= '0' && r <= '9' {
				numStr += string(r)
			} else {
				break
			}
		}
		if numStr != "" {
			n, _ := strconv.Atoi(numStr)
			res = append(res, n)
		} else {
			res = append(res, 0)
		}
	}
	return res
}

// ApplyClientUpdate downloads the latest client binary from GitHub and updates executable
func ApplyClientUpdate(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("URL для скачивания обновления не указан")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к текущему исполняемому файлу: %w", err)
	}
	currentExe, _ = filepath.EvalSymlinks(currentExe)

	// Download new binary to a temporary file
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("ошибка скачивания обновления: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ошибка сервера загрузки: статус %d", resp.StatusCode)
	}

	newExePath := currentExe + ".new"
	out, err := os.OpenFile(newExePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("не удалось создать временный файл обновления: %w", err)
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(newExePath)
		return fmt.Errorf("ошибка записи обновления: %w", err)
	}
	_ = out.Close()

	// Replace current executable
	oldExePath := currentExe + ".old"
	_ = os.Remove(oldExePath)

	if err := os.Rename(currentExe, oldExePath); err != nil {
		return fmt.Errorf("не удалось заменить исполняемый файл: %w", err)
	}

	if err := os.Rename(newExePath, currentExe); err != nil {
		// Rollback
		_ = os.Rename(oldExePath, currentExe)
		return fmt.Errorf("не удалось применить новый бинарник: %w", err)
	}

	_ = os.Remove(oldExePath)
	return nil
}

// RestartSelf restarts the current executable
func RestartSelf() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
