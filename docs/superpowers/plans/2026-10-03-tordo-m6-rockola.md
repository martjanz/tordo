# Tordo M6 (Rockola) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A tordo tab on the computer that runs Tordo becomes its speaker ("Audio de esta compu"); phones on the LAN switch to **Rockola** mode and control that tab's player — anfitriones fully, invitados only adding to the queue, pausing and removing what they added.

**Architecture:** The browser tab keeps owning the queue and the audio: the server only relays. A new in-memory hub (`internal/jukebox`) holds one speaker connection and up to 16 remote connections, both Server-Sent Events. Phones `POST` commands; the hub checks the command against the sender's role and forwards it to the speaker tab, which runs it on the existing player (`player.svelte.js`) and publishes a trimmed snapshot of its state back through the hub to every remote. On the web side a new facade, `playback.svelte.js`, replaces `player.svelte.js` as what components import: it calls the local player, or sends the command to the computer while Rockola mode is on.

**Tech Stack:** Go 1.27 (standard library + `modernc.org/sqlite`), Svelte 5 + Vite 8 + Vitest 5, Server-Sent Events (`EventSource` in the browser, `http.ResponseController` in Go).

**Spec:** `docs/superpowers/specs/2026-10-01-tordo-design.md` (Spanish): §7.5 Rockola (the whole design), §7.1 Reproductor, §9 Red local y seguridad, §10 M6, §11 Pruebas. Earlier plans (`2026-10-02-tordo-m5-revision-red.md` above all: LAN gate, sessions, `settings` wrapper) set the conventions; their decisions still hold.

## Decisions this plan takes (not spelled out in the spec)

Agreed with the user while writing the spec: the speaker is a browser tab (no native audio, no headless mode); Rockola is a mode switch on the phone; roles anfitrión/invitado are marked from the computer; invitados are limited on the server.

1. **Names.** Go package `jukebox`, API `/api/jukebox/...`, JS libraries `jukebox.js`, `speaker.svelte.js`, `rockola.svelte.js`, `playback.svelte.js`; UI text "Audio de esta compu" (computer) and "Rockola" (phone); new component `AudioDeEstaCompu.svelte`.
2. **Who the request is.** The LAN gate already loads the request's session; it now also puts it in the request context (`withSession`). A *local* request (the computer) is always role `host` with session id `0`. A LAN request's role is `host` when its session row has `host = 1`, else `guest`. The role is read from the session loaded for **that** request, so a change in Ajustes applies to the next command.
3. **Speaker endpoints are local only** (wrapper `speakerOnly`, 403 `El audio solo se activa en la computadora que corre Tordo.`). Remote endpoints need a session (LAN gate) or a local request.
4. **One speaker.** Registering a speaker replaces the previous one: the old connection gets a `replaced` event and is closed. A replaced tab turns its switch off and forgets it (no auto-reconnect, so two tabs never fight). After a network error the tab's `EventSource` reconnects on its own and registers again with a new token (the spec's earlier "reusing the token" was dropped: a fresh registration is equivalent and simpler; the spec already says so).
5. **Events.** Speaker stream: `hello {token}`, `command {remote, session, role, command, args}`, `replaced {}`. Remote stream: `hello {remote, session, role}`, then the current `state` (raw JSON as published) or `speaker-off {}`; later `state`, `notice {text}`, `role {role}`, `speaker-off {}`. There is no `speaker-on`: a remote only connects while Rockola mode is on and leaves it on `speaker-off`. Heartbeat: an SSE comment `: ping` every 20 s (`Server.beat`, settable in tests).
6. **Availability.** `/api/status` gains `"jukebox": {"speaker": bool}`; the phone enables the Rockola switch from it (polled every 30 s like the rest of the status). A stale "available" is harmless: the command answers 409 and the phone leaves the mode.
7. **Commands and roles** (in Go and, as a second check, in the speaker tab): invitado and anfitrión: `enqueue`, `toggle`, `removeAt`; anfitrión only: `play`, `playStation`, `stopStation`, `setStation`, `next`, `prev`, `jumpTo`, `moveTo`, `clearUpcoming`, `seek`, `setVolume`, `toggleMute`, `toggleShuffle`, `cycleRepeat`. (`setStation` — "Ajustar" on the running station — and `moveTo` — dragging a row of the queue, added 2026-10-03 — are added to the spec's list.)
8. **Queue positions travel as entry keys.** `jumpTo` and `removeAt` carry `{key}` (the entry's queue key), not a play-order position: positions shift between a published state and the command. `moveTo` carries `{key, toKey}`: move the entry `key` to the place the entry `toKey` holds now. The speaker converts keys to their current positions; an unknown key does nothing.
9. **`addedBy`.** Entries added by a remote `enqueue` carry `addedBy: <session id>`; entries added by the computer or by a local request carry none. An invitado's `removeAt` on an entry whose `addedBy` differs from their session is refused by the speaker with the notice `Solo podés quitar lo que agregaste.`
10. **Invitado's ▶.** The facade turns an invitado's `play(what, {shuffle, startId})` into `enqueue` with `startIfIdle: true`: with `startId` it adds **only that track** (`{tracks: [startId]}`: tapping a song means that song); with `shuffle` it adds the tracks shuffled; otherwise it adds `what` in order. "+ cola" and "a continuación" are plain `enqueue` (no `startIfIdle`).
11. **"Idle" for `startIfIdle`:** nothing playing **and** either the queue was empty or the last track ended (`player.ended`, set when the queue runs out, cleared by `start`). A paused track is not idle: the invitado's tracks are only added.
12. **Notices** of a remote command go to the remote that sent it. The player's `play`, `enqueue` and `playStation` take an optional `say` (default `notify`); the speaker passes one that posts `/api/jukebox/notice`. Texts are the player's usual ones (*3 pistas agregadas a la cola.*, *Suena a continuación.*).
13. **Published state** (`snapshot`): `{queue: {list, order, pos, shuffle, repeat, station, upcoming}, time, duration, playing, seekable, volume, muted, blocked}` with the queue trimmed to the 10 entries before the current one, the current one and the next 100 (`upcoming` = how many entries really come after the current one). Published on every change but `time` (trailing debounce 250 ms) and every 1 s while playing. Body limit 256 KB; a larger snapshot is not sent (the hub keeps the previous one) — it cannot happen with the trim.
14. **Blocked audio.** `player.blocked` is true when `audio.play()` was refused with `NotAllowedError`, and on load while the speaker switch is remembered but nobody has clicked the page yet; it clears on the element's `play` event or on **Reactivar audio**. On reload a remembered speaker reconnects at once (commands and state keep flowing; only sound waits for the click).
15. **Limits:** state ≤ 256 KB, command ≤ 16 KB, notice ≤ 4 KB (text cut to 300 runes), at most 16 remotes (503 `Demasiados controles conectados a la rockola.`). Each connection has a 64-event buffer: a remote that falls behind is dropped (its `EventSource` reconnects and gets the current state); a full speaker buffer answers the command 503 `La compu no responde: probá de nuevo.`
16. **Sessions closing** drops their remotes: `DELETE /api/sessions/{id}` and logout call `DropSession(id)`; a new PIN (which deletes every session) calls `DropRemoteSessions()` (every remote with a session id ≠ 0).
17. **Rockola on the phone is not remembered** across reloads (a reload goes back to "Este dispositivo"). Turning it on pauses the local player; the phone's own queue is untouched.
18. **Seek and volume from a phone** are sent on the range input's `change` (release), not on every `input`, so a drag is one command.

## Global Constraints

- Module `tordo`; Go `1.27.0`; **no new Go dependencies** (only `modernc.org/sqlite v1.59.0`). No cgo. Random ids and tokens: `crypto/rand`.
- npm: no new packages (only `svelte`, `vite`, `vitest`, `@sveltejs/vite-plugin-svelte`, `@fontsource-variable/inter`).
- **Never write to the music roots.** The only writes go to the app folder (`tordo.db`, `config.json`, `cache/`). The jukebox writes nothing: it lives in memory.
- Write endpoints (POST/PUT) are wrapped in `jsonOnly`. Settings endpoints (`/api/sessions…`) stay inside `settings` (local only).
- SQLite `SetMaxOpenConns(1)`: never run a query while another `*sql.Rows` is open.
- Schema: v7 only **adds** the column `sessions.host` (`schemaVersion = 7`, `derivedVersion` stays 2). Session writes are side writes that are **not** personal data.
- User-facing strings (UI, logs, errors) in **Spanish** (rioplatense: *activá*, *probá*, *podés*); identifiers and comments in English. JSON fields in English.
- UI files named in Spanish (`AudioDeEstaCompu.svelte`); libraries in English.
- The web build (`internal/server/dist/`) is **not committed**. `go test ./...` passes without Node and **without network**; `go vet ./...` and `gofmt -l .` clean; `cd web && npx vitest run` passes; `cd web && npm run build` succeeds.
- Commit messages: conventional style (`feat:`, `fix:`, `test:`, `docs:`), **no `Co-Authored-By:` trailer**.
- Git in the worktree: plain single commands (no pipes, no `&&`: the worktree guard refuses them).

## Model per task

Per the user's rule (cheapest model that can do the job): **Haiku** for transcribing fully specified code and docs, **Sonnet** for implementing from prose and multi-file work, **Opus** only for the final whole-branch review.

| Task | What | Model |
|---|---|---|
| 1 | Store: schema v7, `sessions.host` | Haiku |
| 2 | `internal/jukebox`: the hub | Sonnet |
| 3 | Server: session in context, jukebox endpoints, SSE, host mark | Sonnet |
| 4 | Web: player and queue changes | Sonnet |
| 5 | Web: `jukebox.js` (commands, roles, args, snapshot, run) | Sonnet |
| 6 | Web: speaker (`speaker.svelte.js`, Audio de esta compu) | Sonnet |
| 7 | Web: Rockola mode and the `playback` facade | Sonnet |
| 8 | Web: bar, queue, Nav switch, devices | Sonnet |
| 9 | Documentation (README) | Haiku |
| — | Final review of the branch | Opus |
| 10 | Check with the computer and two phones | manual (with the user) |

## File Structure

```
internal/store/schema.sql, store.go     sessions.host; schemaVersion 7; ALTER for v6 catalogs
internal/store/sessions.go              Session.Host; SetSessionHost
internal/jukebox/jukebox.go             Hub, Conn, Event, Role, roles table, errors (new)
internal/jukebox/jukebox_test.go        (new)
internal/server/server.go               routes; Server.jukebox, Server.beat; status "jukebox"
internal/server/lan.go                  withSession / requestRole; DropSession on logout and close
internal/server/settings.go             DropRemoteSessions on a new PIN
internal/server/jukebox.go              speakerOnly, SSE writer, the five handlers, setSessionHost (new)
internal/server/jukebox_test.go         (new)
web/src/lib/queue.js                    export shuffled
web/src/lib/player.svelte.js            say, shuffle/startIfIdle/addedBy, ended, blocked, entryOfKey/posOfKey
web/src/lib/jukebox.js                  pure: ROLES, allowed, cleanArgs, snapshot, run (new)
web/src/lib/speaker.svelte.js           the computer's speaker (new)
web/src/lib/rockola.svelte.js           the phone's Rockola mode (new)
web/src/lib/playback.svelte.js          facade over player / rockola (new)
web/src/lib/api.js                      jukebox calls, setSessionHost
web/src/components/AudioDeEstaCompu.svelte   the switch in Ajustes → Reproducción (new)
web/src/components/Reproductor.svelte, Cola.svelte, Nav.svelte, Dispositivos.svelte, EstacionAjustes.svelte,
  MenuPista.svelte, Acciones.svelte, BotonEstacion.svelte, … (every importer of player.svelte.js)
web/src/pages/Ajustes.svelte            mounts AudioDeEstaCompu
README.md
```

---

### Task 1: Store: schema v7, `sessions.host`

**Files:**
- Modify: `internal/store/schema.sql` (the `sessions` table), `internal/store/store.go` (`schemaVersion = 7`, comment, migration step), `internal/store/sessions.go`
- Test: `internal/store/sessions_test.go`, `internal/store/migrate_test.go`

**Interfaces:**
- Produces: `Session.Host bool` (JSON `host`), filled by `SessionByToken` and `Sessions`; `func (s *Store) SetSessionHost(id int64, host bool) (bool, error)` (false when the session does not exist).

In `schema.sql`, the `sessions` table gains a column (fresh catalogs and catalogs older than v6 get it from `CREATE TABLE`):

```sql
CREATE TABLE IF NOT EXISTS sessions (
	id         INTEGER PRIMARY KEY,
	token_hash TEXT NOT NULL UNIQUE,
	device     TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	last_seen  INTEGER NOT NULL,
	host       INTEGER NOT NULL DEFAULT 0 -- 1: anfitrión of the Rockola
);
```

A v6 catalog already has `sessions` without the column, so `migrate` adds it before running the schema (the order matters only for clarity; `CREATE TABLE IF NOT EXISTS` leaves the existing table alone):

```go
	if v == 6 {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN host INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
```

`store.go` comment gains: "Version 7 (M6) added sessions.host (the Rockola's anfitriones)."

`sessions.go`:

```go
// Session is a device signed in from the LAN.
type Session struct {
	ID        int64  `json:"id"`
	Device    string `json:"device"`
	CreatedAt int64  `json:"createdAt"` // unix ms
	LastSeen  int64  `json:"lastSeen"`  // unix ms
	Host      bool   `json:"host"`      // anfitrión of the Rockola
}

// SetSessionHost marks a session as anfitrión of the Rockola, or not; false
// when it does not exist.
func (s *Store) SetSessionHost(id int64, host bool) (bool, error) {
	found := false
	err := s.sideWrite(false, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE sessions SET host = ? WHERE id = ?`, host, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}
```

`SessionByToken` and `Sessions` select `host` too (`SELECT id, device, created_at, last_seen, host …`, scanned into `&se.Host`; modernc scans an INTEGER 0/1 into a `bool`).

- [ ] **Step 1: Write the failing tests.**
  - `TestSessionHost` (`sessions_test.go`): a new session has `Host == false` in `SessionByToken` and `Sessions`; `SetSessionHost(id, true)` → `true, nil`, both reads show `Host == true`; `SetSessionHost(id, false)` turns it back; an unknown id → `false, nil`; on `sample(t)`, `Changes()` and `PersonalVersion()` do not move.
  - `migrate_test.go`, `TestOpenMigratesV6Sessions`: build a v6 catalog (open a store, `DROP TABLE sessions`, create it with the five v6 columns, insert a row, `PRAGMA user_version = 6`, close), reopen → `user_version` 7, the old row reads with `Host == false`, `SetSessionHost` works on it. Follow the style of the existing migration tests (they rewrite the catalog file with raw SQL, then call `Open`).
- [ ] **Step 2: Run** `go test ./internal/store -run 'Session|Migrat'`: fails.
- [ ] **Step 3: Implement** as above.
- [ ] **Step 4: Run** `go test ./internal/store`: pass.
- [ ] **Step 5: Commit** `feat: schema v7 with the Rockola's anfitrión mark on sessions`

---

### Task 2: `internal/jukebox`: the hub

**Files:**
- Create: `internal/jukebox/jukebox.go`, `internal/jukebox/jukebox_test.go`

**Interfaces:**
- Produces (used by Task 3):

```go
type Role string
const (
	Host  Role = "host"
	Guest Role = "guest"
)
var ErrNoSpeaker, ErrNotSpeaker, ErrUnknownCommand, ErrForbidden, ErrUnknownRemote, ErrTooMany, ErrBusy error
const MaxRemotes = 16
type Event struct { Type string; Data any } // Data is JSON-encoded by the server
type Conn struct { ID string; Session int64 /* unexported fields */ }
func (c *Conn) Events() <-chan Event
func (c *Conn) Done() <-chan struct{}
func New() *Hub
func (h *Hub) AttachSpeaker() (*Conn, string)
func (h *Hub) DetachSpeaker(c *Conn)
func (h *Hub) Speaking() bool
func (h *Hub) Publish(token string, state json.RawMessage) error
func (h *Hub) Notice(token, remote, text string) error
func (h *Hub) AttachRemote(session int64, role Role) (*Conn, error)
func (h *Hub) DetachRemote(c *Conn)
func (h *Hub) Command(remote string, session int64, role Role, command string, args json.RawMessage) error
func (h *Hub) SetRole(session int64, role Role)
func (h *Hub) DropSession(session int64)
func (h *Hub) DropRemoteSessions()
```

The whole package (transcribe; comments are part of the code):

```go
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
```

(Deleting from a map while ranging over it is allowed in Go.)

- [ ] **Step 1: Write the failing tests** (`jukebox_test.go`; a helper `next(t, c) Event` reads one event with a 1 s timeout, `none(t, c)` asserts the channel is empty):
  - `TestSpeakerReplaced`: `AttachSpeaker` → first event `hello` with the token; a second `AttachSpeaker` → the first gets `replaced` and its `Done()` is closed; `Publish(oldToken, …)` → `ErrNotSpeaker`; `Publish(newToken, …)` → nil; `Speaking()` true; `DetachSpeaker(first)` changes nothing; `DetachSpeaker(second)` → `Speaking()` false.
  - `TestRemoteHello`: no speaker → a remote gets `hello` (remote id, session, role) then `speaker-off`; with a speaker but nothing published → only `hello`; after `Publish` → a new remote gets `hello` then that `state` (same bytes).
  - `TestStateFanOut`: three remotes, `Publish` → each gets `state`; `DetachSpeaker` → each gets `speaker-off`.
  - `TestCommandRoles`: for every name in `commands`, a guest sending it gets `ErrForbidden` exactly when the table says `Host`; an unknown name → `ErrUnknownCommand` (even for a host); an accepted command reaches the speaker as `command` with `remote`, `session`, `role`, `command` and `args` (empty args become `{}`).
  - `TestCommandChecks`: a remote id from another session → `ErrUnknownRemote`; an unknown id → `ErrUnknownRemote`; no speaker → `ErrNoSpeaker`.
  - `TestSpeakerBusy`: fill the speaker's buffer with 64 commands (do not read) → the 65th → `ErrBusy`.
  - `TestSlowRemoteDropped`: a remote that never reads; publish 70 times → its `Done()` is closed and it no longer counts toward `MaxRemotes`.
  - `TestTooManyRemotes`: 16 attach, the 17th → `ErrTooMany`; detach one → attach works again.
  - `TestNotice`: `Notice(token, remote, "hola")` → that remote gets `notice {text: hola}`, another remote gets nothing; an unknown remote → nil; a wrong token → `ErrNotSpeaker`.
  - `TestRoleAndDrops`: `SetRole(7, Host)` → session 7's remotes get `role {role: host}`, session 8's nothing; `DropSession(7)` closes 7's; `DropRemoteSessions()` closes every remote but session 0's.
  - Run with `-race`.
- [ ] **Step 2: Run** `go test -race ./internal/jukebox`: fails (no package).
- [ ] **Step 3: Implement** the package as above.
- [ ] **Step 4: Run** `go test -race ./internal/jukebox`: pass. `go vet ./internal/jukebox` clean.
- [ ] **Step 5: Commit** `feat: jukebox hub relaying the Rockola's commands and state`

---

### Task 3: Server: session in context, jukebox endpoints, SSE, host mark

**Files:**
- Create: `internal/server/jukebox.go`, `internal/server/jukebox_test.go`
- Modify: `internal/server/server.go` (fields, routes, status), `internal/server/lan.go` (`withSession`, `requestRole`, drops on logout/close), `internal/server/settings.go` (`DropRemoteSessions` after a new PIN)

**Interfaces:**
- Consumes: Task 1 (`Session.Host`, `SetSessionHost`), Task 2 (the hub).
- Produces (used by the web tasks):

| Route | Wrapper | Body / answer |
|---|---|---|
| `GET /api/jukebox/speaker` | `speakerOnly` | SSE: `hello {token}`, `command {...}`, `replaced {}` |
| `POST /api/jukebox/state` | `speakerOnly(jsonOnly)` | `{token, state}` ≤ 256 KB → 204; wrong token 409 |
| `POST /api/jukebox/notice` | `speakerOnly(jsonOnly)` | `{token, remote, text}` ≤ 4 KB → 204; wrong token 409 |
| `GET /api/jukebox/events` | — | SSE: `hello {remote, session, role}`, `state`, `notice {text}`, `role {role}`, `speaker-off {}`; 503 when 16 remotes |
| `POST /api/jukebox/command` | `jsonOnly` | `{remote, command, args}` ≤ 16 KB → 202; 400 unknown command, 403 role, 404 unknown remote, 409 no speaker, 503 busy |
| `PUT /api/sessions/{id}/host` | `settings(jsonOnly)` | `{host: bool}` → 204; 404 unknown session |

`/api/status` gains `"jukebox": {"speaker": <bool>}`.

**Server fields** (in `server.go`):

```go
	// jukeboxOnce builds the Rockola's hub on first use; beat is the SSE
	// heartbeat (0: 20 s; tests shorten it).
	jukeboxOnce sync.Once
	jukeboxHub  *jukebox.Hub
	beat        time.Duration
```

```go
func (s *Server) jukebox() *jukebox.Hub {
	s.jukeboxOnce.Do(func() { s.jukeboxHub = jukebox.New() })
	return s.jukeboxHub
}
```

**Session in the context** (`lan.go`): `LANHandler` ends with `mux.ServeHTTP(w, withSession(r, se))` instead of `mux.ServeHTTP(w, r)`.

```go
type sessionKey struct{}

// withSession carries the request's LAN session to the handlers.
func withSession(r *http.Request, se store.Session) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionKey{}, se))
}

// requestRole is who sends the request in the Rockola: the computer is
// always anfitrión with session 0; a LAN device is anfitrión when its session
// is marked so. ok is false when the request has neither (cannot happen
// behind the gate).
func requestRole(r *http.Request) (session int64, role jukebox.Role, ok bool) {
	if localRequest(r) {
		return 0, jukebox.Host, true
	}
	se, found := r.Context().Value(sessionKey{}).(store.Session)
	if !found {
		return 0, "", false
	}
	if se.Host {
		return se.ID, jukebox.Host, true
	}
	return se.ID, jukebox.Guest, true
}
```

`logout`: after deleting the session, `s.jukebox().DropSession(se.ID)`. `closeSession`: after a found delete, `s.jukebox().DropSession(id)`. `putConfig` (`settings.go`): after `DeleteSessions()` on a new PIN, `s.jukebox().DropRemoteSessions()`.

**`jukebox.go`** — the SSE writer and handlers:

```go
// speakerOnly wraps the speaker's endpoints: only the computer that runs
// Tordo plays the Rockola.
func speakerOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localRequest(r) {
			http.Error(w, "El audio solo se activa en la computadora que corre Tordo.", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func (s *Server) heartbeat() time.Duration {
	if s.beat > 0 {
		return s.beat
	}
	return 20 * time.Second
}

// stream writes c's events as server-sent events until the client leaves or
// the hub drops c (then what is already queued is written first).
func (s *Server) stream(w http.ResponseWriter, r *http.Request, c *jukebox.Conn) {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if rc.Flush() != nil {
		return
	}
	tick := time.NewTicker(s.heartbeat())
	defer tick.Stop()
	write := func(e jukebox.Event) bool {
		data, err := json.Marshal(e.Data)
		if err != nil {
			log.Printf("rockola: %v", err)
			return true
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-c.Events():
			if !write(e) {
				return
			}
		case <-c.Done():
			for {
				select {
				case e := <-c.Events():
					if !write(e) {
						return
					}
				default:
					return
				}
			}
		case <-tick.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		}
	}
}
```

(`json.Marshal` of a `json.RawMessage` writes it as is, so a state arrives byte for byte.)

Handlers: `jukeboxSpeaker` (`AttachSpeaker`, `defer DetachSpeaker(c)`, `stream`); `jukeboxEvents` (`requestRole` → `AttachRemote` → 503 with the error text on `ErrTooMany`; `defer DetachRemote(c)`; `stream`); `jukeboxState` (decode `{token string; state json.RawMessage}` through `http.MaxBytesReader(w, r.Body, 256<<10)` → 400 `pedido inválido` on a bad or too large body, also when `state` is empty; `ErrNotSpeaker` → 409; else 204); `jukeboxNotice` (`{token, remote, text}` ≤ 4096 bytes; text trimmed and cut to 300 runes; empty text → 400); `jukeboxCommand` (`{remote string; command string; args json.RawMessage}` ≤ 16 KB; map errors: `ErrUnknownCommand` 400, `ErrForbidden` 403, `ErrUnknownRemote` 404, `ErrNoSpeaker` 409, `ErrBusy` 503, each with the error's text; success 202 with no body); `setSessionHost` (`pathID`, decode `{host *bool}` — missing → 400; `SetSessionHost` → 404 when not found; then `s.jukebox().SetRole(id, role)`; 204).

Routes (add to `routeTable`, next to the sessions routes):

```go
		{"PUT /api/sessions/{id}/host", settings(jsonOnly(s.setSessionHost))},
		{"GET /api/jukebox/speaker", speakerOnly(s.jukeboxSpeaker)},
		{"POST /api/jukebox/state", speakerOnly(jsonOnly(s.jukeboxState))},
		{"POST /api/jukebox/notice", speakerOnly(jsonOnly(s.jukeboxNotice))},
		{"GET /api/jukebox/events", s.jukeboxEvents},
		{"POST /api/jukebox/command", jsonOnly(s.jukeboxCommand)},
```

`status` adds `"jukebox": map[string]bool{"speaker": s.jukebox().Speaking()}`.

- [ ] **Step 1: Write the failing tests** (`jukebox_test.go`). The SSE tests run a real server: `httptest.NewServer(s.LANHandler())` for LAN cases (requests from `127.0.0.1` with `Host: 127.0.0.1:<port>` are local; to act as a phone use `s.LANHandler()` through `lanReq` for plain requests, and for SSE as a phone build the request with a cookie from `login(...)` and run it through a `httptest.NewServer` wrapping a handler that rewrites `r.RemoteAddr = phoneAddr` and `r.Host = lanHostIP` before calling `s.LANHandler()`). A helper `readEvent(t, *bufio.Reader) (typ string, data string)` skips `: ping` comments and reads `event:`/`data:` lines up to the blank line, with a 2 s deadline. Set `s.beat = 20 * time.Millisecond`.
  - `TestJukeboxSpeakerFlow`: local GET `/api/jukebox/speaker` → `hello` with a token; `GET /api/status` → `jukebox.speaker == true`; a remote (local) GET `/api/jukebox/events` → `hello` (session 0, role host); POST `/api/jukebox/state` `{token, state: {"x":1}}` → 204 and the remote reads `state` with data `{"x":1}`; POST `/api/jukebox/command` `{remote, command: "next"}` → 202 and the speaker reads `command` with `"command":"next"`, `"args":{}`; POST `/api/jukebox/notice` → the remote reads `notice`; closing the speaker's response body → the remote reads `speaker-off` and status says `false`.
  - `TestJukeboxReplaced`: two speaker streams → the first reads `replaced` and then EOF; state with the first token → 409.
  - `TestJukeboxGuestLimits`: phone session (not host): `events` → `hello` role `guest`; `command` `next` → 403 `Solo el anfitrión puede hacer esto.`; `enqueue` → 202; after `PUT /api/sessions/{id}/host {"host":true}` from the computer → the phone's stream reads `role {role: host}` and `next` → 202.
  - `TestJukeboxRemoteOfAnotherSession`: phone A's remote id used with phone B's cookie → 404.
  - `TestJukeboxNoSpeaker`: command → 409 with `La rockola no está disponible: activá el audio en la compu.`
  - `TestJukeboxSpeakerLocalOnly`: with a phone session, `GET /api/jukebox/speaker`, `POST /api/jukebox/state`, `POST /api/jukebox/notice` → 403; `PUT /api/sessions/1/host` → 403.
  - `TestJukeboxLimits`: state body over 256 KB → 400; command over 16 KB → 400; unknown command → 400; `PUT …/host` without `host` → 400, unknown id → 404.
  - `TestJukeboxDropsOnSignOut`: a phone's events stream ends (EOF) after `DELETE /api/sessions/{id}` from the computer; another phone's ends after its own `POST /api/auth/logout`.
  - `TestJukeboxHeartbeat`: an idle events stream receives `: ping` within 200 ms (read raw lines).
  - The existing `TestLANGateCoversEveryRoute` must still pass (the new routes answer 401 without a session); `TestSessionLifecycle`-style API tests of `GET /api/sessions` now see `"host": false`.
- [ ] **Step 2: Run** `go test ./internal/server -run 'Jukebox|LANGate'`: fails.
- [ ] **Step 3: Implement** as above.
- [ ] **Step 4: Run** `go test -race ./internal/server` and `go test ./...`: pass; `go vet ./...`, `gofmt -l .` clean.
- [ ] **Step 5: Commit** `feat: Rockola endpoints with server-sent events and the anfitrión mark`

---

### Task 4: Web: player and queue changes

**Files:**
- Modify: `web/src/lib/queue.js`, `web/src/lib/player.svelte.js`
- Test: `web/src/lib/queue.test.js`

**Interfaces:**
- Produces (used by Tasks 5–7):
  - `queue.js`: `export function shuffled(items, rand = Math.random)` (the existing private helper, exported; it already copies, works on any array).
  - `player.svelte.js`:
    - `player.ended` (bool): true when the queue ran out at the end of a track; false after `start`.
    - `player.blocked` (bool): see decision 14. `export function unblock()` sets it false (and is what **Reactivar audio** calls before `toggle()` when a track is current and not playing).
    - `play(what, { shuffle = false, startId = null, say = notify } = {})`
    - `enqueue(what, { next = false, shuffle = false, startIfIdle = false, addedBy = null, say = notify } = {})`
    - `playStation(st, { say = notify } = {})`
    - `export function posOfKey(key)` → play-order position of the entry with that key, `-1` when absent.
    - `export function entryOfKey(key)` → the entry or `null`.

Changes in `player.svelte.js`:
- `player` state gains `ended: false, blocked: false`.
- Every `notify(` inside `play`, `enqueue` and `playStation` becomes `say(` (errors included). Other notices stay `notify`.
- `enqueue`: after the request, `let items = res.items`; `if (shuffle) items = Q.shuffled(items)`; `if (addedBy != null) items = items.map((it) => ({ ...it, addedBy }))`. Remember `const firstKey = player.queue.nextKey` before appending (the first new entry's key). After appending and persisting:

```js
  const idle = !player.playing && (wasEmpty || player.ended)
  if (startIfIdle && idle) {
    const pos = player.queue.order.indexOf(firstKey)
    if (pos >= 0) setQueue({ ...player.queue, pos }, true)
  } else if (wasEmpty) start(Q.current(player.queue), 0, false)
  else refreshGains()
```

- `onEnded`, branch `if (!q)`: also `player.ended = true`. `start()`: `player.ended = false`.
- Every `.play().catch(...)` on an audio element (in `start`, `toggle`, the swap in `onEnded`) also sets `player.blocked = true` when the error's `name === 'NotAllowedError'` (keep the existing effects, e.g. `player.loading = false`). `onPlay`: `player.blocked = false`.
- `posOfKey`, `entryOfKey`, `unblock` as above.

`player.svelte.js` has no tests (it needs `Audio`); its new behavior is covered through Task 5's `run` tests with a fake player and by the manual check. Keep the change minimal and readable.

- [ ] **Step 1: Write the failing test** in `queue.test.js`: `shuffled([1,2,3,4], seq)` with a deterministic `rand` returns a permutation and leaves the input untouched; `shuffled([])` → `[]`.
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/queue.test.js`: fails (not exported).
- [ ] **Step 3: Implement** the export and the player changes.
- [ ] **Step 4: Run** `cd web && npx vitest run` and `cd web && npm run build`: pass.
- [ ] **Step 5: Commit** `feat: player options for remote commands (say, shuffle, startIfIdle, addedBy) and blocked/ended state`

---

### Task 5: Web: `jukebox.js`

**Files:**
- Create: `web/src/lib/jukebox.js`, `web/src/lib/jukebox.test.js`

**Interfaces:**
- Consumes: Task 4 (the player's functions, passed in as an object so tests use a fake).
- Produces:

```js
export const ROLES // { enqueue: 'guest', toggle: 'guest', removeAt: 'guest', play: 'host', … } — same table as Go
export const KEEP_PLAYED = 10, KEEP_NEXT = 100
export function allowed(role, command)            // bool; unknown command → false
export function cleanArgs(command, args)          // sanitized args object, or null when invalid
export function snapshot(p)                       // published state of a player-like object (see below)
export async function run(cmd, P, say)            // runs {command, args, session, role} on P
```

Pure module (no Svelte runes, no imports but `./queue.js` if needed). Implementation:

```js
// The Rockola's commands, shared by the computer (which runs them) and the
// phones (which send them). Pure: the player is passed in.

// ROLES is the lowest role that may send each command (as the server's table).
export const ROLES = {
  enqueue: 'guest', toggle: 'guest', removeAt: 'guest',
  play: 'host', playStation: 'host', stopStation: 'host', setStation: 'host',
  next: 'host', prev: 'host', jumpTo: 'host', moveTo: 'host', clearUpcoming: 'host',
  seek: 'host', setVolume: 'host', toggleMute: 'host', toggleShuffle: 'host', cycleRepeat: 'host',
}

export const KEEP_PLAYED = 10 // entries already played that the phones see
export const KEEP_NEXT = 100 // entries to come that the phones see

export function allowed(role, command) {
  const need = ROLES[command]
  return !!need && (need === 'guest' || role === 'host')
}

const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)
const isId = (v) => Number.isInteger(v) && v > 0
const isNum = (v) => typeof v === 'number' && Number.isFinite(v)

// cleanWhat keeps what /api/play understands: {album}, {folder}, {artist},
// {tracks}, {playlist}, {liked}; null when there is nothing valid.
function cleanWhat(w) {
  if (!isObj(w)) return null
  const out = {}
  if (isId(w.album)) out.album = w.album
  if (typeof w.folder === 'string') out.folder = w.folder
  if (typeof w.artist === 'string') out.artist = w.artist
  if (Array.isArray(w.tracks) && w.tracks.length <= 5000 && w.tracks.every(isId)) out.tracks = w.tracks
  if (isId(w.playlist)) out.playlist = w.playlist
  if (w.liked === true) out.liked = true
  return Object.keys(out).length ? out : null
}

export function cleanArgs(command, args) {
  if (!(command in ROLES)) return null
  const a = isObj(args) ? args : {}
  switch (command) {
    case 'play': {
      const what = cleanWhat(a.what)
      return what && { what, shuffle: a.shuffle === true, startId: isId(a.startId) ? a.startId : null }
    }
    case 'enqueue': {
      const what = cleanWhat(a.what)
      return what && { what, next: a.next === true, shuffle: a.shuffle === true, startIfIdle: a.startIfIdle === true }
    }
    case 'playStation':
    case 'setStation':
      return isObj(a.station) ? { station: a.station } : null
    case 'jumpTo':
    case 'removeAt':
      return isId(a.key) ? { key: a.key } : null
    case 'moveTo':
      return isId(a.key) && isId(a.toKey) ? { key: a.key, toKey: a.toKey } : null
    case 'seek':
      return isNum(a.seconds) && a.seconds >= 0 ? { seconds: a.seconds } : null
    case 'setVolume':
      return isNum(a.volume) && a.volume >= 0 && a.volume <= 1 ? { volume: a.volume } : null
    default:
      return {}
  }
}

// snapshot is what the computer publishes: the player's state with the queue
// trimmed around the current entry. upcoming counts every entry still to come.
export function snapshot(p) {
  const q = p.queue
  const from = q.pos < 0 ? 0 : Math.max(0, q.pos - KEEP_PLAYED)
  const to = q.pos < 0 ? KEEP_NEXT : q.pos + 1 + KEEP_NEXT
  const order = q.order.slice(from, to)
  const keys = new Set(order)
  return {
    queue: {
      list: q.list.filter((e) => keys.has(e.key)),
      order,
      pos: q.pos < 0 ? -1 : q.pos - from,
      shuffle: q.shuffle,
      repeat: q.repeat,
      station: q.station,
      upcoming: q.pos < 0 ? q.order.length : q.order.length - q.pos - 1,
    },
    time: p.time,
    duration: p.duration,
    playing: p.playing,
    seekable: p.seekable,
    volume: p.volume,
    muted: p.muted,
    blocked: p.blocked,
  }
}

// run executes a phone's command on the player P, checking role and
// arguments again; say sends a notice back to that phone.
export async function run({ command, args, session, role }, P, say) {
  if (!allowed(role, command)) return say('Solo el anfitrión puede hacer esto.')
  const a = cleanArgs(command, args)
  if (!a) return
  switch (command) {
    case 'play':
      return P.play(a.what, { shuffle: a.shuffle, startId: a.startId, say })
    case 'enqueue':
      return P.enqueue(a.what, { next: a.next, shuffle: a.shuffle, startIfIdle: a.startIfIdle, addedBy: session || null, say })
    case 'playStation':
      return P.playStation(a.station, { say })
    case 'setStation':
      return P.setStation(a.station)
    case 'jumpTo': {
      const pos = P.posOfKey(a.key)
      if (pos >= 0) P.jumpTo(pos)
      return
    }
    case 'moveTo': {
      const from = P.posOfKey(a.key)
      const to = P.posOfKey(a.toKey)
      if (from >= 0 && to >= 0) P.moveTo(from, to)
      return
    }
    case 'removeAt': {
      const entry = P.entryOfKey(a.key)
      if (!entry) return
      if (role !== 'host' && entry.addedBy !== session) return say('Solo podés quitar lo que agregaste.')
      return P.removeAt(P.posOfKey(a.key))
    }
    case 'seek':
      return P.seek(a.seconds)
    case 'setVolume':
      return P.setVolume(a.volume)
    default:
      return P[command]()
  }
}
```

- [ ] **Step 1: Write the failing tests** (`jukebox.test.js`; a fake `P` built with `vi.fn()` for every function, `posOfKey`/`entryOfKey` backed by a small fake queue):
  - `allowed`: every guest command for both roles; a host command → false for `guest`, true for `host`; `'nope'` → false.
  - `ROLES` keys equal the Go table's (list them in the test as a literal array and compare sorted).
  - `cleanArgs`: `play` without `what` → null; `what` with only unknown keys → null; `{album: 3, evil: 1}` → `{album: 3}`; `tracks` with a string → dropped; `seek` −1 → null; `setVolume` 1.5 → null; `jumpTo` key 0 → null; `moveTo` without `toKey` → null; `toggle` with junk → `{}`; unknown command → null.
  - `snapshot`: a 300-entry queue at pos 150 → `order` length 111, `pos` 10, `upcoming` 149, `list` only those keys; at pos −1 → first 100, `pos` −1, `upcoming` 300; carries `time`, `blocked` etc.
  - `run`: guest `next` → `say('Solo el anfitrión puede hacer esto.')`, `P.next` not called; host `next` → called; guest `enqueue` passes `addedBy: 7` (session 7) and `say`; local `enqueue` (session 0) passes `addedBy: null`; guest `removeAt` of an entry with `addedBy: 7` from session 7 → `P.removeAt(pos)`; from session 8 → `say('Solo podés quitar lo que agregaste.')`; host removes anything; `jumpTo` an unknown key → nothing; host `moveTo {key, toKey}` → `P.moveTo(posOfKey(key), posOfKey(toKey))`, guest → refused; invalid args → nothing called, no say.
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/jukebox.test.js`: fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** `cd web && npx vitest run`: pass.
- [ ] **Step 5: Commit** `feat: Rockola commands, roles and published state (web)`

---

### Task 6: Web: speaker (Audio de esta compu)

**Files:**
- Create: `web/src/lib/speaker.svelte.js`, `web/src/lib/speaker.svelte.test.js`, `web/src/components/AudioDeEstaCompu.svelte`
- Modify: `web/src/lib/api.js`, `web/src/pages/Ajustes.svelte`, `web/src/components/Nav.svelte`, `web/src/App.svelte`

**Interfaces:**
- Consumes: Task 3 (endpoints), Task 4 (`player`, `unblock`, `toggle`), Task 5 (`run`, `snapshot`).
- Produces:
  - `api.js`: `jukeboxState: (token, state) => post('/api/jukebox/state', { token, state })`, `jukeboxNotice: (token, remote, text) => post('/api/jukebox/notice', { token, remote, text })`, `jukeboxCommand: (remote, command, args) => post('/api/jukebox/command', { remote, command, args })`, `setSessionHost: (id, host) => put(\`/api/sessions/${id}/host\`, { host })`. (`call` returns `null` for 202/204: extend its check to `res.status === 204 || res.status === 202` — already so.)
  - `speaker.svelte.js`:

```js
export const speaker = $state({ on: false, connected: false })
export function enableSpeaker(connect = (u) => new EventSource(u)) // the switch turned on (a click: audio unlocked)
export function disableSpeaker()
export function resumeSpeaker(connect = …)  // at start: reconnects when remembered (localStorage 'tordo.audio-compu' === '1'), with player.blocked = true
export function reactivate()                 // "Reactivar audio": unblock(); toggle() when a current track is not playing
```

Behavior:
- `enableSpeaker`: save `'1'`, `speaker.on = true`, open `connect('/api/jukebox/speaker')`, attach listeners. `disableSpeaker`: save `''`, close the source, `speaker.on = speaker.connected = false`, stop publishing.
- `hello` → `token = data.token`, `speaker.connected = true`, publish at once.
- `command` → `run(data, P, (text) => api.jukeboxNotice(token, data.remote, text).catch(() => {}))`, where `P` is the player module's namespace (`import * as P from './player.svelte.js'`).
- `replaced` → `disableSpeaker()` and `notify('Otro navegador tomó el audio de esta compu.')`.
- `error` → `speaker.connected = false`; when the source's `readyState` is `2` (closed: the server refused, e.g. a network change), retry `connect` after a backoff (1 s, doubling up to 30 s) while `speaker.on`. A `readyState` of `0` means the browser is already reconnecting: do nothing.
- Publishing: `$effect.root` with an `$effect` that reads `JSON.stringify` of `snapshot(player)` **without** `time` (compute the snapshot, delete `time`, stringify) and schedules a publish 250 ms later (trailing, one pending at a time); a `setInterval` of 1 s publishes while `player.playing`. A publish sends the full `snapshot(player)` (with `time`) via `api.jukeboxState(token, state)`; errors are ignored (the next one retries); a 409 means this tab is no longer the speaker → `disableSpeaker()`.
- `App.svelte`: once `ready` and `auth.local`, call `resumeSpeaker()` once.
- `AudioDeEstaCompu.svelte` (in Ajustes → Reproducción, under normalization; it is not part of the Guardar form: it acts at once):

```svelte
<label class="check">
  <input type="checkbox" checked={speaker.on} onchange={(e) => (e.currentTarget.checked ? enableSpeaker() : disableSpeaker())} />
  <span>
    Usar el audio de esta compu para tordo
    <small>Los teléfonos en modo Rockola eligen qué suena acá. Mantené esta pestaña abierta.</small>
  </span>
</label>
```

- `Nav.svelte`: while `speaker.on`, a chip before `<Estado />`: `🔊 Audio de esta compu` (link to `/ajustes`); when `player.blocked`, the chip is a button **Reactivar audio** calling `reactivate()`; when on but not `speaker.connected`, the chip shows `🔊 Reconectando…`.

- [ ] **Step 1: Write the failing tests** (`speaker.svelte.test.js`; `vi.mock('./api.js')`, `vi.mock('./player.svelte.js')` with a `$state`-free fake `player` object and `vi.fn()` functions, a `FakeSource` class with `listeners`, `emit(type, data)` (calls listeners with `{ data: JSON.stringify(data) }`), `readyState`, `close()`; fake timers):
  - enable → `connect` called with `/api/jukebox/speaker`; `localStorage` remembers; `hello` → `speaker.connected`, `api.jukeboxState` called with the token.
  - `command` `{command: 'next', role: 'host', session: 0, remote: 'r1'}` → fake `P.next` called; a guest `next` → `api.jukeboxNotice(token, 'r1', 'Solo el anfitrión puede hacer esto.')`.
  - `replaced` → `speaker.on` false, storage cleared, `notify` called.
  - error with `readyState` 2 → reconnects after 1 s, then 2 s.
  - `resumeSpeaker` with nothing remembered → no connect; remembered → connect and `player.blocked` true.
  - disable → source closed; no further publishes.
  (Publishing on state changes depends on `$effect`; cover it in the manual check, not here.)
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/speaker.svelte.test.js`: fails.
- [ ] **Step 3: Implement** the library, the component, Ajustes, Nav and App changes.
- [ ] **Step 4: Run** `cd web && npx vitest run` and `cd web && npm run build`: pass.
- [ ] **Step 5: Commit** `feat: Audio de esta compu, the browser tab that plays the Rockola`

---

### Task 7: Web: Rockola mode and the `playback` facade

**Files:**
- Create: `web/src/lib/rockola.svelte.js`, `web/src/lib/rockola.svelte.test.js`, `web/src/lib/playback.svelte.js`, `web/src/lib/playback.svelte.test.js`
- Modify: every component and page importing from `../lib/player.svelte.js` (see the list below) to import from `../lib/playback.svelte.js` instead

**Interfaces:**
- Consumes: Task 3 (events, command), Task 4, Task 5 (`allowed`).
- Produces:
  - `rockola.svelte.js`:

```js
export const rockola = $state({ on: false, connected: false, remote: null, session: 0, role: 'guest', view: null })
export const rockolaAvailable = () => !!app.status?.jukebox?.speaker // reactive
export function setRockola(on, connect = (u) => new EventSource(u))
export async function send(command, args = {})   // POST; handles 403/404/409 (see below)
```

  - `playback.svelte.js` — the same names components use today, plus what the bar needs:

```js
export { player } from './player.svelte.js'       // the local player (EstacionAjustes etc. may still read it)
export function shown()                            // what the bar shows: rockola.view when on, else player
export function remote()                           // rockola.on
export function can(command, entry = null)         // local: true; Rockola: allowed(role, command), and for removeAt a guest needs entry.addedBy === rockola.session
export function play(what, opts = {}), enqueue(what, opts = {}), playStation(st), stopStation(), setStation(st),
  toggle(), next(), prev(), jumpTo(pos), moveTo(from, to), removeAt(pos), clearUpcoming(), seek(seconds), setVolume(v),
  toggleMute(), toggleShuffle(), cycleRepeat()
```

Rockola behavior:
- `setRockola(true)`: if `player.playing`, pause it (`P.toggle()`); `rockola.on = true`; open `connect('/api/jukebox/events')`. `setRockola(false)`: close it, reset `rockola` to its initial values.
- `hello` → `remote`, `session`, `role`, `connected = true`. `state` → `view = data`. `notice` → `notify(data.text)`. `role` → `rockola.role = data.role`. `speaker-off` → `notify('Se perdió la conexión con la rockola.')` and `setRockola(false)` — but on the very first `speaker-off` right after `hello` (the speaker was already gone when turning on) the text is `La rockola no está disponible: activá el audio en la compu.`.
- `error`: `readyState` 0 → `connected = false` (the browser reconnects; the next `hello` brings a new remote id). `readyState` 2 → `notify('Se perdió la conexión con la rockola.')`, `setRockola(false)`, and `checkAuth()` (a closed session shows the PIN screen).
- `send`: `api.jukeboxCommand(rockola.remote, command, args)`; on `ApiError` 409 → `notify(e.message)` and `setRockola(false)`; 403 → `notify(e.message)`; 404 → nothing (the stream is reconnecting); other errors → `notify(e.message)`.

Facade behavior (each function: when `!rockola.on`, call the player's function with the same arguments; else):
- `play(what, { shuffle, startId })`: anfitrión → `send('play', { what, shuffle: !!shuffle, startId: startId ?? null })`; invitado → `send('enqueue', { what: startId != null ? { tracks: [startId] } : what, shuffle: !!shuffle && startId == null, startIfIdle: true })` (decision 10).
- `enqueue(what, { next })` → `send('enqueue', { what, next: !!next })`.
- `playStation(st)` → `send('playStation', { station: st })`; `setStation(st)` → `send('setStation', { station: st })`.
- `jumpTo(pos)` / `removeAt(pos)` → `send(cmd, { key: shown().queue.order[pos] })` (positions are the view's; decision 8). `moveTo(from, to)` → `send('moveTo', { key: order[from], toKey: order[to] })`, with `order = shown().queue.order`.
- `seek(seconds)` → `send('seek', { seconds })`; `setVolume(v)` → `send('setVolume', { volume: v })`.
- The rest → `send(name)`.

Importers to switch to `playback.svelte.js` (same names; nothing else changes in them in this task): `BotonEstacion.svelte`, `MenuPista.svelte`, `Reproductor.svelte`, `Acciones.svelte`, `Duplicados.svelte`, `TarjetaAlbum.svelte`, `ListaCarpeta.svelte`, `SinIdentificar.svelte`, `EstacionAjustes.svelte`, `ListaPistas.svelte`, `ContenidoCarpeta.svelte`, `Cola.svelte`, `pages/Buscar.svelte`, `pages/Estadisticas.svelte`, `pages/Explorar.svelte`, `pages/Lista.svelte`, `pages/Album.svelte`, `pages/Artista.svelte`, `pages/Listas.svelte`, `pages/Inicio.svelte`. After the switch, `grep -rn "player.svelte.js" web/src --include=*.svelte` must list none (only `lib/` files import it).

- [ ] **Step 1: Write the failing tests.**
  - `rockola.svelte.test.js` (`vi.mock('./api.js')` keeping the real `ApiError`, `vi.mock('./player.svelte.js')`, `vi.mock('./app.svelte.js')` for `notify` and `app`, `vi.mock('./auth.svelte.js')`, `FakeSource` as in Task 6): turning on pauses a playing local player and connects to `/api/jukebox/events`; `hello`/`state`/`role` fill `rockola`; `notice` → `notify`; `speaker-off` right after `hello` → the "no está disponible" text and off; later → "Se perdió la conexión…" and off; `send` with a 409 → notice and off; 403 → notice, still on; closed source → off and `checkAuth` called.
  - `playback.svelte.test.js` (mock `./player.svelte.js` and `./rockola.svelte.js`, the latter exposing a plain `rockola` object the test mutates and `send` as `vi.fn()`): off → every function forwards to the player with the same arguments; on as host → `play({album: 1}, {startId: 5})` sends `play {what: {album: 1}, shuffle: false, startId: 5}`; on as guest → the same call sends `enqueue {what: {tracks: [5]}, shuffle: false, startIfIdle: true}`, `play({folder: 'x'}, {shuffle: true})` sends `enqueue {what: {folder: 'x'}, shuffle: true, startIfIdle: true}`; `jumpTo(2)` sends the key at `view.queue.order[2]`; `moveTo(2, 4)` sends `{key: order[2], toKey: order[4]}`; `can('next')` false for a guest, true for a host and when off; `can('removeAt', {addedBy: 3})` true for guest session 3, false for 4; `shown()` is the view when on.
- [ ] **Step 2: Run** `cd web && npx vitest run src/lib/rockola.svelte.test.js src/lib/playback.svelte.test.js`: fails.
- [ ] **Step 3: Implement** both libraries and switch the importers.
- [ ] **Step 4: Run** `cd web && npx vitest run` and `cd web && npm run build`: pass; the grep above is empty.
- [ ] **Step 5: Commit** `feat: Rockola mode on the phone and a playback facade over the local player`

---

### Task 8: Web: bar, queue, Nav switch, devices

**Files:**
- Modify: `web/src/components/Reproductor.svelte`, `web/src/components/Cola.svelte`, `web/src/components/Nav.svelte`, `web/src/components/Dispositivos.svelte`

**Interfaces:**
- Consumes: Task 7 (`shown`, `remote`, `can`, `rockola`, `rockolaAvailable`, `setRockola`), Task 6 (`speaker`), Task 3 (`setSessionHost`, sessions' `host`).

Changes:
- **Reproductor.svelte:** read `const p = $derived(shown())` and use `p` wherever it uses `player` today (`p.queue`, `p.playing`, `p.time`, …). Each control renders only when `can(<command>)`: ⤮ `toggleShuffle`, ⏮ `prev`, ⏭ `next`, 🔁 `cycleRepeat`, the progress input is `disabled` unless `can('seek')`, mute/volume only with `can('setVolume')`; ⏯ is always there. In Rockola mode (`remote()`): the seek and volume inputs send on `onchange` (decision 18; locally keep `oninput`); a label **Rockola · en la compu** above the title (`.rockola-tag`, accent color, small caps, 11 px); and when there is no entry the bar still shows, with the label and the text *La cola de la compu está vacía.* (so the mode is visible and can be turned off). The space-bar shortcut calls the facade's `toggle`.
- **Cola.svelte:** read `p = shown()`; "Vaciar" only with `can('clearUpcoming')`; "Ajustar" and "Detener" of a station only with `can('setStation')`/`can('stopStation')`; a row's play button (`jumpTo`) is a plain row (not a button) unless `can('jumpTo')`; the drag handles show only with `can('moveTo')`; the ✕ shows only when `can('removeAt', u.entry)`. The "y N pistas más" line uses `(p.queue.upcoming ?? next.length) - Math.min(next.length, SHOWN)` so a phone sees the computer's real total.
- **Nav.svelte:** when `!speaker.on && (rockolaAvailable() || rockola.on)`, a toggle button before `<Estado />`: `🎵 Rockola` with `aria-pressed={rockola.on}` and class `on` while on, calling `setRockola(!rockola.on)`; title *Elegí qué suena en la compu*. (A tab that is the speaker never shows it.)
- **Dispositivos.svelte:** each session row gains a checkbox **Anfitrión de la rockola** (`checked={s.host}`, disabled like the close button) calling `api.setSessionHost(s.id, checked)` then `load()`; errors → `notify`. Under the heading, one help line: *Los anfitriones controlan la rockola por completo; el resto solo agrega temas, pausa y quita lo que agregó.*

No new unit tests (view code; the logic is in Tasks 5–7). Verify by building and with the preview (`.claude/launch.json` → the Vite dev server against `go run ./cmd/tordo -dir .run -port 4747 -no-browser`): with two tabs on localhost, one with Audio de esta compu on and the other in Rockola mode — the second shows the first's track, queue and controls; clicking ▶ on an album in the second tab plays it in the first.

- [ ] **Step 1: Implement** the four components.
- [ ] **Step 2: Run** `cd web && npx vitest run` and `cd web && npm run build`: pass.
- [ ] **Step 3: Check in the preview** as described; fix what does not work.
- [ ] **Step 4: Commit** `feat: Rockola in the player bar, queue, top bar and devices`

---

### Task 9: Documentation

**Files:**
- Modify: `README.md` (the spec was already aligned with this plan when it was written)

- `README.md`: status line → M6; a new subsection **Rockola** under "Browsing and listening": what it is; how to turn on *Audio de esta compu* (Ajustes → Reproducción, on the computer; keep the tab open; after reloading, one click on **Reactivar audio**); the phone's 🎵 Rockola switch; anfitriones vs invitados and where to mark them (Ajustes → Red local → Dispositivos); what invitados can do. Match the README's existing tone and length per feature.
- [ ] **Step 1: Edit** the README.
- [ ] **Step 2: Commit** `docs: Rockola in the README`

---

### Final review (Opus)

Whole-branch review against the spec §7.5 and this plan: role checks on both sides, no route outside the LAN gate, no goroutine or connection leaks in the SSE handlers (`-race`), state trimming, the facade covering every importer, Spanish texts. Fix findings before the manual check.

---

### Task 10: Check with the computer and two phones (manual, with the user)

With the LAN on and two phones signed in (one marked anfitrión):

1. Computer: Ajustes → Reproducción → *Usar el audio de esta compu*; the 🔊 chip shows.
2. Guest phone: 🎵 Rockola on → the bar shows the computer's state. ▶ on an album with nothing playing → it starts on the computer; ▶ on another album → it is only added. ✕ on one's own track works; on another's it is not shown. ⏭, volume and Vaciar are not shown.
3. Host phone: ▶ replaces the queue; ⏭, seek and volume work; remove anything.
4. Unmark the host phone in Ajustes → its controls shrink at once.
5. Reload the computer's tab → phones see the state again; the chip asks for **Reactivar audio**; after the click, sound resumes with ⏯.
6. Close the computer's tab → both phones leave Rockola mode with *Se perdió la conexión con la rockola*.
7. Open a second localhost tab and turn the audio on there → the first says *Otro navegador tomó el audio de esta compu*.

Write the findings in the plan (as M5 did) and fix what fails.
