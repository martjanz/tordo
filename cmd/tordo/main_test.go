package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"tordo/internal/config"
)

func TestSetupCreatesConfigAndScans(t *testing.T) {
	parent := t.TempDir()
	app := filepath.Join(parent, "tordo")
	for _, d := range []string{app, filepath.Join(parent, "musica")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	a, err := setup(app)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	cfg, created, err := config.Load(app)
	if err != nil || created {
		t.Fatalf("config.json not saved: %v, created %v", err, created)
	}
	if !reflect.DeepEqual(cfg.Roots, []string{"../musica"}) {
		t.Fatalf("roots = %q", cfg.Roots)
	}
	if a.scanner == nil || a.server.ReadOnly {
		t.Fatal("expected a writable setup with a scanner")
	}
	if a.server.AppDir != app || a.server.Covers == nil || a.server.Covers.CacheDir != filepath.Join(app, "cache", "thumbs") {
		t.Fatalf("server = %+v", a.server)
	}
	if err := a.scanner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(app, dbName)); err != nil {
		t.Fatalf("catalog file: %v", err)
	}
}
