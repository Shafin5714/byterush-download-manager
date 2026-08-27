package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type YoutubeManager struct {
	app         *App
	mu          sync.Mutex
	setupMu     sync.Mutex
	binary      string
	deno        string
	denoChecked bool
	ffmpeg      string
}

const (
	ytDLPUpdateInterval        = 24 * time.Hour
	ytDLPUpdateFailureInterval = 5 * time.Minute
)

var (
	ytDLPDownloadURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	denoDownloadURL  = "https://github.com/denoland/deno/releases/latest/download/deno-x86_64-pc-windows-msvc.zip"
)

func NewYoutubeManager(app *App) *YoutubeManager {
	return &YoutubeManager{app: app}
}

func (y *YoutubeManager) ensureBinary() (string, error) {
	y.setupMu.Lock()
	defer y.setupMu.Unlock()
	if y.binary != "" {
		if _, err := os.Stat(y.binary); err == nil {
			y.checkManagedBinaryUpdate(y.binary, false)
			return y.binary, nil
		}
	}
	candidates := []string{
		y.app.ytdlpPath,
		filepath.Join(y.app.dir, "yt-dlp.exe"),
	}
	for _, c := range candidates {
		if c != "" {
			if _, err := os.Stat(c); err == nil {
				y.binary = c
				y.checkManagedBinaryUpdate(c, false)
				return c, nil
			}
		}
	}
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		y.binary = p
		return p, nil
	}
	// download it
	dest := filepath.Join(y.app.dir, "yt-dlp.exe")
	if err := y.downloadBinary(dest); err != nil {
		return "", fmt.Errorf("yt-dlp not found and auto-download failed: %w", err)
	}
	y.binary = dest
	return dest, nil
}

func (y *YoutubeManager) checkManagedBinaryUpdate(exe string, force bool) {
	managed := filepath.Join(y.app.dir, "yt-dlp.exe")
	if !samePath(exe, managed) {
		return
	}
	stamp := filepath.Join(y.app.dir, "yt-dlp-update-check")
	if !force && !ytDLPUpdateDue(stamp, time.Now()) {
		return
	}

	y.app.hub.Broadcast(Event{Type: "log", Data: "Checking for yt-dlp updates..."})
	cmd := exec.Command(exe, "--update-to", "stable")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.WriteFile(stamp, []byte("failed"), 0644)
		y.app.hub.Broadcast(Event{Type: "log", Data: "yt-dlp update check failed; using the installed version: " + strings.TrimSpace(string(out))})
		return
	}
	_ = os.WriteFile(stamp, []byte("ok"), 0644)
	if force {
		y.app.hub.Broadcast(Event{Type: "log", Data: "yt-dlp update completed; retrying the YouTube download..."})
	}
}

func ytDLPUpdateDue(stamp string, now time.Time) bool {
	info, err := os.Stat(stamp)
	if err != nil {
		return true
	}
	interval := ytDLPUpdateInterval
	if status, readErr := os.ReadFile(stamp); readErr == nil && strings.TrimSpace(string(status)) == "failed" {
		interval = ytDLPUpdateFailureInterval
	}
	return now.Sub(info.ModTime()) >= interval
}

func samePath(a, b string) bool {
	a, errA := filepath.Abs(a)
	b, errB := filepath.Abs(b)
	return errA == nil && errB == nil && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func (y *YoutubeManager) downloadBinary(dest string) error {
	y.app.hub.Broadcast(Event{Type: "log", Data: "Downloading yt-dlp..."})
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(ytDLPDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("yt-dlp download returned %d", resp.StatusCode)
	}
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, dest)
}

func (y *YoutubeManager) ensureDeno() (string, error) {
	y.setupMu.Lock()
	defer y.setupMu.Unlock()
	if y.deno != "" {
		if _, err := os.Stat(y.deno); err == nil {
			return y.deno, nil
		}
	}
	if y.denoChecked {
		return "", fmt.Errorf("Deno is unavailable")
	}
	y.denoChecked = true

	candidates := []string{filepath.Join(y.app.dir, "deno.exe")}
	if p, err := exec.LookPath("deno"); err == nil {
		candidates = append(candidates, p)
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			y.deno = candidate
			return candidate, nil
		}
	}

	dest := filepath.Join(y.app.dir, "deno.exe")
	if err := y.downloadDeno(dest); err != nil {
		return "", fmt.Errorf("Deno not found and auto-download failed: %w", err)
	}
	y.deno = dest
	return dest, nil
}

func (y *YoutubeManager) downloadDeno(dest string) error {
	y.app.hub.Broadcast(Event{Type: "log", Data: "Downloading Deno (needed for YouTube verification; one-time setup)..."})
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(denoDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Deno download returned %d", resp.StatusCode)
	}

	archive := dest + ".zip"
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		_ = os.Remove(archive)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(archive)
		return err
	}
	defer os.Remove(archive)

	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.Base(filepath.FromSlash(zf.Name)) != "deno.exe" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		tmp := dest + ".tmp"
		out, err := os.Create(tmp)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			_ = os.Remove(tmp)
			return copyErr
		}
		if closeErr != nil {
			_ = os.Remove(tmp)
			return closeErr
		}
		return os.Rename(tmp, dest)
	}
	return fmt.Errorf("deno.exe not found in downloaded archive")
}

func (y *YoutubeManager) youtubeRuntimeArgs() []string {
	deno, err := y.ensureDeno()
	if err != nil {
		y.app.hub.Broadcast(Event{Type: "log", Data: err.Error() + "; YouTube downloads may fail"})
		return nil
	}
	return []string{"--js-runtimes", "deno:" + deno}
}

func (y *YoutubeManager) ensureFFmpeg() (string, error) {
	y.mu.Lock()
	cached := y.ffmpeg
	y.mu.Unlock()
	if cached != "" {
		if _, err := os.Stat(cached); err == nil {
			return cached, nil
		}
	}
	dest := filepath.Join(y.app.dir, "ffmpeg.exe")
	if _, err := os.Stat(dest); err == nil {
		y.mu.Lock()
		y.ffmpeg = dest
		y.mu.Unlock()
		return dest, nil
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		y.mu.Lock()
		y.ffmpeg = p
		y.mu.Unlock()
		return p, nil
	}
	if err := y.downloadFFmpeg(dest); err != nil {
		return "", fmt.Errorf("ffmpeg not found and auto-download failed: %w", err)
	}
	y.mu.Lock()
	y.ffmpeg = dest
	y.mu.Unlock()
	return dest, nil
}

func (y *YoutubeManager) downloadFFmpeg(dest string) error {
	url := "https://github.com/BtbN/FFmpeg-Builds/releases/latest/download/ffmpeg-master-latest-win64-gpl.zip"
	y.app.hub.Broadcast(Event{Type: "log", Data: "Downloading ffmpeg (needed for high-quality merges; first download may take a while)..."})
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ffmpeg download returned %d", resp.StatusCode)
	}
	tmp := dest + ".zip"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	defer os.Remove(tmp)

	y.app.hub.Broadcast(Event{Type: "log", Data: "Extracting ffmpeg..."})
	zr, err := zip.OpenReader(tmp)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if !strings.HasSuffix(zf.Name, "bin/ffmpeg.exe") {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
		return nil
	}
	return fmt.Errorf("ffmpeg.exe not found in downloaded archive")
}

func needsMerge(f string) bool {
	return f != "" && f != "best" && (strings.Contains(f, "+") || strings.HasPrefix(f, "bestvideo"))
}

type YoutubeFormat struct {
	ID       string `json:"id"`
	Ext      string `json:"ext"`
	Label    string `json:"label"`
	Height   int    `json:"height,omitempty"`
	FPS      int    `json:"fps,omitempty"`
	VCodec   string `json:"vcodec,omitempty"`
	ACodec   string `json:"acodec,omitempty"`
	Filesize int64  `json:"filesize,omitempty"`
	Audio    bool   `json:"audio"`
}

type YoutubeEntry struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration,omitempty"`
}

type YoutubeInfo struct {
	Title      string          `json:"title"`
	ID         string          `json:"id"`
	Duration   float64         `json:"duration,omitempty"`
	Thumbnail  string          `json:"thumbnail,omitempty"`
	IsPlaylist bool            `json:"isPlaylist"`
	Entries    []YoutubeEntry  `json:"entries,omitempty"`
	Formats    []YoutubeFormat `json:"formats,omitempty"`
}

type rawInfo struct {
	Title     string  `json:"title"`
	ID        string  `json:"id"`
	Duration  float64 `json:"duration"`
	Thumbnail string  `json:"thumbnail"`
	Formats   []struct {
		FormatID       string  `json:"format_id"`
		Ext            string  `json:"ext"`
		Height         float64 `json:"height"`
		Width          float64 `json:"width"`
		FPS            float64 `json:"fps"`
		VCodec         string  `json:"vcodec"`
		ACodec         string  `json:"acodec"`
		Filesize       int64   `json:"filesize"`
		FilesizeApprox int64   `json:"filesize_approx"`
	} `json:"formats"`
	Entries []struct {
		ID       string  `json:"id"`
		Title    string  `json:"title"`
		Duration float64 `json:"duration"`
	} `json:"entries"`
}

func (y *YoutubeManager) Info(url string) (*YoutubeInfo, error) {
	exe, err := y.ensureBinary()
	if err != nil {
		return nil, err
	}
	args := append(y.youtubeRuntimeArgs(), "-J", "--no-warnings", "--flat-playlist", "--no-playlist", url)
	cmd := exec.Command(exe, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	var raw rawInfo
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp output: %w", err)
	}
	info := &YoutubeInfo{
		Title:     raw.Title,
		ID:        raw.ID,
		Duration:  raw.Duration,
		Thumbnail: raw.Thumbnail,
	}
	entries := raw.Entries
	hasEntries := len(entries) > 0
	hasFormats := len(raw.Formats) > 0
	if hasEntries && !hasFormats {
		info.IsPlaylist = true
		for _, e := range entries {
			info.Entries = append(info.Entries, YoutubeEntry{ID: e.ID, Title: e.Title, Duration: e.Duration})
		}
	} else {
		seen := map[string]bool{}
		for _, f := range raw.Formats {
			if f.VCodec == "none" && f.ACodec == "none" {
				continue // storyboards / thumbnails
			}
			if f.VCodec == "none" {
				if seen["a:"+f.FormatID] {
					continue
				}
				seen["a:"+f.FormatID] = true
				label := fmt.Sprintf("Audio • %s", f.Ext)
				info.Formats = append(info.Formats, YoutubeFormat{
					ID: f.FormatID, Ext: f.Ext, Label: label, Audio: true,
					ACodec: f.ACodec, Filesize: f.Filesize,
				})
				continue
			}
			if f.ACodec == "none" {
				// video-only stream: offer it merged with the best audio stream
				if f.Height <= 0 {
					continue
				}
				key := fmt.Sprintf("m:%d:%s", int(f.Height), f.Ext)
				if seen[key] {
					continue
				}
				seen[key] = true
				res := fmt.Sprintf("%dp", int(f.Height))
				if f.FPS > 30 {
					res = fmt.Sprintf("%s %dfps", res, int(f.FPS))
				}
				fs := f.Filesize
				if fs == 0 {
					fs = f.FilesizeApprox
				}
				size := ""
				if fs > 0 {
					size = fmt.Sprintf(" • %s", humanSize(fs))
				}
				info.Formats = append(info.Formats, YoutubeFormat{
					ID: f.FormatID + "+bestaudio/best", Ext: f.Ext, Label: fmt.Sprintf("%s • %s (merge)%s", res, f.Ext, size),
					Height: int(f.Height), FPS: int(f.FPS), VCodec: f.VCodec, Filesize: fs,
				})
				continue
			}
			if seen["v:"+f.FormatID] {
				continue
			}
			seen["v:"+f.FormatID] = true
			res := ""
			if f.Height > 0 {
				res = fmt.Sprintf("%dp", int(f.Height))
			} else {
				res = "video"
			}
			if f.FPS > 30 {
				res = fmt.Sprintf("%s %dfps", res, int(f.FPS))
			}
			size := ""
			fs := f.Filesize
			if fs == 0 {
				fs = f.FilesizeApprox
			}
			if fs > 0 {
				size = fmt.Sprintf(" • %s", humanSize(fs))
			}
			info.Formats = append(info.Formats, YoutubeFormat{
				ID: f.FormatID, Ext: f.Ext, Label: fmt.Sprintf("%s • %s%s", res, f.Ext, size),
				Height: int(f.Height), FPS: int(f.FPS), VCodec: f.VCodec, ACodec: f.ACodec,
				Filesize: fs,
			})
		}
	}
	return info, nil
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func (y *YoutubeManager) Run(ctx context.Context, d *Download) error {
	exe, err := y.ensureBinary()
	if err != nil {
		return err
	}
	req := YoutubeDownloadRequest{URL: d.URL, Folder: d.Folder}
	if d.Youtube != nil {
		req = *d.Youtube
		if req.URL == "" {
			req.URL = d.URL
		}
		if req.Folder == "" {
			req.Folder = d.Folder
		}
	}

	ffmpeg := ""
	if p, err := y.ensureFFmpeg(); err == nil {
		ffmpeg = p
	} else {
		y.app.hub.Broadcast(Event{Type: "log", Data: "ffmpeg unavailable (" + err.Error() + ") — falling back to best combined quality"})
		if needsMerge(req.Format) {
			req.Format = "best"
		}
	}

	targetContainer := req.Container
	if targetContainer == "" || targetContainer == "auto" {
		if strings.Contains(req.Format, "[ext=mp4]") {
			targetContainer = "mp4"
		}
	}

	args := append(y.youtubeRuntimeArgs(), "--newline", "--progress", "--no-warnings", "--no-colors")
	if req.Format != "" && req.Format != "best" {
		args = append(args, "-f", req.Format)
	}
	if ffmpeg != "" {
		args = append(args, "--ffmpeg-location", filepath.Dir(ffmpeg))
	}
	if targetContainer != "" && targetContainer != "auto" {
		args = append(args, "--merge-output-format", targetContainer, "--remux-video", targetContainer)
	}
	if req.PlaylistItems != "" {
		args = append(args, "--playlist-items", req.PlaylistItems)
	} else {
		args = append(args, "--no-playlist")
	}
	args = append(args,
		"-o", filepath.Join(d.Folder, "%(title)s.%(ext)s"),
		"--print", "after_move:filepath",
		req.URL,
	)
	y.app.hub.Broadcast(Event{Type: "log", Data: "yt-dlp " + strings.Join(args, " ")})

	final, runErr := y.run(ctx, exe, args, d)
	if runErr != nil && isHTTP403(runErr) {
		y.app.hub.Broadcast(Event{Type: "log", Data: "YouTube returned HTTP 403; refreshing yt-dlp and retrying once over IPv4..."})
		y.setupMu.Lock()
		y.checkManagedBinaryUpdate(exe, true)
		y.setupMu.Unlock()
		final, runErr = y.run(ctx, exe, append([]string{"--force-ipv4"}, args...), d)
	}
	if runErr != nil {
		if isHTTP403(runErr) {
			return fmt.Errorf("yt-dlp failed after an IPv4 retry: %w; turn off any VPN/proxy and try again", runErr)
		}
		return runErr
	}
	if final == "" {
		return fmt.Errorf("could not determine output file")
	}
	y.app.dl.setYoutubeOutput(d, final)
	return nil
}

func (y *YoutubeManager) run(ctx context.Context, exe string, args []string, d *Download) (string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var final string
	var outputMu sync.Mutex
	var pipeWG sync.WaitGroup
	pipeWG.Add(2)
	go func() {
		defer pipeWG.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if !y.parseProgress(d, line) && !strings.HasPrefix(line, "[") {
				// --print after_move:filepath emits the raw path
				outputMu.Lock()
				final = strings.TrimSpace(line)
				outputMu.Unlock()
			}
		}
	}()
	var errBuf strings.Builder
	go func() {
		defer pipeWG.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if y.parseProgress(d, line) {
				continue
			}
			if errBuf.Len() > 0 {
				errBuf.WriteByte('\n')
			}
			errBuf.WriteString(line)
		}
	}()

	err = cmd.Wait()
	pipeWG.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w: %s", err, strings.TrimSpace(errBuf.String()))
	}

	outputMu.Lock()
	defer outputMu.Unlock()
	return final, nil
}

func (y *YoutubeManager) parseProgress(d *Download, line string) bool {
	m := ytProgressRe.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	pct, _ := strconv.ParseFloat(m[1], 64)
	total := parseSize(m[2], m[3])
	speed := parseSize(m[4], m[5])
	y.app.dl.updateYoutubeProgress(d, int64(pct/100.0*float64(total)), total, speed)
	return true
}

func isHTTP403(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "http error 403")
}

var ytProgressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*([\d.]+)\s*([KMGTPE])?i?B(?:\s+at\s+([\d.]+)\s*([KMGTPE])?i?B/s)?`)

func parseSize(v, unit string) int64 {
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseFloat(v, 64)
	mult := 1.0
	switch unit {
	case "K":
		mult = 1024
	case "M":
		mult = 1024 * 1024
	case "G":
		mult = 1024 * 1024 * 1024
	case "T":
		mult = 1024 * 1024 * 1024 * 1024
	case "P":
		mult = 1024 * 1024 * 1024 * 1024 * 1024
	case "E":
		mult = 1024 * 1024 * 1024 * 1024 * 1024 * 1024
	}
	return int64(n * mult)
}
