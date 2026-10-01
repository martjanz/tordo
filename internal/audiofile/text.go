package audiofile

import (
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// text8 decodes 8-bit text: UTF-8 when valid, Latin-1 otherwise. Many taggers
// write UTF-8 into fields declared Latin-1; real Latin-1 with accents is
// almost never valid UTF-8.
func text8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return latin1(b)
}

// utf16Text decodes UTF-16 with an optional byte order mark; without one,
// bigEndian decides.
func utf16Text(b []byte, bigEndian bool) string {
	if len(b) >= 2 {
		switch {
		case b[0] == 0xFF && b[1] == 0xFE:
			bigEndian, b = false, b[2:]
		case b[0] == 0xFE && b[1] == 0xFF:
			bigEndian, b = true, b[2:]
		}
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		if bigEndian {
			u[i] = be16(b[2*i:])
		} else {
			u[i] = le16(b[2*i:])
		}
	}
	return string(utf16.Decode(u))
}

// clean drops surrounding NULs and spaces.
func clean(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r == 0 || unicode.IsSpace(r) })
}
