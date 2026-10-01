package covers

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"tordo/internal/audiofile"
	"tordo/internal/grouping"
	"tordo/internal/store"
)

// pngImage is a w×h picture, red on the left half and blue on the right.
func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 0, 0, 255}
			if x >= w/2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// mp3With is an ID3v2.3 tag with one APIC frame (data may be empty), then
// twenty silent MPEG-1 Layer III frames (128 kbps, 44.1 kHz: 417 bytes each).
func mp3With(data []byte) []byte {
	frame := append([]byte{0}, []byte("image/png\x00")...)
	frame = append(frame, 3, 0)
	frame = append(frame, data...)
	var body []byte
	body = append(body, "APIC"...)
	body = binary.BigEndian.AppendUint32(body, uint32(len(frame)))
	body = append(body, 0, 0)
	body = append(body, frame...)
	n := len(body)
	tag := []byte{'I', 'D', '3', 3, 0, 0, byte(n>>21) & 0x7F, byte(n>>14) & 0x7F, byte(n>>7) & 0x7F, byte(n) & 0x7F}
	out := append(tag, body...)
	for i := 0; i < 20; i++ {
		out = append(out, 0xFF, 0xFB, 0x90, 0x00)
		out = append(out, make([]byte, 413)...)
	}
	return out
}

type fixture struct {
	covers *Covers
	store  *store.Store
	music  string // the music folder, absolute
}

// setup writes files under <tmp>/musica and catalogs them with the given
// tags, from an app folder at <tmp>/tordo.
func setup(t *testing.T, files map[string][]byte, tags map[string]audiofile.Tags) fixture {
	t.Helper()
	base := t.TempDir()
	app, music := filepath.Join(base, "tordo"), filepath.Join(base, "musica")
	var rows []store.FileRow
	for name, data := range files {
		p := filepath.Join(music, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		kind := "audio"
		if filepath.Ext(name) == ".jpg" || filepath.Ext(name) == ".png" {
			kind = "image"
		}
		rows = append(rows, store.FileRow{Path: "../musica/" + name, Size: int64(len(data)), MTime: 1, Kind: kind})
	}
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	roots := []string{"../musica"}
	if err := st.SyncFiles(rows, roots); err != nil {
		t.Fatal(err)
	}
	env := store.ReadEnv{Version: 1}
	pending, _ := st.PendingReads(env)
	var rs []store.ReadResult
	for _, p := range pending {
		rs = append(rs, store.ReadResult{Target: p, Env: env, Fingerprint: p.Path,
			Info: audiofile.Info{Format: "mp3", Codec: "mp3", DurationMs: 1000, Tags: tags[p.Path[len("../musica/"):]]}})
	}
	if err := st.SaveReads(rs); err != nil {
		t.Fatal(err)
	}
	tracks, images, _ := st.GroupingInput()
	if err := st.ReplaceGrouping(grouping.Build(tracks, images, roots)); err != nil {
		t.Fatal(err)
	}
	return fixture{New(st, app, filepath.Join(app, "cache", "thumbs")), st, music}
}

func (f fixture) album(t *testing.T, title string) int64 {
	t.Helper()
	as, _ := f.store.Albums("")
	for _, a := range as {
		if a.Title == title {
			return a.ID
		}
	}
	t.Fatalf("no album %q", title)
	return 0
}

func get(serve func(http.ResponseWriter, *http.Request, int64), id int64, query string, header ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/covers/x/"+strconv.FormatInt(id, 10)+query, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	serve(rec, req, id)
	return rec
}

func TestAlbumCover(t *testing.T) {
	folderImage, embedded := pngImage(t, 800, 400), pngImage(t, 10, 10)
	tag := func(album string) audiofile.Tags {
		return audiofile.Tags{Title: "t", Artist: "a", Album: album, HasCover: true}
	}
	f := setup(t, map[string][]byte{
		"Con tapa/01.mp3":    mp3With(embedded),
		"Con tapa/02.mp3":    mp3With(embedded),
		"Con tapa/cover.png": folderImage,
		"Embebida/01.mp3":    mp3With(nil), // claims a picture, holds an empty one
		"Embebida/02.mp3":    mp3With(embedded),
		"Sin tapa/01.mp3":    mp3With(nil),
		"Sin tapa/02.mp3":    mp3With(nil),
		"Suelto/youtube.mp3": mp3With(embedded),
	}, map[string]audiofile.Tags{
		"Con tapa/01.mp3": tag("Con tapa"), "Con tapa/02.mp3": tag("Con tapa"),
		"Embebida/01.mp3": tag("Embebida"), "Embebida/02.mp3": tag("Embebida"),
		"Sin tapa/01.mp3": tag("Sin tapa"), "Sin tapa/02.mp3": {Title: "u", Album: "Sin tapa"},
		"Suelto/youtube.mp3": {Title: "y", HasCover: true},
	})

	rec := get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), folderImage) || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("folder image first: %d %s", rec.Code, rec.Header())
	}
	tag1 := rec.Header().Get("ETag")
	if rec := get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "", "If-None-Match", tag1); rec.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d", rec.Code)
	}

	rec = get(f.covers.ServeAlbum, f.album(t, "Con tapa"), "?size=320")
	img, err := jpeg.Decode(rec.Body)
	if err != nil || img.Bounds().Dx() != 320 || img.Bounds().Dy() != 160 || rec.Header().Get("ETag") == tag1 {
		t.Fatalf("thumbnail: %v %v, etag %q", img.Bounds(), err, rec.Header().Get("ETag"))
	}
	if r, _, b, _ := img.At(10, 80).RGBA(); r < 0xE000 || b > 0x2000 {
		t.Fatalf("left half is not red: %v", img.At(10, 80))
	}
	cached, _ := filepath.Glob(filepath.Join(f.covers.CacheDir, "*.jpg"))
	if len(cached) != 1 {
		t.Fatalf("cache = %q", cached)
	}

	rec = get(f.covers.ServeAlbum, f.album(t, "Embebida"), "")
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), embedded) {
		t.Fatalf("second file stands in for an empty picture: %d", rec.Code)
	}
	if rec := get(f.covers.ServeAlbum, f.album(t, "Sin tapa"), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("no picture: %d", rec.Code)
	}
	if rec := get(f.covers.ServeAlbum, 9999, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown album: %d", rec.Code)
	}

	items, _ := f.store.PlayFolder("../musica/Suelto")
	rec = get(f.covers.ServeTrack, items[0].ID, "?size=999") // not a thumbnail size: the original
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), embedded) {
		t.Fatalf("track picture: %d", rec.Code)
	}
}

func TestBrowseOnlyKeepsNoThumbnails(t *testing.T) {
	f := setup(t, map[string][]byte{"A/cover.png": pngImage(t, 400, 400), "A/01.mp3": []byte("x"), "A/02.mp3": []byte("x")},
		map[string]audiofile.Tags{"A/01.mp3": {Album: "A"}, "A/02.mp3": {Album: "A"}})
	f.covers.CacheDir = ""
	rec := get(f.covers.ServeAlbum, f.album(t, "A"), "?size=160")
	if img, err := jpeg.Decode(rec.Body); err != nil || img.Bounds().Dx() != 160 {
		t.Fatalf("thumbnail: %v", err)
	}
}

func TestShrinkKeepsSmallImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	if shrink(img, 160) != image.Image(img) {
		t.Fatal("a small image was scaled")
	}
	if b := shrink(image.NewRGBA(image.Rect(0, 0, 50, 1000)), 160).Bounds(); b.Dx() != 8 || b.Dy() != 160 {
		t.Fatalf("tall image: %v", b)
	}
}

func TestShrinkRefusesHugePictures(t *testing.T) {
	data := pngImage(t, 4, 4)
	// Claim 10000×10000 in the IHDR chunk (bytes 16–24) and fix its CRC.
	binary.BigEndian.PutUint32(data[16:], 10000)
	binary.BigEndian.PutUint32(data[20:], 10000)
	binary.BigEndian.PutUint32(data[29:], crc32.ChecksumIEEE(data[12:29]))
	if _, err := shrinkJPEG(data, 160); !errors.Is(err, errTooLarge) {
		t.Fatalf("a picture of 100 megapixels: %v", err)
	}
	if _, err := shrinkJPEG(pngImage(t, 8, 8), 160); err != nil {
		t.Fatalf("a small picture: %v", err)
	}
}
