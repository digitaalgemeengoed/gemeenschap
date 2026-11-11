package main

// Platform: hoe de server ervoor staat. De dienst kan zelf niet bij Docker of
// de schijf van de server (en hoort dat ook niet te kunnen); server/status.sh
// schrijft elke vijf minuten een statusbestand, dat hier alleen gelezen wordt.

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"sort"
	"time"
)

type Dienst struct {
	Naam      string `json:"Service"`
	Container string `json:"Name"`
	Stand     string `json:"State"`  // running, exited, …
	Gezond    string `json:"Health"` // healthy, unhealthy, starting of leeg
	Status    string `json:"Status"` // "Up 3 hours (healthy)"
	Beeld     string `json:"Image"`
}

func (d Dienst) Goed() bool {
	return d.Stand == "running" && (d.Gezond == "" || d.Gezond == "healthy")
}

type Status struct {
	Tijd     time.Time `json:"tijd"`
	Infra    string    `json:"infra"`
	Inhoud   string    `json:"inhoud"`
	Pauze    bool      `json:"pauze"`
	Mislukt  []string  `json:"mislukt"`
	Uitrol   []string  `json:"uitrol"`
	Diensten []Dienst  `json:"diensten"`
	BackUp   struct {
		Map   string    `json:"map"`
		Tijd  time.Time `json:"tijd"`
		Bytes int64     `json:"bytes"`
	} `json:"backup"`
	Spiegel int64 `json:"spiegel_bytes"`
	Schijf  struct {
		Totaal int64 `json:"totaal"`
		Vrij   int64 `json:"vrij"`
	} `json:"schijf"`
	Geheugen struct {
		Totaal      int64 `json:"totaal"`
		Beschikbaar int64 `json:"beschikbaar"`
	} `json:"geheugen"`
	Opgestart string `json:"opgestart"`
}

// Oud: het statusbestand is niet meer bijgewerkt (de timer staat stil).
func (s *Status) Oud() bool { return time.Since(s.Tijd) > 15*time.Minute }

// BackUpOud: de laatste back-up is meer dan een dag en een paar uur oud.
func (s *Status) BackUpOud() bool {
	return s.BackUp.Tijd.IsZero() || time.Since(s.BackUp.Tijd) > 27*time.Hour
}

func (s *Status) SchijfVol() bool {
	return s.Schijf.Totaal > 0 && s.Schijf.Vrij*10 < s.Schijf.Totaal
}

func (s *Status) Problemen() int {
	n := 0
	for _, d := range s.Diensten {
		if !d.Goed() {
			n++
		}
	}
	if s.Oud() || s.BackUpOud() || s.SchijfVol() || len(s.Mislukt) > 0 {
		n++
	}
	return n
}

func leesStatus(pad string) (*Status, error) {
	if pad == "" {
		return nil, errors.New("geen statusbestand ingesteld")
	}
	b, err := os.ReadFile(pad)
	if err != nil {
		return nil, err
	}
	var s Status
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	sort.Slice(s.Diensten, func(i, j int) bool { return s.Diensten[i].Naam < s.Diensten[j].Naam })
	return &s, nil
}

func (a *App) platform(w http.ResponseWriter, r *http.Request, s *Sessie) {
	st, err := leesStatus(a.StatusPad)
	data := map[string]any{"Status": st, "Links": a.Links}
	if err != nil {
		data["Fout"] = err.Error()
	}
	a.toon(w, r, "platform", http.StatusOK, pagina{Titel: "Platform", Hier: "platform", Sessie: s, Data: data})
}
