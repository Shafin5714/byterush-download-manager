package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadDenoExtractsExecutable(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	f, err := zw.Create("deno.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("fake deno executable")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(archive.Bytes())
	}))
	defer server.Close()
	oldURL := denoDownloadURL
	denoDownloadURL = server.URL
	defer func() { denoDownloadURL = oldURL }()

	dataDir := t.TempDir()
	app := NewApp(dataDir, "")
	dest := filepath.Join(dataDir, "deno.exe")
	if err := app.yt.downloadDeno(dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "fake deno executable" {
		t.Fatalf("unexpected extracted contents: %q", got)
	}
}

func TestYoutubeRuntimeArgsUseManagedDeno(t *testing.T) {
	dataDir := t.TempDir()
	deno := filepath.Join(dataDir, "deno.exe")
	if err := os.WriteFile(deno, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}
	app := NewApp(dataDir, "")
	args := app.yt.youtubeRuntimeArgs()
	if len(args) != 2 || args[0] != "--js-runtimes" || args[1] != "deno:"+deno {
		t.Fatalf("unexpected runtime args: %#v", args)
	}
}

func TestIsHTTP403(t *testing.T) {
	if !isHTTP403(errors.New("unable to download video data: HTTP Error 403: Forbidden")) {
		t.Fatal("expected HTTP 403 error to be recognized")
	}
	if isHTTP403(errors.New("HTTP Error 429: Too Many Requests")) {
		t.Fatal("did not expect a non-403 error to be recognized")
	}
}

func TestYoutubeProgressParsesRedirectedStderrFormat(t *testing.T) {
	app := NewApp(t.TempDir(), "")
	d := &Download{ID: "youtube-progress", Status: StatusActive}
	app.dl.mu.Lock()
	app.dl.downloads[d.ID] = d
	app.dl.mu.Unlock()

	line := "[download]  12.5% of ~ 830.32MiB at  2.50MiB/s ETA 04:30"
	if !app.yt.parseProgress(d, line) {
		t.Fatal("expected yt-dlp progress line to be recognized")
	}
	got := app.dl.Get(d.ID)
	wantTotal := parseSize("830.32", "M")
	if got.TotalSize != wantTotal {
		t.Fatalf("total size = %d, want %d", got.TotalSize, wantTotal)
	}
	wantDownloaded := int64(0.125 * float64(wantTotal))
	if got.Downloaded != wantDownloaded {
		t.Fatalf("downloaded = %d, want %d", got.Downloaded, wantDownloaded)
	}
	if got.Speed != parseSize("2.50", "M") {
		t.Fatalf("speed = %d, want %d", got.Speed, parseSize("2.50", "M"))
	}
}

func TestYtDLPUpdateDueRetriesFailuresSooner(t *testing.T) {
	stamp := filepath.Join(t.TempDir(), "yt-dlp-update-check")
	if err := os.WriteFile(stamp, []byte("failed"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := os.Chtimes(stamp, now, now); err != nil {
		t.Fatal(err)
	}
	if ytDLPUpdateDue(stamp, now) {
		t.Fatal("a recent failed check should be briefly throttled")
	}
	if !ytDLPUpdateDue(stamp, now.Add(ytDLPUpdateFailureInterval)) {
		t.Fatal("a failed check should be retried after the short failure interval")
	}
	if err := os.WriteFile(stamp, []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(stamp, now, now); err != nil {
		t.Fatal(err)
	}
	if ytDLPUpdateDue(stamp, now.Add(time.Hour)) {
		t.Fatal("a successful check should use the full update interval")
	}
	if !ytDLPUpdateDue(stamp, now.Add(ytDLPUpdateInterval)) {
		t.Fatal("a successful check should be retried after the full update interval")
	}
}

func TestQueuedDownloadCanBePausedAndResumed(t *testing.T) {
	app := NewApp(t.TempDir(), "")
	app.settings.MaxActive = 0
	d, err := app.dl.Add(AddRequest{URL: "https://example.test/file.bin", Folder: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.dl.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	if got := app.dl.Get(d.ID); got == nil || got.Status != StatusPaused {
		t.Fatalf("download after pause = %+v, want paused", got)
	}
	if err := app.dl.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	if got := app.dl.Get(d.ID); got == nil || got.Status != StatusQueued {
		t.Fatalf("download after resume = %+v, want queued", got)
	}
}

func TestYoutubeRequestPersistsForResume(t *testing.T) {
	dataDir := t.TempDir()
	app := NewApp(dataDir, "")
	app.settings.MaxActive = 0
	want := YoutubeDownloadRequest{
		URL:           "https://www.youtube.com/watch?v=resume-test",
		Format:        "399+bestaudio/best",
		PlaylistItems: "2-4",
		Folder:        t.TempDir(),
		Container:     "mp4",
	}
	d, err := app.dl.Add(AddRequest{URL: want.URL, Folder: want.Folder, Kind: "youtube", Youtube: &want})
	if err != nil {
		t.Fatal(err)
	}
	app.dl.flushSave()

	loaded := NewStore(dataDir).LoadDownloads()[d.ID]
	if loaded == nil || loaded.Youtube == nil {
		t.Fatalf("persisted download is missing its YouTube request: %+v", loaded)
	}
	if *loaded.Youtube != want {
		t.Fatalf("persisted YouTube request = %+v, want %+v", *loaded.Youtube, want)
	}
	if loaded.Status != StatusPaused {
		t.Fatalf("restored queued download status = %q, want paused", loaded.Status)
	}
}

func TestActiveHTTPDownloadCanPauseAndResume(t *testing.T) {
	content := bytes.Repeat([]byte("pause-resume"), 64*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", fmt.Sprint(len(content)))
		if r.Method == http.MethodHead {
			return
		}
		start, end := 0, len(content)-1
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		for offset := start; offset <= end; {
			next := offset + 16*1024
			if next > end+1 {
				next = end + 1
			}
			if _, err := w.Write(content[offset:next]); err != nil {
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			offset = next
			time.Sleep(4 * time.Millisecond)
		}
	}))
	defer server.Close()

	app := NewApp(filepath.Join(t.TempDir(), "state"), "")
	app.settings.MaxActive = 1
	app.settings.Connections = 1
	d, err := app.dl.Add(AddRequest{URL: server.URL + "/large.bin", Folder: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	waitForDownloadStatus(t, app.dl, d.ID, 3*time.Second, func(got *Download) bool {
		return got.Status == StatusActive && got.Downloaded > 0
	})
	if err := app.dl.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	paused := waitForDownloadStatus(t, app.dl, d.ID, 3*time.Second, func(got *Download) bool {
		return got.Status == StatusPaused
	})
	if paused.Downloaded <= 0 || paused.Downloaded >= int64(len(content)) {
		t.Fatalf("paused at %d bytes, want a partial download", paused.Downloaded)
	}
	if err := app.dl.Resume(d.ID); err != nil {
		t.Fatal(err)
	}
	completed := waitForDownloadStatus(t, app.dl, d.ID, 5*time.Second, func(got *Download) bool {
		return got.Status == StatusCompleted
	})
	got, err := os.ReadFile(completed.FinalFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("resumed file length = %d, want %d", len(got), len(content))
	}
}

func TestActiveDownloadCanBePausedImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "1048576")
		if r.Method == http.MethodHead {
			return
		}
		w.Header().Set("Content-Range", "bytes 0-1048575/1048576")
		w.WriteHeader(http.StatusPartialContent)
		<-r.Context().Done()
	}))
	defer server.Close()

	app := NewApp(filepath.Join(t.TempDir(), "state"), "")
	app.settings.MaxActive = 1
	app.settings.Connections = 1
	d, err := app.dl.Add(AddRequest{URL: server.URL + "/slow.bin", Folder: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.dl.Pause(d.ID); err != nil {
		t.Fatal(err)
	}
	waitForDownloadStatus(t, app.dl, d.ID, 3*time.Second, func(got *Download) bool {
		return got.Status == StatusPaused
	})
}

func waitForDownloadStatus(t *testing.T, manager *DownloadManager, id string, timeout time.Duration, ready func(*Download) bool) *Download {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if d := manager.Get(id); d != nil && ready(d) {
			return d
		}
		time.Sleep(10 * time.Millisecond)
	}
	d := manager.Get(id)
	t.Fatalf("download did not reach expected state before timeout: %+v", d)
	return nil
}

func TestProbeFallsBackToRangedGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Range"); got != "bytes=0-0" {
			t.Fatalf("probe Range header = %q", got)
		}
		w.Header().Set("Content-Range", "bytes 0-0/12345")
		w.Header().Set("Content-Disposition", `attachment; filename="fixture.bin"`)
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("x"))
	}))
	defer server.Close()

	result, err := probeURL(context.Background(), server.Client(), server.URL+"/redirect-name", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.size != 12345 || !result.acceptRanges || result.filename != "fixture.bin" {
		t.Fatalf("unexpected probe result: %+v", result)
	}
}

func TestProbeReportsCloudflareChallenge(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("CF-Mitigated", "challenge")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := probeURL(context.Background(), server.Client(), server.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "browser verification") {
		t.Fatalf("expected browser verification error, got %v", err)
	}
}

func TestDownloadHTTPForwardsBrowserContext(t *testing.T) {
	content := bytes.Repeat([]byte("byterush"), 4096)
	wantHeaders := map[string]string{
		"Cookie":     "cf_clearance=test-token",
		"Referer":    "https://example.test/downloads",
		"User-Agent": "Test Browser/1.0",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for name, want := range wantHeaders {
			if got := r.Header.Get(name); got != want {
				t.Errorf("%s header = %q, want %q", name, got, want)
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Disposition", `attachment; filename="protected.bin"`)
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(content)))
			return
		}
		var start, end int
		if _, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end); err != nil {
			t.Errorf("invalid Range header: %q", r.Header.Get("Range"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(content)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(content[start : end+1])
	}))
	defer server.Close()

	dataDir := t.TempDir()
	app := NewApp(filepath.Join(dataDir, "state"), "")
	download := &Download{
		URL:            server.URL + "/protected",
		Folder:         filepath.Join(dataDir, "downloads"),
		requestHeaders: sanitizeRequestHeaders(wantHeaders),
	}
	if err := os.MkdirAll(download.Folder, 0755); err != nil {
		t.Fatal(err)
	}
	if err := app.dl.downloadHTTP(context.Background(), download); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(download.FinalFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("downloaded content length = %d, want %d", len(got), len(content))
	}
}

func TestSanitizeRequestHeaders(t *testing.T) {
	got := sanitizeRequestHeaders(map[string]string{
		"cookie":        "session=ok",
		"Authorization": "secret",
		"Referer":       "https://example.test/\r\nX-Bad: yes",
	})
	if got["Cookie"] != "session=ok" {
		t.Fatalf("Cookie header missing: %#v", got)
	}
	if _, exists := got["Authorization"]; exists || len(got) != 1 {
		t.Fatalf("unsafe headers were retained: %#v", got)
	}
}
