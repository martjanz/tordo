// Package startcache keeps a copy of the last catalog snapshot and the
// user's data in the app folder, so a start can show the catalog before
// reading the store: one sequential read of a compact file instead of the
// store's pages scattered through a large file, minutes on a hard disk.
package startcache

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"tordo/internal/store"
)

// version changes when Data's meaning changes; a cache of another version
// is not loaded (the next build writes a new one).
const version = 1

// Data is what the cache holds.
type Data struct {
	Snapshot store.Snapshot
	Personal store.Personal
	Counts   store.Counts // the status's figures: counting reads the whole store
}

// Save writes d to file, replacing it only once fully written.
func Save(file string, d Data) error { return save(file, version, d) }

func save(file string, v int, d Data) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), filepath.Base(file)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // nothing once renamed
	zw, err := gzip.NewWriterLevel(tmp, gzip.BestSpeed)
	if err != nil {
		tmp.Close()
		return err
	}
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if err := enc.Encode(d); err != nil {
		tmp.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return rename(tmp.Name(), file)
}

// rename replaces file with tmp. Windows refuses while someone reads file
// (a start loading the cache): it tries again for a while.
func rename(tmp, file string) error {
	var err error
	for range renameTries {
		if err = os.Rename(tmp, file); err == nil {
			return nil
		}
		time.Sleep(renameWait)
	}
	return err
}

const (
	renameTries = 50
	renameWait  = 20 * time.Millisecond
)

// Load reads the cache; an error wrapping fs.ErrNotExist when there is none.
func Load(file string) (Data, error) {
	var d Data
	f, err := os.Open(file)
	if err != nil {
		return d, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return d, fmt.Errorf("caché del catálogo ilegible: %w", err)
	}
	dec := gob.NewDecoder(zr)
	var v int
	if err := dec.Decode(&v); err != nil {
		return d, fmt.Errorf("caché del catálogo ilegible: %w", err)
	}
	if v != version {
		return d, fmt.Errorf("caché del catálogo de otra versión (%d)", v)
	}
	if err := dec.Decode(&d); err != nil {
		return Data{}, fmt.Errorf("caché del catálogo ilegible: %w", err)
	}
	if _, err := io.Copy(io.Discard, zr); err != nil {
		return Data{}, fmt.Errorf("caché del catálogo ilegible: %w", err)
	}
	return d, nil
}
