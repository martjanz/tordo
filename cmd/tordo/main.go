// Command tordo catalogs a music library and serves it, to browse and
// listen, on localhost: it opens the browser at start.
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
	"tordo/internal/covers"
	"tordo/internal/platform"
	"tordo/internal/scan"
	"tordo/internal/server"
	"tordo/internal/store"
	"tordo/internal/stream"
)

const dbName = "tordo.db"

func main() {
	dir := flag.String("dir", os.Getenv("TORDO_DIR"), "carpeta de la app (por defecto, la del ejecutable)")
	port := flag.Int("port", 0, "puerto HTTP (0 = uno libre)")
	noBrowser := flag.Bool("no-browser", false, "no abrir el navegador")
	flag.Parse()
	if err := run(*dir, *port, !*noBrowser); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	server  *server.Server
	scanner *scan.Scanner // nil in browse-only mode
	close   func() error
}

func run(dirOverride string, port int, browser bool) error {
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
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Tordo en %s (Ctrl+C para salir)", url)
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
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
		a.scanner = &scan.Scanner{AppDir: appDir, Roots: roots, Ignore: cfg.Ignore, Store: st}
	}
	thumbs := "" // browse-only mode writes nothing, thumbnails included
	if !readOnly {
		thumbs = filepath.Join(appDir, "cache", "thumbs")
	}
	ffmpeg := stream.LookFFmpeg()
	if ffmpeg == "" {
		log.Print("sin ffmpeg: WMA, APE, WavPack, AIFF y ALAC no se pueden reproducir")
	}
	a.server = &server.Server{Store: st, Scanner: a.scanner, AppDir: appDir, Streamer: stream.Streamer{FFmpeg: ffmpeg},
		Covers: covers.New(st, appDir, thumbs), ReadOnly: readOnly}
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
