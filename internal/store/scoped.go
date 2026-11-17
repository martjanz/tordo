package store

import "encoding/json"

// Scoped reads and writes: a correction concerns one album (and its copies),
// and reading the whole catalog for it took seconds on a large catalog.

// inJSON matches a column against a JSON array passed as one argument, so a
// list of any length needs no placeholders.
const inJSON = `(SELECT value FROM json_each(?))`

func jsonList[T any](vs []T) string {
	b, _ := json.Marshal(nonNil(vs)) // a slice of strings or ints always marshals
	return string(b)
}

// nonNil tells an empty scope (nothing) from a nil one (everything).
func nonNil[T any](vs []T) []T {
	if vs == nil {
		return []T{}
	}
	return vs
}

// inScope is the WHERE clause for a column in scope; nil means no clause.
func inScope[T any](column string, scope []T) (string, []any) {
	if scope == nil {
		return "", nil
	}
	return ` WHERE ` + column + ` IN ` + inJSON, []any{jsonList(scope)}
}

// AlbumsWithFingerprints returns the albums with a track of those
// fingerprints: the albums a correction on them reaches.
func (s *Store) AlbumsWithFingerprints(fps []string) ([]int64, error) {
	rows, err := s.db.Query(`SELECT DISTINCT t.album_id FROM tracks t JOIN files f ON f.id = t.file_id
		WHERE t.album_id IS NOT NULL AND f.fingerprint IN `+inJSON+` ORDER BY t.album_id`, jsonList(fps))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ApplyAlbumIdentities is ApplyIdentities for those albums only: ids must
// cover them (the titles and genres of their tracks not in ids are cleared),
// ids of other albums are skipped, and the rest of the catalog is not read.
// unmappedDelta is added to the unmapped genre counts (a count down to zero
// is removed).
func (s *Store) ApplyAlbumIdentities(albumIDs []int64, ids []AlbumIdentity, unmappedDelta map[string]int) error {
	return s.applyIdentities(nonNil(albumIDs), ids, unmappedDelta)
}
