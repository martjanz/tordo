// Package mediafile tells which files and folders of a music library matter.
package mediafile

import (
	"path"
	"strings"
)

type Kind string

const (
	Audio    Kind = "audio"
	Image    Kind = "image"
	Lyrics   Kind = "lyrics"
	Playlist Kind = "playlist"
	None     Kind = ""
)

var byExt = map[string]Kind{
	".mp3": Audio, ".flac": Audio, ".m4a": Audio, ".aac": Audio, ".opus": Audio, ".ogg": Audio, ".oga": Audio,
	".wma": Audio, ".wav": Audio, ".aif": Audio, ".aiff": Audio, ".aifc": Audio, ".ape": Audio, ".wv": Audio,
	".jpg": Image, ".jpeg": Image, ".png": Image, ".webp": Image,
	".lrc": Lyrics,
	".m3u": Playlist, ".m3u8": Playlist,
}

// Classify tells a file's kind from its name. Files of kind None are not catalogued.
func Classify(name string) Kind {
	if strings.HasPrefix(name, "._") {
		return None // AppleDouble resource forks that macOS leaves on exFAT/FAT drives
	}
	return byExt[strings.ToLower(path.Ext(name))]
}

// builtinIgnore are folders of other apps and of the system, matched against
// a folder's lowercased name.
var builtinIgnore = []string{
	"ableton", "audio music apps", "djay", "downloading", "*.musiclibrary",
	"previous libraries.localized", "$recycle.bin", "system volume information", "@eadir", "lost+found",
}

// IgnoredDir reports whether the folder at rel (relative to its root,
// '/'-separated) must not be walked: hidden folders, the built-in list, and
// the user's patterns. A pattern without '/' matches a folder name anywhere;
// one with '/' matches the folder's path from its root. Patterns use
// path.Match syntax and are case-insensitive.
func IgnoredDir(rel string, extra []string) bool {
	base := path.Base(rel)
	if strings.HasPrefix(base, ".") {
		return true
	}
	lb, lr := strings.ToLower(base), strings.ToLower(rel)
	for _, p := range builtinIgnore {
		if ok, _ := path.Match(p, lb); ok {
			return true
		}
	}
	for _, p := range extra {
		p = strings.ToLower(strings.Trim(p, "/"))
		target := lb
		if strings.Contains(p, "/") {
			target = lr
		}
		if ok, _ := path.Match(p, target); ok {
			return true
		}
	}
	return false
}
