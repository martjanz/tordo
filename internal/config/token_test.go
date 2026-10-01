package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscogsToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(`{"roots": ["../musica"], "discogsToken": "abc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(dir)
	if err != nil || cfg.DiscogsToken != "abc" {
		t.Fatalf("token %q, err %v", cfg.DiscogsToken, err)
	}
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := Load(dir); again.DiscogsToken != "abc" {
		t.Fatal("token lost on save")
	}
}
