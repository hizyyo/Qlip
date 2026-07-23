package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"

	"clipboard-manager/backend"
	"github.com/webview/webview_go"
)

//go:embed frontend/dist frontend/dist/assets
var frontendFS embed.FS

func main() {
	storage := backend.NewStorage()

	cfgDir, _ := os.UserConfigDir()
	imgDir := filepath.Join(cfgDir, "clipflow", "images")
	os.MkdirAll(imgDir, 0755)

	imgHandler := backend.NewImageHandler(storage)

	clip := backend.NewClipboard(func(ch backend.ClipboardChange) {
		switch ch.Type {
		case "text":
			storage.AddItem("text", ch.Text)
		case "image":
			path := imgHandler.CheckAndSave()
			if path != "" {
				storage.AddItem("image", path)
			}
		}
	})
	clip.Start()

	api := backend.NewServer(storage, imgDir)
	port := 1421
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	subFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	mux.Handle("/", http.FileServer(http.FS(subFS)))

	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("ClipFlow API running on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	w := webview.New(false)
	defer w.Destroy()

	hwnd := uintptr(w.Window())

	w.SetTitle("ClipFlow")
	w.SetSize(420, 600, webview.HintNone)
	w.Navigate(fmt.Sprintf("http://%s", addr))

	backend.ApplyFrameless(hwnd, 420, 600)

	toggle := backend.NewToggle(hwnd)
	mover := backend.NewWindowMover(hwnd)

	if err := w.Bind("__toggle", toggle.AnimatedToggle); err != nil {
		log.Printf("Bind failed: %v", err)
	}
	if err := w.Bind("__moveBy", mover.MoveBy); err != nil {
		log.Printf("Bind window move failed: %v", err)
	}

	if err := w.Bind("__hideNow", toggle.Hide); err != nil {
		log.Printf("Bind failed: %v", err)
	}

	hk := backend.NewHotkey(func() {
		toggle.AnimatedToggle()
	})
	if err := hk.Register(backend.MOD_ALT, 0x56); err != nil {
		log.Printf("Hotkey registration failed: %v", err)
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		httpServer.Shutdown(context.Background())
		w.Terminate()
		os.Exit(0)
	}()

	w.Run()
}
