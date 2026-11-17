package store

import "encoding/json"

// The saved stations were written with a Spanish vocabulary for views, modes,
// facet names and some facet values; version 8 renames them. The store does
// not depend on catalog, so the old and new words are spelled out here.

var oldViews = map[string]string{"pistas": "tracks", "albumes": "albums", "artistas": "artists"}

var oldFacetNames = map[string]string{
	"decada": "decade", "anio": "year", "artista": "artist", "pais": "country", "genero": "genre",
	"estilo": "style", "coleccion": "collection", "carpeta": "folder", "calidad": "quality",
	"formato": "format", "estado": "state", "megusta": "liked", "puntaje": "rating", "escucha": "listened",
}

// oldFacetValues are keyed by the new facet name.
var oldFacetValues = map[string]map[string]string{
	"quality":  {"sin-perdida": "lossless", "con-perdida": "lossy"},
	"state":    {"sin-identificar": "unidentified", "no-reproducible": "unplayable", "con-duplicados": "duplicated"},
	"liked":    {"si": "yes"},
	"listened": {"nunca": "never", "hace-un-anio": "year-ago"},
}

// upgradeStationDef rewrites a saved station's JSON from the old vocabulary to
// the new one, keeping every other field. It reports whether anything changed;
// when not (or when def is not a JSON object) it returns def untouched.
func upgradeStationDef(def []byte) ([]byte, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(def, &top) != nil || top == nil {
		return def, false
	}
	changed := false
	if renameString(top, "mode", oldViews) {
		changed = true
	}
	if raw, ok := top["source"]; ok {
		var src map[string]json.RawMessage
		if json.Unmarshal(raw, &src) == nil && src != nil {
			srcChanged := renameString(src, "view", oldViews)
			if fraw, ok := src["facets"]; ok {
				var facets map[string]string
				if json.Unmarshal(fraw, &facets) == nil && facets != nil {
					out := make(map[string]string, len(facets))
					fChanged := false
					for name, value := range facets {
						newName := name
						if n, ok := oldFacetNames[name]; ok {
							newName, fChanged = n, true
						}
						if n, ok := oldFacetValues[newName][value]; ok {
							value, fChanged = n, true
						}
						out[newName] = value
					}
					if fChanged {
						b, _ := json.Marshal(out)
						src["facets"] = b
						srcChanged = true
					}
				}
			}
			if srcChanged {
				b, _ := json.Marshal(src)
				top["source"] = b
				changed = true
			}
		}
	}
	if !changed {
		return def, false
	}
	b, err := json.Marshal(top)
	if err != nil {
		return def, false
	}
	return b, true
}

// renameString maps the string at m[key] through words; it reports a change.
func renameString(m map[string]json.RawMessage, key string, words map[string]string) bool {
	raw, ok := m[key]
	if !ok {
		return false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return false
	}
	n, ok := words[s]
	if !ok {
		return false
	}
	b, _ := json.Marshal(n)
	m[key] = b
	return true
}
