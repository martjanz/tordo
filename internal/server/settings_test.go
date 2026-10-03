package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/discogs"
	"tordo/internal/engine"
	"tordo/internal/httpx"
	"tordo/internal/identify"
)

// noMB stands for MusicBrainz: nothing in these tests may call it.
type noMB struct{ identify.API }

// settingsServer is a first start: <tmp>/tordo next to musica/ and otra/ (both
// empty), no config.json, and a Discogs check that answers verify.
func settingsServer(t *testing.T, readOnly bool) *Server {
	t.Helper()
	disk := t.TempDir()
	appDir := filepath.Join(disk, "tordo")
	for _, d := range []string{appDir, filepath.Join(disk, "musica"), filepath.Join(disk, "otra")} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	st := newStore(t)
	e := &engine.Engine{AppDir: appDir, Store: st, MB: noMB{}, ReadOnly: readOnly}
	s := &Server{AppDir: appDir, Store: st, Engine: e}
	e.Start(config.Config{Roots: []string{}, Ignore: []string{}}, true)
	t.Cleanup(e.Close)
	return s
}

func putConfig(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	return do(t, s.Handler(), "PUT", "/api/config", body, "Content-Type", "application/json")
}

func getConfig(t *testing.T, s *Server) configView {
	t.Helper()
	var v configView
	decode(t, do(t, s.Handler(), "GET", "/api/config", ""), &v)
	return v
}

func saved(t *testing.T, s *Server) config.Config {
	t.Helper()
	cfg, missing, err := config.Load(s.AppDir)
	if err != nil || missing {
		t.Fatalf("config.json: missing=%v err=%v", missing, err)
	}
	return cfg
}

func TestGetConfigPending(t *testing.T) {
	s := settingsServer(t, false)
	rec := do(t, s.Handler(), "GET", "/api/config", "")
	var v configView
	decode(t, rec, &v)
	if !v.SetupPending || v.ReadOnly || len(v.Roots) != 0 || v.Roots == nil || !v.Normalize {
		t.Fatalf("view = %+v", v)
	}
	if len(v.Suggested) != 2 || v.Suggested[0] != "../musica" || v.Suggested[1] != "../otra" {
		t.Fatalf("suggested = %v", v.Suggested)
	}
	if v.LAN.Enabled || v.LAN.Port != config.DefaultPort || v.LAN.HasPin {
		t.Fatalf("lan = %+v", v.LAN)
	}
	for _, leak := range []string{"pinHash", "discogsToken"} {
		if strings.Contains(rec.Body.String(), leak) {
			t.Fatalf("%s in %s", leak, rec.Body)
		}
	}
}

func TestPutConfigSaves(t *testing.T) {
	s := settingsServer(t, false)
	if rec := putConfig(t, s, `{"roots":["../musica"],"ignore":["../x"],"normalize":true,"lan":{"port":4747}}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("ignore with ..: %d %s", rec.Code, rec.Body)
	}
	body := func(token, extra string) string {
		return `{"roots":["../musica"],"ignore":[" Podcasts ",""],"token":` + token + `,"normalize":false,"lan":{"enabled":false,"port":4747` + extra + `}}`
	}
	var v configView
	decode(t, putConfig(t, s, body(`"abcd1234efgh"`, "")), &v)
	if v.SetupPending || v.Normalize || !v.HasToken || v.TokenHint != "…efgh" || len(v.Roots) != 1 || !v.Roots[0].Available {
		t.Fatalf("view = %+v", v)
	}
	if len(v.Ignore) != 1 || v.Ignore[0] != "Podcasts" || len(v.Suggested) != 1 || v.Suggested[0] != "../otra" {
		t.Fatalf("view = %+v", v)
	}
	cfg := saved(t, s)
	if cfg.DiscogsToken != "abcd1234efgh" || cfg.Normalizing() || cfg.Roots[0] != "../musica" || cfg.Ignore[0] != "Podcasts" {
		t.Fatalf("saved = %+v", cfg)
	}
	var status struct {
		Normalize    bool `json:"normalize"`
		SetupPending bool `json:"setupPending"`
	}
	status.Normalize = true
	decode(t, do(t, s.Handler(), "GET", "/api/status", ""), &status)
	if status.Normalize || status.SetupPending {
		t.Fatalf("status = %+v", status)
	}

	decode(t, putConfig(t, s, body(`null`, "")), &v)
	if !v.HasToken || saved(t, s).DiscogsToken != "abcd1234efgh" {
		t.Fatalf("null token dropped it: %+v", v)
	}
	decode(t, putConfig(t, s, body(`""`, "")), &v)
	if v.HasToken || v.TokenHint != "" || saved(t, s).DiscogsToken != "" {
		t.Fatalf("empty token kept it: %+v", v)
	}
}

func TestPutConfigLAN(t *testing.T) {
	s := settingsServer(t, false)
	body := func(enabled bool, port int, pin string) string {
		return `{"roots":["../musica"],"ignore":[],"normalize":true,"lan":{"enabled":` + map[bool]string{true: "true", false: "false"}[enabled] +
			`,"port":` + itoa(int64(port)) + `,"pin":` + pin + `}}`
	}
	put := func(b string) *httptest.ResponseRecorder { return putConfig(t, s, b) }

	if rec := put(body(true, 4747, `null`)); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "elegí un PIN") {
		t.Fatalf("no PIN: %d %s", rec.Code, rec.Body)
	}
	if rec := put(body(true, 4747, `"12"`)); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), auth.ErrPINFormat.Error()) {
		t.Fatalf("short PIN: %d %s", rec.Code, rec.Body)
	}
	if rec := put(body(false, 80, `null`)); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "entre 1024 y 65535") {
		t.Fatalf("port 80: %d %s", rec.Code, rec.Body)
	}
	if _, missing, _ := config.Load(s.AppDir); !missing {
		t.Fatal("a rejected request wrote config.json")
	}

	var v configView
	decode(t, put(body(true, 4747, `"482915"`)), &v)
	if !v.LAN.Enabled || !v.LAN.HasPin || v.LAN.Port != 4747 {
		t.Fatalf("lan = %+v", v.LAN)
	}
	hash := saved(t, s).LAN.PinHash
	if !auth.VerifyPIN(hash, "482915") {
		t.Fatal("the saved hash does not verify the PIN")
	}

	if _, err := s.Store.CreateSession("h1", "Teléfono"); err != nil {
		t.Fatal(err)
	}
	decode(t, put(body(true, 4748, `null`)), &v)
	if saved(t, s).LAN.PinHash != hash || v.LAN.Port != 4748 {
		t.Fatalf("null PIN changed the hash: %+v", v.LAN)
	}
	if ss, _ := s.Store.Sessions(); len(ss) != 1 {
		t.Fatalf("sessions after keeping the PIN: %d", len(ss))
	}
	decode(t, put(body(true, 4748, `"739104"`)), &v)
	if h := saved(t, s).LAN.PinHash; h == hash || !auth.VerifyPIN(h, "739104") {
		t.Fatal("the new PIN was not saved")
	}
	if ss, _ := s.Store.Sessions(); len(ss) != 0 {
		t.Fatalf("sessions after a new PIN: %d", len(ss))
	}
}

func TestPutConfigRoots(t *testing.T) {
	s := settingsServer(t, false)
	if rec := putConfig(t, s, `{"roots":["../nada"],"normalize":true,"lan":{"port":4747}}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing root: %d %s", rec.Code, rec.Body)
	}
	if rec := putConfig(t, s, `{`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad JSON: %d", rec.Code)
	}
	if rec := do(t, s.Handler(), "PUT", "/api/config", `{}`); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("not JSON: %d", rec.Code)
	}
}

func TestCheckRootEndpoint(t *testing.T) {
	s := settingsServer(t, false)
	if err := os.WriteFile(filepath.Join(filepath.Dir(s.AppDir), "suelto.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	post := func(path string) *httptest.ResponseRecorder {
		return do(t, s.Handler(), "POST", "/api/config/root", `{"path":"`+path+`"}`, "Content-Type", "application/json")
	}
	var r rootView
	decode(t, post("../musica"), &r)
	if r.Path != "../musica" || !r.Available {
		t.Fatalf("root = %+v", r)
	}
	if rec := post("../suelto.txt"); rec.Code != http.StatusBadRequest {
		t.Fatalf("file: %d %s", rec.Code, rec.Body)
	}
}

func TestCheckTokenEndpoint(t *testing.T) {
	s := settingsServer(t, false)
	post := func(token string) *httptest.ResponseRecorder {
		return do(t, s.Handler(), "POST", "/api/config/token", `{"token":"`+token+`"}`, "Content-Type", "application/json")
	}
	for _, c := range []struct {
		err  error
		want string
	}{{discogs.ErrUnauthorized, `{"valid":false}`}, {httpx.ErrOffline, `{"valid":null}`}, {nil, `{"valid":true}`}} {
		s.VerifyToken = func(context.Context, string) error { return c.err }
		if rec := post("abc"); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != c.want {
			t.Fatalf("%v: %d %s", c.err, rec.Code, rec.Body)
		}
	}
	if rec := post("  "); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty token: %d", rec.Code)
	}
}

func TestSettingsBrowseOnly(t *testing.T) {
	s := settingsServer(t, true)
	if rec := putConfig(t, s, `{"roots":[],"normalize":true,"lan":{"port":4747}}`); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only PUT: %d %s", rec.Code, rec.Body)
	}
	if v := getConfig(t, s); !v.ReadOnly {
		t.Fatalf("view = %+v", v)
	}
}

func TestSettingsNeedAnEngine(t *testing.T) {
	h := (&Server{Store: newStore(t)}).Handler()
	if rec := do(t, h, "GET", "/api/config", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("without engine: %d", rec.Code)
	}
}

func TestEngineBackedScanAndBrowseOnlyParts(t *testing.T) {
	s := settingsServer(t, false)
	if rec := do(t, s.Handler(), "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	ro := settingsServer(t, true)
	if rec := do(t, ro.Handler(), "POST", "/api/scan", "{}", "Content-Type", "application/json"); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only scan: %d", rec.Code)
	}
	if rec := do(t, ro.Handler(), "POST", "/api/identify", "{}", "Content-Type", "application/json"); rec.Code != http.StatusConflict {
		t.Fatalf("browse-only identify: %d", rec.Code)
	}
}

func TestSettingsLocalOnly(t *testing.T) {
	called := false
	h := settings(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest("GET", "/api/config", nil)
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusForbidden || called || !strings.Contains(rec.Body.String(), "Los ajustes solo se cambian en la computadora que corre Tordo.") {
		t.Fatalf("not local: %d %s (called %v)", rec.Code, rec.Body, called)
	}
	rec = httptest.NewRecorder()
	h(rec, withLocal(req))
	if !called {
		t.Fatalf("local request refused: %d", rec.Code)
	}
}
