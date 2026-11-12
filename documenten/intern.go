package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Voor WordPress, op het interne netwerk: het niveau van een lid in een ruimte
// (zoals uit Redmine bijgehouden), zodat de site weet wie namens een project
// iets mag doen (bijvoorbeeld een evenement in de agenda zetten). Met hetzelfde
// geheim als de berichten (DOC_BERICHT_GEHEIM); Caddy laat /intern/ niet door.
//
//	GET /intern/niveau?ruimte=<sleutel>&email=<adres>  →  {"niveau": 0..3}
func (a *App) internNiveau(w http.ResponseWriter, r *http.Request) {
	if a.Berichten == nil || a.Berichten.Geheim == "" ||
		subtle.ConstantTimeCompare([]byte(r.Header.Get("X-DG-Sleutel")), []byte(a.Berichten.Geheim)) != 1 {
		http.Error(w, "geen toegang", http.StatusForbidden)
		return
	}
	ruimte := strings.TrimSpace(r.URL.Query().Get("ruimte"))
	email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
	if ruimte == "" || email == "" {
		http.Error(w, "ruimte en email zijn nodig", http.StatusBadRequest)
		return
	}
	var n int
	err := a.Reg.db.QueryRowContext(r.Context(),
		`SELECT l.niveau FROM lid l JOIN ruimte r ON r.id = l.ruimte_id
		 WHERE r.sleutel = $1 AND l.email = $2 AND r.status <> 'weg'`, ruimte, email).Scan(&n)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "database", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"niveau": n, "beheren": n >= Beheren})
}
