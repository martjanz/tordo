package fingerprint

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func pattern(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7) + seed
	}
	return b
}

func TestSameAudioDifferentTags(t *testing.T) {
	audio := pattern(300<<10, 1)
	a := append([]byte("ID3 short tag"), audio...)
	b := append([]byte("ID3 a much longer tag with a cover"), audio...)
	fa, err := Of(write(t, a), 13, int64(len(audio)), false)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := Of(write(t, b), 34, int64(len(audio)), false)
	if err != nil {
		t.Fatal(err)
	}
	if fa != fb || len(fa) != 32 {
		t.Fatalf("fingerprints differ: %s / %s", fa, fb)
	}
}

func TestDifferentAudio(t *testing.T) {
	a, b := pattern(300<<10, 1), pattern(300<<10, 2)
	fa, _ := Of(write(t, a), 0, int64(len(a)), false)
	fb, _ := Of(write(t, b), 0, int64(len(b)), false)
	if fa == fb {
		t.Fatal("different audio, same fingerprint")
	}
	short := pattern(1000, 1)
	fs, _ := Of(write(t, short), 0, 1000, false)
	fs2, _ := Of(write(t, short), 0, 999, false)
	if fs == fs2 {
		t.Fatal("the length must be part of the fingerprint")
	}
}

func TestEmptyPayload(t *testing.T) {
	f, err := Of(write(t, []byte("x")), 0, 0, false)
	if err != nil || f != "" {
		t.Fatalf("got %q, %v", f, err)
	}
}

// page builds an Ogg page holding one packet (< 255*255 bytes).
func page(seq uint32, granule int64, packet []byte) []byte {
	var lacing []byte
	n := len(packet)
	for ; n >= 255; n -= 255 {
		lacing = append(lacing, 255)
	}
	lacing = append(lacing, byte(n))
	h := make([]byte, 27)
	copy(h, "OggS")
	binary.LittleEndian.PutUint64(h[6:], uint64(granule))
	binary.LittleEndian.PutUint32(h[14:], 7)
	binary.LittleEndian.PutUint32(h[18:], seq)
	binary.LittleEndian.PutUint32(h[22:], seq*31) // a "CRC" that changes with the page
	h[26] = byte(len(lacing))
	return bytes.Join([][]byte{h, lacing, packet}, nil)
}

func TestOggIgnoresPageRenumbering(t *testing.T) {
	audio := [][]byte{pattern(4000, 1), pattern(4000, 2), pattern(4000, 3)}
	build := func(commentPages int) ([]byte, int64) {
		var head []byte
		for i := 0; i < commentPages; i++ {
			head = append(head, page(uint32(i), 0, []byte("comments"))...)
		}
		file := append([]byte{}, head...)
		for i, a := range audio {
			file = append(file, page(uint32(commentPages+i), int64(48000*(i+1)), a)...)
		}
		return file, int64(len(head))
	}
	a, offA := build(2)
	b, offB := build(5)
	fa, err := Of(write(t, a), offA, int64(len(a))-offA, true)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := Of(write(t, b), offB, int64(len(b))-offB, true)
	if err != nil {
		t.Fatal(err)
	}
	if fa != fb {
		t.Fatalf("renumbered pages changed the fingerprint: %s / %s", fa, fb)
	}
	audio[2] = pattern(4000, 9)
	c, offC := build(2)
	fc, _ := Of(write(t, c), offC, int64(len(c))-offC, true)
	if fc == fa {
		t.Fatal("different audio, same Ogg fingerprint")
	}
}
