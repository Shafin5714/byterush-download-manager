package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DownloadManager struct {
	app        *App
	mu         sync.Mutex
	downloads  map[string]*Download
	pending    []string
	activeCnt  int
	cancels    map[string]context.CancelFunc
	saveDirty  bool
	saveTicker *time.Ticker
}

func NewDownloadManager(app *App) *DownloadManager {
	m := &DownloadManager{
		app:       app,
		downloads: map[string]*Download{},
		cancels:   map[string]context.CancelFunc{},
	}
	for id, d := range app.store.LoadDownloads() {
		m.downloads[id] = d
	}
	go m.progressTicker()
	m.saveTicker = time.NewTicker(5 * time.Second)
	go func() {
		for range m.saveTicker.C {
			m.flushSave()
		}
	}()
	return m
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *DownloadManager) flushSave() {
	m.mu.Lock()
	if !m.saveDirty {
		m.mu.Unlock()
		return
	}
	m.saveDirty = false
	m.mu.Unlock()
	m.app.store.SaveDownloads(m.snapshotAll())
}

func (m *DownloadManager) markDirty() {
	m.mu.Lock()
	m.saveDirty = true
	m.mu.Unlock()
}

func (m *DownloadManager) All() []*Download {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Download, 0, len(m.downloads))
	for _, d := range m.downloads {
		out = append(out, d.clone())
	}
	return out
}

func (m *DownloadManager) Get(id string) *Download {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.downloads[id]
	if d == nil {
		return nil
	}
	return d.clone()
}

func (m *DownloadManager) snapshotAll() map[string]*Download {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]*Download{}
	for id, d := range m.downloads {
		out[id] = d.clone()
	}
	return out
}

func (m *DownloadManager) broadcast(d *Download) {
	m.app.hub.Broadcast(Event{Type: "update", Data: d.clone()})
}

func (m *DownloadManager) Add(req AddRequest) (*Download, error) {
	if req.Folder == "" {
		req.Folder = m.app.settings.DownloadDir
	}
	if req.Kind == "" {
		req.Kind = "http"
	}
	if err := os.MkdirAll(req.Folder, 0755); err != nil {
		return nil, err
	}
	d := &Download{
		ID:             newID(),
		URL:            req.URL,
		Filename:       req.Filename,
		Folder:         req.Folder,
		Kind:           req.Kind,
		Status:         StatusQueued,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		requestHeaders: sanitizeRequestHeaders(req.RequestHeaders),
	}
	m.mu.Lock()
	m.downloads[d.ID] = d
	m.pending = append(m.pending, d.ID)
	m.mu.Unlock()
	m.markDirty()
	m.app.hub.Broadcast(Event{Type: "added", Data: d.clone()})
	m.kick()
	return d.clone(), nil
}

func (m *DownloadManager) Remove(id string) error {
	m.mu.Lock()
	d := m.downloads[id]
	if d == nil {
		m.mu.Unlock()
		return fmt.Errorf("download %s not found", id)
	}
	if c, ok := m.cancels[id]; ok {
		d.cancelled = true
		c()
	}
	delete(m.downloads, id)
	m.pending = removeString(m.pending, id)
	m.mu.Unlock()
	m.markDirty()
	m.flushSave()
	m.app.hub.Broadcast(Event{Type: "removed", Data: id})
	return nil
}

func removeString(list []string, s string) []string {
	out := list[:0]
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

func (m *DownloadManager) Pause(id string) error {
	m.mu.Lock()
	d := m.downloads[id]
	if d == nil {
		m.mu.Unlock()
		return fmt.Errorf("download %s not found", id)
	}
	if d.Status == StatusActive {
		if c, ok := m.cancels[id]; ok {
			c()
		}
	}
	m.mu.Unlock()
	return nil
}

func (m *DownloadManager) PauseAll() {
	m.mu.Lock()
	ids := []string{}
	for id, d := range m.downloads {
		if d.Status == StatusActive || d.Status == StatusQueued {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Pause(id)
	}
}

func (m *DownloadManager) Resume(id string) error {
	m.mu.Lock()
	d := m.downloads[id]
	if d == nil {
		m.mu.Unlock()
		return fmt.Errorf("download %s not found", id)
	}
	if d.Status == StatusPaused || d.Status == StatusError {
		d.Status = StatusQueued
		d.Error = ""
		d.Speed = 0
		d.ETA = 0
		m.pending = append(m.pending, id)
		m.mu.Unlock()
		m.markDirty()
		m.broadcast(d)
		m.kick()
		return nil
	}
	m.mu.Unlock()
	return nil
}

func (m *DownloadManager) ResumeAll() {
	m.mu.Lock()
	ids := []string{}
	for id, d := range m.downloads {
		if d.Status == StatusPaused || d.Status == StatusError {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Resume(id)
	}
}

func (m *DownloadManager) kick() {
	m.mu.Lock()
	for len(m.pending) > 0 && m.activeCnt < m.app.settings.MaxActive {
		id := m.pending[0]
		m.pending = m.pending[1:]
		d := m.downloads[id]
		if d == nil || d.Status != StatusQueued {
			continue
		}
		d.Status = StatusActive
		d.UpdatedAt = time.Now()
		m.activeCnt++
		go m.run(d)
		m.broadcast(d)
	}
	m.mu.Unlock()
}

func (m *DownloadManager) run(d *Download) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[d.ID] = cancel
	m.mu.Unlock()

	finish := func(st Status, errMsg string) {
		cancel()
		m.mu.Lock()
		delete(m.cancels, d.ID)
		m.activeCnt--
		if st == StatusPaused {
			d.Status = StatusPaused
		} else if d.cancelled {
			d.Status = StatusCancelled
		} else if st == StatusError {
			d.Status = StatusError
			d.Error = errMsg
		} else {
			d.Status = st
		}
		d.Speed = 0
		d.ETA = 0
		d.UpdatedAt = time.Now()
		m.mu.Unlock()
		m.markDirty()
		m.broadcast(d)
		m.kick()
	}

	if d.Kind == "youtube" {
		err := m.app.yt.Run(ctx, d)
		if err != nil {
			if ctx.Err() != nil {
				finish(StatusPaused, "")
			} else {
				finish(StatusError, err.Error())
			}
			return
		}
		finish(StatusCompleted, "")
		return
	}

	err := m.downloadHTTP(ctx, d)
	if err != nil {
		if ctx.Err() != nil {
			finish(StatusPaused, "")
		} else {
			finish(StatusError, err.Error())
		}
		return
	}
	finish(StatusCompleted, "")
}

func (m *DownloadManager) progressTicker() {
	t := time.NewTicker(250 * time.Millisecond)
	for range t.C {
		m.mu.Lock()
		updates := []*Download{}
		for _, d := range m.downloads {
			if d.Status != StatusActive {
				continue
			}
			delta := d.Downloaded - d.lastBytes
			d.Speed = delta * 4
			d.lastBytes = d.Downloaded
			if d.TotalSize > 0 && d.Speed > 0 && d.TotalSize > d.Downloaded {
				d.ETA = (d.TotalSize - d.Downloaded) / d.Speed
			} else {
				d.ETA = 0
			}
			d.UpdatedAt = time.Now()
			updates = append(updates, d.clone())
		}
		m.mu.Unlock()
		for _, u := range updates {
			m.broadcast(u)
		}
	}
}

func (m *DownloadManager) downloadHTTP(ctx context.Context, d *Download) error {
	client := &http.Client{Transport: m.app.transport}
	info, err := probeURL(ctx, client, d.URL, d.requestHeaders)
	if err != nil {
		return err
	}

	if d.Filename == "" {
		d.Filename = info.filename
	}
	d.TotalSize = info.size
	d.FinalFile = filepath.Join(d.Folder, d.Filename)
	d.TempFile = d.FinalFile + ".br"

	if info.size >= 0 {
		if fi, err := os.Stat(d.FinalFile); err == nil && fi.Size() == info.size {
			d.Downloaded = info.size
			return nil
		}
	}

	segmented := info.size > 0 && info.acceptRanges
	conns := m.app.settings.Connections
	if conns < 1 {
		conns = 1
	}

	m.mu.Lock()
	if !segmented {
		// single stream: resume by current segment offset or saved downloaded count
		var cur int64
		if len(d.Segments) > 0 {
			cur = d.Segments[0].Current
		} else if d.Downloaded > 0 && info.size > 0 && d.Downloaded < info.size {
			cur = d.Downloaded
		} else if info.size <= 0 {
			if fi, err := os.Stat(d.TempFile); err == nil {
				cur = fi.Size()
			}
		}
		if _, err := os.Stat(d.TempFile); err != nil {
			cur = 0
		}
		if info.size > 0 && cur >= info.size {
			cur = 0
		}
		d.Segments = []SegmentState{{Index: 0, Start: 0, End: -1, Current: cur}}
		d.Downloaded = cur
	} else if len(d.Segments) == 0 {
		n := int64(conns)
		minSeg := int64(1 << 20)
		if n > info.size/minSeg {
			n = info.size / minSeg
		}
		if n < 1 {
			n = 1
		}
		segs := make([]SegmentState, 0, n)
		for i := int64(0); i < n; i++ {
			start := info.size * i / n
			end := info.size*(i+1)/n - 1
			segs = append(segs, SegmentState{Index: int(i), Start: start, End: end, Current: start})
		}
		d.Segments = segs
		d.Downloaded = 0
	} else {
		// resume: re-check temp file exists
		if _, err := os.Stat(d.TempFile); err != nil {
			for i := range d.Segments {
				d.Segments[i].Current = d.Segments[i].Start
			}
		}
		sum := int64(0)
		for i := range d.Segments {
			sum += d.Segments[i].Current - d.Segments[i].Start
		}
		if info.size > 0 && sum > info.size {
			sum = info.size
		}
		d.Downloaded = sum
	}
	total := d.TotalSize
	m.mu.Unlock()

	if total > 0 {
		// Windows os.Truncate requires the file to exist; create it first.
		tf, err := os.OpenFile(d.TempFile, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		if err := tf.Truncate(total); err != nil {
			tf.Close()
			return err
		}
		tf.Close()
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(d.Segments))
	for i := range d.Segments {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if err := m.downloadSegment(ctx, d, idx); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return err
	}

	m.mu.Lock()
	sum := int64(0)
	for i := range d.Segments {
		sum += d.Segments[i].Current - d.Segments[i].Start
	}
	if total > 0 && sum > total {
		sum = total
	}
	d.Downloaded = sum
	done := sum > 0 && (total <= 0 || sum >= total)
	segsDebug := make([]SegmentState, len(d.Segments))
	copy(segsDebug, d.Segments)
	m.mu.Unlock()

	if done {
		if err := os.Rename(d.TempFile, d.FinalFile); err != nil {
			return err
		}
		d.Downloaded = sum
		d.Filename = filepath.Base(d.FinalFile)
		return nil
	}
	return fmt.Errorf("incomplete download: got %d of %d bytes (segments: %+v)", sum, total, segsDebug)
}

const maxSegmentRetries = 6

func (m *DownloadManager) downloadSegment(ctx context.Context, d *Download, idx int) error {
	backoff := 750 * time.Millisecond
	for attempt := 0; ; attempt++ {
		err := m.trySegment(ctx, d, idx)
		if err == nil {
			m.mu.Lock()
			seg := d.Segments[idx]
			m.mu.Unlock()
			if seg.End < 0 || seg.Current > seg.End {
				return nil
			}
		} else if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt >= maxSegmentRetries {
			if err != nil {
				return err
			}
			return fmt.Errorf("segment %d still incomplete after retries", idx)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
}

func (m *DownloadManager) trySegment(ctx context.Context, d *Download, idx int) error {
	m.mu.Lock()
	seg := d.Segments[idx]
	m.mu.Unlock()

	if seg.End >= 0 && seg.Current > seg.End {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", d.URL, nil)
	if err != nil {
		return err
	}
	if seg.End >= 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", seg.Current, seg.End))
	}
	applyRequestHeaders(req, d.requestHeaders)
	client := &http.Client{Transport: m.app.transport}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		m.mu.Lock()
		seg.Current = seg.End + 1
		d.Segments[idx] = seg
		m.mu.Unlock()
		return nil
	}
	if resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK {
		if seg.End >= 0 && resp.StatusCode == http.StatusOK && idx > 0 {
			return fmt.Errorf("server does not support range requests")
		}
	} else {
		return probeStatusError(resp)
	}

	if resp.StatusCode == http.StatusOK && seg.End >= 0 {
		// server ignored the range and sent the body from offset 0
		seg.Current = seg.Start
		m.mu.Lock()
		d.Segments[idx] = seg
		sum := int64(0)
		for i := range d.Segments {
			sum += d.Segments[i].Current - d.Segments[i].Start
		}
		d.Downloaded = sum
		m.mu.Unlock()
	}
	f, err := os.OpenFile(d.TempFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if seg.End < 0 {
		if _, err := f.Seek(seg.Current, io.SeekStart); err != nil {
			return err
		}
	}

	buf := make([]byte, 256*1024)
	for {
		if seg.End >= 0 && seg.Current > seg.End {
			return nil
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if seg.End >= 0 {
				rem := seg.End - seg.Current + 1
				if rem <= 0 {
					return nil
				}
				if int64(n) > rem {
					n = int(rem)
				}
			}
			m.app.limiter.Wait(n)
			var werr error
			if seg.End >= 0 {
				_, werr = f.WriteAt(buf[:n], seg.Current)
			} else {
				_, werr = f.Write(buf[:n])
			}
			if werr != nil {
				return werr
			}
			seg.Current += int64(n)
			m.mu.Lock()
			d.Segments[idx] = seg
			d.Downloaded += int64(n)
			if d.TotalSize > 0 && d.Downloaded > d.TotalSize {
				d.Downloaded = d.TotalSize
			}
			m.mu.Unlock()
		}
		if seg.End >= 0 && seg.Current > seg.End {
			return nil
		}
		if rerr == io.EOF {
			if seg.End >= 0 && seg.Current <= seg.End {
				return fmt.Errorf("connection closed early (segment %d: %d/%d)",
					idx, seg.Current-seg.Start, seg.End-seg.Start+1)
			}
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

type probeResult struct {
	size         int64
	acceptRanges bool
	filename     string
}

func probeURL(ctx context.Context, client *http.Client, rawURL string, headers map[string]string) (*probeResult, error) {
	probeClient := *client
	probeClient.Timeout = 15 * time.Second
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return nil, err
	}
	applyRequestHeaders(req, headers)
	resp, headErr := probeClient.Do(req)
	if headErr == nil {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			return resultFromResponse(resp.Header, resp.StatusCode, responseURL(resp, rawURL)), nil
		}
		resp.Body.Close()
	}

	// A number of file hosts reject HEAD even though GET is supported. A one-byte
	// ranged GET both verifies access and obtains the real size from Content-Range.
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req2.Header.Set("Range", "bytes=0-0")
	applyRequestHeaders(req2, headers)
	resp2, getErr := probeClient.Do(req2)
	if getErr != nil {
		if headErr != nil {
			return nil, headErr
		}
		return nil, getErr
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK && resp2.StatusCode != http.StatusPartialContent {
		return nil, probeStatusError(resp2)
	}
	return resultFromResponse(resp2.Header, resp2.StatusCode, responseURL(resp2, rawURL)), nil
}

func responseURL(resp *http.Response, fallback string) string {
	if resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return fallback
}

func probeStatusError(resp *http.Response) error {
	if strings.EqualFold(resp.Header.Get("CF-Mitigated"), "challenge") {
		return fmt.Errorf("HTTP %d: server requires browser verification; use the ByteRush browser extension so the browser session can be handed off", resp.StatusCode)
	}
	return fmt.Errorf("unexpected HTTP status %d while checking download", resp.StatusCode)
}

func resultFromResponse(h http.Header, status int, rawURL string) *probeResult {
	r := &probeResult{size: -1}
	contentRange := h.Get("Content-Range")
	if contentRange != "" {
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 {
			if size, err := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64); err == nil {
				r.size = size
			}
		}
	}
	// Content-Length on a 206 response is only the selected range length, not
	// the whole file. Leave the total unknown if Content-Range omitted it.
	if r.size < 0 && status != http.StatusPartialContent {
		if size, err := strconv.ParseInt(h.Get("Content-Length"), 10, 64); err == nil {
			r.size = size
		}
	}
	r.acceptRanges = status == http.StatusPartialContent || strings.Contains(strings.ToLower(h.Get("Accept-Ranges")), "bytes")
	if cd := h.Get("Content-Disposition"); cd != "" {
		r.filename = filenameFromDisposition(cd)
	}
	if r.filename == "" {
		if parsed, err := url.Parse(rawURL); err == nil {
			r.filename = filepath.Base(strings.TrimSuffix(parsed.Path, "/"))
		}
	}
	r.filename = sanitizeFilename(r.filename)
	if r.filename == "" || r.filename == "." || r.filename == ".." {
		r.filename = "download.bin"
	}
	return r
}

var forwardedRequestHeaders = map[string]string{
	"accept":          "Accept",
	"accept-language": "Accept-Language",
	"cookie":          "Cookie",
	"referer":         "Referer",
	"user-agent":      "User-Agent",
}

func sanitizeRequestHeaders(headers map[string]string) map[string]string {
	clean := make(map[string]string)
	for name, value := range headers {
		canonical, ok := forwardedRequestHeaders[strings.ToLower(strings.TrimSpace(name))]
		if !ok || strings.ContainsAny(value, "\r\n") {
			continue
		}
		clean[canonical] = value
	}
	return clean
}

func applyRequestHeaders(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		req.Header.Set(name, value)
	}
}

func filenameFromDisposition(cd string) string {
	parts := strings.Split(cd, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToLower(p), "filename*=") {
			v := strings.TrimPrefix(p, "filename*=")
			if i := strings.LastIndex(v, "'"); i >= 0 {
				v = v[i+1:]
			}
			v = strings.Trim(v, `"`)
			v = percentDecode(v)
			return v
		}
		if strings.HasPrefix(strings.ToLower(p), "filename=") {
			v := strings.TrimPrefix(p, "filename=")
			return strings.Trim(v, `"`)
		}
	}
	return ""
}

func percentDecode(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var b byte
			if _, err := fmt.Sscanf(s[i+1:i+3], "%02x", &b); err == nil {
				out = append(out, b)
				i += 2
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}

var illegalChars = []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*'}

func sanitizeFilename(name string) string {
	var sb strings.Builder
	for _, c := range name {
		if c < 32 {
			continue
		}
		bad := false
		for _, b := range illegalChars {
			if c == b {
				bad = true
				break
			}
		}
		if bad {
			sb.WriteRune('_')
		} else {
			sb.WriteRune(c)
		}
	}
	out := strings.TrimSpace(sb.String())
	if len(out) > 200 {
		out = out[len(out)-200:]
	}
	return out
}
