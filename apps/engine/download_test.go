package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
