//go:build !windows

package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"node-control/internal/client"
	"node-control/internal/native"
)

func runDesktopApp(url string, store *client.ServerStore, forceTui bool) {
	hasDisplay := os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""

	if forceTui || !hasDisplay {
		nativeApp := native.NewNativeApp(store)
		if err := nativeApp.Run(); err != nil {
			log.Fatalf("[Node Client] Ошибка TUI: %v", err)
		}
		return
	}

	// Try standalone application window via Chrome/Chromium app mode
	browsers := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"microsoft-edge",
	}

	tmpDataDir := filepath.Join(os.TempDir(), "node-control-app-profile")

	for _, b := range browsers {
		if path, err := exec.LookPath(b); err == nil {
			cmd := exec.Command(path,
				"--app="+url,
				"--class=NodeControl",
				"--user-data-dir="+tmpDataDir,
				"--window-size=1280,850",
			)
			if err := cmd.Run(); err == nil {
				return
			}
		}
	}

	// Fallback to xdg-open or TUI
	if xdg, err := exec.LookPath("xdg-open"); err == nil {
		_ = exec.Command(xdg, url).Start()
		select {} // Keep server alive
	}

	nativeApp := native.NewNativeApp(store)
	_ = nativeApp.Run()
}
