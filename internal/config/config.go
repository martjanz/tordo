// Package config reads and writes config.json in the app folder.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "config.json"

type Config struct {
	Roots        []string `json:"roots"`               // catalog form: relative to the app folder, '/'-separated
	Ignore       []string `json:"ignore"`              // extra folder patterns to skip (see mediafile.IgnoredDir)
	DiscogsToken string   `json:"discogsToken"`        // optional: enables Discogs search
	Normalize    *bool    `json:"normalize,omitempty"` // volume normalization; nil (absent): on
	LAN          LAN      `json:"lan"`
	ListenMode   bool     `json:"listenMode"` // listen mode: the server refuses curation (scan, identify, collections, genre aliases)
}

// DefaultPort is the LAN port when config.json does not say.
const DefaultPort = 4747

// LAN is the access from other devices on the local network.
type LAN struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`    // 0 (absent): DefaultPort
	PinHash string `json:"pinHash"` // auth.HashPIN; "" when no PIN was chosen
}

// PortOrDefault is the LAN port to listen on.
func (l LAN) PortOrDefault() int {
	if l.Port == 0 {
		return DefaultPort
	}
	return l.Port
}

// Normalizing reports whether the player evens out the volume of tracks.
func (c Config) Normalizing() bool { return c.Normalize == nil || *c.Normalize }

// Load reads config.json from appDir. When the file does not exist it returns
// the defaults (the sibling folders of appDir as roots, no ignore patterns,
// normalization on, LAN off) and created=true: setup is pending. Load never
// writes; the caller saves.
func Load(appDir string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		roots, err := DefaultRoots(appDir)
		if err != nil {
			return Config{}, false, err
		}
		on := true
		return Config{Roots: roots, Ignore: []string{}, Normalize: &on, LAN: LAN{Port: DefaultPort}}, true, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	if cfg.Roots == nil {
		cfg.Roots = []string{}
	}
	if cfg.Ignore == nil {
		cfg.Ignore = []string{}
	}
	return cfg, false, nil
}

func Save(appDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Write a temporary file next to config.json and rename it over, so a crash
	// never leaves a half-written config.json.
	tmp, err := os.CreateTemp(appDir, "config-*.json")
	if err != nil {
		return err
	}
	_, err = tmp.Write(append(data, '\n'))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), filepath.Join(appDir, FileName))
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// DefaultRoots lists the sibling folders of appDir as "../<name>", skipping
// hidden and system folders.
func DefaultRoots(appDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Dir(appDir))
	if err != nil {
		return nil, err
	}
	self := filepath.Base(appDir)
	roots := []string{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == self || skipRoot(name) {
			continue
		}
		roots = append(roots, "../"+name)
	}
	return roots, nil
}

func skipRoot(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
		return true
	}
	switch strings.ToLower(name) {
	case "system volume information", "recycler", "lost+found":
		return true
	}
	return false
}
