package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreloadReadsTheWholeCatalogFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "tordo.db")
	s, err := Open(file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.Preload()
	if err != nil {
		t.Fatal(err)
	}
	if n != before.Size() {
		t.Fatalf("read %d bytes, the file has %d", n, before.Size())
	}
	after, _ := os.Stat(file)
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatal("Preload changed the file")
	}
}

func TestPreloadOfAnInMemoryCatalogReadsNothing(t *testing.T) {
	s, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.Preload(); n != 0 || err != nil {
		t.Fatalf("Preload = %d, %v", n, err)
	}
}
