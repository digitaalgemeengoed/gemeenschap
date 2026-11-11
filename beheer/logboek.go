package main

// Het logboek: elke wijziging, met wie, wanneer en waarom. Eén JSON-regel per
// wijziging in een bestand (alleen toevoegen). De back-up neemt het mee.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Regel struct {
	Tijd      time.Time `json:"tijd"`
	Wie       string    `json:"wie"`  // e-mailadres
	Naam      string    `json:"naam"` // naam van wie het deed
	Actie     string    `json:"actie"`
	Onderwerp string    `json:"onderwerp"` // het lid of de organisatie
	Details   string    `json:"details,omitempty"`
	Reden     string    `json:"reden"`
}

type Logboek struct {
	Pad string
	mu  sync.Mutex
}

func (l *Logboek) Schrijf(r Regel) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r.Tijd.IsZero() {
		r.Tijd = time.Now()
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.Pad, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Laatste: de laatste n regels, de nieuwste eerst; met filter alleen de
// regels waarin dat voorkomt (onderwerp, wie, actie).
func (l *Logboek) Laatste(n int, filter string) ([]Regel, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.Pad)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	filter = strings.ToLower(filter)
	var alle []Regel
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var r Regel
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(r.Onderwerp+" "+r.Wie+" "+r.Actie), filter) {
			continue
		}
		alle = append(alle, r)
		if len(alle) > 4*n {
			alle = append([]Regel(nil), alle[len(alle)-n:]...)
		}
	}
	if len(alle) > n {
		alle = alle[len(alle)-n:]
	}
	for i, j := 0, len(alle)-1; i < j; i, j = i+1, j-1 {
		alle[i], alle[j] = alle[j], alle[i]
	}
	return alle, sc.Err()
}

// Uitvoer: het hele logboek, voor de back-up (dg-beheer logboek).
func (l *Logboek) Uitvoer(w io.Writer) error {
	f, err := os.Open(l.Pad)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

// Terugzetten: het logboek uit een back-up; regels die er al staan, blijven
// staan (samenvoegen op de hele regel).
func (l *Logboek) Terugzetten(in io.Reader) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	er := map[string]bool{}
	if f, err := os.Open(l.Pad); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			er[sc.Text()] = true
		}
		f.Close()
	}
	var nieuw []string
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		t := sc.Text()
		var r Regel
		if json.Unmarshal([]byte(t), &r) != nil || er[t] {
			continue
		}
		er[t] = true
		nieuw = append(nieuw, t)
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if len(nieuw) == 0 {
		return 0, nil
	}
	f, err := os.OpenFile(l.Pad, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	for _, t := range nieuw {
		if _, err := fmt.Fprintln(f, t); err != nil {
			return 0, err
		}
	}
	return len(nieuw), f.Sync()
}
