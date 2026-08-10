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
	app        *App
	mu         sync.Mutex
	binary     string
	ffmpeg     string
	lastOutput string
}

func NewYoutubeManager(app *App) *YoutubeManager {
	return &YoutubeManager{app: app}
}

func (y *YoutubeManager) ensureBinary() (string, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.binary != "" {
		if _, err := os.Stat(y.binary); err == nil {
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

func (y *YoutubeManager) downloadBinary(dest string) error {
	url := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	y.app.hub.Broadcast(Event{Type: "log", Data: "Downloading yt-dlp..."})
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
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
	Title       string          `json:"title"`
	ID          string          `json:"id"`
	Duration    float64         `json:"duration,omitempty"`
	Thumbnail   string          `json:"thumbnail,omitempty"`
	IsPlaylist  bool            `json:"isPlaylist"`
	Entries     []YoutubeEntry  `json:"entries,omitempty"`
	Formats     []YoutubeFormat `json:"formats,omitempty"`
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

func (y *YoutubeManager) setReq(req YoutubeDownloadRequest) {
	y.app.setReq(req)
}

func (y *YoutubeManager) Info(url string) (*YoutubeInfo, error) {	exe, err := y.ensureBinary()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, "-J", "--no-warnings", "--flat-playlist", "--no-playlist", url)
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
	req := y.app.youtubeReq
	if req == nil {
		return fmt.Errorf("missing youtube request")
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

	args := []string{"--newline", "--progress", "--no-warnings"}
	if req.Format != "" && req.Format != "best" {
		args = append(args, "-f", req.Format)
	}
	if ffmpeg != "" {
		args = append(args, "--ffmpeg-location", filepath.Dir(ffmpeg))
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

	cmd := exec.CommandContext(ctx, exe, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			if m := ytProgressRe.FindStringSubmatch(line); m != nil {
				pct, _ := strconv.ParseFloat(m[1], 64)
				total := parseSize(m[2], m[3])
				speed := parseSize(m[4], m[5])
				y.updateProgress(d, int64(pct/100.0*float64(total)), total, speed)
			} else if !strings.HasPrefix(line, "[") {
				// --print after_move:filepath emits the raw path
				y.mu.Lock()
				y.lastOutput = strings.TrimSpace(line)
				y.mu.Unlock()
			}
		}
	}()
	var errBuf strings.Builder
	go func() {
		io.Copy(&errBuf, stderr)
	}()

	err = cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("yt-dlp failed: %w: %s", err, strings.TrimSpace(errBuf.String()))
	}

	// final path was printed via --print after_move:filepath
	y.mu.Lock()
	final := y.lastOutput
	y.mu.Unlock()
	if final == "" {
		return fmt.Errorf("could not determine output file")
	}
	d.FinalFile = final
	d.Filename = filepath.Base(final)
	d.TotalSize = d.Downloaded
	y.app.hub.Broadcast(Event{Type: "update", Data: d.clone()})
	return nil
}

var ytProgressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*([\d.]+)\s*([KMG])?i?B(?:\s+at\s+([\d.]+)\s*([KMG])?i?B/s)?`)

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
	}
	return int64(n * mult)
}

func (y *YoutubeManager) updateProgress(d *Download, down, total int64, speed int64) {
	y.mu.Lock()
	d.Downloaded = down
	d.TotalSize = total
	d.Speed = speed
	if d.Speed > 0 && total > down {
		d.ETA = int64((time.Duration(total-down) / time.Duration(d.Speed)) * time.Second)
	}
	y.mu.Unlock()
	y.app.hub.Broadcast(Event{Type: "update", Data: d.clone()})
}
