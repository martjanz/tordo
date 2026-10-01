// Package genres maps raw genre values (tags, MusicBrainz) to a two-level
// taxonomy based on Discogs': genre and style.
package genres

import (
	"slices"
	"strings"

	"tordo/internal/nameparse"
)

// Mapping is a genre, or a style under its genre.
type Mapping struct {
	Genre string `json:"genre"`
	Style string `json:"style,omitempty"`
}

// Key folds a raw value for lookups: normalized like names, with a word
// repeated right after itself dropped ("Pop Pop Internacional").
func Key(raw string) string {
	var out []string
	for _, w := range strings.Fields(nameparse.Norm(raw)) {
		if len(out) == 0 || out[len(out)-1] != w {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

var (
	known      = map[string]Mapping{} // genre and style names by Key
	styleOwner = map[string]string{}  // style Key → genre
)

func init() {
	for _, g := range Taxonomy {
		known[Key(g.Name)] = Mapping{Genre: g.Name}
		for _, s := range g.Styles {
			known[Key(s)] = Mapping{Genre: g.Name, Style: s}
			styleOwner[Key(s)] = g.Name
		}
	}
}

// Normalizer maps raw values with the built-in aliases and the user's.
type Normalizer struct {
	user map[string]Mapping // by Key; an empty Genre means "not a genre"
}

func New(user map[string]Mapping) *Normalizer { return &Normalizer{user: user} }

// Map returns what a raw value means. ok is false when (part of) it is
// unknown: the value then belongs in the Géneros por mapear queue. A value
// that is not a genre ("Other", "Uruguay") maps to nothing with ok true.
func (n *Normalizer) Map(raw string) ([]Mapping, bool) {
	if ms, ok := n.one(Key(raw)); ok {
		return ms, true
	}
	var parts []string
	for _, p := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '/' || r == ';' || r == '|' }) {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return nil, false
	}
	var out []Mapping
	all := true
	for i := 0; i < len(parts); {
		j := len(parts)
		for ; j > i; j-- { // the longest known run first: "Folk, World, & Country"
			if ms, ok := n.one(Key(strings.Join(parts[i:j], ", "))); ok {
				out = appendNew(out, ms...)
				break
			}
		}
		if j == i {
			all = false
			i++
		} else {
			i = j
		}
	}
	return out, all
}

func (n *Normalizer) one(k string) ([]Mapping, bool) {
	if m, ok := n.user[k]; ok {
		if m.Genre == "" {
			return nil, true
		}
		return []Mapping{m}, true
	}
	if k == "" || ignored[k] || isNumber(k) {
		return nil, true
	}
	if m, ok := known[k]; ok {
		return []Mapping{m}, true
	}
	if m, ok := aliases[k]; ok {
		return []Mapping{m}, true
	}
	return nil, false
}

func isNumber(k string) bool {
	return strings.Trim(k, "0123456789 ") == ""
}

func appendNew(ms []Mapping, add ...Mapping) []Mapping {
	for _, m := range add {
		if !slices.Contains(ms, m) {
			ms = append(ms, m)
		}
	}
	return ms
}

// FromDiscogs pairs a Discogs release's genres and styles: a style goes
// under its taxonomy genre when the release lists that genre, else under the
// release's first genre; genres left without a style stand alone.
func FromDiscogs(gs, styles []string) []Mapping {
	if len(gs) == 0 {
		return nil
	}
	var out []Mapping
	used := map[string]bool{}
	for _, s := range styles {
		g := gs[0]
		if o := styleOwner[Key(s)]; o != "" && slices.Contains(gs, o) {
			g = o
		}
		out = appendNew(out, Mapping{Genre: g, Style: s})
		used[g] = true
	}
	for _, g := range gs {
		if !used[g] {
			out = appendNew(out, Mapping{Genre: g})
		}
	}
	return out
}

// Valid reports whether genre is in the taxonomy and style ("" for none) is
// one of its styles.
func Valid(genre, style string) bool {
	for _, g := range Taxonomy {
		if g.Name == genre {
			return style == "" || slices.Contains(g.Styles, style)
		}
	}
	return false
}
