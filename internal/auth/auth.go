// Package auth checks the PIN that devices on the LAN need, limits wrong
// attempts and makes session tokens. The sessions themselves live in the
// store; the PIN's hash lives in config.json.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrPINFormat = errors.New("el PIN tiene que tener entre 6 y 12 dígitos, solo números")

const (
	// Iterations is the PBKDF2 cost of new hashes.
	Iterations = 100_000

	maxIterations = 10_000_000
	hashScheme    = "pbkdf2-sha256"
	saltLen       = 16
	keyLen        = 32
)

// CheckFormat accepts 6 to 12 ASCII digits.
func CheckFormat(pin string) error {
	if len(pin) < 6 || len(pin) > 12 {
		return ErrPINFormat
	}
	for i := 0; i < len(pin); i++ {
		if pin[i] < '0' || pin[i] > '9' {
			return ErrPINFormat
		}
	}
	return nil
}

// HashPIN derives a key from the PIN with PBKDF2-SHA256: the result is
// "pbkdf2-sha256$<iterations>$<salt>$<key>" (base64.RawURLEncoding).
func HashPIN(pin string) (string, error) {
	if err := CheckFormat(pin); err != nil {
		return "", err
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, pin, salt, Iterations, keyLen)
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	return hashScheme + "$" + strconv.Itoa(Iterations) + "$" + enc.EncodeToString(salt) + "$" + enc.EncodeToString(key), nil
}

// VerifyPIN tells whether pin matches hash; false for a malformed hash.
func VerifyPIN(hash, pin string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != hashScheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < 1 || iter > maxIterations {
		return false
	}
	enc := base64.RawURLEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pin, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// Limiter slows down guessing: the first FreeAttempts wrong PINs are free,
// then each wrong PIN locks logins for a time that starts at FirstLock and
// doubles with every further failure, up to MaxLock. A right PIN resets it.
// One limiter serves every device: there is a single user.
type Limiter struct {
	Now func() time.Time // nil: time.Now

	mu       sync.Mutex
	failures int
	until    time.Time
}

const (
	FreeAttempts = 5
	FirstLock    = 30 * time.Second
	MaxLock      = time.Hour
)

func (l *Limiter) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// Wait is how long logins stay locked; 0 when they are allowed.
func (l *Limiter) Wait() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d := l.until.Sub(l.now()); d > 0 {
		return d
	}
	return 0
}

// Fail records a wrong PIN and, past the free attempts, locks logins.
func (l *Limiter) Fail() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures < FreeAttempts+64 { // enough to reach MaxLock; avoids overflow
		l.failures++
	}
	if l.failures <= FreeAttempts {
		return
	}
	lock := MaxLock
	if shift := l.failures - FreeAttempts - 1; shift < 16 {
		lock = min(FirstLock<<shift, MaxLock)
	}
	l.until = l.now().Add(lock)
}

// Reset forgets every failure: after a right PIN.
func (l *Limiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.failures = 0
	l.until = time.Time{}
}

// NewToken makes a session token (32 random bytes, base64.RawURLEncoding)
// and the hash the store keeps of it.
func NewToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, TokenHash(token), nil
}

// TokenHash is the hex SHA-256 of a session token.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
