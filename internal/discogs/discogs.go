// Package discogs is a small client for the Discogs API: genres and styles
// of releases and masters, and release search (which needs a token).
package discogs

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

const BaseURL = "https://api.discogs.com"

var (
	ErrNotFound     = errors.New("no existe en Discogs")
	ErrUnauthorized = errors.New("Discogs rechazó el token")
	ErrNoToken      = errors.New("sin token de Discogs")
)

type Client struct {
	Base  string
	Token string
	HTTP  *httpx.Client
}

// New returns a client for the real API: 25 requests per minute without a
// token, 55 with one (Discogs allows 60).
func New(token string) *Client {
	h := http.Header{"User-Agent": {httpx.UserAgent}}
	rate := 25.0 / 60
	if token != "" {
		h.Set("Authorization", "Discogs token="+token)
		rate = 55.0 / 60
	}
	return &Client{Base: BaseURL, Token: token, HTTP: &httpx.Client{Limiter: &httpx.Limiter{Rate: rate, Burst: 1}, Header: h}}
}

func (c *Client) HasToken() bool { return c.Token != "" }

// Info is the genres and styles of a release or a master.
type Info struct {
	ID       int // the release (0 for a master)
	MasterID int
	Genres   []string
	Styles   []string
}

type entity struct {
	ID       int      `json:"id"`
	MasterID int      `json:"master_id"`
	Genres   []string `json:"genres"`
	Styles   []string `json:"styles"`
}

func (c *Client) Release(ctx context.Context, id int) (Info, error) {
	var e entity
	if err := c.get(ctx, fmt.Sprintf("/releases/%d", id), &e); err != nil {
		return Info{}, err
	}
	return Info{ID: e.ID, MasterID: e.MasterID, Genres: e.Genres, Styles: e.Styles}, nil
}

func (c *Client) Master(ctx context.Context, id int) (Info, error) {
	var e entity
	if err := c.get(ctx, fmt.Sprintf("/masters/%d", id), &e); err != nil {
		return Info{}, err
	}
	return Info{MasterID: e.ID, Genres: e.Genres, Styles: e.Styles}, nil
}

// Result is a release found by a search.
type Result struct {
	ID       int
	MasterID int
	Artist   string
	Title    string
	Year     int
	Genres   []string
	Styles   []string
}

// Search finds up to 5 releases by artist and title. It needs a token.
func (c *Client) Search(ctx context.Context, artist, title string) ([]Result, error) {
	if c.Token == "" {
		return nil, ErrNoToken
	}
	q := url.Values{"type": {"release"}, "artist": {artist}, "release_title": {title}, "per_page": {"5"}}
	var resp struct {
		Results []struct {
			ID       int      `json:"id"`
			MasterID int      `json:"master_id"`
			Title    string   `json:"title"` // "Artist - Title"
			Year     string   `json:"year"`
			Genre    []string `json:"genre"`
			Style    []string `json:"style"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/database/search?"+q.Encode(), &resp); err != nil {
		return nil, err
	}
	out := []Result{}
	for _, r := range resp.Results {
		a, t, ok := strings.Cut(r.Title, " - ")
		if !ok {
			a, t = "", r.Title
		}
		y, _ := strconv.Atoi(r.Year)
		out = append(out, Result{ID: r.ID, MasterID: r.MasterID, Artist: a, Title: t, Year: y, Genres: r.Genre, Styles: r.Style})
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	body, err := c.HTTP.Get(ctx, c.Base+path)
	var se *httpx.StatusError
	if errors.As(err, &se) {
		switch se.Code {
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrUnauthorized
		}
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

var reURL = regexp.MustCompile(`discogs\.com/(?:[^/]+/)*(release|master)/(\d+)`)

// ParseURL reads a discogs.com link to a release or a master.
func ParseURL(u string) (kind string, id int, ok bool) {
	m := reURL.FindStringSubmatch(u)
	if m == nil {
		return "", 0, false
	}
	id, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], id, true
}
