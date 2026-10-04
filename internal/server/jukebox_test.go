package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// sse is an open event stream.
type sse struct {
	resp *http.Response
	r    *bufio.Reader
}

func (e *sse) close() { e.resp.Body.Close() }

// readEvent skips pings and reads one event, failing after 2 s.
func readEvent(t *testing.T, e *sse) (typ, data string) {
	t.Helper()
	type ev struct{ typ, data string }
	ch := make(chan ev, 1)
	go func() {
		var cur ev
		for {
			line, err := e.r.ReadString('\n')
			if err != nil {
				ch <- ev{"EOF", err.Error()}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, ":"):
			case strings.HasPrefix(line, "event: "):
				cur.typ = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				cur.data = strings.TrimPrefix(line, "data: ")
			case line == "" && cur.typ != "":
				ch <- cur
				return
			}
		}
	}()
	select {
	case v := <-ch:
		return v.typ, v.data
	case <-time.After(2 * time.Second):
		t.Fatal("no event within 2 s")
		return "", ""
	}
}

func wantEvent(t *testing.T, e *sse, typ string) string {
	t.Helper()
	got, data := readEvent(t, e)
	if got != typ {
		t.Fatalf("event %q (%s), want %q", got, data, typ)
	}
	return data
}

func openSSE(t *testing.T, url string, headers ...string) *sse {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("GET %s: %d %s", url, resp.StatusCode, b)
	}
	e := &sse{resp, bufio.NewReader(resp.Body)}
	t.Cleanup(e.close)
	return e
}

// jukeboxRig is a LAN handler served for real, once as the computer and once
// as a phone.
type jukeboxRig struct {
	s     *Server
	h     http.Handler
	local *httptest.Server
	lan   *httptest.Server
}

func newRig(t *testing.T) *jukeboxRig {
	t.Helper()
	s := lanServer(t)
	s.beat = 20 * time.Millisecond
	h := s.LANHandler()
	local := httptest.NewServer(h)
	lan := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = phoneAddr
		r.Host = lanHostIP
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(local.Close)
	t.Cleanup(lan.Close)
	return &jukeboxRig{s, h, local, lan}
}

// phoneSession logs a phone in and returns its cookie and session id.
func (g *jukeboxRig) phoneSession(t *testing.T, device string) (*http.Cookie, int64) {
	t.Helper()
	rec := login(g.h, testPIN, device)
	c := cookieOf(rec)
	if rec.Code != http.StatusOK || c == nil {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	var list []struct {
		ID     int64
		Device string
	}
	decode(t, mac(g.h, "GET", "/api/sessions", ""), &list)
	for _, se := range list {
		if se.Device == device {
			return c, se.ID
		}
	}
	t.Fatal("session not listed")
	return nil, 0
}

func (g *jukeboxRig) post(t *testing.T, base, path, body string, headers ...string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func wantStatus(t *testing.T, resp *http.Response, code int, text string) {
	t.Helper()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != code {
		t.Fatalf("status %d (%s), want %d", resp.StatusCode, b, code)
	}
	if text != "" && !strings.Contains(string(b), text) {
		t.Fatalf("body %q, want %q", b, text)
	}
}

func field(t *testing.T, data, key string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(m[key])
}

func speakerOn(t *testing.T, g *jukeboxRig) (*sse, string) {
	t.Helper()
	sp := openSSE(t, g.local.URL+"/api/jukebox/speaker")
	return sp, field(t, wantEvent(t, sp, "hello"), "token")
}

func jukeboxSpeaking(t *testing.T, g *jukeboxRig) bool {
	t.Helper()
	var st struct{ Jukebox struct{ Speaker bool } }
	decode(t, mac(g.h, "GET", "/api/status", ""), &st)
	return st.Jukebox.Speaker
}

func TestJukeboxSpeakerFlow(t *testing.T) {
	g := newRig(t)
	sp, token := speakerOn(t, g)
	if token == "" || !jukeboxSpeaking(t, g) {
		t.Fatalf("token %q, speaking %v", token, jukeboxSpeaking(t, g))
	}
	rem := openSSE(t, g.local.URL+"/api/jukebox/events")
	hello := wantEvent(t, rem, "hello")
	if field(t, hello, "session") != "0" || field(t, hello, "role") != "host" {
		t.Errorf("hello = %s", hello)
	}
	remote := field(t, hello, "remote")

	body := fmt.Sprintf(`{"token":%q,"state":{"x":1}}`, token)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/state", body), http.StatusNoContent, "")
	if got := wantEvent(t, rem, "state"); got != `{"x":1}` {
		t.Errorf("state = %s", got)
	}

	cmd := fmt.Sprintf(`{"remote":%q,"command":"next"}`, remote)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/command", cmd), http.StatusAccepted, "")
	got := wantEvent(t, sp, "command")
	if !strings.Contains(got, `"command":"next"`) || !strings.Contains(got, `"args":{}`) {
		t.Errorf("command = %s", got)
	}

	notice := fmt.Sprintf(`{"token":%q,"remote":%q,"text":"  Listo  "}`, token, remote)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/notice", notice), http.StatusNoContent, "")
	if got := wantEvent(t, rem, "notice"); field(t, got, "text") != "Listo" {
		t.Errorf("notice = %s", got)
	}

	sp.close()
	wantEvent(t, rem, "speaker-off")
	deadline := time.Now().Add(2 * time.Second)
	for jukeboxSpeaking(t, g) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if jukeboxSpeaking(t, g) {
		t.Error("still speaking after the speaker left")
	}
}

func TestJukeboxReplaced(t *testing.T) {
	g := newRig(t)
	first, oldToken := speakerOn(t, g)
	speakerOn(t, g)
	wantEvent(t, first, "replaced")
	if typ, _ := readEvent(t, first); typ != "EOF" {
		t.Errorf("after replaced: %q, want EOF", typ)
	}
	body := fmt.Sprintf(`{"token":%q,"state":{"x":1}}`, oldToken)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/state", body), http.StatusConflict, "")
}

func TestJukeboxGuestLimits(t *testing.T) {
	g := newRig(t)
	speakerOn(t, g)
	c, id := g.phoneSession(t, "Celu")
	rem := openSSE(t, g.lan.URL+"/api/jukebox/events", withCookie(c)...)
	hello := wantEvent(t, rem, "hello")
	if field(t, hello, "role") != "guest" || field(t, hello, "session") != fmt.Sprint(id) {
		t.Fatalf("hello = %s", hello)
	}
	remote := field(t, hello, "remote")
	send := func(command string) *http.Response {
		return g.post(t, g.lan.URL, "/api/jukebox/command",
			fmt.Sprintf(`{"remote":%q,"command":%q}`, remote, command), withCookie(c)...)
	}
	wantStatus(t, send("next"), http.StatusForbidden, "Solo el anfitrión puede hacer esto.")
	wantStatus(t, send("enqueue"), http.StatusAccepted, "")

	put := mac(g.h, "PUT", fmt.Sprintf("/api/sessions/%d/host", id), `{"host":true}`, "Content-Type", "application/json")
	if put.Code != http.StatusNoContent {
		t.Fatalf("PUT host: %d %s", put.Code, put.Body)
	}
	if got := wantEvent(t, rem, "role"); field(t, got, "role") != "host" {
		t.Errorf("role = %s", got)
	}
	// The next request reads the new mark from the store.
	wantStatus(t, send("next"), http.StatusAccepted, "")
}

func TestJukeboxRemoteOfAnotherSession(t *testing.T) {
	g := newRig(t)
	speakerOn(t, g)
	a, _ := g.phoneSession(t, "A")
	b, _ := g.phoneSession(t, "B")
	rem := openSSE(t, g.lan.URL+"/api/jukebox/events", withCookie(a)...)
	remote := field(t, wantEvent(t, rem, "hello"), "remote")
	body := fmt.Sprintf(`{"remote":%q,"command":"enqueue"}`, remote)
	wantStatus(t, g.post(t, g.lan.URL, "/api/jukebox/command", body, withCookie(b)...), http.StatusNotFound, "")
}

func TestJukeboxNoSpeaker(t *testing.T) {
	g := newRig(t)
	rem := openSSE(t, g.local.URL+"/api/jukebox/events")
	remote := field(t, wantEvent(t, rem, "hello"), "remote")
	wantEvent(t, rem, "speaker-off")
	body := fmt.Sprintf(`{"remote":%q,"command":"next"}`, remote)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/command", body), http.StatusConflict,
		"La rockola no está disponible: activá el audio en la compu.")
}

func TestJukeboxSpeakerLocalOnly(t *testing.T) {
	g := newRig(t)
	c, _ := g.phoneSession(t, "Celu")
	for _, rt := range [][2]string{
		{"GET", "/api/jukebox/speaker"}, {"POST", "/api/jukebox/state"}, {"POST", "/api/jukebox/notice"}, {"PUT", "/api/sessions/1/host"},
	} {
		rec := phone(g.h, rt[0], rt[1], `{"token":"x","state":{},"host":true}`, withCookie(c, "Content-Type", "application/json")...)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s from a phone: %d", rt[0], rt[1], rec.Code)
		}
	}
}

func TestJukeboxLimits(t *testing.T) {
	g := newRig(t)
	_, token := speakerOn(t, g)
	rem := openSSE(t, g.local.URL+"/api/jukebox/events")
	remote := field(t, wantEvent(t, rem, "hello"), "remote")

	big := fmt.Sprintf(`{"token":%q,"state":{"pad":%q}}`, token, strings.Repeat("x", 256<<10))
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/state", big), http.StatusBadRequest, "")
	empty := fmt.Sprintf(`{"token":%q}`, token)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/state", empty), http.StatusBadRequest, "")
	bigCmd := fmt.Sprintf(`{"remote":%q,"command":"enqueue","args":{"pad":%q}}`, remote, strings.Repeat("x", 16<<10))
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/command", bigCmd), http.StatusBadRequest, "")
	bogus := fmt.Sprintf(`{"remote":%q,"command":"bogus"}`, remote)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/command", bogus), http.StatusBadRequest, "")
	noText := fmt.Sprintf(`{"token":%q,"remote":%q,"text":"   "}`, token, remote)
	wantStatus(t, g.post(t, g.local.URL, "/api/jukebox/notice", noText), http.StatusBadRequest, "")

	_, id := g.phoneSession(t, "Celu")
	put := func(path, body string) int {
		return mac(g.h, "PUT", path, body, "Content-Type", "application/json").Code
	}
	if code := put(fmt.Sprintf("/api/sessions/%d/host", id), `{}`); code != http.StatusBadRequest {
		t.Errorf("host missing: %d", code)
	}
	if code := put("/api/sessions/999/host", `{"host":true}`); code != http.StatusNotFound {
		t.Errorf("unknown session: %d", code)
	}
}

func TestJukeboxTooManyRemotes(t *testing.T) {
	g := newRig(t)
	for i := 0; i < 16; i++ {
		wantEvent(t, openSSE(t, g.local.URL+"/api/jukebox/events"), "hello")
	}
	resp, err := http.Get(g.local.URL + "/api/jukebox/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	wantStatus(t, resp, http.StatusServiceUnavailable, "Demasiados controles")
}

func TestJukeboxDropsOnSignOut(t *testing.T) {
	g := newRig(t)
	a, idA := g.phoneSession(t, "A")
	b, _ := g.phoneSession(t, "B")
	remA := openSSE(t, g.lan.URL+"/api/jukebox/events", withCookie(a)...)
	remB := openSSE(t, g.lan.URL+"/api/jukebox/events", withCookie(b)...)
	wantEvent(t, remA, "hello")
	wantEvent(t, remB, "hello")

	if rec := mac(g.h, "DELETE", fmt.Sprintf("/api/sessions/%d", idA), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE session: %d", rec.Code)
	}
	for {
		typ, _ := readEvent(t, remA)
		if typ == "EOF" {
			break
		}
	}
	if rec := phone(g.h, "POST", "/api/auth/logout", "", withCookie(b, "Content-Type", "application/json")...); rec.Code != http.StatusOK {
		t.Fatalf("logout: %d", rec.Code)
	}
	for {
		typ, _ := readEvent(t, remB)
		if typ == "EOF" {
			break
		}
	}
}

func TestJukeboxHeartbeat(t *testing.T) {
	g := newRig(t)
	rem := openSSE(t, g.local.URL+"/api/jukebox/events")
	wantEvent(t, rem, "hello")
	wantEvent2 := make(chan string, 1)
	go func() {
		for {
			line, err := rem.r.ReadString('\n')
			if err != nil || strings.HasPrefix(line, ": ping") {
				wantEvent2 <- line
				return
			}
		}
	}()
	select {
	case line := <-wantEvent2:
		if !strings.HasPrefix(line, ": ping") {
			t.Errorf("got %q", line)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no ping within 200 ms")
	}
}
