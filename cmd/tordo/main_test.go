package main

import (
	"os"
	"path/filepath"
	"testing"

	"tordo/internal/config"
)

func appFolders(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	app := filepath.Join(parent, "tordo")
	music := filepath.Join(parent, "musica")
	for _, d := range []string{app, music} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return app, music
}

func TestSetupFirstUseIsPending(t *testing.T) {
	app, music := appFolders(t)
	if err := os.WriteFile(filepath.Join(music, "a.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := setup(app)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	if !a.engine.SetupPending() || a.server.Engine != a.engine {
		t.Fatal("expected the first-use assistant")
	}
	if _, err := os.Stat(filepath.Join(app, config.FileName)); !os.IsNotExist(err) {
		t.Fatalf("config.json must not be written: %v", err)
	}
	st, err := a.server.Store.Counts()
	if err != nil {
		t.Fatal(err)
	}
	if st.Files != 0 {
		t.Fatalf("nothing should be scanned: %+v", st)
	}
	if a.server.Covers == nil || a.server.Covers.CacheDir != filepath.Join(app, "cache", "thumbs") || a.server.AppDir != app {
		t.Fatalf("server = %+v", a.server)
	}
	if a.engine.LAN == nil || a.engine.Current().Identifier == nil {
		t.Fatal("expected a writable engine")
	}
}

func TestSetupWithConfigIsNotPending(t *testing.T) {
	app, _ := appFolders(t)
	if err := config.Save(app, config.Config{Roots: []string{"../musica"}}); err != nil {
		t.Fatal(err)
	}
	a, err := setup(app)
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	if a.engine.SetupPending() {
		t.Fatal("an existing config.json never shows the assistant")
	}
	if a.engine.Current().Scanner == nil {
		t.Fatal("expected a scanner")
	}
}
