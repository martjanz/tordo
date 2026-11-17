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
	// RescanMinutes is how often the folders are checked for changes; 0: never.
	RescanMinutes int `json:"rescanMinutes"`
	// Language is the installation's interface language; "": the browser's.
	Language string `json:"language"`
}

func (s *Server) configView() configView {
	var cfg config.Config
	if rt := s.Engine.Current(); rt != nil {
		cfg = rt.Config
	}
	v := configView{SetupPending: s.Engine.SetupPending(), ReadOnly: s.readOnly(), Roots: []rootView{}, Suggested: []string{},
		Ignore: append([]string{}, cfg.Ignore...), HasToken: cfg.DiscogsToken != "", Normalize: cfg.Normalizing(), ListenMode: cfg.ListenMode, RescanMinutes: int(cfg.RescanEvery().Minutes()), Language: cfg.LanguageOrAuto(),
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
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	root, err := config.Root(s.AppDir, req.Path)
	if err != nil {
		fail(w, err)
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
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeError(w, http.StatusBadRequest, codeMissingToken, "token is required", nil)
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
			return nil, reject(http.StatusBadRequest, codeInvalidIgnore, fmt.Sprintf("pattern %q cannot contain ..", p), map[string]any{"pattern": p})
		}
		out = append(out, p)
	}
	if len(out) > maxIgnore {
		return nil, reject(http.StatusBadRequest, codeTooManyIgnore, fmt.Sprintf("more than %d folders to ignore", maxIgnore), map[string]any{"max": maxIgnore})
	}
	return out, nil
}

// maxRescanMinutes is a week.
const maxRescanMinutes = 7 * 24 * 60

// putConfig validates and saves the settings, and restarts what depends on
// them. A null token or PIN keeps the saved one; a "" token removes it. An
// absent (or null) roots, ignore, normalize, lan, listenMode, rescanMinutes or language keeps the saved
// value, so the page can save listen mode on its own.
func (s *Server) putConfig(w http.ResponseWriter, r *http.Request) {
	if s.readOnly() {
		fail(w, engine.ErrReadOnly)
		return
	}
	var req struct {
		Roots      *[]string `json:"roots"`
		Ignore     *[]string `json:"ignore"`
		Token      *string   `json:"token"`
		Normalize  *bool     `json:"normalize"`
		ListenMode *bool     `json:"listenMode"`
		Rescan     *int      `json:"rescanMinutes"`
		Language   *string   `json:"language"`
		LAN        *struct {
			Enabled bool    `json:"enabled"`
			Port    int     `json:"port"`
			Pin     *string `json:"pin"`
		} `json:"lan"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, codeInvalidBody, "invalid body", nil)
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
				return invalid(reject(http.StatusBadRequest, codeInvalidPort, "port must be between 1024 and 65535", nil))
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
				return invalid(reject(http.StatusBadRequest, codePINNeededForLAN, "choose a PIN to use Tordo from the network", nil))
			}
		}
		if req.Rescan != nil && (*req.Rescan < 0 || *req.Rescan > maxRescanMinutes) {
			return invalid(reject(http.StatusBadRequest, codeInvalidRescan, fmt.Sprintf("change check interval must be between 0 and %d minutes", maxRescanMinutes), map[string]any{"max": maxRescanMinutes}))
		}
		if req.Language != nil && *req.Language != "" && !slices.Contains(config.Languages, *req.Language) {
			return invalid(reject(http.StatusBadRequest, codeUnknownLanguage, fmt.Sprintf("unknown language: %q", *req.Language), map[string]any{"language": *req.Language}))
		}
		cfg := cur
		cfg.Roots, cfg.Ignore, cfg.LAN = roots, ignore, lan
		if req.Rescan != nil {
			cfg.RescanMinutes = req.Rescan
		}
		if req.Language != nil {
			cfg.Language = *req.Language
		}
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
				log.Printf("could not close the sessions: %v", err)
			}
			s.jukebox().DropRemoteSessions()
		}
		writeJSON(w, s.configView())
	case errors.As(err, &verr):
		fail(w, verr.Err)
	case errors.Is(err, engine.ErrReadOnly):
		fail(w, err)
	default:
		log.Printf("could not save config.json: %v", err)
		writeError(w, http.StatusInternalServerError, codeConfigSaveFailed, "could not save config.json", nil)
	}
}
