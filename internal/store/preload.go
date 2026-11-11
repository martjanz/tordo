package store

import (
	"io"
	"os"
)

// Preload reads the catalog file once, start to end, and returns how many
// bytes it read (0 for an in-memory catalog). A catalog that grew over many
// scans has its tables' pages interleaved through the file, so reading one
// table in order jumps all over it: on a hard disk with a cold cache that is
// tens of thousands of seeks, minutes for a large collection. One sequential
// read leaves the file in the system's cache for the snapshot that follows.
// It only reads.
func (s *Store) Preload() (int64, error) {
	if s.file == "" {
		return 0, nil
	}
	f, err := os.Open(s.file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.CopyBuffer(io.Discard, f, make([]byte, 1<<20))
}
