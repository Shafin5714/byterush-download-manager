package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

func (a *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", a.handlePing)
	mux.HandleFunc("GET /api/settings", a.handleGetSettings)
	mux.HandleFunc("POST /api/settings", a.handleSaveSettings)
	mux.HandleFunc("GET /api/downloads", a.handleListDownloads)
	mux.HandleFunc("POST /api/downloads", a.handleAddDownload)
	mux.HandleFunc("POST /api/downloads/{id}/pause", a.handlePause)
	mux.HandleFunc("POST /api/downloads/{id}/resume", a.handleResume)
	mux.HandleFunc("POST /api/downloads/{id}/cancel", a.handleCancel)
	mux.HandleFunc("POST /api/downloads/pause-all", a.handlePauseAll)
	mux.HandleFunc("POST /api/downloads/resume-all", a.handleResumeAll)
	mux.HandleFunc("POST /api/youtube/info", a.handleYoutubeInfo)
	mux.HandleFunc("POST /api/youtube/download", a.handleYoutubeDownload)
	mux.HandleFunc("GET /api/events", a.hub.ServeHTTP)
	mux.HandleFunc("POST /api/shutdown", a.handleShutdown)
	return mux
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}

func (a *App) handlePing(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"app":     "byterush-engine",
		"version": a.version,
		"port":    a.port,
	})
}

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	st := *a.settings
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, st)
}

func (a *App) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var req Settings
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	a.mu.Lock()
	if req.DownloadDir != "" {
		a.settings.DownloadDir = req.DownloadDir
	}
	if req.MaxActive > 0 {
		a.settings.MaxActive = req.MaxActive
	}
	if req.Connections > 0 {
		a.settings.Connections = req.Connections
	}
	if req.SpeedLimitKBs >= 0 {
		a.settings.SpeedLimitKBs = req.SpeedLimitKBs
	}
	st := *a.settings
	a.mu.Unlock()
	a.limiter.SetRate(st.SpeedLimitKBs)
	a.store.SaveSettings(&st)
	a.hub.Broadcast(Event{Type: "settings", Data: st})
	writeJSON(w, http.StatusOK, st)
}

func (a *App) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	list := a.dl.All()
	writeJSON(w, http.StatusOK, list)
}

func (a *App) handleAddDownload(w http.ResponseWriter, r *http.Request) {
	var req AddRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") && !strings.HasPrefix(req.URL, "ftp://") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported URL scheme"})
		return
	}
	if isYouTubeURL(req.URL) && req.Kind != "youtube" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "YouTube links must be downloaded through the YouTube flow"})
		return
	}
	d, err := a.dl.Add(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, d)
}

func (a *App) handlePause(w http.ResponseWriter, r *http.Request) {
	if err := a.dl.Pause(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleResume(w http.ResponseWriter, r *http.Request) {
	if err := a.dl.Resume(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleCancel(w http.ResponseWriter, r *http.Request) {
	if err := a.dl.Remove(r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handlePauseAll(w http.ResponseWriter, r *http.Request) {
	a.dl.PauseAll()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleResumeAll(w http.ResponseWriter, r *http.Request) {
	a.dl.ResumeAll()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleYoutubeInfo(w http.ResponseWriter, r *http.Request) {
	var req YoutubeInfoRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	info, err := a.yt.Info(req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *App) handleYoutubeDownload(w http.ResponseWriter, r *http.Request) {
	var req YoutubeDownloadRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	folder := req.Folder
	if folder == "" {
		a.mu.Lock()
		folder = a.settings.DownloadDir
		a.mu.Unlock()
	}
	if err := os.MkdirAll(folder, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	d, err := a.dl.Add(AddRequest{URL: req.URL, Folder: folder, Kind: "youtube"})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.yt.setReq(d.ID, req)
	writeJSON(w, http.StatusAccepted, d)
}

func (a *App) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go func() {
		time.Sleep(200 * time.Millisecond)
		a.Shutdown()
	}()
}

func isYouTubeURL(u string) bool {
	return strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be")
}
