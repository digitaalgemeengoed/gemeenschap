package main

// Chat (Matrix): een kamer per werkgroep en per project, en vaste kamers
// voor het hele netwerk. Documenten weet al wie waar lid van is (Redmine,
// WordPress, Keycloak); het account @documenten:<domein> houdt de kamers
// daarmee bij, elke ronde van synchroniseerSteeds.
//
//	#algemeen      iedereen met een chataccount
//	#cooperatie    members en de kerngroep (zoals de ruimte Coöperatie)
//	#kerngroep     de kerngroep
//	#werkgroep-x   de leden van die werkgroep op de site
//	#project-x     wie in Redmine aan dat project meewerkt (met een rol)
//
// Wie erbij hoort, krijgt een uitnodiging; wie er niet meer bij hoort, gaat
// eruit. Wie zelf weggaat of een uitnodiging afslaat, wordt niet opnieuw
// uitgenodigd. Projectleiders en beheerders van een werkgroep (Beheren in
// Documenten) en de kerngroep in de netwerkkamers zijn moderator.
//
// De kamers zijn niet versleuteld van begin tot eind: zo kan wie later
// lid wordt het eerdere gesprek lezen (zoals in een werkgroep op de site).
// Privégesprekken in Element zijn wel versleuteld.
//
// Koppelen aan een persoon gaat op e-mailadres: MAS zet het adres uit
// Keycloak bij het account in Synapse. Wie nog nooit in de chat was, heeft
// nog geen account; die wordt uitgenodigd zodra hij er een heeft.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	chatModerator = 50
	chatBeheer    = 100
)

type Chat struct {
	URL      string // http://synapse:8008
	Server   string // het domein: @naam:<server>
	Publiek  string // https://chat.<domein> (Element Web), voor links
	Token    string // van @documenten:<domein>, met beheerrechten in Synapse
	HTTP     *http.Client
	ik       string
	mu       sync.Mutex
	mensen   map[string]string // e-mailadres → @naam:server
	gelezen  map[string]bool   // accounts waarvan we de adressen al hebben
	ververst time.Time
}

// KamerLink: de kamer openen in Element Web.
func (c *Chat) KamerLink(alias string) string {
	if c == nil || c.Publiek == "" {
		return ""
	}
	return c.Publiek + "/#/room/" + url.PathEscape("#"+alias+":"+c.Server)
}

type matrixFout struct {
	Status int
	Code   string `json:"errcode"`
	Tekst  string `json:"error"`
}

func (f *matrixFout) Error() string {
	return fmt.Sprintf("matrix %d %s: %s", f.Status, f.Code, f.Tekst)
}

func (c *Chat) vraag(ctx context.Context, methode, pad string, lichaam, doel any) error {
	var r io.Reader
	if lichaam != nil {
		b, _ := json.Marshal(lichaam)
		r = bytes.NewReader(b)
	}
	for poging := 0; ; poging++ {
		if r != nil {
			if s, ok := r.(io.Seeker); ok {
				s.Seek(0, io.SeekStart)
			}
		}
		req, err := http.NewRequestWithContext(ctx, methode, strings.TrimRight(c.URL, "/")+pad, r)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Token)
		if lichaam != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests && poging < 5 {
			var w struct {
				Ms int `json:"retry_after_ms"`
			}
			json.Unmarshal(data, &w)
			wacht := time.Duration(max(w.Ms, 500)) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wacht):
			}
			continue
		}
		if resp.StatusCode >= 300 {
			f := &matrixFout{Status: resp.StatusCode}
			json.Unmarshal(data, f)
			return f
		}
		if doel != nil {
			return json.Unmarshal(data, doel)
		}
		return nil
	}
}

func isMatrixCode(err error, code string) bool {
	var f *matrixFout
	return errors.As(err, &f) && f.Code == code
}

// begin: wie zijn we, en geen snelheidslimiet voor ons.
func (c *Chat) begin(ctx context.Context) error {
	if c.ik != "" {
		return nil
	}
	var wie struct {
		UserID string `json:"user_id"`
	}
	if err := c.vraag(ctx, http.MethodGet, "/_matrix/client/v3/account/whoami", nil, &wie); err != nil {
		return err
	}
	c.ik = wie.UserID
	if err := c.vraag(ctx, http.MethodPost, "/_synapse/admin/v1/users/"+url.PathEscape(c.ik)+"/override_ratelimit",
		map[string]int{"messages_per_second": 0, "burst_count": 0}, nil); err != nil {
		log.Printf("chat: snelheidslimiet uitzetten: %v", err)
	}
	return nil
}

// Mensen: e-mailadres → account, voor iedereen met een chataccount.
func (c *Chat) Mensen(ctx context.Context) (map[string]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mensen == nil || time.Since(c.ververst) > 6*time.Hour {
		c.mensen, c.gelezen, c.ververst = map[string]string{}, map[string]bool{}, time.Now()
	}
	van := ""
	for {
		var lijst struct {
			Users []struct {
				Name        string `json:"name"`
				Deactivated bool   `json:"deactivated"`
				Locked      bool   `json:"locked"`
			} `json:"users"`
			Next string `json:"next_token"`
		}
		q := url.Values{"limit": {"200"}, "guests": {"false"}, "deactivated": {"false"}}
		if van != "" {
			q.Set("from", van)
		}
		if err := c.vraag(ctx, http.MethodGet, "/_synapse/admin/v2/users?"+q.Encode(), nil, &lijst); err != nil {
			return nil, err
		}
		for _, u := range lijst.Users {
			if c.gelezen[u.Name] || u.Name == c.ik || u.Deactivated || u.Locked {
				continue
			}
			var info struct {
				Threepids []struct {
					Medium  string `json:"medium"`
					Address string `json:"address"`
				} `json:"threepids"`
			}
			if err := c.vraag(ctx, http.MethodGet, "/_synapse/admin/v2/users/"+url.PathEscape(u.Name), nil, &info); err != nil {
				return nil, err
			}
			for _, t := range info.Threepids {
				if t.Medium == "email" {
					c.mensen[strings.ToLower(t.Address)] = u.Name
				}
			}
			c.gelezen[u.Name] = true
		}
		if lijst.Next == "" {
			break
		}
		van = lijst.Next
	}
	uit := make(map[string]string, len(c.mensen))
	for k, v := range c.mensen {
		uit[k] = v
	}
	return uit, nil
}

// --- De kamers -------------------------------------------------------------------

type gewensteKamer struct {
	Sleutel    string // in chat_kamer, en de alias
	Naam       string
	Onderwerp  string
	Leden      map[string]bool // e-mailadressen
	Moderators map[string]bool // e-mailadressen
	Iedereen   bool            // alle accounts (de kamer Algemeen)
	Weg        bool            // de ruimte bestaat niet meer: iedereen eruit
}

// chatKamers: welke kamers er moeten zijn, met wie.
func (a *App) chatKamers(ctx context.Context) ([]gewensteKamer, error) {
	var uit []gewensteKamer
	set := func(l []string) map[string]bool {
		m := map[string]bool{}
		for _, e := range l {
			m[strings.ToLower(e)] = true
		}
		return m
	}
	kern, members := map[string]bool{}, map[string]bool{}
	if a.Keycloak != nil {
		k, err := a.Keycloak.Leden(ctx, groepKerngroep)
		if err != nil {
			return nil, err
		}
		m, err := a.Keycloak.Leden(ctx, groepMembers)
		if err != nil {
			return nil, err
		}
		kern, members = set(k), set(m)
	}
	coop := map[string]bool{}
	for e := range members {
		coop[e] = true
	}
	for e := range kern {
		coop[e] = true
	}
	uit = append(uit,
		gewensteKamer{Sleutel: "algemeen", Naam: "Algemeen", Onderwerp: "Voor iedereen in het netwerk van Digitaal GemeenGoed: kennismaken, vragen, aankondigingen.", Iedereen: true, Moderators: kern},
		gewensteKamer{Sleutel: "cooperatie", Naam: "Coöperatie", Onderwerp: "Voor de members en de kerngroep van de coöperatie.", Leden: coop, Moderators: kern},
		gewensteKamer{Sleutel: "kerngroep", Naam: "Kerngroep", Onderwerp: "Overleg van de kerngroep.", Leden: kern, Moderators: map[string]bool{}},
	)

	rows, err := a.Reg.db.QueryContext(ctx, `SELECT r.sleutel, r.soort, r.naam, r.status, coalesce(l.email, ''), coalesce(l.niveau, 0)
		FROM ruimte r LEFT JOIN lid l ON l.ruimte_id = r.id AND l.niveau > 0
		WHERE r.soort IN ('project', 'werkgroep')
		ORDER BY r.sleutel`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perSleutel := map[string]*gewensteKamer{}
	var volgorde []string
	for rows.Next() {
		var sleutel, soort, naam, status, email string
		var niveau int
		if err := rows.Scan(&sleutel, &soort, &naam, &status, &email, &niveau); err != nil {
			return nil, err
		}
		k := perSleutel[sleutel]
		if k == nil {
			k = &gewensteKamer{Leden: map[string]bool{}, Moderators: map[string]bool{}, Weg: status == "weg"}
			if soort == "project" {
				k.Sleutel, k.Naam, k.Onderwerp = "project-"+sleutel, "Project: "+naam, "Overleg in het project "+naam+". Documenten: "+a.Basis+"/r/"+sleutel
			} else {
				k.Sleutel, k.Naam, k.Onderwerp = sleutel, "Werkgroep: "+naam, "Overleg in de werkgroep "+naam+". Documenten: "+a.Basis+"/r/"+sleutel
			}
			perSleutel[sleutel] = k
			volgorde = append(volgorde, sleutel)
		}
		if email != "" {
			k.Leden[strings.ToLower(email)] = true
			if niveau >= Beheren {
				k.Moderators[strings.ToLower(email)] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, s := range volgorde {
		uit = append(uit, *perSleutel[s])
	}
	return uit, nil
}

// SynchroniseerChat: kamers aanmaken en de leden bijhouden.
func (a *App) SynchroniseerChat(ctx context.Context) error {
	c := a.Chat
	if err := c.begin(ctx); err != nil {
		return fmt.Errorf("aanmelden: %w", err)
	}
	mensen, err := c.Mensen(ctx)
	if err != nil {
		return fmt.Errorf("accounts: %w", err)
	}
	kamers, err := a.chatKamers(ctx)
	if err != nil {
		return err
	}
	var fouten []string
	for _, k := range kamers {
		if err := a.chatKamer(ctx, k, mensen); err != nil {
			fouten = append(fouten, k.Sleutel+": "+err.Error())
		}
	}
	if fouten != nil {
		return errors.New(strings.Join(fouten, "; "))
	}
	return nil
}

func (a *App) chatKamer(ctx context.Context, k gewensteKamer, mensen map[string]string) error {
	c := a.Chat
	var kamerID string
	err := a.Reg.db.QueryRowContext(ctx, `SELECT kamer FROM chat_kamer WHERE sleutel = $1`, k.Sleutel).Scan(&kamerID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if kamerID == "" {
		if k.Weg {
			return nil // nooit gemaakt, en hoeft niet meer
		}
		if kamerID, err = c.maakKamer(ctx, k); err != nil {
			return err
		}
		if _, err := a.Reg.db.ExecContext(ctx, `INSERT INTO chat_kamer (sleutel, kamer) VALUES ($1, $2) ON CONFLICT (sleutel) DO UPDATE SET kamer = $2`, k.Sleutel, kamerID); err != nil {
			return err
		}
		log.Printf("chat: kamer #%s aangemaakt (%s)", k.Sleutel, kamerID)
	}

	// Wie er moet zijn (accounts).
	moet := map[string]bool{}
	if !k.Weg {
		if k.Iedereen {
			for _, mx := range mensen {
				moet[mx] = true
			}
		}
		for e := range k.Leden {
			if mx := mensen[e]; mx != "" {
				moet[mx] = true
			}
		}
	}
	moderators := map[string]bool{}
	for e := range k.Moderators {
		if mx := mensen[e]; mx != "" && moet[mx] {
			moderators[mx] = true
		}
	}

	// Wie er nu is.
	var leden struct {
		Chunk []struct {
			StateKey string `json:"state_key"`
			Sender   string `json:"sender"`
			Content  struct {
				Membership string `json:"membership"`
			} `json:"content"`
		} `json:"chunk"`
	}
	if err := c.vraag(ctx, http.MethodGet, "/_matrix/client/v3/rooms/"+url.PathEscape(kamerID)+"/members", nil, &leden); err != nil {
		if isMatrixCode(err, "M_FORBIDDEN") || isMatrixCode(err, "M_NOT_FOUND") {
			// De kamer is weg of wij zijn eruit: opnieuw beginnen.
			a.Reg.db.ExecContext(ctx, `DELETE FROM chat_kamer WHERE sleutel = $1`, k.Sleutel)
			return fmt.Errorf("kamer niet (meer) bereikbaar, volgende ronde een nieuwe: %w", err)
		}
		return err
	}
	nu := map[string]string{}
	zelfWeg := map[string]bool{}
	for _, ev := range leden.Chunk {
		nu[ev.StateKey] = ev.Content.Membership
		if ev.Content.Membership == "leave" && ev.Sender == ev.StateKey {
			zelfWeg[ev.StateKey] = true // zelf weggegaan of uitnodiging afgeslagen
		}
	}

	var erbij, eraf []string
	for mx := range moet {
		switch nu[mx] {
		case "join", "invite", "ban":
		default:
			if !zelfWeg[mx] {
				erbij = append(erbij, mx)
			}
		}
	}
	for mx, m := range nu {
		if mx == c.ik || moet[mx] {
			continue
		}
		if m == "join" || m == "invite" {
			eraf = append(eraf, mx)
		}
	}
	sort.Strings(erbij)
	sort.Strings(eraf)
	for _, mx := range erbij {
		if err := c.vraag(ctx, http.MethodPost, "/_matrix/client/v3/rooms/"+url.PathEscape(kamerID)+"/invite", map[string]string{"user_id": mx}, nil); err != nil {
			log.Printf("chat: %s uitnodigen in #%s: %v", mx, k.Sleutel, err)
		}
	}
	reden := "Niet meer bij " + k.Naam
	if k.Weg {
		reden = k.Naam + " bestaat niet meer"
	}
	for _, mx := range eraf {
		if err := c.vraag(ctx, http.MethodPost, "/_matrix/client/v3/rooms/"+url.PathEscape(kamerID)+"/kick", map[string]string{"user_id": mx, "reason": reden}, nil); err != nil {
			log.Printf("chat: %s uit #%s: %v", mx, k.Sleutel, err)
		}
	}
	if len(erbij)+len(eraf) > 0 {
		log.Printf("chat: #%s: %d uitgenodigd, %d eruit", k.Sleutel, len(erbij), len(eraf))
	}
	return c.zetModerators(ctx, kamerID, moderators)
}

func (c *Chat) maakKamer(ctx context.Context, k gewensteKamer) (string, error) {
	var uit struct {
		RoomID string `json:"room_id"`
	}
	err := c.vraag(ctx, http.MethodPost, "/_matrix/client/v3/createRoom", map[string]any{
		"preset":          "private_chat",
		"visibility":      "private",
		"name":            k.Naam,
		"topic":           k.Onderwerp,
		"room_alias_name": k.Sleutel,
		"initial_state": []map[string]any{
			{"type": "m.room.history_visibility", "state_key": "", "content": map[string]string{"history_visibility": "shared"}},
			{"type": "m.room.guest_access", "state_key": "", "content": map[string]string{"guest_access": "forbidden"}},
		},
		// Wij (de maker) hebben vanzelf alle rechten; in kamerversie 12 mag
		// de maker niet in "users" staan.
		"power_level_content_override": map[string]any{
			"users_default":  0,
			"events_default": 0,
			"state_default":  chatModerator,
			"invite":         chatModerator,
			"kick":           chatModerator,
			"ban":            chatModerator,
			"redact":         chatModerator,
			"events": map[string]int{
				"m.room.name":               chatBeheer,
				"m.room.power_levels":       chatBeheer,
				"m.room.canonical_alias":    chatBeheer,
				"m.room.join_rules":         chatBeheer,
				"m.room.history_visibility": chatBeheer,
				"m.room.encryption":         chatBeheer,
				"m.room.tombstone":          chatBeheer,
				"m.room.server_acl":         chatBeheer,
				"m.room.topic":              chatModerator,
				"m.room.avatar":             chatModerator,
				"m.room.pinned_events":      chatModerator,
			},
		},
	}, &uit)
	if isMatrixCode(err, "M_ROOM_IN_USE") {
		// De alias bestaat al (bijvoorbeeld na het herstellen van een back-up): die kamer gebruiken.
		var d struct {
			RoomID string `json:"room_id"`
		}
		if err := c.vraag(ctx, http.MethodGet, "/_matrix/client/v3/directory/room/"+url.PathEscape("#"+k.Sleutel+":"+c.Server), nil, &d); err != nil {
			return "", err
		}
		return d.RoomID, nil
	}
	return uit.RoomID, err
}

// zetModerators: wie moderator hoort te zijn (50), en wie niet meer.
// Alleen niveau 50 raken we aan; wat iemand in Element zelf zette, blijft.
func (c *Chat) zetModerators(ctx context.Context, kamerID string, moderators map[string]bool) error {
	pad := "/_matrix/client/v3/rooms/" + url.PathEscape(kamerID) + "/state/m.room.power_levels/"
	var pl map[string]any
	if err := c.vraag(ctx, http.MethodGet, pad, nil, &pl); err != nil {
		return err
	}
	users, _ := pl["users"].(map[string]any)
	if users == nil {
		users = map[string]any{}
	}
	veranderd := false
	for mx := range moderators {
		if n, _ := users[mx].(float64); n < chatModerator {
			users[mx] = chatModerator
			veranderd = true
		}
	}
	for mx, v := range users {
		if n, _ := v.(float64); n == chatModerator && !moderators[mx] && mx != c.ik {
			delete(users, mx)
			veranderd = true
		}
	}
	if !veranderd {
		return nil
	}
	pl["users"] = users
	return c.vraag(ctx, http.MethodPut, pad, pl, nil)
}

// chatLink: de kamer van een ruimte in Documenten (leeg: geen chat).
func (a *App) chatLink(rm *Ruimte) string {
	if a.Chat == nil || rm == nil {
		return ""
	}
	switch rm.Soort {
	case "project":
		return a.Chat.KamerLink("project-" + rm.Sleutel)
	case "werkgroep":
		return a.Chat.KamerLink(rm.Sleutel)
	case "cooperatie":
		return a.Chat.KamerLink("cooperatie")
	}
	return ""
}
