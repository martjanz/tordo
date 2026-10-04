package jukebox

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// next reads one event, failing after a second.
func next(t *testing.T, c *Conn) Event {
	t.Helper()
	select {
	case e := <-c.Events():
		return e
	case <-time.After(time.Second):
		t.Fatal("no event arrived")
		return Event{}
	}
}

// none asserts that no event is queued.
func none(t *testing.T, c *Conn) {
	t.Helper()
	select {
	case e := <-c.Events():
		t.Fatalf("unexpected event %q", e.Type)
	default:
	}
}

func closed(c *Conn) bool {
	select {
	case <-c.Done():
		return true
	default:
		return false
	}
}

func TestSpeakerReplaced(t *testing.T) {
	h := New()
	first, oldToken := h.AttachSpeaker()
	e := next(t, first)
	if e.Type != "hello" || e.Data.(map[string]string)["token"] != oldToken {
		t.Fatalf("first event = %+v", e)
	}
	second, newToken := h.AttachSpeaker()
	if e := next(t, first); e.Type != "replaced" {
		t.Fatalf("first got %q, want replaced", e.Type)
	}
	if !closed(first) {
		t.Fatal("first speaker not closed")
	}
	if err := h.Publish(oldToken, json.RawMessage(`{}`)); !errors.Is(err, ErrNotSpeaker) {
		t.Fatalf("old token publish = %v", err)
	}
	if err := h.Publish(newToken, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("new token publish = %v", err)
	}
	if !h.Speaking() {
		t.Fatal("should be speaking")
	}
	h.DetachSpeaker(first)
	if !h.Speaking() {
		t.Fatal("detaching the replaced speaker changed state")
	}
	h.DetachSpeaker(second)
	if h.Speaking() {
		t.Fatal("still speaking after detach")
	}
}

func TestRemoteHello(t *testing.T) {
	h := New()
	r, err := h.AttachRemote(5, Guest)
	if err != nil {
		t.Fatal(err)
	}
	e := next(t, r)
	data := e.Data.(map[string]any)
	if e.Type != "hello" || data["remote"] != r.ID || data["session"] != int64(5) || data["role"] != Guest {
		t.Fatalf("hello = %+v", e)
	}
	if e := next(t, r); e.Type != "speaker-off" {
		t.Fatalf("got %q, want speaker-off", e.Type)
	}

	_, token := h.AttachSpeaker()
	r2, _ := h.AttachRemote(0, Host)
	if e := next(t, r2); e.Type != "hello" {
		t.Fatalf("got %q", e.Type)
	}
	none(t, r2)

	state := json.RawMessage(`{"playing":true}`)
	if err := h.Publish(token, state); err != nil {
		t.Fatal(err)
	}
	r3, _ := h.AttachRemote(0, Host)
	if e := next(t, r3); e.Type != "hello" {
		t.Fatalf("got %q", e.Type)
	}
	e = next(t, r3)
	if e.Type != "state" || string(e.Data.(json.RawMessage)) != string(state) {
		t.Fatalf("state = %+v", e)
	}
}

func TestStateFanOut(t *testing.T) {
	h := New()
	sp, token := h.AttachSpeaker()
	var rs []*Conn
	for i := 0; i < 3; i++ {
		r, _ := h.AttachRemote(int64(i), Guest)
		next(t, r) // hello
		rs = append(rs, r)
	}
	if err := h.Publish(token, json.RawMessage(`{"n":1}`)); err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if e := next(t, r); e.Type != "state" {
			t.Fatalf("got %q", e.Type)
		}
	}
	h.DetachSpeaker(sp)
	for _, r := range rs {
		if e := next(t, r); e.Type != "speaker-off" {
			t.Fatalf("got %q", e.Type)
		}
	}
}

func TestCommandRoles(t *testing.T) {
	h := New()
	sp, _ := h.AttachSpeaker()
	next(t, sp) // hello
	guest, _ := h.AttachRemote(3, Guest)
	host, _ := h.AttachRemote(4, Host)
	for name, need := range commands {
		err := h.Command(guest.ID, 3, Guest, name, nil)
		if need == Host {
			if !errors.Is(err, ErrForbidden) {
				t.Errorf("guest %s = %v, want forbidden", name, err)
			}
		} else if err != nil {
			t.Errorf("guest %s = %v", name, err)
		}
		if err := h.Command(host.ID, 4, Host, name, nil); err != nil {
			t.Errorf("host %s = %v", name, err)
		}
		for len(sp.Events()) > 0 {
			<-sp.Events()
		}
	}
	if err := h.Command(host.ID, 4, Host, "bogus", nil); !errors.Is(err, ErrUnknownCommand) {
		t.Fatalf("bogus = %v", err)
	}

	if err := h.Command(host.ID, 4, Host, "seek", json.RawMessage(`{"to":3}`)); err != nil {
		t.Fatal(err)
	}
	e := next(t, sp)
	cmd := e.Data.(map[string]any)
	if e.Type != "command" || cmd["remote"] != host.ID || cmd["session"] != int64(4) || cmd["role"] != Host ||
		cmd["command"] != "seek" || string(cmd["args"].(json.RawMessage)) != `{"to":3}` {
		t.Fatalf("command = %+v", e)
	}
	if err := h.Command(host.ID, 4, Host, "next", nil); err != nil {
		t.Fatal(err)
	}
	e = next(t, sp)
	if string(e.Data.(map[string]any)["args"].(json.RawMessage)) != `{}` {
		t.Fatalf("empty args = %+v", e.Data)
	}
}

func TestCommandChecks(t *testing.T) {
	h := New()
	r, _ := h.AttachRemote(3, Host)
	if err := h.Command(r.ID, 9, Host, "next", nil); !errors.Is(err, ErrUnknownRemote) {
		t.Fatalf("other session = %v", err)
	}
	if err := h.Command("nope", 3, Host, "next", nil); !errors.Is(err, ErrUnknownRemote) {
		t.Fatalf("unknown id = %v", err)
	}
	if err := h.Command(r.ID, 3, Host, "next", nil); !errors.Is(err, ErrNoSpeaker) {
		t.Fatalf("no speaker = %v", err)
	}
}

func TestSpeakerBusy(t *testing.T) {
	h := New()
	h.AttachSpeaker() // its hello occupies one slot
	r, _ := h.AttachRemote(1, Host)
	for i := 0; i < buffer-1; i++ {
		if err := h.Command(r.ID, 1, Host, "next", nil); err != nil {
			t.Fatalf("command %d = %v", i, err)
		}
	}
	if err := h.Command(r.ID, 1, Host, "next", nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("overflow = %v", err)
	}
}

func TestSlowRemoteDropped(t *testing.T) {
	h := New()
	_, token := h.AttachSpeaker()
	slow, _ := h.AttachRemote(1, Guest)
	for i := 0; i < 70; i++ {
		h.Publish(token, json.RawMessage(`{}`))
	}
	if !closed(slow) {
		t.Fatal("slow remote not dropped")
	}
	// the dropped remote no longer counts toward the limit
	for i := 0; i < MaxRemotes; i++ {
		if _, err := h.AttachRemote(1, Guest); err != nil {
			t.Fatalf("attach %d = %v", i, err)
		}
	}
}

func TestTooManyRemotes(t *testing.T) {
	h := New()
	var rs []*Conn
	for i := 0; i < MaxRemotes; i++ {
		r, err := h.AttachRemote(1, Guest)
		if err != nil {
			t.Fatal(err)
		}
		rs = append(rs, r)
	}
	if _, err := h.AttachRemote(1, Guest); !errors.Is(err, ErrTooMany) {
		t.Fatalf("17th = %v", err)
	}
	h.DetachRemote(rs[0])
	if _, err := h.AttachRemote(1, Guest); err != nil {
		t.Fatalf("after detach = %v", err)
	}
}

func TestNotice(t *testing.T) {
	h := New()
	_, token := h.AttachSpeaker()
	a, _ := h.AttachRemote(1, Guest)
	b, _ := h.AttachRemote(2, Guest)
	next(t, a)
	next(t, b)
	if err := h.Notice(token, a.ID, "hola"); err != nil {
		t.Fatal(err)
	}
	e := next(t, a)
	if e.Type != "notice" || e.Data.(map[string]string)["text"] != "hola" {
		t.Fatalf("notice = %+v", e)
	}
	none(t, b)
	if err := h.Notice(token, "gone", "x"); err != nil {
		t.Fatalf("unknown remote = %v", err)
	}
	if err := h.Notice("bad", a.ID, "x"); !errors.Is(err, ErrNotSpeaker) {
		t.Fatalf("wrong token = %v", err)
	}
}

func TestRoleAndDrops(t *testing.T) {
	h := New()
	a, _ := h.AttachRemote(7, Guest)
	b, _ := h.AttachRemote(8, Guest)
	own, _ := h.AttachRemote(0, Host)
	for _, c := range []*Conn{a, b, own} {
		next(t, c) // hello
		next(t, c) // speaker-off
	}
	h.SetRole(7, Host)
	e := next(t, a)
	if e.Type != "role" || e.Data.(map[string]Role)["role"] != Host {
		t.Fatalf("role = %+v", e)
	}
	none(t, b)

	h.DropSession(7)
	if !closed(a) || closed(b) {
		t.Fatal("DropSession should close only session 7")
	}
	h.DropRemoteSessions()
	if !closed(b) {
		t.Fatal("session 8 not closed")
	}
	if closed(own) {
		t.Fatal("the computer's remote must stay")
	}
}
