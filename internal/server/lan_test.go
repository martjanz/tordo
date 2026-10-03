package server

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/engine"
	"tordo/internal/store"
)

const (
	testPIN   = "482915"
	phoneAddr = "192.168.1.20:5000"
	macAddr   = "127.0.0.1:5000"
	lanHostIP = "192.168.1.5:4747"
)

// lanServer has LAN enabled with a PIN, a fixed set of addresses and a test
// web app.
func lanServer(t *testing.T) *Server {
	t.Helper()
	hash, err := auth.HashPIN(testPIN)
	if err != nil {
		t.Fatal(err)
	}
	st := newStore(t)
	e := &engine.Engine{AppDir: t.TempDir(), Store: st}
	e.Use(&engine.Runtime{Config: config.Config{Roots: []string{}, Ignore: []string{},
		LAN: config.LAN{Enabled: true, Port: 4747, PinHash: hash}}})
	return &Server{Store: st, Engine: e, Static: fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}},
		interfaces: func() ([]net.IP, string) {
			return []net.IP{net.ParseIP("192.168.1.5"), net.ParseIP("10.0.0.2")}, "mimac"
		}}
}

// lanReq runs a request through the LAN handler.
func lanReq(h http.Handler, method, target, body, remote, host string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = remote
	req.Host = host
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func phone(h http.Handler, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	return lanReq(h, method, target, body, phoneAddr, lanHostIP, headers...)
}

func mac(h http.Handler, method, target, body string, headers ...string) *httptest.ResponseRecorder {
	return lanReq(h, method, target, body, macAddr, "127.0.0.1:4747", headers...)
}

func cookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	return nil
}

func login(h http.Handler, pin, device string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]string{"pin": pin, "device": device})
	return phone(h, "POST", "/api/auth/login", string(b), "Content-Type", "application/json")
}

// withCookie is a header list carrying the session cookie.
func withCookie(c *http.Cookie, more ...string) []string {
	return append([]string{"Cookie", c.Name + "=" + c.Value}, more...)
}

func TestLANHostCheck(t *testing.T) {
	for _, h := range []string{"192.168.1.5:4747", "[fe80::1]:4747", "localhost:4747", "mimac.local:4747", "MIMAC:4747", "192.168.1.5"} {
		if !lanHost(h, "mimac") {
			t.Errorf("%q rejected", h)
		}
	}
	for _, h := range []string{"evil.example:4747", "mimac.evil.example", "mimac.evil.example:4747", "", ".local", ":4747"} {
		if lanHost(h, "mimac") {
			t.Errorf("%q accepted", h)
		}
	}
	if lanHost(".local", "") || lanHost("", "") {
		t.Error("empty hostname matches")
	}
}

func TestLANGate(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	for _, p := range []string{"/api/status", "/stream/1"} {
		if rec := phone(h, "GET", p, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without cookie: %d", p, rec.Code)
		}
	}
	if rec := phone(h, "GET", "/", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
		t.Errorf("/: %d %s", rec.Code, rec.Body)
	}
	var a struct {
		Local, Session, PinRequired bool
		Device                      string
	}
	rec := phone(h, "GET", "/api/auth", "")
	decode(t, rec, &a)
	if a.Local || a.Session || !a.PinRequired {
		t.Errorf("auth = %+v", a)
	}
	if rec := mac(h, "GET", "/api/status", ""); rec.Code != http.StatusOK {
		t.Errorf("local status: %d", rec.Code)
	}
	if rec := mac(h, "GET", "/api/config", ""); rec.Code != http.StatusOK {
		t.Errorf("local config: %d", rec.Code)
	}
	decode(t, mac(h, "GET", "/api/auth", ""), &a)
	if !a.Local || a.PinRequired {
		t.Errorf("local auth = %+v", a)
	}
	// Loopback but addressed by LAN name: it is not the computer's browser.
	if rec := lanReq(h, "GET", "/api/status", "", macAddr, lanHostIP); rec.Code != http.StatusUnauthorized {
		t.Errorf("loopback with LAN host: %d", rec.Code)
	}
	// A phone cannot claim to be local with a loopback Host.
	if rec := lanReq(h, "GET", "/api/config", "", phoneAddr, "127.0.0.1:4747"); rec.Code != http.StatusUnauthorized {
		t.Errorf("phone with loopback host: %d", rec.Code)
	}
	if rec := lanReq(h, "GET", "/api/status", "", phoneAddr, "evil.example:4747"); rec.Code != http.StatusForbidden {
		t.Errorf("foreign host: %d", rec.Code)
	}
	if rec := phone(h, "GET", "/api/status", "", "Sec-Fetch-Site", "cross-site"); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site: %d", rec.Code)
	}
	if rec := phone(h, "GET", "/", ""); rec.Header().Get("Cross-Origin-Resource-Policy") != "same-origin" {
		t.Errorf("CORP = %q", rec.Header().Get("Cross-Origin-Resource-Policy"))
	}
}

func TestLogin(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	if rec := login(h, "000000", "Pixel"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong PIN: %d", rec.Code)
	}
	if rec := phone(h, "POST", "/api/auth/login", `{"pin":"`+testPIN+`"}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("no JSON content type: %d", rec.Code)
	}
	rec := login(h, testPIN, "Pixel")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	c := cookieOf(rec)
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 7776000 || c.Path != "/" || c.Value == "" {
		t.Fatalf("cookie = %+v", c)
	}
	if got := phone(h, "GET", "/api/status", "", withCookie(c)...); got.Code != http.StatusOK {
		t.Fatalf("status with cookie: %d", got.Code)
	}
	var a struct {
		Local, Session, PinRequired bool
		Device                      string
	}
	decode(t, phone(h, "GET", "/api/auth", "", withCookie(c)...), &a)
	if !a.Session || a.Device != "Pixel" || a.PinRequired || a.Local {
		t.Fatalf("auth = %+v", a)
	}
	if got := phone(h, "GET", "/api/config", "", withCookie(c)...); got.Code != http.StatusForbidden {
		t.Fatalf("config with cookie: %d", got.Code)
	}
	// A garbage cookie is cleared.
	got := phone(h, "GET", "/api/status", "", "Cookie", sessionCookie+"=nope")
	if cc := cookieOf(got); got.Code != http.StatusUnauthorized || cc == nil || cc.MaxAge >= 0 {
		t.Fatalf("garbage cookie: %d %+v", got.Code, cc)
	}
	// Local computers do not log in.
	if got := mac(h, "POST", "/api/auth/login", `{"pin":"`+testPIN+`"}`, "Content-Type", "application/json"); got.Code != http.StatusBadRequest {
		t.Fatalf("local login: %d", got.Code)
	}
}

func TestLoginLimiter(t *testing.T) {
	s := lanServer(t)
	clock := time.Unix(1_800_000_000, 0)
	s.limiter.Now = func() time.Time { return clock }
	h := s.LANHandler()
	for i := 1; i <= 6; i++ {
		if rec := login(h, "000000", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong PIN %d: %d", i, rec.Code)
		}
	}
	rec := login(h, testPIN, "")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("locked: %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "30 s") {
		t.Fatalf("body = %q", rec.Body)
	}
	clock = clock.Add(30 * time.Second)
	if rec := login(h, testPIN, ""); rec.Code != http.StatusOK {
		t.Fatalf("after the lock: %d", rec.Code)
	}
}

func TestLoginClosedAndReadOnly(t *testing.T) {
	s := lanServer(t)
	s.Engine.Use(&engine.Runtime{Config: config.Config{Roots: []string{}, Ignore: []string{}, LAN: config.LAN{Port: 4747}}})
	if rec := login(s.LANHandler(), testPIN, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("LAN off: %d", rec.Code)
	}
	s = lanServer(t)
	s.ReadOnly = true
	if rec := login(s.LANHandler(), testPIN, ""); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only: %d", rec.Code)
	}
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: freePort(t)}); err == nil {
		t.Fatal("browse-only opened the LAN")
	}
}

func TestSessionsEndpoints(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	c1 := cookieOf(login(h, testPIN, "Pixel"))
	c2 := cookieOf(login(h, testPIN, "iPad"))
	var list []store.Session
	decode(t, mac(h, "GET", "/api/sessions", ""), &list)
	if len(list) != 2 {
		t.Fatalf("sessions = %+v", list)
	}
	if rec := phone(h, "GET", "/api/sessions", "", withCookie(c1)...); rec.Code != http.StatusForbidden {
		t.Fatalf("remote list: %d", rec.Code)
	}
	if rec := phone(h, "DELETE", "/api/sessions/"+strconv.FormatInt(list[0].ID, 10), "", withCookie(c1)...); rec.Code != http.StatusForbidden {
		t.Fatalf("remote delete: %d", rec.Code)
	}
	var gone *http.Cookie
	for _, se := range list {
		if se.Device == "Pixel" {
			if rec := mac(h, "DELETE", "/api/sessions/"+strconv.FormatInt(se.ID, 10), ""); rec.Code != http.StatusNoContent {
				t.Fatalf("delete: %d", rec.Code)
			}
			gone = c1
			if rec := mac(h, "DELETE", "/api/sessions/"+strconv.FormatInt(se.ID, 10), ""); rec.Code != http.StatusNotFound {
				t.Fatalf("delete again: %d", rec.Code)
			}
		}
	}
	rec := phone(h, "GET", "/api/status", "", withCookie(gone)...)
	if cc := cookieOf(rec); rec.Code != http.StatusUnauthorized || cc == nil || cc.MaxAge >= 0 {
		t.Fatalf("closed session: %d %+v", rec.Code, cc)
	}
	rec = phone(h, "POST", "/api/auth/logout", "{}", withCookie(c2, "Content-Type", "application/json")...)
	if cc := cookieOf(rec); rec.Code != http.StatusOK || cc == nil || cc.MaxAge >= 0 {
		t.Fatalf("logout: %d %+v", rec.Code, cc)
	}
	if rec := phone(h, "GET", "/api/status", "", withCookie(c2)...); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", rec.Code)
	}
	if rec := phone(h, "POST", "/api/auth/logout", "{}", "Content-Type", "application/json"); rec.Code != http.StatusOK {
		t.Fatalf("logout without a session: %d", rec.Code)
	}
}

func TestSessionTouch(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	c := cookieOf(login(h, testPIN, "Pixel"))
	seen := func() int64 {
		se, ok, err := s.Store.SessionByToken(auth.TokenHash(c.Value))
		if err != nil || !ok {
			t.Fatalf("session: %v %v", ok, err)
		}
		return se.LastSeen
	}
	before := seen()
	s.now = func() time.Time { return time.Now().Add(time.Hour) }
	rec := phone(h, "GET", "/api/status", "", withCookie(c)...)
	if rec.Code != http.StatusOK || cookieOf(rec) != nil {
		t.Fatalf("1 hour: %d cookie %+v", rec.Code, cookieOf(rec))
	}
	if seen() != before {
		t.Fatal("touched after one hour")
	}
	s.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
	time.Sleep(2 * time.Millisecond)
	rec = phone(h, "GET", "/api/status", "", withCookie(c)...)
	nc := cookieOf(rec)
	if rec.Code != http.StatusOK || nc == nil || nc.Value != c.Value || nc.MaxAge != 7776000 {
		t.Fatalf("2 days: %d cookie %+v", rec.Code, nc)
	}
	if seen() <= before {
		t.Fatal("last seen not updated")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestApplyLAN(t *testing.T) {
	s := lanServer(t)
	t.Cleanup(func() { s.ApplyLAN(config.LAN{}) })
	port := freePort(t)
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port}); err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/api/auth"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port}); err != nil {
		t.Fatalf("same port again: %v", err)
	}
	// Another port moves it.
	port2 := freePort(t)
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port2}); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get(url); err == nil {
		t.Fatal("old port still open")
	}
	// A taken port: an error naming it, the listener stays where it was.
	taken, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	tp := taken.Addr().(*net.TCPAddr).Port
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: tp}); err == nil || !strings.Contains(err.Error(), strconv.Itoa(tp)) {
		t.Fatalf("taken port: %v", err)
	}
	if err := s.ApplyLAN(config.LAN{Enabled: false, Port: port2}); err != nil {
		t.Fatal(err)
	}
	if _, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port2) + "/api/auth"); err == nil {
		t.Fatal("still open after disabling")
	}
}

func TestApplyLANReopenAtOnce(t *testing.T) {
	s := lanServer(t)
	t.Cleanup(func() { s.ApplyLAN(config.LAN{}) })
	port := freePort(t)
	for i := 0; i < 50; i++ {
		if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port}); err != nil {
			t.Fatalf("enable %d: %v", i, err)
		}
		if err := s.ApplyLAN(config.LAN{}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLANURLs(t *testing.T) {
	ips := []net.IP{net.ParseIP("192.168.1.5"), net.ParseIP("10.0.0.2"), net.ParseIP("8.8.8.8"), net.ParseIP("127.0.0.1"), net.ParseIP("fe80::1")}
	got := lanURLs(ips, "mimac", 4747)
	want := []string{"http://192.168.1.5:4747/", "http://10.0.0.2:4747/", "http://mimac.local:4747/"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("urls = %v", got)
	}
	if got := lanURLs(ips, "", 4747); len(got) != 2 {
		t.Fatalf("no hostname: %v", got)
	}
	if got := lanURLs(nil, "", 4747); got == nil || len(got) != 0 {
		t.Fatalf("empty: %#v", got)
	}
}

func TestLANStateAndQR(t *testing.T) {
	s := lanServer(t)
	t.Cleanup(func() { s.ApplyLAN(config.LAN{}) })
	h := s.LANHandler()
	var st LANState
	decode(t, mac(h, "GET", "/api/lan", ""), &st)
	if !st.Enabled || st.Port != 4747 || st.Listening || st.URLs == nil || len(st.URLs) != 0 {
		t.Fatalf("not listening: %+v", st)
	}
	if rec := mac(h, "GET", "/api/lan/qr?url=http://192.168.1.5:4747/", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("qr while not listening: %d", rec.Code)
	}
	port := freePort(t)
	s.Engine.Use(&engine.Runtime{Config: config.Config{Roots: []string{}, Ignore: []string{}, LAN: config.LAN{Enabled: true, Port: port, PinHash: "x"}}})
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port}); err != nil {
		t.Fatal(err)
	}
	decode(t, mac(h, "GET", "/api/lan", ""), &st)
	p := strconv.Itoa(port)
	if !st.Listening || len(st.URLs) != 3 || st.URLs[0] != "http://192.168.1.5:"+p+"/" || st.URLs[2] != "http://mimac.local:"+p+"/" {
		t.Fatalf("listening: %+v", st)
	}
	rec := mac(h, "GET", "/api/lan/qr?url="+st.URLs[0], "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/svg+xml" ||
		rec.Header().Get("Cache-Control") != "no-store" || !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("qr: %d %v %.40s", rec.Code, rec.Header(), rec.Body)
	}
	if rec := mac(h, "GET", "/api/lan/qr?url=http://evil.example/", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("other url: %d", rec.Code)
	}
	if rec := phone(h, "GET", "/api/lan", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("phone without session: %d", rec.Code)
	}
}

func TestLANListenerIsIPv4Only(t *testing.T) {
	s := lanServer(t)
	t.Cleanup(func() { s.ApplyLAN(config.LAN{}) })
	port := freePort(t)
	if err := s.ApplyLAN(config.LAN{Enabled: true, Port: port}); err != nil {
		t.Fatal(err)
	}
	s.lan.mu.Lock()
	addr := s.lan.ln.Addr().String()
	s.lan.mu.Unlock()
	if want := "0.0.0.0:" + strconv.Itoa(port); addr != want {
		t.Fatalf("listening on %s, want %s", addr, want)
	}
}

func TestLANGateRefusesPublicAddresses(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	c := cookieOf(login(h, testPIN, "Pixel"))
	if c == nil {
		t.Fatal("no session")
	}
	for _, addr := range []string{"8.8.8.8:5000", "[2001:db8::1]:5000", "garbage"} {
		for _, withSession := range []bool{true, false} {
			var hdr []string
			if withSession {
				hdr = withCookie(c)
			}
			if rec := lanReq(h, "GET", "/api/status", "", addr, lanHostIP, hdr...); rec.Code != http.StatusForbidden {
				t.Errorf("%s (session %v): %d", addr, withSession, rec.Code)
			}
		}
	}
	for _, addr := range []string{"192.168.1.20:5000", "10.1.2.3:5000", "172.16.0.9:5000", "169.254.1.1:5000", "[fe80::1]:5000", "[fd00::1]:5000"} {
		if rec := lanReq(h, "GET", "/api/status", "", addr, lanHostIP, withCookie(c)...); rec.Code != http.StatusOK {
			t.Errorf("%s: %d", addr, rec.Code)
		}
	}
}

func TestSessionExpiresAfter90IdleDays(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	c := cookieOf(login(h, testPIN, "Pixel"))
	exists := func() bool {
		_, ok, err := s.Store.SessionByToken(auth.TokenHash(c.Value))
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	day := 24 * time.Hour
	s.now = func() time.Time { return time.Now().Add(89 * day) }
	if rec := phone(h, "GET", "/api/status", "", withCookie(c)...); rec.Code != http.StatusOK || cookieOf(rec) == nil {
		t.Fatalf("89 days: %d cookie %+v", rec.Code, cookieOf(rec))
	}
	if !exists() {
		t.Fatal("session gone after 89 days")
	}

	// A session idle for 91 days is closed, whichever way it is used.
	c2 := cookieOf(login(h, testPIN, "Viejo"))
	s.now = func() time.Time { return time.Now().Add(91 * day) }
	var a struct{ Session, PinRequired bool }
	decode(t, phone(h, "GET", "/api/auth", "", withCookie(c2)...), &a)
	if a.Session || !a.PinRequired {
		t.Fatalf("auth of an expired session: %+v", a)
	}
	rec := phone(h, "GET", "/api/status", "", withCookie(c2)...)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "ingresá el PIN") {
		t.Fatalf("expired: %d %s", rec.Code, rec.Body)
	}
	if cc := cookieOf(rec); cc == nil || cc.MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", cc)
	}
	if _, ok, _ := s.Store.SessionByToken(auth.TokenHash(c2.Value)); ok {
		t.Fatal("expired session still stored")
	}
}

// Every route behind the LAN listener needs a session, except the ones decision
// 14 names and the static shell. A route added to routeTable without the gate
// knowing about it fails here.
func TestLANGateCoversEveryRoute(t *testing.T) {
	s := lanServer(t)
	h := s.LANHandler()
	open := map[string]bool{"GET /api/auth": true, "POST /api/auth/login": true, "POST /api/auth/logout": true, "GET /": true}
	idRe := regexp.MustCompile(`\{[^}]*\}`)
	routes := s.routeTable()
	if len(routes) < 50 {
		t.Fatalf("only %d routes", len(routes))
	}
	for _, rt := range routes {
		if open[rt.pattern] {
			continue
		}
		method, path, _ := strings.Cut(rt.pattern, " ")
		path = idRe.ReplaceAllString(path, "1")
		rec := phone(h, method, path, "", "Content-Type", "application/json")
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d", rt.pattern, rec.Code)
		}
	}
}
