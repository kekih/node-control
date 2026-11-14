package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"node-control/internal/agent"
)

func main() {
	portFlag := flag.String("port", "", "Port for management API (default 9090 or $PORT)")
	sharePortFlag := flag.String("share-port", "", "Dedicated port for public file sharing (default 9091 or $NODE_SHARE_PORT)")
	publicHostFlag := flag.String("public-host", "", "Public IP or domain for generated share links (or $NODE_PUBLIC_HOST)")
	tailscaleOnlyFlag := flag.Bool("tailscale-only", true, "Restrict management API strictly to Tailscale network (default true or $NODE_TAILSCALE_ONLY)")
	secretFlag := flag.String("secret", "", "Secret token for authentication (or $NODE_SECRET)")
	dirFlag := flag.String("dir", "", "Working directory for file manager and scripts (or $NODE_DIR)")
	flag.Parse()

	port := *portFlag
	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "9090"
		}
	}

	sharePort := *sharePortFlag
	if sharePort == "" {
		sharePort = os.Getenv("NODE_SHARE_PORT")
		if sharePort == "" {
			sharePort = os.Getenv("SHARE_PORT")
			if sharePort == "" {
				sharePort = "9091"
			}
		}
	}

	publicHost := *publicHostFlag
	if publicHost == "" {
		publicHost = os.Getenv("NODE_PUBLIC_HOST")
		if publicHost == "" {
			publicHost = os.Getenv("PUBLIC_HOST")
		}
	}

	tailscaleOnly := *tailscaleOnlyFlag
	if envTs := os.Getenv("NODE_TAILSCALE_ONLY"); envTs != "" {
		if b, err := strconv.ParseBool(envTs); err == nil {
			tailscaleOnly = b
		}
	}

	secret := *secretFlag
	if secret == "" {
		secret = os.Getenv("NODE_SECRET")
	}

	workDir := *dirFlag
	if workDir == "" {
		workDir = os.Getenv("NODE_DIR")
		if workDir == "" {
			workDir, _ = os.Getwd()
		}
	}
	workDir, _ = filepath.Abs(workDir)

	// Check local Tailscale IP
	tsIP := agent.FindLocalTailscaleIP()

	fmt.Println("==================================================")
	fmt.Println("             🚀 NODE AGENT (VPS DAEMON)           ")
	fmt.Println("==================================================")
	fmt.Printf(" [✓] Порт управления (Tailscale):  0.0.0.0:%s\n", port)
	fmt.Printf(" [✓] Порт раздачи файлов (Public): 0.0.0.0:%s\n", sharePort)
	fmt.Printf(" [✓] Рабочая папка:                %s\n", workDir)

	if tailscaleOnly {
		fmt.Printf(" [🔒] Режим Tailscale:             ВКЛЮЧЕН (доступ к управлению только через 100.64.0.0/10)\n")
		if tsIP != "" {
			fmt.Printf(" [✓] Обнаружен узел Tailscale:     http://%s:%s\n", tsIP, port)
		} else {
			fmt.Printf(" [!] Tailscale IP не обнаружен локально (подключение через внешние узлы 100.64.0.0/10)\n")
		}
	} else {
		fmt.Printf(" [!] Режим Tailscale:             ОТКЛЮЧЕН (доступ открыт для всех IP)\n")
	}

	if publicHost != "" {
		agent.SetCachedPublicIP(publicHost)
		fmt.Printf(" [🌐] Публичный хост для ссылок:   %s (порт %s, без Tailscale)\n", publicHost, sharePort)
	} else {
		fmt.Printf(" [🌐] Определение публичного IP:   фоновый поиск внешнего IP для ссылок...\n")
		go func() {
			ip := agent.DetectPublicIP()
			if ip != "" {
				fmt.Printf(" [✓] Определен публичный IP:       %s (ссылки генерируются как http://%s:%s/share/...)\n", ip, ip, sharePort)
			}
		}()
	}

	if secret != "" {
		fmt.Printf(" [✓] Аутентификация:               ЗАЩИЩЕНО (Secret Token активен)\n")
	} else {
		fmt.Printf(" [!] Аутентификация:               ОТКРЫТА (Secret Token не задан)\n")
	}
	fmt.Println("--------------------------------------------------")
	fmt.Println(" Доступные сервисы:")
	fmt.Println("   • Управление сервером (Tailscale /api, скрипты, базы, файлы)")
	fmt.Println("   • Публичная раздача файлов (Отдельный порт, без Tailscale)")
	fmt.Println("==================================================")

	handler := agent.NewAgentHandler(secret, workDir, tailscaleOnly, sharePort, publicHost)

	// 1. Management server (Tailscale protected)
	mgmtServer := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
	}

	// 2. Dedicated Share server (Public, NO Tailscale restriction)
	shareHandler := handler.FileManager().NewShareRouter()
	shareServer := &http.Server{
		Addr:         ":" + sharePort,
		Handler:      shareHandler,
		ReadTimeout:  30 * time.Minute,
		WriteTimeout: 30 * time.Minute,
	}

	// Start share server in background
	go func() {
		log.Printf("[Share Server] Публичный сервер раздачи файлов запущен на порту %s (доступен без Tailscale)", sharePort)
		if err := shareServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Share Server] Ошибка: %v", err)
		}
	}()

	// Handle graceful shutdown for both servers
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n[Node Agent] Остановка серверов...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = mgmtServer.Shutdown(ctx)
		_ = shareServer.Shutdown(ctx)
		os.Exit(0)
	}()

	log.Printf("[Node Agent] Основной сервер управления запущен на порту %s", port)
	if err := mgmtServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Node Agent] Критическая ошибка запуска: %v", err)
	}
}
