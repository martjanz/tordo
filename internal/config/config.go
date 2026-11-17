// Package config reads and writes config.json in the app folder.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const FileName = "config.json"

type Config struct {
	Roots        []string `json:"roots"`               // catalog form: relative to the app folder, '/'-separated
	Ignore       []string `json:"ignore"`              // extra folder patterns to skip (see mediafile.IgnoredDir)
	DiscogsToken string   `json:"discogsToken"`        // optional: enables Discogs search
	Normalize    *bool    `json:"normalize,omitempty"` // volume normalization; nil (absent): on
	LAN          LAN      `json:"lan"`
	ListenMode   bool     `json:"listenMode"` // listen mode: the server refuses curation (scan, identify, collections, genre aliases)
	// RescanMinutes is how often the folders are checked for changes; nil
	// (absent): DefaultRescanMinutes, 0: never.
	RescanMinutes *int `json:"rescanMinutes,omitempty"`
	// Language is the interface language of the whole installation; "" (absent):
	// follow the browser's. See LanguageOrAuto.
	Language string `json:"language,omitempty"`
}

// Languages are the interface languages, as saved in config.json.
var Languages = []string{"es", "en", "pt-BR"}

// LanguageOrAuto is the configured language in its canonical form, or "" when
// none is set (or it is not one of Languages): the browser's language decides.
func (c Config) LanguageOrAuto() string {
	for _, l := range Languages {
		if strings.EqualFold(c.Language, l) {
			return l
		}
	}
	return ""
}

// DefaultRescanMinutes is the rescan interval when config.json does not say.
const DefaultRescanMinutes = 60

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

// RescanEvery is how often the folders are checked for changes; 0 means never.
func (c Config) RescanEvery() time.Duration {
	if c.RescanMinutes == nil {
		return DefaultRescanMinutes * time.Minute
	}
	return time.Duration(max(*c.RescanMinutes, 0)) * time.Minute
}

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
	// Language is read apart so a value of the wrong type never fails the load.
	type plain Config
	aux := struct {
		*plain
		Language json.RawMessage `json:"language"`
	}{plain: (*plain)(&cfg)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return Config{}, false, fmt.Errorf("%s: %w", FileName, err)
	}
	_ = json.Unmarshal(aux.Language, &cfg.Language) // not a string: stays ""
	if cfg.Language != "" && cfg.LanguageOrAuto() == "" {
		log.Printf("unknown language %q in config.json: using the browser's language", cfg.Language)
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
