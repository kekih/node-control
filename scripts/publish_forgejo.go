package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

const (
	token     = "50e4b1061e7df701466f2b12d4d961286fee33f3"
	serverURL = "http://home-server:3000"
	repoOwner = "Leuteg"
	repoName  = "node-control"
	tag       = "1.0.4"
	title     = "v1.0.4 - Критическая остановка скриптов, исправление запуска и OTA обновления GitHub"
)

func main() {
	bodyBytes, err := os.ReadFile("release_notes_104.md")
	if err != nil {
		panic(err)
	}

	// 1. Create Release
	releasePayload := map[string]interface{}{
		"tag_name":   tag,
		"name":       title,
		"body":       string(bodyBytes),
		"draft":      false,
		"prerelease": false,
	}
	pBytes, _ := json.Marshal(releasePayload)

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/repos/%s/%s/releases", serverURL, repoOwner, repoName), bytes.NewReader(pBytes))
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// Check if release already exists
		getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/tags/%s", serverURL, repoOwner, repoName, tag), nil)
		getReq.Header.Set("Authorization", "token "+token)
		getResp, err := client.Do(getReq)
		if err == nil && getResp.StatusCode == http.StatusOK {
			respBody, _ = io.ReadAll(getResp.Body)
			_ = getResp.Body.Close()
		} else {
			fmt.Printf("Create release error (status %d): %s\n", resp.StatusCode, string(respBody))
			return
		}
	}

	var res struct {
		ID      int64  `json:"id"`
		HTMLURL string `json:"html_url"`
	}
	_ = json.Unmarshal(respBody, &res)
	fmt.Printf("[✓] Релиз создан! ID: %d, URL: %s\n", res.ID, res.HTMLURL)

	// 2. Upload assets
	assets := []string{
		"bin/node-client.exe",
		"bin/node-client-linux-amd64",
		"bin/node-agent.exe",
		"bin/node-agent-linux-amd64",
		"bin/Node.apk",
	}

	for _, aPath := range assets {
		uploadAsset(client, res.ID, aPath)
	}
	fmt.Println("[✓] Все файлы успешно загружены в релиз Forgejo!")
}

func uploadAsset(client *http.Client, releaseID int64, filePath string) {
	fileName := filepath.Base(filePath)
	fmt.Printf(" Загрузка %s...\n", fileName)

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("  [!] Ошибка открытия %s: %v\n", filePath, err)
		return
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("attachment", fileName)
	if err != nil {
		fmt.Printf("  [!] Ошибка CreateFormFile: %v\n", err)
		return
	}
	if _, err := io.Copy(part, file); err != nil {
		fmt.Printf("  [!] Ошибка копирования: %v\n", err)
		return
	}
	_ = writer.Close()

	uploadURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/releases/%d/assets?name=%s", serverURL, repoOwner, repoName, releaseID, fileName)
	req, _ := http.NewRequest("POST", uploadURL, body)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  [!] Ошибка загрузки: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		fmt.Printf("  [✓] %s успешно прикреплен!\n", fileName)
	} else {
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("  [!] Ошибка ответа (%d): %s\n", resp.StatusCode, string(b))
	}
}
