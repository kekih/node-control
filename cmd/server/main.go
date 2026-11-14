package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"node-control/internal/agent"
)

func main() {
	portFlag := flag.String("port", "", "Port to listen on (default 9090 or $PORT)")
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

	fmt.Println("==================================================")
	fmt.Println("             🚀 NODE AGENT (VPS DAEMON)           ")
	fmt.Println("==================================================")
	fmt.Printf(" [✓] Listening on:     0.0.0.0:%s\n", port)
	fmt.Printf(" [✓] Work Directory:   %s\n", workDir)
	if secret != "" {
		fmt.Printf(" [✓] Authentication:   PROTECTED (Secret token enabled)\n")
	} else {
		fmt.Printf(" [!] Authentication:   OPEN (No secret specified, dev mode)\n")
	}
	fmt.Println("--------------------------------------------------")
	fmt.Println(" Available Services:")
	fmt.Println("   • File Manager (Upload/Download/Share/Delete)")
	fmt.Println("   • Database Manager (PostgreSQL & MySQL KV & SQL)")
	fmt.Println("   • Script Runner (Python, Node.js, Go + Auto-Deps)")
	fmt.Println("==================================================")

	handler := agent.NewAgentHandler(secret, workDir)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  30 * time.Minute, // Support large uploads/streaming
		WriteTimeout: 30 * time.Minute,
	}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n[Node Agent] Остановка сервера...")
		_ = server.Close()
		os.Exit(0)
	}()

	log.Printf("[Node Agent] Сервер успешно запущен на порту %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[Node Agent] Критическая ошибка запуска: %v", err)
	}
}
