package server

import (
	"net/http"

	"tordo/internal/catalog"
)

// duplicateCounts is how many groups of each kind Revisar has.
type duplicateCounts struct {
	Exact   int `json:"exact"`
	Formats int `json:"formats"`
	Suspect int `json:"suspect"`
}

// duplicates answers Revisar → Duplicados: the counts of every kind and the
// groups of ?tipo= (exactas, formatos, sospechas; anything else: exactas).
func (s *Server) duplicates(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	d := c.Duplicates()
	groups := d.Exact
	switch r.URL.Query().Get("tipo") {
	case "formatos":
		groups = d.Formats
	case "sospechas":
		groups = d.Suspect
	}
	writeJSON(w, struct {
		Counts      duplicateCounts    `json:"counts"`
		Recoverable int64              `json:"recoverable"`
		Groups      []catalog.DupGroup `json:"groups"`
	}{duplicateCounts{len(d.Exact), len(d.Formats), len(d.Suspect)}, d.Recoverable, groups})
}

// formatsToConvert answers Revisar → Formatos a convertir.
func (s *Server) formatsToConvert(w http.ResponseWriter, r *http.Request) {
	c, err := s.catalog()
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, c.ToConvert())
}
