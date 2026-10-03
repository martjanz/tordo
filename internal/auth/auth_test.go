package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCheckFormat(t *testing.T) {
	for _, pin := range []string{"123456", "000000", "123456789012"} {
		if err := CheckFormat(pin); err != nil {
			t.Errorf("CheckFormat(%q) = %v, want nil", pin, err)
		}
	}
	for _, pin := range []string{"12345", "1234567890123", "12345a", "１２３４５６", " 123456", ""} {
		if err := CheckFormat(pin); !errors.Is(err, ErrPINFormat) {
			t.Errorf("CheckFormat(%q) = %v, want ErrPINFormat", pin, err)
		}
	}
}

func TestHashAndVerify(t *testing.T) {
	h, err := HashPIN("482915")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" || parts[1] != "100000" {
		t.Fatalf("unexpected hash format %q", h)
	}
	h2, err := HashPIN("482915")
	if err != nil {
		t.Fatal(err)
	}
	if h == h2 {
		t.Error("two hashes of the same PIN are equal: salt missing")
	}
	if !VerifyPIN(h, "482915") {
		t.Error("VerifyPIN rejects the right PIN")
	}
	if VerifyPIN(h, "482916") {
		t.Error("VerifyPIN accepts a wrong PIN")
	}
	rest := "$" + parts[2] + "$" + parts[3]
	for _, bad := range []string{"", "pbkdf2-sha256$x$y$z", "pbkdf2-sha256$0" + rest, "pbkdf2-sha256$20000000" + rest} {
		if VerifyPIN(bad, "482915") {
			t.Errorf("VerifyPIN accepts malformed hash %q", bad)
		}
	}
	if _, err := HashPIN("12"); !errors.Is(err, ErrPINFormat) {
		t.Errorf("HashPIN(12) = %v, want ErrPINFormat", err)
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	l := &Limiter{Now: func() time.Time { return now }}
	for i := 0; i < FreeAttempts; i++ {
		l.Fail()
		if w := l.Wait(); w != 0 {
			t.Fatalf("failure %d: Wait = %v, want 0", i+1, w)
		}
	}
	l.Fail()
	if w := l.Wait(); w != 30*time.Second {
		t.Fatalf("6th failure: Wait = %v, want 30s", w)
	}
	now = now.Add(30 * time.Second)
	if w := l.Wait(); w != 0 {
		t.Fatalf("after 30s: Wait = %v, want 0", w)
	}
	l.Fail()
	if w := l.Wait(); w != 60*time.Second {
		t.Fatalf("7th failure: Wait = %v, want 60s", w)
	}
	now = now.Add(time.Minute)
	l.Fail()
	if w := l.Wait(); w != 120*time.Second {
		t.Fatalf("8th failure: Wait = %v, want 120s", w)
	}
	for i := 0; i < 20; i++ {
		now = now.Add(l.Wait())
		l.Fail()
		if w := l.Wait(); w > MaxLock {
			t.Fatalf("Wait = %v, over MaxLock", w)
		}
	}
	if w := l.Wait(); w != MaxLock {
		t.Fatalf("Wait = %v, want MaxLock", w)
	}
	l.Reset()
	if w := l.Wait(); w != 0 {
		t.Fatalf("after Reset: Wait = %v, want 0", w)
	}
	for i := 0; i < FreeAttempts; i++ {
		l.Fail()
	}
	if w := l.Wait(); w != 0 {
		t.Fatalf("after Reset, %d failures: Wait = %v, want 0", FreeAttempts, w)
	}
}

func TestTokens(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 43 {
		t.Errorf("token length = %d, want 43", len(token))
	}
	token2, _, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == token2 {
		t.Error("two tokens are equal")
	}
	if TokenHash(token) != hash {
		t.Error("TokenHash(token) != hash")
	}
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		t.Errorf("hash %q is not 64 hex characters", hash)
	}
}
