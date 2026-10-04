package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"tordo/internal/auth"
	"tordo/internal/config"
	"tordo/internal/discogs"
	"tordo/internal/engine"
	"tordo/internal/httpx"
)

const maxIgnore = 100

type rootView struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

type lanView struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
	HasPin  bool `json:"hasPin"`
}

// configView is the configuration as the settings show it: neither the token
// nor the PIN hash is sent back, only the token's last characters.
type configView struct {
	SetupPending bool       `json:"setupPending"`
	ReadOnly     bool       `json:"readOnly"`
	Roots        []rootView `json:"roots"`
	Suggested    []string   `json:"suggested"` // sibling folders that are not roots
	Ignore       []string   `json:"ignore"`
	HasToken     bool       `json:"hasToken"`
	TokenHint    string     `json:"tokenHint"`
	Normalize    bool       `json:"normalize"`
	LAN          lanView    `json:"lan"`
	ListenMode   bool       `json:"listenMode"`
}

func (s *Server) configView() configView {
	var cfg config.Config
	if rt := s.Engine.Current(); rt != nil {
		cfg = rt.Config
	}
	v := configView{SetupPending: s.Engine.SetupPending(), ReadOnly: s.readOnly(), Roots: []rootView{}, Suggested: []string{},
		Ignore: append([]string{}, cfg.Ignore...), HasToken: cfg.DiscogsToken != "", Normalize: cfg.Normalizing(), ListenMode: cfg.ListenMode,
		LAN: lanView{Enabled: cfg.LAN.Enabled, Port: cfg.LAN.PortOrDefault(), HasPin: cfg.LAN.PinHash != ""}}
	for _, r := range cfg.Roots {
		v.Roots = append(v.Roots, rootView{r, config.Available(s.AppDir, r)})
	}
	siblings, _ := config.DefaultRoots(s.AppDir)
	for _, r := range siblings {
		if !slices.Contains(cfg.Roots, r) {
			v.Suggested = append(v.Suggested, r)
		}
	}
	if tok := []rune(cfg.DiscogsToken); len(tok) >= 8 {
		v.TokenHint = "…" + string(tok[len(tok)-4:])
	}
	return v
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.configView())
}

// checkRoot turns a folder the user typed into a root, or says why it
// cannot be one.
func (s *Server) checkRoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	root, err := config.Root(s.AppDir, req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, rootView{root, true})
}

// verifyToken checks that Discogs takes the token via /oauth/identity, once:
// the person is waiting for the answer.
func verifyToken(ctx context.Context, token string) error {
	c := discogs.New(token)
	c.HTTP.Attempts = 1
	return c.Identity(ctx)
}

// checkToken tells whether Discogs takes a token: valid true or false, or
// null when Discogs could not be reached.
func (s *Server) checkToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		http.Error(w, "falta el token", http.StatusBadRequest)
		return
	}
	verify := s.VerifyToken
	if verify == nil {
		verify = verifyToken
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	var valid *bool
	switch err := verify(ctx, token); {
	case errors.Is(err, discogs.ErrUnauthorized):
		valid = new(bool)
	case errors.Is(err, httpx.ErrOffline), errors.Is(err, context.DeadlineExceeded):
	default: // any other answer (a 404 included) means Discogs took the token
		valid = new(bool)
		*valid = true
	}
	writeJSON(w, map[string]*bool{"valid": valid})
}

// cleanIgnore trims the patterns and drops the empty ones.
func cleanIgnore(in []string) ([]string, error) {
	out := []string{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.Contains(p, "..") {
			return nil, fmt.Errorf("el patrón %q no puede llevar ..", p)
		}
		out = append(out, p)
	}
	if len(out) > maxIgnore {
		return nil, fmt.Errorf("hay más de %d carpetas para ignorar", maxIgnore)
	}
	return out, nil
}

// putConfig validates and saves the settings, and restarts what depends on
// them. A null token or PIN keeps the saved one; a "" token removes it. An
// absent (or null) roots, ignore, normalize, lan or listenMode keeps the saved
// value, so the page can save listen mode on its own.
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	if s.readOnly() {
		http.Error(w, engine.ErrReadOnly.Error(), http.StatusConflict)
		return
	}
	var req struct {
		Roots      *[]string `json:"roots"`
		Ignore     *[]string `json:"ignore"`
		Token      *string   `json:"token"`
		Normalize  *bool     `json:"normalize"`
		ListenMode *bool     `json:"listenMode"`
		LAN        *struct {
			Enabled bool    `json:"enabled"`
			Port    int     `json:"port"`
			Pin     *string `json:"pin"`
		} `json:"lan"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}
	// A new PIN is saved and every session closed as one step with respect to
	// logins (see login): they take the same lock.
	newPin := req.LAN != nil && req.LAN.Pin != nil
	if newPin {
		s.loginMu.Lock()
		defer s.loginMu.Unlock()
	}
	invalid := func(err error) (config.Config, error) { return config.Config{}, &engine.ValidationError{Err: err} }
	// "Keep the saved token/PIN" is resolved inside Update, under the lock
	// that serializes every change. Roots and ignore are never nil: the
	// engine tells "same parts" by comparing them.
	err := s.Engine.Update(func(cur config.Config) (config.Config, error) {
		roots := append([]string{}, cur.Roots...)
		if req.Roots != nil {
			roots = append([]string{}, *req.Roots...)
		}
		// Kept roots pass as saved ones; there must still be one (first use).
		if err := config.CheckRoots(s.AppDir, roots, cur.Roots); err != nil {
			return invalid(err)
		}
		ignore := append([]string{}, cur.Ignore...)
		if req.Ignore != nil {
			var err error
			if ignore, err = cleanIgnore(*req.Ignore); err != nil {
				return invalid(err)
			}
		}
		lan := cur.LAN
		if req.LAN != nil {
			if req.LAN.Port < 1024 || req.LAN.Port > 65535 {
				return invalid(errors.New("el puerto tiene que estar entre 1024 y 65535"))
			}
			lan = config.LAN{Enabled: req.LAN.Enabled, Port: req.LAN.Port, PinHash: cur.LAN.PinHash}
			if req.LAN.Pin != nil {
				if err := auth.CheckFormat(*req.LAN.Pin); err != nil {
					return invalid(err)
				}
				var err error
				if lan.PinHash, err = auth.HashPIN(*req.LAN.Pin); err != nil {
					return config.Config{}, err
				}
			}
			if lan.Enabled && lan.PinHash == "" {
				return invalid(errors.New("elegí un PIN para usar Tordo desde la red"))
			}
		}
		cfg := cur
		cfg.Roots, cfg.Ignore, cfg.LAN = roots, ignore, lan
		if req.Normalize != nil {
			cfg.Normalize = req.Normalize
		}
		if req.ListenMode != nil {
			cfg.ListenMode = *req.ListenMode
		}
		if req.Token != nil {
			cfg.DiscogsToken = strings.TrimSpace(*req.Token)
		}
		return cfg, nil
	})
	var verr *engine.ValidationError
	switch {
	case err == nil:
		if newPin {
			if err := s.Store.DeleteSessions(); err != nil {
				log.Printf("no se pudieron cerrar las sesiones: %v", err)
			}
			s.jukebox().DropRemoteSessions()
		}
		writeJSON(w, s.configView())
	case errors.As(err, &verr):
		http.Error(w, verr.Error(), http.StatusBadRequest)
	case errors.Is(err, engine.ErrReadOnly):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		log.Printf("no se pudo guardar config.json: %v", err)
		http.Error(w, "no se pudo guardar config.json", http.StatusInternalServerError)
	}
}
