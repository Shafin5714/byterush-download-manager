package main

import (
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusActive    Status = "active"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusError     Status = "error"
	StatusCancelled Status = "cancelled"
)

type SegmentState struct {
	Index   int   `json:"index"`
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
	Current int64 `json:"current"`
}

type Download struct {
	ID         string         `json:"id"`
	URL        string         `json:"url"`
	Filename   string         `json:"filename"`
	TempFile   string         `json:"tempFile"`
	FinalFile  string         `json:"finalFile"`
	Folder     string         `json:"folder"`
	Kind       string         `json:"kind"`
	TotalSize  int64          `json:"totalSize"`
	Downloaded int64          `json:"downloaded"`
	Speed      int64          `json:"speed"`
	ETA        int64          `json:"eta"`
	Status     Status         `json:"status"`
	Segments   []SegmentState `json:"segments"`
	Error      string         `json:"error,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`

	lastBytes int64
	cancelled bool
}

func (d *Download) clone() *Download {
	c := *d
	c.Segments = make([]SegmentState, len(d.Segments))
	copy(c.Segments, d.Segments)
	return &c
}

type Settings struct {
	DownloadDir   string `json:"downloadDir"`
	MaxActive     int    `json:"maxActive"`
	Connections   int    `json:"connections"`
	SpeedLimitKBs int    `json:"speedLimitKBs"`
}

func DefaultSettings(dir string) *Settings {
	return &Settings{
		DownloadDir:   dir,
		MaxActive:     3,
		Connections:   8,
		SpeedLimitKBs: 0,
	}
}

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type AddRequest struct {
	URL       string `json:"url"`
	Filename  string `json:"filename"`
	Folder    string `json:"folder"`
	Kind      string `json:"kind"`
	Connections int  `json:"connections"`
}

type YoutubeInfoRequest struct {
	URL string `json:"url"`
}

type YoutubeDownloadRequest struct {
	URL           string `json:"url"`
	Format        string `json:"format"`
	PlaylistItems string `json:"playlistItems"`
	Folder        string `json:"folder"`
	Container     string `json:"container"`
}
