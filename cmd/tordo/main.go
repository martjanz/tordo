// Command tordo catalogs a music library and serves the catalog on localhost.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"tordo/internal/appdir"
	"tordo/internal/config"
	"tordo/internal/discogs"
	"tordo/internal/identify"
	"tordo/internal/musicbrainz"
	"tordo/internal/scan"
	"tordo/internal/server"
	"tordo/internal/store"
)

const dbName = "tordo.db"

func main() {
	dir := flag.String("dir", os.Getenv("TORDO_DIR"), "carpeta de la app (por defecto, la del ejecutable)")
	port := flag.Int("port", 0, "puerto HTTP (0 = uno libre)")
	flag.Parse()
	if err := run(*dir, *port); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	server  *server.Server
	scanner *scan.Scanner    // nil in browse-only mode
	runner  *identify.Runner // nil in browse-only mode
	close   func() error
}

func run(dirOverride string, port int) error {
	a, err := setup(dirOverride)
	if err != nil {
		return err
	}
	defer a.close()
	if a.scanner != nil {
		go func() {
			if err := a.scanner.Run(context.Background()); err != nil {
				log.Printf("escaneo: %v", err)
			}
		}()
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	log.Printf("Tordo en http://%s/api/status (Ctrl+C para salir)", ln.Addr())
	return http.Serve(ln, a.server.Handler())
}

// setup resolves the app folder, loads (or creates) config.json and opens the
// catalog. A folder that is not writable gives browse-only mode: no scanner.
func setup(dirOverride string) (*app, error) {
	appDir, err := appdir.Resolve(dirOverride)
	if err != nil {
		return nil, err
	}
	readOnly := !appdir.Writable(appDir)
	cfg, created, err := config.Load(appDir)
	if err != nil {
		return nil, err
	}
	if created && !readOnly {
		if err := config.Save(appDir, cfg); err != nil {
			return nil, err
		}
		log.Printf("se creó %s con las carpetas %q; editalo para cambiarlas", config.FileName, cfg.Roots)
	}
	st, err := openStore(appDir, readOnly)
	if err != nil {
		return nil, err
	}
	a := &app{close: st.Close}
	if readOnly {
		log.Print("la carpeta de la app no es escribible: modo consulta")
	} else {
		roots := make([]string, len(cfg.Roots))
		for i, r := range cfg.Roots {
			roots[i] = path.Clean(r)
		}
		if cfg.DiscogsToken == "" {
			log.Print("sin token de Discogs: los géneros salen de MusicBrainz, de sus enlaces a Discogs y de los tags")
		}
		runner := &identify.Runner{AppDir: appDir, Store: st, MB: musicbrainz.New(), Discogs: discogs.New(cfg.DiscogsToken)}
		a.runner = runner
		a.scanner = &scan.Scanner{AppDir: appDir, Roots: roots, Ignore: cfg.Ignore, Store: st, OnDone: runner.Trigger}
		a.close = func() error {
			runner.Close()
			return st.Close()
		}
	}
	a.server = &server.Server{Store: st, Scanner: a.scanner, ReadOnly: readOnly, AppDir: appDir}
	if a.runner != nil { // only when set: a nil *Runner in the interface would not read as "none"
		a.server.Identifier = a.runner
	}
	return a, nil
}

// openStore opens the catalog. Browse-only mode with no catalog file yet
// falls back to an empty in-memory one.
func openStore(appDir string, readOnly bool) (*store.Store, error) {
	p := filepath.Join(appDir, dbName)
	if readOnly {
		if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
			return store.OpenMemory()
		}
	}
	return store.Open(p, readOnly)
}
