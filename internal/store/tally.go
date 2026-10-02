package store

import (
	"strconv"
	"strings"
	"time"
)

// PlayTally is the plays of a period resolved to the catalog's current
// tracks: each play counts once, for one track (see PlayTally).
type PlayTally struct {
	Counts     map[int64]int // plays by file id; never nil
	Plays      int           // every play of the period, resolved or not
	ListenedMs int64
}

// PlayTally sums the plays since a time (the zero time: all of them). A play
// goes to the file it was played from when that file is present with the
// same audio, else to the lowest present file with that audio; a play whose
// audio is no longer in the catalog only counts in the totals.
func (s *Store) PlayTally(since time.Time) (PlayTally, error) {
	out := PlayTally{Counts: map[int64]int{}}
	var from int64
	if !since.IsZero() {
		from = since.UnixMilli()
	}
	rows, err := s.db.Query(`SELECT ref, file_id, COUNT(*), COALESCE(SUM(listened_ms), 0)
		FROM plays WHERE played_at >= ? GROUP BY ref, file_id`, from)
	if err != nil {
		return out, err
	}
	type group struct {
		ref   string
		file  int64
		plays int
	}
	var groups []group
	var refs []string
	seen := map[string]bool{}
	for rows.Next() {
		var g group
		var ms int64
		if err := rows.Scan(&g.ref, &g.file, &g.plays, &ms); err != nil {
			rows.Close()
			return out, err
		}
		groups = append(groups, g)
		out.Plays += g.plays
		out.ListenedMs += ms
		if !seen[g.ref] {
			seen[g.ref] = true
			refs = append(refs, g.ref)
		}
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}

	present := map[string]map[int64]bool{} // present files by ref
	lowest := map[string]int64{}
	err = s.presentFilesByRef(refs, func(ref string, id int64) {
		if present[ref] == nil {
			present[ref] = map[int64]bool{}
		}
		present[ref][id] = true
		if low, ok := lowest[ref]; !ok || id < low {
			lowest[ref] = id
		}
	})
	if err != nil {
		return out, err
	}

	for _, g := range groups {
		switch {
		case present[g.ref][g.file]:
			out.Counts[g.file] += g.plays
		case len(present[g.ref]) > 0:
			out.Counts[lowest[g.ref]] += g.plays
		}
	}
	return out, nil
}

func stringArgs(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// presentFilesByRef calls fn for every present track file of each ref. It
// looks fingerprint refs up through the files(fingerprint) index and "id:N"
// refs (files without a fingerprint) by primary key, instead of evaluating
// the trackRef expression over every file.
func (s *Store) presentFilesByRef(refs []string, fn func(ref string, id int64)) error {
	var prints []string
	var ids []int64
	for _, r := range refs {
		if n, ok := strings.CutPrefix(r, "id:"); ok {
			if id, err := strconv.ParseInt(n, 10, 64); err == nil {
				ids = append(ids, id)
			}
			continue
		}
		prints = append(prints, r)
	}
	for _, chunk := range chunks(prints) {
		rows, err := s.db.Query(`SELECT f.fingerprint, f.id FROM files f JOIN tracks t ON t.file_id = f.id
			WHERE f.missing = 0 AND f.fingerprint IN (`+placeholders(len(chunk))+`)`, stringArgs(chunk)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var ref string
			var id int64
			if err := rows.Scan(&ref, &id); err != nil {
				rows.Close()
				return err
			}
			fn(ref, id)
		}
		if err := closeRows(rows); err != nil {
			return err
		}
	}
	for _, chunk := range chunks(ids) {
		rows, err := s.db.Query(`SELECT f.id FROM files f JOIN tracks t ON t.file_id = f.id
			WHERE f.missing = 0 AND f.fingerprint = '' AND f.id IN (`+placeholders(len(chunk))+`)`, int64Args(chunk)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			fn("id:"+strconv.FormatInt(id, 10), id)
		}
		if err := closeRows(rows); err != nil {
			return err
		}
	}
	return nil
}
