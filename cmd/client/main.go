package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"node-control/internal/client"
	"node-control/internal/web"
)

func main() {
	webFlag := flag.Bool("web", false, "Запустить как локальный веб-сервер (по умолчанию открывается нативное GUI окно)")
	tuiFlag := flag.Bool("tui", false, "Запустить в текстовом консольном режиме (TUI)")
	portFlag := flag.String("port", "", "Порт для режима веб-сервера (по умолчанию 8080)")
	dataFlag := flag.String("data", "", "Папка для сохранения конфигураций (servers.json)")
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

	if *webFlag {
		runWebServer(store, *portFlag, dataDir)
		return
	}

	// Запуск локального бекенда на свободном порту 127.0.0.1
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("[Node Client] Ошибка привязки локального порта: %v", err)
	}
	port := fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
	localURL := fmt.Sprintf("http://127.0.0.1:%s", port)

	mgr := client.NewClientManager(store)
	mux := http.NewServeMux()
	mgr.RegisterRoutes(mux)

	fileServer := http.FileServer(web.GetFileSystem())
	mux.Handle("/", fileServer)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Minute,
		WriteTimeout: 15 * time.Minute,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[Node Client] Сервер остановлен: %v", err)
		}
	}()

	// Открываем нативное GUI окно рабочего стола
	runDesktopApp(localURL, store, *tuiFlag)

	// Завершение работы при закрытии окна
	_ = server.Close()
}

func runWebServer(store *client.ServerStore, port string, dataDir string) {
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
