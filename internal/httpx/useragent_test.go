package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserAgentNamesTheProject(t *testing.T) {
	if !strings.HasPrefix(UserAgent, "Tordo/") || !strings.Contains(UserAgent, "https://github.com/martjanz/tordo") {
		t.Fatalf("UserAgent = %q", UserAgent)
	}
}

func TestGetSendsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()
	c := fastClient()
	c.Header = http.Header{"User-Agent": {UserAgent}}
	if _, err := c.Get(context.Background(), srv.URL); err != nil || got != UserAgent {
		t.Fatalf("err %v, User-Agent %q", err, got)
	}
}
