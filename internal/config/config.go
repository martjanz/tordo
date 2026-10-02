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
}

// Normalizing reports whether the player evens out the volume of tracks.
func (c Config) Normalizing() bool { return c.Normalize == nil || *c.Normalize }

// Load reads config.json from appDir. When the file does not exist it returns
// defaults (the sibling folders of appDir as roots) and created=true.
func Load(appDir string) (Config, bool, error) {
	data, err := os.ReadFile(filepath.Join(appDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		roots, err := DefaultRoots(appDir)
		if err != nil {
			return Config{}, false, err
		}
		on := true
		return Config{Roots: roots, Ignore: []string{}, Normalize: &on}, true, nil
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
	return os.WriteFile(filepath.Join(appDir, FileName), append(data, '\n'), 0o644)
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
