// Package musicbrainz is a small client for the MusicBrainz web service (at
// most one request per second, as its rules ask) and the Cover Art Archive.
package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"tordo/internal/httpx"
)

const (
	BaseURL          = "https://musicbrainz.org/ws/2"
	CoverBaseURL     = "https://coverartarchive.org"
	VariousArtistsID = "89ad4ac3-39f7-470e-963a-56509c546377"
)

var (
	ErrNotFound  = errors.New("no existe en MusicBrainz")
	ErrInvalidID = errors.New("id de MusicBrainz inválido")
)

var validID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ValidID reports whether id has the form of a MusicBrainz identifier.
func ValidID(id string) bool { return validID.MatchString(id) }

type Client struct {
	Base      string        // BaseURL in production
	CoverBase string        // CoverBaseURL in production
	HTTP      *httpx.Client // MusicBrainz
	Covers    *httpx.Client // Cover Art Archive
}

// New returns a client for the real services: one request per second to
// MusicBrainz, two per second to the Cover Art Archive.
func New() *Client {
	ua := http.Header{"User-Agent": {httpx.UserAgent}}
	return &Client{
		Base: BaseURL, CoverBase: CoverBaseURL,
		HTTP:   &httpx.Client{Limiter: &httpx.Limiter{Rate: 1, Burst: 1}, Header: ua},
		Covers: &httpx.Client{Limiter: &httpx.Limiter{Rate: 2, Burst: 2}, Header: ua},
	}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, v any) error {
	q.Set("fmt", "json")
	body, err := c.HTTP.Get(ctx, c.Base+path+"?"+q.Encode())
	if err != nil {
		return notFound(err)
	}
	return json.Unmarshal(body, v)
}

func notFound(err error) error {
	var se *httpx.StatusError
	if errors.As(err, &se) && se.Code == http.StatusNotFound {
		return ErrNotFound
	}
	return err
}

type artistCredit []struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

func (ac artistCredit) text() string {
	var b strings.Builder
	for _, c := range ac {
		b.WriteString(c.Name + c.JoinPhrase)
	}
	return b.String()
}

func (ac artistCredit) ids() []string {
	var out []string
	for _, c := range ac {
		out = append(out, c.Artist.ID)
	}
	return out
}

func (ac artistCredit) names() []string {
	var out []string
	for _, c := range ac {
		out = append(out, c.Name)
	}
	return out
}

func (ac artistCredit) various() bool {
	for _, c := range ac {
		if c.Artist.ID == VariousArtistsID {
			return true
		}
	}
	return false
}

type relations []struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

func (rs relations) discogs() []string {
	var out []string
	for _, r := range rs {
		if r.Type == "discogs" && r.URL.Resource != "" {
			out = append(out, r.URL.Resource)
		}
	}
	return out
}

type genreList []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func (gs genreList) names() []string {
	var out []string
	for _, g := range gs {
		out = append(out, g.Name)
	}
	return out
}

// year reads the year of a MusicBrainz date ("1986-11-10", "1986", "").
func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// terms turns free text into Lucene terms: lowercase (so "and", "or" and
// "not" are words, not operators) and without Lucene's syntax characters.
func terms(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`+-&|!(){}[]^"~*?:\/`, r) {
			return ' '
		}
		return r
	}, strings.ToLower(s))
	return strings.Join(strings.Fields(s), " ")
}

type SearchQuery struct {
	Title   string
	Artist  string // ignored when Various
	Various bool   // search releases credited to Various Artists
}

// ReleaseHit is a release found by a search.
type ReleaseHit struct {
	ID          string
	Title       string
	Artist      string
	ArtistNames []string
	Various     bool
	Year        int
	Country     string
	Tracks      int
	Score       int // MusicBrainz's own relevance, 0–100
}

type searchResponse struct {
	Releases []struct {
		ID           string       `json:"id"`
		Score        int          `json:"score"`
		Title        string       `json:"title"`
		ArtistCredit artistCredit `json:"artist-credit"`
		Date         string       `json:"date"`
		Country      string       `json:"country"`
		TrackCount   int          `json:"track-count"`
	} `json:"releases"`
}

// SearchReleases finds up to 10 releases by title and artist. An empty
// title searches nothing.
func (c *Client) SearchReleases(ctx context.Context, q SearchQuery) ([]ReleaseHit, error) {
	title := terms(q.Title)
	if title == "" {
		return nil, nil
	}
	lq := "release:(" + title + ")"
	switch {
	case q.Various:
		lq += " AND arid:" + VariousArtistsID
	case terms(q.Artist) != "":
		lq += " AND artist:(" + terms(q.Artist) + ")"
	}
	var resp searchResponse
	if err := c.get(ctx, "/release", url.Values{"query": {lq}, "limit": {"10"}}, &resp); err != nil {
		return nil, err
	}
	var out []ReleaseHit
	for _, r := range resp.Releases {
		out = append(out, ReleaseHit{ID: r.ID, Title: r.Title, Artist: r.ArtistCredit.text(), ArtistNames: r.ArtistCredit.names(),
			Various: r.ArtistCredit.various(), Year: year(r.Date), Country: r.Country, Tracks: r.TrackCount, Score: r.Score})
	}
	return out, nil
}

type Track struct {
	Disc        int // the medium's position
	Position    int
	Title       string
	LengthMs    int64
	RecordingID string
}

type Release struct {
	ID           string
	Title        string
	Artist       string // the artist credit as written
	ArtistIDs    []string
	ArtistNames  []string
	Various      bool
	Year         int
	Country      string
	GroupID      string
	OriginalYear int // the release group's first release
	Tracks       []Track
	Genres       []string
	DiscogsURLs  []string
}

type releaseResponse struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Date         string       `json:"date"`
	Country      string       `json:"country"`
	ArtistCredit artistCredit `json:"artist-credit"`
	ReleaseGroup struct {
		ID               string `json:"id"`
		FirstReleaseDate string `json:"first-release-date"`
	} `json:"release-group"`
	Media []struct {
		Position int `json:"position"`
		Tracks   []struct {
			Position  int    `json:"position"`
			Title     string `json:"title"`
			Length    int64  `json:"length"`
			Recording struct {
				ID     string `json:"id"`
				Length int64  `json:"length"`
			} `json:"recording"`
		} `json:"tracks"`
	} `json:"media"`
	Relations relations `json:"relations"`
	Genres    genreList `json:"genres"`
}

// Release looks up a release with its tracks, release group, Discogs links
// and genres.
func (c *Client) Release(ctx context.Context, id string) (Release, error) {
	if !ValidID(id) {
		return Release{}, ErrInvalidID
	}
	var r releaseResponse
	inc := url.Values{"inc": {"recordings artist-credits release-groups url-rels genres"}}
	if err := c.get(ctx, "/release/"+id, inc, &r); err != nil {
		return Release{}, err
	}
	rel := Release{ID: r.ID, Title: r.Title, Artist: r.ArtistCredit.text(), ArtistIDs: r.ArtistCredit.ids(),
		ArtistNames: r.ArtistCredit.names(), Various: r.ArtistCredit.various(), Year: year(r.Date), Country: r.Country,
		GroupID: r.ReleaseGroup.ID, OriginalYear: year(r.ReleaseGroup.FirstReleaseDate), Genres: r.Genres.names(),
		DiscogsURLs: r.Relations.discogs()}
	for _, m := range r.Media {
		for _, t := range m.Tracks {
			length := t.Length
			if length == 0 {
				length = t.Recording.Length
			}
			rel.Tracks = append(rel.Tracks, Track{Disc: m.Position, Position: t.Position, Title: t.Title, LengthMs: length,
				RecordingID: t.Recording.ID})
		}
	}
	return rel, nil
}

// GroupLinks is what a release group adds when its release lacks it.
type GroupLinks struct {
	DiscogsURLs []string // Discogs masters, usually
	Genres      []string
}

func (c *Client) ReleaseGroupLinks(ctx context.Context, id string) (GroupLinks, error) {
	if !ValidID(id) {
		return GroupLinks{}, ErrInvalidID
	}
	var r struct {
		Relations relations `json:"relations"`
		Genres    genreList `json:"genres"`
	}
	if err := c.get(ctx, "/release-group/"+id, url.Values{"inc": {"url-rels genres"}}, &r); err != nil {
		return GroupLinks{}, err
	}
	return GroupLinks{DiscogsURLs: r.Relations.discogs(), Genres: r.Genres.names()}, nil
}

type Artist struct {
	ID       string
	Name     string
	SortName string
	Type     string // Person, Group…
	Country  string // ISO 3166-1 alpha-2, "" when unknown
	Area     string
	Begin    string
	End      string
}

// Artist looks up an artist. Its country is MusicBrainz's country field,
// else its area's country code, else the country of its area's subdivision
// code ("BR-RJ" → "BR").
func (c *Client) Artist(ctx context.Context, id string) (Artist, error) {
	if !ValidID(id) {
		return Artist{}, ErrInvalidID
	}
	var r struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		SortName string `json:"sort-name"`
		Type     string `json:"type"`
		Country  string `json:"country"`
		Area     *struct {
			Name string   `json:"name"`
			ISO1 []string `json:"iso-3166-1-codes"`
			ISO2 []string `json:"iso-3166-2-codes"`
		} `json:"area"`
		LifeSpan struct {
			Begin string `json:"begin"`
			End   string `json:"end"`
		} `json:"life-span"`
	}
	if err := c.get(ctx, "/artist/"+id, url.Values{}, &r); err != nil {
		return Artist{}, err
	}
	a := Artist{ID: r.ID, Name: r.Name, SortName: r.SortName, Type: r.Type, Country: r.Country,
		Begin: r.LifeSpan.Begin, End: r.LifeSpan.End}
	if r.Area != nil {
		a.Area = r.Area.Name
		switch {
		case a.Country != "":
		case len(r.Area.ISO1) > 0:
			a.Country = r.Area.ISO1[0]
		case len(r.Area.ISO2) > 0 && len(r.Area.ISO2[0]) >= 2:
			a.Country = r.Area.ISO2[0][:2]
		}
	}
	return a, nil
}

// Cover returns the front cover (500 px) of a release or release group
// (kind "release" or "release-group") from the Cover Art Archive;
// ErrNotFound when it has none.
func (c *Client) Cover(ctx context.Context, kind, id string) ([]byte, error) {
	if kind != "release" && kind != "release-group" {
		return nil, fmt.Errorf("musicbrainz: carátula de %q", kind)
	}
	if !ValidID(id) {
		return nil, ErrInvalidID
	}
	b, err := c.Covers.Get(ctx, c.CoverBase+"/"+kind+"/"+id+"/front-500")
	return b, notFound(err)
}
