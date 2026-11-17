package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"tordo/internal/appdir"
)

// Root errors. A *RootError wraps one of them with the folder(s) concerned, so
// the server can answer with a code and the paths as parameters.
var (
	ErrRootRequired    = errors.New("folder is required")
	ErrRootNotFound    = errors.New("folder does not exist")
	ErrRootNotDir      = errors.New("not a folder")
	ErrRootOtherDisk   = errors.New("folder must be on the same disk as the app")
	ErrRootIsApp       = errors.New("that is the app folder")
	ErrRootContainsApp = errors.New("that folder contains the app folder")
	ErrRootInsideApp   = errors.New("that folder is inside the app folder")
	ErrRootsRequired   = errors.New("choose at least one folder")
	ErrRootMalformed   = errors.New("folder is not in catalog form")
	ErrRootDuplicate   = errors.New("repeated folder")
	ErrRootsNested     = errors.New("folders are nested")
)

// RootError is a rejected root: Err says why, Path is the folder (and Other
// the second one when two overlap).
type RootError struct {
	Err         error
	Path, Other string
}

func (e *RootError) Error() string {
	if e.Other != "" {
		return fmt.Sprintf("%v: %s, %s", e.Err, e.Path, e.Other)
	}
	if e.Path != "" {
		return fmt.Sprintf("%v: %s", e.Err, e.Path)
	}
	return e.Err.Error()
}

func (e *RootError) Unwrap() error { return e.Err }

// Root turns a folder the user typed (absolute, or relative to the app
// folder) into a root in catalog form: relative to appDir, '/'-separated.
// The folder must exist, and neither be, contain nor sit inside the app folder.
func Root(appDir, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", &RootError{Err: ErrRootRequired}
	}
	abs := input
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(appDir, filepath.FromSlash(abs))
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", &RootError{Err: ErrRootNotFound, Path: input}
	}
	if !st.IsDir() {
		return "", &RootError{Err: ErrRootNotDir, Path: input}
	}
	abs = canonicalCase(abs)
	rel, err := appdir.Rel(appDir, abs)
	if err != nil {
		// Another drive on Windows: the catalog only keeps relative paths.
		return "", &RootError{Err: ErrRootOtherDisk, Path: input}
	}
	rel = path.Clean(rel)
	switch {
	case rel == ".":
		return "", &RootError{Err: ErrRootIsApp, Path: input}
	case onlyParents(rel):
		return "", &RootError{Err: ErrRootContainsApp, Path: input}
	case rel != ".." && !strings.HasPrefix(rel, "../"):
		return "", &RootError{Err: ErrRootInsideApp, Path: input}
	}
	return rel, nil
}

// canonicalCase rewrites abs component by component to match the real
// on-disk name, so a case-variant spelling ("musica" vs "Musica") always
// resolves to the same catalog path on case-insensitive filesystems
// (Windows, macOS). On case-sensitive filesystems this is a no-op, since
// the exact name always matches.
func canonicalCase(abs string) string {
	vol := filepath.VolumeName(abs)
	rest := strings.TrimPrefix(abs[len(vol):], string(filepath.Separator))
	if rest == "" {
		return abs
	}
	cur := vol + string(filepath.Separator)
	for _, part := range strings.Split(rest, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		cur = filepath.Join(cur, canonicalName(cur, part))
	}
	return cur
}

// canonicalName returns the entry in dir whose name matches name, preferring
// an exact match and falling back to a case-insensitive one. If dir can't be
// read or no entry matches (the caller already Stat'd the full path), name is
// returned unchanged.
func canonicalName(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return name
	}
	for _, e := range entries {
		if e.Name() == name {
			return name
		}
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), name) {
			return e.Name()
		}
	}
	return name
}

// onlyParents reports whether rel is "..", "../.." and so on.
func onlyParents(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		if part != ".." {
			return false
		}
	}
	return true
}

// CheckRoots validates the roots of a settings change. Roots in saved (the
// current configuration) are accepted as they are, even when unavailable (an
// unplugged drive); new ones must pass Root and already be in catalog form.
// There must be at least one, and none may repeat or contain another.
func CheckRoots(appDir string, roots, saved []string) error {
	if len(roots) == 0 {
		return &RootError{Err: ErrRootsRequired}
	}
	for i, r := range roots {
		if !slices.Contains(saved, r) {
			rel, err := Root(appDir, r)
			if err != nil {
				return err
			}
			if rel != r {
				return &RootError{Err: ErrRootMalformed, Path: r}
			}
		}
		for _, o := range roots[:i] {
			switch {
			case o == r:
				return &RootError{Err: ErrRootDuplicate, Path: r}
			case strings.HasPrefix(r, o+"/"), strings.HasPrefix(o, r+"/"):
				return &RootError{Err: ErrRootsNested, Path: o, Other: r}
			}
		}
	}
	return nil
}

// Available reports whether a root in catalog form is a folder right now.
func Available(appDir, root string) bool {
	st, err := os.Stat(appdir.Abs(appDir, root))
	return err == nil && st.IsDir()
}
