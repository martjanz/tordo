// Package jukebox relays the Rockola: one speaker (a browser tab on the
// computer that runs Tordo) plays, and remotes (phones) send it commands and
// receive its state. It keeps nothing on disk.
package jukebox

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
)

// Role is what a remote may do: an anfitrión controls everything, an
// invitado only adds, pauses and removes what they added.
type Role string

const (
	Host  Role = "host"
	Guest Role = "guest"
)

var (
	ErrNoSpeaker      = errors.New("La rockola no está disponible: activá el audio en la compu.")
	ErrNotSpeaker     = errors.New("este navegador ya no es el parlante")
	ErrUnknownCommand = errors.New("orden desconocida")
	ErrForbidden      = errors.New("Solo el anfitrión puede hacer esto.")
	ErrUnknownRemote  = errors.New("control desconocido")
	ErrTooMany        = errors.New("Demasiados controles conectados a la rockola.")
	ErrBusy           = errors.New("La compu no responde: probá de nuevo.")
)

// MaxRemotes is how many remotes may be connected at once.
const MaxRemotes = 16

// buffer is how many events wait for a slow connection before it is dropped.
const buffer = 64

// commands maps each command to the lowest role that may send it.
var commands = map[string]Role{
	"enqueue": Guest, "toggle": Guest, "removeAt": Guest,
	"play": Host, "playStation": Host, "stopStation": Host, "setStation": Host,
	"next": Host, "prev": Host, "jumpTo": Host, "moveTo": Host, "clearUpcoming": Host,
	"seek": Host, "setVolume": Host, "toggleMute": Host, "toggleShuffle": Host, "cycleRepeat": Host,
}

// Event is one server-sent event; Data is encoded as JSON.
type Event struct {
	Type string
	Data any
}

// Conn is one open event stream: the speaker's or a remote's.
type Conn struct {
	ID      string
	Session int64 // remotes: the LAN session, 0 for the computer itself
	role    Role
	events  chan Event
	done    chan struct{}
	once    sync.Once
}

// Events delivers the connection's events.
func (c *Conn) Events() <-chan Event { return c.events }

// Done is closed when the hub drops the connection; events already queued
// may still be read.
func (c *Conn) Done() <-chan struct{} { return c.done }

func (c *Conn) close() { c.once.Do(func() { close(c.done) }) }

// send queues an event without blocking; false when the buffer is full.
func (c *Conn) send(e Event) bool {
	select {
	case c.events <- e:
		return true
	default:
		return false
	}
}

func newConn(session int64, role Role) *Conn {
	return &Conn{ID: randomID(), Session: session, role: role, events: make(chan Event, buffer), done: make(chan struct{})}
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b) // never fails (crypto/rand panics instead)
	return hex.EncodeToString(b)
}

// Hub holds the speaker, its last state and the remotes.
type Hub struct {
	mu      sync.Mutex
	speaker *Conn
	token   string
	state   json.RawMessage
	remotes map[string]*Conn
}

func New() *Hub { return &Hub{remotes: map[string]*Conn{}} }

// AttachSpeaker registers a new speaker and returns its connection and the
// token it publishes with. A previous speaker gets "replaced" and is dropped.
func (h *Hub) AttachSpeaker() (*Conn, string) {
	c := newConn(0, Host)
	token := randomID()
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.speaker; old != nil {
		old.send(Event{"replaced", struct{}{}})
		old.close()
	}
	h.speaker, h.token, h.state = c, token, nil
	c.send(Event{"hello", map[string]string{"token": token}})
	return c, token
}

// DetachSpeaker forgets c if it is still the speaker: its state goes and the
// remotes get "speaker-off".
func (h *Hub) DetachSpeaker(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c.close()
	if h.speaker != c {
		return
	}
	h.speaker, h.token, h.state = nil, "", nil
	h.broadcastLocked(Event{"speaker-off", struct{}{}})
}

// Speaking tells whether a speaker is connected.
func (h *Hub) Speaking() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.speaker != nil
}

// Publish stores the speaker's state and sends it to every remote.
func (h *Hub) Publish(token string, state json.RawMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.speaker == nil || token != h.token {
		return ErrNotSpeaker
	}
	h.state = state
	h.broadcastLocked(Event{"state", state})
	return nil
}

// Notice sends a text to one remote (the one whose command caused it). An
// unknown remote (it left) is not an error.
func (h *Hub) Notice(token, remote, text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.speaker == nil || token != h.token {
		return ErrNotSpeaker
	}
	if c, ok := h.remotes[remote]; ok {
		h.sendLocked(c, Event{"notice", map[string]string{"text": text}})
	}
	return nil
}

// AttachRemote opens a remote's stream: "hello", then the current state or
// "speaker-off".
func (h *Hub) AttachRemote(session int64, role Role) (*Conn, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.remotes) >= MaxRemotes {
		return nil, ErrTooMany
	}
	c := newConn(session, role)
	h.remotes[c.ID] = c
	c.send(Event{"hello", map[string]any{"remote": c.ID, "session": session, "role": role}})
	if h.speaker != nil && h.state != nil {
		c.send(Event{"state", h.state})
	} else if h.speaker == nil {
		c.send(Event{"speaker-off", struct{}{}})
	}
	return c, nil
}

// DetachRemote forgets a remote.
func (h *Hub) DetachRemote(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(c)
}

// Command checks a remote's command against its role and hands it to the
// speaker. The remote must belong to the session sending the command; role
// is the sender's current role (read for this request).
func (h *Hub) Command(remote string, session int64, role Role, command string, args json.RawMessage) error {
	need, known := commands[command]
	if !known {
		return ErrUnknownCommand
	}
	if need == Host && role != Host {
		return ErrForbidden
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.remotes[remote]
	if !ok || c.Session != session {
		return ErrUnknownRemote
	}
	if h.speaker == nil {
		return ErrNoSpeaker
	}
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	cmd := map[string]any{"remote": remote, "session": session, "role": role, "command": command, "args": args}
	if !h.speaker.send(Event{"command", cmd}) {
		return ErrBusy
	}
	return nil
}

// SetRole tells a session's remotes their new role.
func (h *Hub) SetRole(session int64, role Role) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.remotes {
		if c.Session == session {
			c.role = role
			h.sendLocked(c, Event{"role", map[string]Role{"role": role}})
		}
	}
}

// DropSession closes the remotes of a session that signed out.
func (h *Hub) DropSession(session int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.remotes {
		if c.Session == session {
			h.dropLocked(c)
		}
	}
}

// DropRemoteSessions closes every remote of a LAN session (a new PIN closed
// them all); the computer's own remotes stay.
func (h *Hub) DropRemoteSessions() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range h.remotes {
		if c.Session != 0 {
			h.dropLocked(c)
		}
	}
}

func (h *Hub) broadcastLocked(e Event) {
	for _, c := range h.remotes {
		h.sendLocked(c, e)
	}
}

// sendLocked queues e for a remote, dropping it when it fell behind (its
// EventSource reconnects and gets the current state).
func (h *Hub) sendLocked(c *Conn, e Event) {
	if !c.send(e) {
		h.dropLocked(c)
	}
}

func (h *Hub) dropLocked(c *Conn) {
	delete(h.remotes, c.ID)
	c.close()
}
