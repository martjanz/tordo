package store

import (
	"database/sql"
	"strings"
	"time"
)

// Session is a device signed in from the LAN.
type Session struct {
	ID        int64  `json:"id"`
	Device    string `json:"device"`
	CreatedAt int64  `json:"createdAt"` // unix ms
	LastSeen  int64  `json:"lastSeen"`  // unix ms
	Host      bool   `json:"host"`      // anfitrión of the Rockola
}

const (
	maxDeviceName     = 60
	defaultDeviceName = "Dispositivo"
)

// CreateSession records a device that signed in; tokenHash is the SHA-256
// (hex) of its cookie's token. The device name is trimmed and cut to 60
// runes; an empty one becomes "Dispositivo".
func (s *Store) CreateSession(tokenHash, device string) (int64, error) {
	device = strings.TrimSpace(device)
	if r := []rune(device); len(r) > maxDeviceName {
		device = strings.TrimSpace(string(r[:maxDeviceName]))
	}
	if device == "" {
		device = defaultDeviceName
	}
	var id int64
	err := s.sideWrite(false, func(tx *sql.Tx) error {
		ms := now().UnixMilli()
		res, err := tx.Exec(`INSERT INTO sessions (token_hash, device, created_at, last_seen) VALUES (?, ?, ?, ?)`,
			tokenHash, device, ms, ms)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// SessionByToken finds the session of a cookie's token hash.
func (s *Store) SessionByToken(tokenHash string) (Session, bool, error) {
	var se Session
	err := s.db.QueryRow(`SELECT id, device, created_at, last_seen, host FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&se.ID, &se.Device, &se.CreatedAt, &se.LastSeen, &se.Host)
	if err == sql.ErrNoRows {
		return se, false, nil
	}
	if err != nil {
		return se, false, err
	}
	return se, true, nil
}

// TouchSession marks a session as used now.
func (s *Store) TouchSession(id int64) error {
	return s.sideWrite(false, func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE sessions SET last_seen = ? WHERE id = ?`, now().UnixMilli(), id)
		return err
	})
}

// Sessions lists the signed-in devices, most recently seen first.
func (s *Store) Sessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, device, created_at, last_seen, host FROM sessions ORDER BY last_seen DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	out := []Session{}
	for rows.Next() {
		var se Session
		if err := rows.Scan(&se.ID, &se.Device, &se.CreatedAt, &se.LastSeen, &se.Host); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, se)
	}
	return out, closeRows(rows)
}

// DeleteSession signs a device out; false when it does not exist.
func (s *Store) DeleteSession(id int64) (bool, error) {
	found := false
	err := s.sideWrite(false, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}

// DeleteSessions signs every device out (a new PIN).
func (s *Store) DeleteSessions() error {
	return s.sideWrite(false, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM sessions`)
		return err
	})
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

// PruneSessions deletes the sessions last seen before the given time and
// returns how many.
func (s *Store) PruneSessions(before time.Time) (int, error) {
	n := 0
	err := s.sideWrite(false, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM sessions WHERE last_seen < ?`, before.UnixMilli())
		if err != nil {
			return err
		}
		c, err := res.RowsAffected()
		n = int(c)
		return err
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
