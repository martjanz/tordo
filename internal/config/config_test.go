package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func appUnder(t *testing.T, siblings ...string) string {
	t.Helper()
	parent := t.TempDir()
	app := filepath.Join(parent, "tordo")
	for _, d := range append([]string{"tordo"}, siblings...) {
		if err := os.MkdirAll(filepath.Join(parent, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return app
}

func TestLoadMissingGivesDefaults(t *testing.T) {
	app := appUnder(t, "musica", "musica-ordenar", ".Trashes", "$RECYCLE.BIN", "System Volume Information")
	cfg, created, err := Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("created = false for a missing config.json")
	}
	want := []string{"../musica", "../musica-ordenar"}
	if !reflect.DeepEqual(cfg.Roots, want) {
		t.Fatalf("Roots = %q, want %q", cfg.Roots, want)
	}
	if cfg.Ignore == nil {
		t.Fatal("Ignore must be an empty slice, not nil")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	app := appUnder(t)
	in := Config{Roots: []string{"../musica"}, Ignore: []string{"Soulseek/incomplete"}}
	if err := Save(app, in); err != nil {
		t.Fatal(err)
	}
	out, created, err := Load(app)
	if err != nil || created {
		t.Fatalf("Load = %v, created %v", err, created)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v, want %+v", out, in)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	app := appUnder(t)
	if err := os.WriteFile(filepath.Join(app, FileName), []byte("{roots"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(app); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestLoadNullListsBecomeEmpty(t *testing.T) {
	app := appUnder(t)
	if err := os.WriteFile(filepath.Join(app, FileName), []byte(`{"roots":["../m"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(app)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Ignore == nil {
		t.Fatal("Ignore must default to an empty slice")
	}
}
