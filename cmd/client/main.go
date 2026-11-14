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
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}

func main() {
	portFlag := flag.String("port", "", "Client port to listen on (default 8080)")
	dataFlag := flag.String("data", "", "Directory to save client configs like servers.json")
	openFlag := flag.Bool("open", true, "Auto-open browser on startup")
	flag.Parse()

	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
	}

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
	mgr := client.NewClientManager(store)

	mux := http.NewServeMux()

	// Register Client API and Proxy
	mgr.RegisterRoutes(mux)

	// Embedded Static Assets
	fileServer := http.FileServer(web.GetFileSystem())
	mux.Handle("/", fileServer)

	serverAddr := ":" + port
	localURL := fmt.Sprintf("http://localhost:%s", port)

	fmt.Println("==================================================")
	fmt.Println("             🖥️  NODE CLIENT MANAGER               ")
	fmt.Println("==================================================")
	fmt.Printf(" [✓] Client Web UI:    %s\n", localURL)
	fmt.Printf(" [✓] Configuration:    %s\n", filepath.Join(dataDir, "servers.json"))
	fmt.Println("--------------------------------------------------")
	fmt.Println(" Controls:")
	fmt.Println("   • Server Switcher & Add/Remove Nodes")
	fmt.Println("   • File Manager & Public Sharing Links")
	fmt.Println("   • Database Manager (PostgreSQL & MySQL)")
	fmt.Println("   • Script Runner (Python, Node.js, Go + Auto-Deps)")
	fmt.Println("==================================================")

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Minute,
		WriteTimeout: 15 * time.Minute,
	}

	// Auto open browser if requested
	if *openFlag {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(localURL)
		}()
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n[Node Client] Выключение клиента...")
		_ = server.Close()
		os.Exit(0)
	}()

	log.Printf("[Node Client] Клиент успешно запущен на порту %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Node Client] Ошибка запуска: %v", err)
	}
}
