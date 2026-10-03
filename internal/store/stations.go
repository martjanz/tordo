package store

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// SavedStation is a station the user saved. Def is its definition as JSON,
// as the server wrote it.
type SavedStation struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Def       json.RawMessage `json:"def"`
	CreatedAt int64           `json:"createdAt"`
	UpdatedAt int64           `json:"updatedAt"`
}

var (
	ErrStationName = errors.New("escribí un nombre para la estación")
	errStationDef  = errors.New("definición inválida")
)

// stationFields validates what a write to a station carries.
func stationFields(name string, def []byte) (string, error) {
	name, err := cleanName(name, ErrStationName)
	if err != nil {
		return "", err
	}
	if !json.Valid(def) {
		return "", errStationDef
	}
	return name, nil
}

// Stations lists the saved stations, most recently updated first.
func (s *Store) Stations() ([]SavedStation, error) {
	rows, err := s.db.Query(`SELECT id, name, def, created_at, updated_at FROM stations ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	out := []SavedStation{}
	for rows.Next() {
		var st SavedStation
		var def string
		if err := rows.Scan(&st.ID, &st.Name, &def, &st.CreatedAt, &st.UpdatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		st.Def = json.RawMessage(def)
		out = append(out, st)
	}
	return out, closeRows(rows)
}

// Station reads one saved station.
func (s *Store) Station(id int64) (SavedStation, bool, error) {
	var st SavedStation
	var def string
	err := s.db.QueryRow(`SELECT id, name, def, created_at, updated_at FROM stations WHERE id = ?`, id).
		Scan(&st.ID, &st.Name, &def, &st.CreatedAt, &st.UpdatedAt)
	if err == sql.ErrNoRows {
		return st, false, nil
	}
	if err != nil {
		return st, false, err
	}
	st.Def = json.RawMessage(def)
	return st, true, nil
}

// CreateStation saves a station.
func (s *Store) CreateStation(name string, def []byte) (int64, error) {
	name, err := stationFields(name, def)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.sideWrite(true, func(tx *sql.Tx) error {
		ms := now().UnixMilli()
		res, err := tx.Exec(`INSERT INTO stations (name, def, created_at, updated_at) VALUES (?, ?, ?, ?)`, name, string(def), ms, ms)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		return err
	})
	return id, err
}

// UpdateStation replaces a station's name and definition; false when it does
// not exist.
func (s *Store) UpdateStation(id int64, name string, def []byte) (bool, error) {
	name, err := stationFields(name, def)
	if err != nil {
		return false, err
	}
	found := false
	err = s.sideWrite(true, func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE stations SET name = ?, def = ?, updated_at = ? WHERE id = ?`, name, string(def), now().UnixMilli(), id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}

// DeleteStation deletes a saved station; false when it does not exist.
func (s *Store) DeleteStation(id int64) (bool, error) {
	found := false
	err := s.sideWrite(true, func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM stations WHERE id = ?`, id)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		found = n > 0
		return err
	})
	return found && err == nil, err
}
