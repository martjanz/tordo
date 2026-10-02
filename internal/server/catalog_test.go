package server

import (
	"net/http"
	"net/url"
	"testing"

	"tordo/internal/catalog"
	"tordo/internal/store"
)

func TestExploreEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var r catalog.Result
	decode(t, do(t, h, "GET", "/api/explore", ""), &r)
	if r.Total != 1 || len(r.Albums) != 1 || r.Albums[0].Title != "Locura" || r.Albums[0].ArtistKey != "virus" || r.Query.View != "albumes" {
		t.Fatalf("albums = %+v", r)
	}
	if got := r.Facets["estado"]; len(got) != 1 || got[0].Value != catalog.StateUnplayable {
		t.Fatalf("without ffmpeg the WMA track is not playable: %+v", got)
	}
	r = catalog.Result{}
	decode(t, do(t, h, "GET", "/api/explore?ver=pistas&formato=wma&limite=1", ""), &r)
	if r.Total != 1 || len(r.Tracks) != 1 || r.Tracks[0].Title != "Pecados para dos" {
		t.Fatalf("tracks = %+v", r)
	}
	r = catalog.Result{}
	decode(t, do(t, h, "GET", "/api/explore?ver=artistas&desde=5", ""), &r)
	if r.Total != 1 || len(r.Artists) != 0 {
		t.Fatalf("artists past the end = %+v", r)
	}
}

func TestCatalogFollowsChanges(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var r catalog.Result
	decode(t, do(t, h, "GET", "/api/explore?pais=AR", ""), &r)
	if r.Total != 0 {
		t.Fatalf("before: %+v", r)
	}
	as, _ := s.Store.Albums("")
	if err := s.Store.ApplyIdentities([]store.AlbumIdentity{{AlbumID: as[0].ID, Status: store.StatusAuto, ReleaseID: "r1",
		Country: "AR"}}, nil); err != nil {
		t.Fatal(err)
	}
	decode(t, do(t, h, "GET", "/api/explore?pais=AR", ""), &r)
	if r.Total != 1 {
		t.Fatalf("the catalog was not rebuilt: %+v", r)
	}
}

func TestArtistEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var p catalog.ArtistPage
	decode(t, do(t, h, "GET", "/api/artists/virus", ""), &p)
	if p.Name != "Virus" || len(p.Albums) != 1 {
		t.Fatalf("artist = %+v", p)
	}
	if rec := do(t, h, "GET", "/api/artists/"+url.PathEscape("nadie/x"), ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown artist: %d", rec.Code)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	var pl playResponse
	decode(t, do(t, h, "GET", "/api/play?artist=virus", ""), &pl)
	if len(pl.Items) != 2 || pl.Items[0].Title != "Una luna de miel en la mano" {
		t.Fatalf("play the artist: %+v", pl)
	}
	if rec := do(t, h, "GET", "/api/play?artist=nadie", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("play an unknown artist: %d", rec.Code)
	}
}

func TestSearchEndpoint(t *testing.T) {
	s, _ := mediaServer(t, "")
	h := s.Handler()
	var f struct {
		Q string `json:"q"`
		catalog.Found
	}
	decode(t, do(t, h, "GET", "/api/search?q=pecad", ""), &f)
	if f.Q != "pecad" || len(f.Tracks) != 1 || f.Tracks[0].Title != "Pecados para dos" || len(f.Albums) != 0 {
		t.Fatalf("found = %+v", f)
	}
	f.Found = catalog.Found{}
	decode(t, do(t, h, "GET", "/api/search?q=virus&limite=1", ""), &f)
	if len(f.Artists) != 1 || len(f.Albums) != 1 || len(f.Tracks) != 1 || f.Totals["tracks"] != 2 {
		t.Fatalf("virus = %+v", f)
	}
	f.Found = catalog.Found{}
	decode(t, do(t, h, "GET", "/api/search?q=v", ""), &f)
	if len(f.Tracks) != 0 || f.Tracks == nil {
		t.Fatalf("too short = %+v", f)
	}
}

func TestAlbumPage(t *testing.T) {
	s, _ := mediaServer(t, "")
	id := onlyAlbum(t, s)
	var p struct {
		Title     string `json:"title"`
		ArtistKey string `json:"artistKey"`
		Tracks    []struct {
			Title      string `json:"title"`
			ArtistKey  string `json:"artistKey"`
			Playable   bool   `json:"playable"`
			Transcoded bool   `json:"transcoded"`
		} `json:"tracks"`
		Candidates []store.Candidate `json:"candidates"`
	}
	decode(t, do(t, s.Handler(), "GET", "/api/albums/"+itoa(id), ""), &p)
	if p.Title != "Locura" || p.ArtistKey != "virus" || len(p.Tracks) != 2 || p.Candidates == nil {
		t.Fatalf("album = %+v", p)
	}
	if !p.Tracks[0].Playable || p.Tracks[1].Playable || p.Tracks[0].ArtistKey != "virus" {
		t.Fatalf("without ffmpeg: %+v", p.Tracks)
	}
	s.Streamer.FFmpeg = "ffmpeg"
	decode(t, do(t, s.Handler(), "GET", "/api/albums/"+itoa(id), ""), &p)
	if !p.Tracks[1].Playable || !p.Tracks[1].Transcoded {
		t.Fatalf("with ffmpeg: %+v", p.Tracks)
	}
}
