package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/qr"
	"tordo/internal/store"
)

const (
	sessionCookie = "tordo_sesion"
	sessionDays   = 90
	sessionMaxAge = sessionDays * 24 * time.Hour
	touchAfter    = 24 * time.Hour
)

// lanListener is the second listener, on 0.0.0.0:<port>, while the LAN is on.
type lanListener struct {
	mu   sync.Mutex
	srv  *http.Server
	port int // 0: closed
}

// LANState is what Ajustes shows about the LAN.
type LANState struct {
	Enabled   bool     `json:"enabled"`
	Port      int      `json:"port"`
	Listening bool     `json:"listening"`
	URLs      []string `json:"urls"` // never nil; empty when not listening
}

func (s *Server) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// ApplyLAN opens, moves or closes the LAN listener (0.0.0.0:<port>) to match
// c. Browse-only mode: an error, nothing opened. A port that cannot be opened
// leaves the listener as it was.
func (s *Server) ApplyLAN(c config.LAN) error {
	l := &s.lan
	l.mu.Lock()
	defer l.mu.Unlock()
	if !c.Enabled {
		l.closeLocked()
		return nil
	}
	if s.readOnly() {
		return fmt.Errorf("modo consulta: no se puede abrir el acceso desde la red")
	}
	port := c.PortOrDefault()
	if l.srv != nil && l.port == port {
		return nil
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("no se pudo abrir el puerto %d: %w", port, err)
	}
	l.closeLocked()
	l.srv = &http.Server{Handler: s.LANHandler(), ReadHeaderTimeout: 10 * time.Second}
	l.port = ln.Addr().(*net.TCPAddr).Port
	go func(srv *http.Server) {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("red local: %v", err)
		}
	}(l.srv)
	return nil
}

func (l *lanListener) closeLocked() {
	if l.srv != nil {
		l.srv.Close()
		l.srv, l.port = nil, 0
	}
}

func (s *Server) listeningPort() int {
	s.lan.mu.Lock()
	defer s.lan.mu.Unlock()
	return s.lan.port
}

// hostInfo is the host's up, non-loopback addresses and its lowercase name
// without a trailing ".local".
func (s *Server) hostInfo() ([]net.IP, string) {
	if s.interfaces != nil {
		return s.interfaces()
	}
	var ips []net.IP
	if ifs, err := net.Interfaces(); err == nil {
		for _, ifc := range ifs {
			if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := ifc.Addrs()
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok {
					ips = append(ips, n.IP)
				}
			}
		}
	}
	return ips, hostName()
}

func hostName() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(h), ".local")
}

// currentHostName is the host name the gate compares Host headers with.
func (s *Server) currentHostName() string {
	if s.interfaces != nil {
		_, h := s.interfaces()
		return h
	}
	return hostName()
}

// lanURLs lists the addresses to open Tordo from another device: one per
// private IPv4 address, then the host's .local name (decision 16).
func lanURLs(ips []net.IP, hostname string, port int) []string {
	urls := []string{}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil && v4.IsPrivate() {
			urls = append(urls, fmt.Sprintf("http://%s:%d/", v4, port))
		}
	}
	if hostname != "" {
		urls = append(urls, fmt.Sprintf("http://%s.local:%d/", hostname, port))
	}
	return urls
}

func (s *Server) lanState() LANState {
	cfg := s.parts().lan
	st := LANState{Enabled: cfg.Enabled, Port: cfg.PortOrDefault(), URLs: []string{}}
	if port := s.listeningPort(); port != 0 {
		ips, name := s.hostInfo()
		st.Listening = true
		st.URLs = lanURLs(ips, name, port)
	}
	return st
}

// bareHost is a Host header without its port, lowercased, without the
// brackets of an IPv6 literal.
func bareHost(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

// lanHost tells whether a Host header may reach the LAN listener (decision
// 11): an IP literal, localhost, the machine's name or <name>.local. Anything
// else could be a DNS rebinding.
func lanHost(host, hostname string) bool {
	h := bareHost(host)
	if h == "" {
		return false
	}
	if net.ParseIP(h) != nil || h == "localhost" {
		return true
	}
	hostname = strings.ToLower(hostname)
	return hostname != "" && (h == hostname || h == hostname+".local")
}

// loopbackHost: the names a browser on this computer uses for it.
func loopbackHost(host string) bool {
	h := bareHost(host)
	return h == "localhost" || h == "127.0.0.1" || h == "::1"
}

func loopbackAddr(remote string) bool {
	h, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// openPath: what the LAN listener answers without a session (decision 14).
func openPath(r *http.Request) bool {
	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/stream/") {
		return true
	}
	switch {
	case r.Method == http.MethodGet && p == "/api/auth":
		return true
	case r.Method == http.MethodPost && (p == "/api/auth/login" || p == "/api/auth/logout"):
		return true
	}
	return false
}

func clearCookie() *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

func sessionCookieFor(token string) *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: token, Path: "/", MaxAge: int(sessionMaxAge / time.Second),
		HttpOnly: true, SameSite: http.SameSiteLaxMode}
}

// LANHandler serves the LAN listener: the same routes as Handler, behind the
// LAN gate (decisions 11 and 14).
func (s *Server) LANHandler() http.Handler {
	mux := s.routes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !lanHost(r.Host, s.currentHostName()) {
			http.Error(w, "prohibido", http.StatusForbidden)
			return
		}
		if !sameSiteOnly(w, r) {
			return
		}
		if loopbackAddr(r.RemoteAddr) && loopbackHost(r.Host) {
			mux.ServeHTTP(w, withLocal(r))
			return
		}
		if openPath(r) {
			mux.ServeHTTP(w, r)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			http.SetCookie(w, clearCookie())
			http.Error(w, "ingresá el PIN", http.StatusUnauthorized)
			return
		}
		se, ok, err := s.Store.SessionByToken(auth.TokenHash(c.Value))
		if err != nil {
			fail(w, err)
			return
		}
		if !ok {
			http.SetCookie(w, clearCookie())
			http.Error(w, "ingresá el PIN", http.StatusUnauthorized)
			return
		}
		if s.clock().Sub(time.UnixMilli(se.LastSeen)) > touchAfter {
			if err := s.Store.TouchSession(se.ID); err != nil {
				fail(w, err)
				return
			}
			http.SetCookie(w, sessionCookieFor(c.Value))
		}
		mux.ServeHTTP(w, r)
	})
}

// requestSession is the session of the request's cookie, if it has a valid one.
func (s *Server) requestSession(r *http.Request) (store.Session, bool, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return store.Session{}, false, nil
	}
	return s.Store.SessionByToken(auth.TokenHash(c.Value))
}

// authState tells the page whether it must ask for the PIN.
func (s *Server) authState(w http.ResponseWriter, r *http.Request) {
	local := localRequest(r)
	var se store.Session
	var has bool
	if !local {
		var err error
		if se, has, err = s.requestSession(r); err != nil {
			fail(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{"local": local, "session": has, "device": se.Device, "pinRequired": !local && !has})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if localRequest(r) {
		http.Error(w, "desde esta computadora no hace falta el PIN", http.StatusBadRequest)
		return
	}
	if s.readOnly() {
		http.Error(w, "modo consulta: no se puede iniciar sesión", http.StatusConflict)
		return
	}
	lan := s.parts().lan
	if !lan.Enabled || lan.PinHash == "" {
		http.Error(w, "el acceso desde la red está apagado", http.StatusForbidden)
		return
	}
	var in struct {
		PIN    string `json:"pin"`
		Device string `json:"device"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&in); err != nil {
		http.Error(w, "pedido inválido", http.StatusBadRequest)
		return
	}
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	if wait := s.limiter.Wait(); wait > 0 {
		secs := int((wait + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		http.Error(w, fmt.Sprintf("Demasiados intentos. Probá de nuevo en %d s.", secs), http.StatusTooManyRequests)
		return
	}
	if !auth.VerifyPIN(lan.PinHash, in.PIN) {
		s.limiter.Fail()
		http.Error(w, "PIN incorrecto", http.StatusUnauthorized)
		return
	}
	s.limiter.Reset()
	if _, err := s.Store.PruneSessions(s.clock().Add(-sessionMaxAge)); err != nil {
		fail(w, err)
		return
	}
	token, hash, err := auth.NewToken()
	if err != nil {
		fail(w, err)
		return
	}
	if _, err := s.Store.CreateSession(hash, in.Device); err != nil {
		fail(w, err)
		return
	}
	http.SetCookie(w, sessionCookieFor(token))
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	se, ok, err := s.requestSession(r)
	if err != nil {
		fail(w, err)
		return
	}
	if ok {
		if _, err := s.Store.DeleteSession(se.ID); err != nil {
			fail(w, err)
			return
		}
	}
	http.SetCookie(w, clearCookie())
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.Sessions()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, list)
}

func (s *Server) closeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	found, err := s.Store.DeleteSession(id)
	if err != nil {
		fail(w, err)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) lanInfo(w http.ResponseWriter, r *http.Request) { writeJSON(w, s.lanState()) }

func (s *Server) lanQR(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("url")
	listed := false
	for _, known := range s.lanState().URLs {
		listed = listed || known == u
	}
	if !listed {
		http.Error(w, "esa dirección no es una de las de la red", http.StatusBadRequest)
		return
	}
	code, err := qr.Encode(u)
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(code.SVG(4)))
}
