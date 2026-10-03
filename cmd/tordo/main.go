// Command tordo catalogs a music library and serves it, to browse and
// listen, on localhost: it opens the browser at start.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"tordo/internal/appdir"
	"tordo/internal/config"
	"tordo/internal/covers"
	"tordo/internal/engine"
	"tordo/internal/loudness"
	"tordo/internal/musicbrainz"
	"tordo/internal/platform"
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
	server *server.Server
	engine *engine.Engine
	loud   *loudness.Runner // nil in browse-only mode or without ffmpeg
	close  func() error
}

func run(dirOverride string, port int, browser bool) error {
	a, err := setup(dirOverride)
	if err != nil {
		return err
	}
	defer a.close()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	log.Printf("Tordo en %s (Ctrl+C para salir)", url)
	if urls := a.server.LANURLs(); len(urls) > 0 {
		log.Printf("Desde la red: %s", urls[0])
	}
	if browser {
		if err := platform.Open(url); err != nil {
			log.Printf("no se pudo abrir el navegador: %v", err)
		}
	}
	return http.Serve(ln, a.server.Handler())
}

// setup resolves the app folder, loads config.json and opens the catalog,
// then starts the engine, which builds the scanner and the identification
// runner from the settings. Without a config.json nothing is scanned until
// the assistant saves one. A folder that is not writable gives browse-only
// mode: no scanner, no settings changes.
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
	st, err := openStore(appDir, readOnly)
	if err != nil {
		return nil, err
	}
	a := &app{}
	ffmpeg := stream.LookFFmpeg()
	a.server = &server.Server{Store: st, ReadOnly: readOnly, AppDir: appDir}
	eng := &engine.Engine{AppDir: appDir, Store: st, ReadOnly: readOnly, MB: musicbrainz.New()}
	if readOnly {
		log.Print("la carpeta de la app no es escribible: modo consulta")
	} else {
		if ffmpeg != "" {
			a.loud = &loudness.Runner{Catalog: st, AppDir: appDir, FFmpeg: ffmpeg}
			a.server.Loudness = a.loud // never a nil *Runner in the interfaces
			eng.Loudness = a.loud
		}
		eng.LAN = a.server.ApplyLAN
	}
	a.engine = eng
	a.server.Engine = eng
	a.close = func() error {
		eng.Close()
		if a.loud != nil {
			a.loud.Close()
		}
		return st.Close()
	}
	pending := created && !readOnly
	if pending {
		log.Print("Primera vez: elegí las carpetas en el navegador (asistente).")
	}
	thumbs := "" // browse-only mode writes nothing, thumbnails included
	if !readOnly {
		thumbs = filepath.Join(appDir, "cache", "thumbs")
	}
	if ffmpeg == "" {
		log.Print("sin ffmpeg: WMA, APE, WavPack, AIFF y ALAC no se pueden reproducir")
	}
	a.server.Streamer = stream.Streamer{FFmpeg: ffmpeg}
	a.server.Covers = covers.New(st, appDir, thumbs)
	eng.Start(cfg, pending)
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
