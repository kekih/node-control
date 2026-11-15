package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"node-control/internal/client"
	"node-control/internal/native"
	"node-control/internal/web"
)

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}

func main() {
	webFlag := flag.Bool("web", false, "Запустить в режиме веб-сервера с браузером (по умолчанию запускается нативное приложение)")
	portFlag := flag.String("port", "", "Порт для веб-режима (по умолчанию 8080)")
	dataFlag := flag.String("data", "", "Папка для сохранения конфигураций (servers.json)")
	openFlag := flag.Bool("open", true, "Автоматически открывать браузер при флаге --web")
	flag.Parse()

	dataDir := *dataFlag
	if dataDir == "" {
		dataDir = os.Getenv("NODE_CLIENT_DATA")
		if dataDir == "" {
			dataDir = "."
		}
	}
	dataDir, _ = filepath.Abs(dataDir)
	_ = os.MkdirAll(dataDir, 0755)

	store := client.NewServerStore(dataDir)

	// Если указан флаг --web, запускаем веб-сервер
	if *webFlag {
		runWebServer(store, *portFlag, *openFlag, dataDir)
		return
	}

	// По умолчанию запускается нативное приложение (без веб-интерфейса)
	nativeApp := native.NewNativeApp(store)
	if err := nativeApp.Run(); err != nil {
		log.Fatalf("[Node Client] Ошибка запуска нативного приложения: %v", err)
	}
}

func runWebServer(store *client.ServerStore, port string, autoOpen bool, dataDir string) {
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}

	mgr := client.NewClientManager(store)
	mux := http.NewServeMux()

	mgr.RegisterRoutes(mux)

	fileServer := http.FileServer(web.GetFileSystem())
	mux.Handle("/", fileServer)

	serverAddr := ":" + port
	localURL := fmt.Sprintf("http://localhost:%s", port)

	fmt.Println("==================================================")
	fmt.Println("        🖥️  NODE CLIENT (РЕЖИМ WEB UI)            ")
	fmt.Println("==================================================")
	fmt.Printf(" [✓] Client Web UI:    %s\n", localURL)
	fmt.Printf(" [✓] Configuration:    %s\n", filepath.Join(dataDir, "servers.json"))
	fmt.Println("==================================================")

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Minute,
		WriteTimeout: 15 * time.Minute,
	}

	if autoOpen {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(localURL)
		}()
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n[Node Client] Выключение клиента...")
		_ = server.Close()
		os.Exit(0)
	}()

	log.Printf("[Node Client] Веб-сервер запущен на порту %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Node Client] Ошибка запуска: %v", err)
	}
}
