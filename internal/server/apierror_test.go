package server

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tordo/internal/config"
	"tordo/internal/jukebox"
)

// wantError checks that rec is an error body with the status and code, and
// returns it.
func wantError(t *testing.T, rec *httptest.ResponseRecorder, status int, code errCode) apiError {
	t.Helper()
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("body %q is not an error body: %v", rec.Body, err)
	}
	if rec.Code != status || e.Code != code || e.Message == "" {
		t.Fatalf("got %d %+v, want %d %s", rec.Code, e, status, code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	return e
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusTooManyRequests, codeTooManyAttempts, "too many attempts", map[string]any{"seconds": 12})
	e := wantError(t, rec, http.StatusTooManyRequests, codeTooManyAttempts)
	if e.Message != "too many attempts" || e.Params["seconds"] != float64(12) {
		t.Fatalf("body = %+v", e)
	}

	rec = httptest.NewRecorder()
	writeError(rec, http.StatusNotFound, codeAlbumNotFound, "album not found", nil)
	wantError(t, rec, http.StatusNotFound, codeAlbumNotFound)
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, has := raw["params"]; has {
		t.Fatalf("params in %s", rec.Body)
	}
}

func TestErrorFor(t *testing.T) {
	status, code, msg, _, ok := errorFor(jukebox.ErrForbidden)
	if !ok || status != http.StatusForbidden || code != codeHostOnly || msg == "" {
		t.Fatalf("jukebox.ErrForbidden = %d %s %q %v", status, code, msg, ok)
	}
	status, code, _, params, ok := errorFor(&config.RootError{Err: config.ErrRootNotFound, Path: "musica"})
	if !ok || status != http.StatusBadRequest || code != codeRootNotFound || params["path"] != "musica" {
		t.Fatalf("root = %d %s %v %v", status, code, params, ok)
	}
	_, code, _, params, _ = errorFor(&config.RootError{Err: config.ErrRootsNested, Path: "a", Other: "a/b"})
	if code != codeRootsNested || params["first"] != "a" || params["second"] != "a/b" {
		t.Fatalf("nested = %s %v", code, params)
	}
	if _, _, _, _, ok := errorFor(os.ErrNotExist); ok {
		t.Fatal("an unknown error was mapped")
	}

	rec := httptest.NewRecorder()
	fail(rec, os.ErrNotExist)
	if e := wantError(t, rec, http.StatusInternalServerError, codeInternal); e.Message != "internal error" {
		t.Fatalf("message = %q", e.Message)
	}
}

// codeConstants lists the value of every errCode constant in codes.go.
func codeConstants(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "codes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "errCode" {
			return true
		}
		for _, v := range vs.Values {
			if lit, ok := v.(*ast.BasicLit); ok {
				codes = append(codes, strings.Trim(lit.Value, `"`))
			}
		}
		return true
	})
	return codes
}

// errorBlock returns the text of the `error: { ... }` object of a message file.
func errorBlock(t *testing.T, lang, src string) string {
	t.Helper()
	i := strings.Index(src, "error: {")
	if i < 0 {
		t.Fatalf("%s.js has no error block", lang)
	}
	start := i + len("error: ")
	depth := 0
	for j := start; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[start : j+1]
			}
		}
	}
	t.Fatalf("%s.js: unterminated error block", lang)
	return ""
}

func TestErrorCodesTranslated(t *testing.T) {
	codes := codeConstants(t)
	if len(codes) < 50 {
		t.Fatalf("only %d codes found", len(codes))
	}
	for _, lang := range []string{"en", "es", "pt-BR"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "lib", "i18n", lang+".js"))
		if err != nil {
			t.Fatal(err)
		}
		block := errorBlock(t, lang, string(b))
		for _, c := range codes {
			if !regexp.MustCompile(`\b` + c + `\s*:`).MatchString(block) {
				t.Errorf("%s.js has no error.%s", lang, c)
			}
		}
	}
}

// plainErrorAllowed lists where a plain-text response is right: none of them
// is an API error the UI shows.
var plainErrorAllowed = map[string]string{
	"static.go": "serves the web app's files: a plain 404 for other paths and for missing files",
}

// TestNoPlainHTTPError keeps API errors going through writeError and fail.
func TestNoPlainHTTPError(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile(`http\.(Error|NotFound)\(`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || plainErrorAllowed[f] != "" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if loc := plain.FindIndex(b); loc != nil {
			line := strings.Count(string(b[:loc[0]]), "\n") + 1
			t.Errorf("%s:%d uses a plain-text response; use writeError or fail", f, line)
		}
	}
}
