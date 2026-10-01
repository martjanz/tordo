package store

import (
	"encoding/json"
	"time"
)

// CoverNone marks a release the Cover Art Archive has no front cover for.
const CoverNone = "none"

type ReleaseTrack struct {
	Disc        int    `json:"disc"`
	Position    int    `json:"position"`
	Title       string `json:"title"`
	LengthMs    int64  `json:"lengthMs"`
	RecordingID string `json:"recordingId"`
}

// DiscogsInfo is what Discogs said about a release.
type DiscogsInfo struct {
	Done     bool     `json:"done"`     // MusicBrainz's links to Discogs were followed
	Searched bool     `json:"searched"` // searched on Discogs (needs a token)
	ID       int      `json:"id"`
	MasterID int      `json:"masterId"`
	Genres   []string `json:"genres"`
	Styles   []string `json:"styles"`
}

// Release is a MusicBrainz release as tordo keeps it, with what enrichment
// added.
type Release struct {
	ID           string         `json:"id"`
	GroupID      string         `json:"groupId"`
	Title        string         `json:"title"`
	Artist       string         `json:"artist"` // the artist credit as MusicBrainz writes it
	ArtistIDs    []string       `json:"artistIds"`
	ArtistNames  []string       `json:"artistNames"`
	Various      bool           `json:"various"` // credited to Various Artists
	Year         int            `json:"year"`
	OriginalYear int            `json:"originalYear"` // the release group's first release
	Country      string         `json:"country"`
	Tracks       []ReleaseTrack `json:"tracks"`
	MBGenres     []string       `json:"mbGenres"`
	DiscogsURLs  []string       `json:"discogsUrls"`
	GroupChecked bool           `json:"groupChecked"` // the release group's links were looked up
	Discogs      DiscogsInfo    `json:"discogs"`
	Cover        string         `json:"cover"` // path relative to the app folder, CoverNone, or "" (not looked up)
}

type Artist struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	SortName string `json:"sortName"`
	Type     string `json:"type"`
	Country  string `json:"country"` // ISO 3166-1 alpha-2, "" when unknown
	Area     string `json:"area"`
	Begin    string `json:"begin"`
	End      string `json:"end"`
}

func (s *Store) SaveRelease(r Release) error { return s.saveDoc("releases", r.ID, r) }

func (s *Store) Releases() (map[string]Release, error) { return loadDocs[Release](s, "releases") }

func (s *Store) SaveArtist(a Artist) error { return s.saveDoc("artists", a.ID, a) }

func (s *Store) Artists() (map[string]Artist, error) { return loadDocs[Artist](s, "artists") }

// saveDoc and loadDocs keep JSON documents in a table (id, data,
// fetched_at). table is always one of the constants above, never input.
func (s *Store) saveDoc(table, id string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO `+table+` (id, data, fetched_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data, fetched_at = excluded.fetched_at`, id, string(b), time.Now().UnixMilli())
	return err
}

func loadDocs[T any](s *Store, table string) (map[string]T, error) {
	rows, err := s.db.Query(`SELECT id, data FROM ` + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]T{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}
