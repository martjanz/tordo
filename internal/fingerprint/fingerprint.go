// Package fingerprint identifies the audio content of a file, ignoring its
// tags and embedded pictures, so moved, renamed or re-tagged files keep their
// identity and identical rips are recognized.
package fingerprint

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

const chunk = 64 << 10

// Of fingerprints the payload [offset, offset+length) of the file at path:
// its first and last 64 KiB plus its length. With ogg, page headers are
// skipped (see hashOgg). An empty payload has no fingerprint ("").
func Of(path string, offset, length int64, ogg bool) (string, error) {
	if length <= 0 {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if ogg {
		err = hashOgg(h, f, offset, offset+length)
	} else {
		err = hashRange(h, f, offset, length)
	}
	if err != nil {
		return "", err
	}
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(length))
	h.Write(n[:])
	return hex.EncodeToString(h.Sum(nil))[:32], nil
}

func hashRange(w io.Writer, r io.ReaderAt, off, n int64) error {
	if n <= 2*chunk {
		_, err := io.Copy(w, io.NewSectionReader(r, off, n))
		return err
	}
	if _, err := io.Copy(w, io.NewSectionReader(r, off, chunk)); err != nil {
		return err
	}
	_, err := io.Copy(w, io.NewSectionReader(r, off+n-chunk, chunk))
	return err
}

// hashOgg hashes the data of the pages in [start, end) without their headers
// (rewriting comments renumbers and re-checksums every later page): the
// first 64 KiB of page data, then the last page's granule position and data.
func hashOgg(w io.Writer, r io.ReaderAt, start, end int64) error {
	buf := make([]byte, 27+255)
	left := int64(chunk)
	for off := start; off < end && left > 0; {
		data, n, err := oggPage(r, off, buf)
		if err != nil {
			return err
		}
		take := min(left, n)
		if _, err := io.Copy(w, io.NewSectionReader(r, data, take)); err != nil {
			return err
		}
		left -= take
		off = data + n
	}
	tail := min(end-start, 64<<10)
	b := make([]byte, tail)
	if _, err := r.ReadAt(b, end-tail); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	for i := len(b) - 27; i >= 0; i-- {
		if string(b[i:i+4]) != "OggS" {
			continue
		}
		data, n, err := oggPage(r, end-tail+int64(i), buf)
		if err != nil || data+n > end {
			continue // "OggS" inside page data
		}
		w.Write(b[i+6 : i+14]) // granule position
		_, err = io.Copy(w, io.NewSectionReader(r, data, n))
		return err
	}
	return nil
}

// oggPage reads the page header at off and returns where its data starts and
// how long it is.
func oggPage(r io.ReaderAt, off int64, buf []byte) (data, n int64, err error) {
	if _, err := r.ReadAt(buf[:27], off); err != nil {
		return 0, 0, err
	}
	if string(buf[:4]) != "OggS" {
		return 0, 0, errors.New("página Ogg inválida")
	}
	segs := int(buf[26])
	if _, err := r.ReadAt(buf[27:27+segs], off+27); err != nil {
		return 0, 0, err
	}
	for _, l := range buf[27 : 27+segs] {
		n += int64(l)
	}
	return off + 27 + int64(segs), n, nil
}
