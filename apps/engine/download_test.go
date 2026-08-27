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
