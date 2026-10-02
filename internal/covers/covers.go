// Package covers serves album and track pictures: the album folder's image,
// a picture embedded in its files or the Cover Art Archive image the
// identification downloaded, whole or shrunk to a thumbnail. It never
// writes to the music folders: thumbnails are kept in the app's cache
// folder.
package covers

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tordo/internal/appdir"
	"tordo/internal/audiofile"
	"tordo/internal/store"
)

// Sizes are the thumbnail sizes asked for with ?size= (pixels of the longer
// side); any other size gets the original picture.
var Sizes = []int{160, 320, 640}

// maxCandidates caps the files tried for an album: some files claim a
// picture but hold an empty one.
const maxCandidates = 3

// maxImage caps a folder image: anything larger is not a cover.
const maxImage = 32 << 20

var errNone = errors.New("sin imagen")

type Covers struct {
	Store    *store.Store
	AppDir   string
	CacheDir string // where thumbnails go; "" keeps none (browse-only mode)
	sem      chan struct{}
}

// New returns the cover server. Decoding large pictures takes memory, so at
// most a few thumbnails are made at once.
func New(st *store.Store, appDir, cacheDir string) *Covers {
	return &Covers{Store: st, AppDir: appDir, CacheDir: cacheDir, sem: make(chan struct{}, 2)}
}

// source is a place a picture can come from.
type source struct {
	path     string // absolute
	embedded bool   // a picture inside an audio file; else an image file
}

func (c *Covers) albumSources(id int64) ([]source, bool, error) {
	files, ok, err := c.Store.AlbumCover(id)
	if err != nil || !ok {
		return nil, ok, err
	}
	var out []source
	if files.Folder != "" {
		out = append(out, source{appdir.Abs(c.AppDir, files.Folder), false})
	}
	embedded, err := c.Store.AlbumPictureFiles(id)
	if err != nil {
		return nil, true, err
	}
	for _, f := range embedded[:min(len(embedded), maxCandidates)] {
		out = append(out, source{appdir.Abs(c.AppDir, f), true})
	}
	if files.Cache != "" {
		out = append(out, source{appdir.Abs(c.AppDir, files.Cache), false})
	}
	return out, true, nil
}

func (c *Covers) trackSources(id int64) ([]source, bool, error) {
	f, ok, err := c.Store.TrackFile(id)
	if err != nil || !ok {
		return nil, ok, err
	}
	return []source{{appdir.Abs(c.AppDir, f.Path), true}}, true, nil
}

// ServeAlbum answers GET /api/covers/album/{id}[?size=N].
func (c *Covers) ServeAlbum(w http.ResponseWriter, r *http.Request, id int64) {
	srcs, ok, err := c.albumSources(id)
	c.serve(w, r, srcs, ok, err)
}

// ServeTrack answers GET /api/covers/track/{id}[?size=N]: the picture
// embedded in the track's own file.
func (c *Covers) ServeTrack(w http.ResponseWriter, r *http.Request, id int64) {
	srcs, ok, err := c.trackSources(id)
	c.serve(w, r, srcs, ok, err)
}

func (c *Covers) serve(w http.ResponseWriter, r *http.Request, srcs []source, ok bool, err error) {
	if err != nil {
		log.Printf("carátula: %v", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	if !ok || len(srcs) == 0 {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	size := thumbSize(r.URL.Query().Get("size"))
	tag := etag(srcs, size)
	// The URL never changes for an album: the browser revalidates it, and a
	// matching tag saves reading the picture again.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", tag)
	if r.Header.Get("If-None-Match") == tag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	data, mime, err := c.picture(srcs, size, tag)
	if errors.Is(err, errNone) {
		http.Error(w, "sin carátula", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("carátula: %v", err)
		http.Error(w, "error interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

func thumbSize(s string) int {
	n, _ := strconv.Atoi(s)
	for _, z := range Sizes {
		if n == z {
			return n
		}
	}
	return 0
}

// etag identifies what a request would get: the candidate files as they
// are now, and the size.
func etag(srcs []source, size int) string {
	h := sha1.New()
	for _, s := range srcs {
		fmt.Fprintf(h, "%s|%v", s.path, s.embedded)
		if st, err := os.Stat(s.path); err == nil {
			fmt.Fprintf(h, "|%d|%d", st.Size(), st.ModTime().UnixNano())
		}
		h.Write([]byte{0})
	}
	fmt.Fprintf(h, "size=%d", size)
	return `"` + hex.EncodeToString(h.Sum(nil))[:20] + `"`
}

// picture returns the first picture the sources give, shrunk to size (when
// it is larger), from the thumbnail cache when it is there.
func (c *Covers) picture(srcs []source, size int, tag string) ([]byte, string, error) {
	var cached string
	if size > 0 && c.CacheDir != "" {
		cached = filepath.Join(c.CacheDir, strings.Trim(tag, `"`)+".jpg")
		if b, err := os.ReadFile(cached); err == nil {
			return b, "image/jpeg", nil
		}
	}
	data, mime, err := original(srcs)
	if err != nil || size == 0 {
		return data, mime, err
	}
	thumb, err := c.shrinkLimited(data, size)
	if err != nil {
		return data, mime, nil // a format the standard library does not decode (WebP…): send it whole
	}
	if cached != "" {
		if err := writeAtomic(cached, thumb); err != nil {
			log.Printf("miniatura: %v", err)
		}
	}
	return thumb, "image/jpeg", nil
}

func original(srcs []source) ([]byte, string, error) {
	for _, s := range srcs {
		if s.embedded {
			data, mime, err := audiofile.Picture(s.path)
			if err == nil {
				return data, mime, nil
			}
			if !errors.Is(err, audiofile.ErrNoPicture) {
				log.Printf("carátula de %s: %v", s.path, err)
			}
			continue
		}
		st, err := os.Stat(s.path)
		if err != nil || st.Size() > maxImage {
			continue
		}
		data, err := os.ReadFile(s.path)
		if err != nil {
			continue
		}
		mime := http.DetectContentType(data)
		if strings.HasPrefix(mime, "image/") {
			return data, mime, nil
		}
	}
	return nil, "", errNone
}

// shrinkLimited runs shrinkJPEG holding one slot of the decode semaphore.
func (c *Covers) shrinkLimited(data []byte, size int) ([]byte, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()
	return shrinkJPEG(data, size)
}

// maxPixels is the largest picture worth decoding for a thumbnail: decoding
// takes about four bytes per pixel.
const maxPixels = 40_000_000

var errTooLarge = errors.New("picture too large to decode")

// shrinkJPEG decodes a picture and, when its longer side exceeds size,
// scales it down and encodes it as JPEG.
func shrinkJPEG(data []byte, size int) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, errTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, shrink(img, size), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeAtomic(name string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".thumb-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), name)
}
