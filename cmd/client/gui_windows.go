//go:build windows

package main

import (
	"log"

	"github.com/jchv/go-webview2"
	"node-control/internal/client"
	"node-control/internal/native"
)

func runDesktopApp(url string, store *client.ServerStore, forceTui bool) {
	if forceTui {
		nativeApp := native.NewNativeApp(store)
		if err := nativeApp.Run(); err != nil {
			log.Fatalf("[Node Client] Ошибка TUI: %v", err)
		}
		return
	}

	w := webview2.New(false)
	if w == nil {
		// Fallback to TUI if WebView2 runtime is missing
		log.Println("[Node Client] WebView2 недоступен, запуск в консольном режиме...")
		nativeApp := native.NewNativeApp(store)
		_ = nativeApp.Run()
		return
	}
	defer w.Destroy()

	w.SetTitle("Node Control")
	w.SetSize(1280, 850, webview2.HintNone)
	w.Navigate(url)
	w.Run()
}
