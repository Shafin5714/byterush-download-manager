package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Store struct {
	dir string
	mu  sync.Mutex
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) downloadsPath() string { return filepath.Join(s.dir, "downloads.json") }
func (s *Store) settingsPath() string  { return filepath.Join(s.dir, "settings.json") }

func (s *Store) LoadDownloads() map[string]*Download {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.downloadsPath())
	if err != nil {
		return map[string]*Download{}
	}
	var list map[string]*Download
	if err := json.Unmarshal(data, &list); err != nil {
		return map[string]*Download{}
	}
	for _, d := range list {
		if d.Status == StatusActive || d.Status == StatusQueued {
			d.Status = StatusPaused
		}
	}
	return list
}

func (s *Store) SaveDownloads(downloads map[string]*Download) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(downloads, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.downloadsPath(), data)
}

func (s *Store) LoadSettings() *Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.settingsPath())
	if err != nil {
		return nil
	}
	var st Settings
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	return &st
}

func (s *Store) SaveSettings(st *Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.settingsPath(), data)
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var _ = time.Now
