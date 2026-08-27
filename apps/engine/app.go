package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type App struct {
	mu        sync.Mutex
	dir       string
	ytdlpPath string
	version   string
	port      int

	settings  *Settings
	store     *Store
	hub       *Hub
	dl        *DownloadManager
	yt        *YoutubeManager
	limiter   *Limiter
	transport *http.Transport

	server *http.Server
	done   chan struct{}
}

func NewApp(dir, ytdlpPath string) *App {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}
	a := &App{
		dir:       dir,
		ytdlpPath: ytdlpPath,
		version:   "0.1.0",
		hub:       NewHub(),
		limiter:   NewLimiter(),
		done:      make(chan struct{}),
		transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConnsPerHost:   64,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		},
	}
	a.store = NewStore(dir)
	st := a.store.LoadSettings()
	if st == nil {
		st = DefaultSettings(filepath.Join(os.Getenv("USERPROFILE"), "Downloads", "ByteRush"))
		a.store.SaveSettings(st)
	}
	if st.SpeedLimitKBs > 0 {
		a.limiter.SetRate(st.SpeedLimitKBs)
	}
	a.settings = st
	a.yt = NewYoutubeManager(a)
	a.dl = NewDownloadManager(a)
	return a
}

func (a *App) Start(port int) (int, error) {
	var ln net.Listener
	var err error
	for i := 0; i < 20; i++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
		port++
	}
	if err != nil {
		return 0, fmt.Errorf("cannot bind port: %w", err)
	}
	a.port = port
	a.server = &http.Server{
		Handler:           cors(a.routes()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go a.server.Serve(ln)
	log.Printf("byterush engine %s listening on 127.0.0.1:%d", a.version, port)
	return port, nil
}

func (a *App) Shutdown() {
	log.Printf("shutting down...")
	a.dl.flushSave()
	if a.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		a.server.Shutdown(ctx)
		cancel()
	}
	select {
	case <-a.done:
	default:
		close(a.done)
	}
}

func (a *App) Wait() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		a.Shutdown()
	case <-a.done:
	}
}

func main() {
	port := 29641
	dir := ""
	ytdlp := ""
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &port)
				i++
			}
		case "--dir":
			if i+1 < len(args) {
				dir = args[i+1]
				i++
			}
		case "--ytdlp":
			if i+1 < len(args) {
				ytdlp = args[i+1]
				i++
			}
		}
	}
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), ".byterush", "engine")
	}

	app := NewApp(dir, ytdlp)
	bound, err := app.Start(port)
	if err != nil {
		log.Fatalf("engine failed to start: %v", err)
	}
	// print actual port so the host process can parse it
	fmt.Printf("LISTENING %d\n", bound)
	app.Wait()
}
