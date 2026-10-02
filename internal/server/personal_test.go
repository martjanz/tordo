package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tordo/internal/catalog"
	"tordo/internal/store"
)

func put(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, h, method, target, body, "Content-Type", jsonCT)
}

type marksBody struct {
	Tracks      map[string]store.Mark `json:"tracks"`
	Albums      map[string]store.Mark `json:"albums"`
	Artists     []string              `json:"artists"`
	Collections []string              `json:"collections"`
}

func getMarks(t *testing.T, h http.Handler) marksBody {
	t.Helper()
	var m marksBody
	decode(t, do(t, h, "GET", "/api/marks", ""), &m)
	return m
}

// ids finds the only track and album of newStore.
func ids(t *testing.T, st *store.Store) (track, album int64) {
	t.Helper()
	as, err := st.Albums("")
	if err != nil || len(as) != 1 {
		t.Fatalf("albums = %v, %v", as, err)
	}
	d, _, _ := st.Album(as[0].ID)
	return d.Tracks[0].ID, as[0].ID
}

func TestLikesEndpoint(t *testing.T) {
	st := newStore(t)
	s := &Server{Store: st}
	h := s.Handler()
	tr, al := ids(t, st)
	like := func(body string) int { return put(t, h, "PUT", "/api/likes", body).Code }

	for _, b := range []string{
		`{"kind":"track","id":` + itoa(tr) + `,"liked":true}`,
		`{"kind":"album","id":` + itoa(al) + `,"liked":true}`,
		`{"kind":"artist","key":"virus","liked":true}`,
		`{"kind":"collection","key":"../musica/Virus - Locura","liked":true}`,
	} {
		if c := like(b); c != http.StatusNoContent {
			t.Fatalf("%s: %d", b, c)
		}
	}
	m := getMarks(t, h)
	if !m.Tracks[itoa(tr)].Liked || !m.Albums[itoa(al)].Liked || len(m.Artists) != 1 || m.Artists[0] != "virus" ||
		len(m.Collections) != 1 || m.Collections[0] != "../musica/Virus - Locura" {
		t.Fatalf("marks = %+v", m)
	}
	for body, want := range map[string]int{
		`{"kind":"x","id":1,"liked":true}`:                   http.StatusBadRequest,
		`{"kind":"track","liked":true}`:                      http.StatusBadRequest,
		`{"kind":"artist","liked":true}`:                     http.StatusBadRequest,
		`{"kind":"track","id":999,"liked":true}`:             http.StatusNotFound,
		`{"kind":"album","id":999,"liked":true}`:             http.StatusNotFound,
		`{"kind":"collection","key":"../nope","liked":true}`: http.StatusNotFound,
		`nope`: http.StatusBadRequest,
	} {
		if c := like(body); c != want {
			t.Fatalf("%s: %d, want %d", body, c, want)
		}
	}
	if rec := do(t, h, "PUT", "/api/likes", `{}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("no content type: %d", rec.Code)
	}
	ro := (&Server{Store: st, ReadOnly: true}).Handler()
	if rec := put(t, ro, "PUT", "/api/likes", `{"kind":"track","id":`+itoa(tr)+`,"liked":false}`); rec.Code != http.StatusConflict {
		t.Fatalf("read-only: %d", rec.Code)
	}
}

func TestRatingsEndpoint(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, _ := ids(t, st)
	rate := func(body string) int { return put(t, h, "PUT", "/api/ratings", body).Code }
	if c := rate(`{"kind":"track","id":` + itoa(tr) + `,"stars":4}`); c != http.StatusNoContent {
		t.Fatalf("rate: %d", c)
	}
	if r := getMarks(t, h).Tracks[itoa(tr)].Rating; r != 4 {
		t.Fatalf("rating = %d", r)
	}
	if c := rate(`{"kind":"track","id":` + itoa(tr) + `,"stars":6}`); c != http.StatusBadRequest {
		t.Fatalf("6 stars: %d", c)
	}
	if c := rate(`{"kind":"artist","id":1,"stars":3}`); c != http.StatusBadRequest {
		t.Fatalf("artist: %d", c)
	}
	if c := rate(`{"kind":"track","id":999,"stars":3}`); c != http.StatusNotFound {
		t.Fatalf("unknown: %d", c)
	}
	ro := (&Server{Store: st, ReadOnly: true}).Handler()
	if rec := put(t, ro, "PUT", "/api/ratings", `{"kind":"track","id":1,"stars":3}`); rec.Code != http.StatusConflict {
		t.Fatalf("read-only: %d", rec.Code)
	}
}

func TestPlaysEndpoint(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, _ := ids(t, st)
	count := func() int {
		p, err := st.Personal()
		if err != nil {
			t.Fatal(err)
		}
		return p.Plays[tr].Count
	}
	play := func(h http.Handler, body string) int { return put(t, h, "POST", "/api/plays", body).Code }
	body := `{"id":` + itoa(tr) + `,"listenedMs":5000,"device":"test"}`
	if c := play(h, body); c != http.StatusNoContent || count() != 1 {
		t.Fatalf("play: %d, count %d", c, count())
	}
	ro := (&Server{Store: st, ReadOnly: true}).Handler()
	if c := play(ro, body); c != http.StatusNoContent || count() != 1 {
		t.Fatalf("read-only: %d, count %d", c, count())
	}
	if c := play(h, `{"id":999,"listenedMs":1}`); c != http.StatusNotFound {
		t.Fatalf("unknown: %d", c)
	}
	if c := play(h, `{"id":`+itoa(tr)+`,"listenedMs":-1}`); c != http.StatusBadRequest {
		t.Fatalf("negative: %d", c)
	}
	if rec := do(t, h, "POST", "/api/plays", body); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("no content type: %d", rec.Code)
	}
}

func TestExploreFollowsLikes(t *testing.T) {
	st := newStore(t)
	s := &Server{Store: st}
	h := s.Handler()
	_, al := ids(t, st)
	total := func() int {
		var r catalog.Result
		decode(t, do(t, h, "GET", "/api/explore?megusta=si", ""), &r)
		return r.Total
	}
	if n := total(); n != 0 {
		t.Fatalf("total = %d", n)
	}
	built := s.cat.cat
	if built == nil {
		t.Fatal("catalog not built")
	}
	body := func(l string) string { return `{"kind":"album","id":` + itoa(al) + `,"liked":` + l + `}` }
	if c := put(t, h, "PUT", "/api/likes", body("true")).Code; c != http.StatusNoContent {
		t.Fatalf("like: %d", c)
	}
	if n := total(); n != 1 {
		t.Fatalf("after like: %d", n)
	}
	if s.cat.cat != built {
		t.Fatal("the like rebuilt the catalog")
	}
	put(t, h, "PUT", "/api/likes", body("false"))
	if n := total(); n != 0 {
		t.Fatalf("after unlike: %d", n)
	}
}

func TestLikedEndpoint(t *testing.T) {
	st := newStore(t)
	h := (&Server{Store: st}).Handler()
	tr, _ := ids(t, st)
	put(t, h, "PUT", "/api/likes", `{"kind":"track","id":`+itoa(tr)+`,"liked":true}`)
	put(t, h, "PUT", "/api/likes", `{"kind":"artist","key":"virus","liked":true}`)
	rec := do(t, h, "GET", "/api/liked", "")
	var l catalog.LikedLists
	decode(t, rec, &l)
	if len(l.Tracks) != 1 || len(l.Artists) != 1 {
		t.Fatalf("liked = %s", rec.Body)
	}
	var raw map[string]any
	decode(t, rec, &raw)
	for _, k := range []string{"albums", "collections"} {
		if v, ok := raw[k].([]any); !ok || len(v) != 0 {
			t.Fatalf("%s = %v", k, raw[k])
		}
	}
}
