// Package appdir resolves the application directory and converts between
// absolute paths and the portable, app-relative form stored in the catalog.
package appdir

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Resolve returns override (made absolute) if set, otherwise the directory
// holding the running executable.
func Resolve(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// Rel converts an absolute path into catalog form: relative to appDir and
// '/'-separated, so the same catalog works on every OS and mount point.
func Rel(appDir, abs string) (string, error) {
	r, err := filepath.Rel(appDir, plain(abs))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(r), nil
}

// Abs converts a catalog path back into an absolute OS path.
func Abs(appDir, rel string) string {
	p := filepath.Join(appDir, filepath.FromSlash(rel))
	if trimmedByWindows(p) {
		return Verbatim(p)
	}
	return p
}

// Verbatim returns an absolute path in the form Windows takes literally
// (\\?\...). In the usual form it drops the spaces and dots that end a name,
// so it cannot find a folder called like that (one copied from another
// system). Elsewhere the path stays as it is.
func Verbatim(abs string) string {
	if runtime.GOOS != "windows" || strings.HasPrefix(abs, verbatimPrefix) {
		return abs
	}
	if share, ok := strings.CutPrefix(abs, `\\`); ok {
		return verbatimShare + share
	}
	return verbatimPrefix + abs
}

const (
	verbatimPrefix = `\\?\`
	verbatimShare  = `\\?\UNC\`
)

// plain undoes Verbatim.
func plain(abs string) string {
	if runtime.GOOS != "windows" {
		return abs
	}
	if share, ok := strings.CutPrefix(abs, verbatimShare); ok {
		return `\\` + share
	}
	return strings.TrimPrefix(abs, verbatimPrefix)
}

// trimmedByWindows reports whether Windows would change a name of p.
func trimmedByWindows(p string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	for _, name := range strings.Split(p[len(filepath.VolumeName(p)):], `\`) {
		if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
			return true
		}
	}
	return false
}

// Writable reports whether files can be created in dir.
func Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".tordo-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	return os.Remove(name) == nil
}
