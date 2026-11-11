// dg-documenten: documenten bewaren en delen, voor de coöperatie en per
// project. Een register (Postgres) naast een objectopslag (S3, Garage).
// Zie ONTWERP.md.
//
//	dg-documenten                    de dienst (poort 8080)
//	dg-documenten gezond             voor de healthcheck van Docker
//	dg-documenten spiegel <map>      objecten die er nog niet zijn naar <map> (back-up)
//	dg-documenten terugspiegelen <map>  ontbrekende objecten uit <map> terug naar de opslag
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func env(naam, standaard string) string {
	if v := os.Getenv(naam); v != "" {
		return v
	}
	return standaard
}

func moet(naam string) string {
	v := os.Getenv(naam)
	if v == "" {
		log.Fatalf("%s ontbreekt", naam)
	}
	return v
}

func main() {
	log.SetFlags(0)
	opdracht := ""
	if len(os.Args) > 1 {
		opdracht = os.Args[1]
	}
	if opdracht == "gezond" {
		resp, err := http.Get("http://127.0.0.1" + env("ADRES", ":8080") + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("postgres", moet("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	reg := &Register{db: db}
	opslag := &S3{
		Eindpunt: env("S3_EINDPUNT", "http://garage:3900"),
		Regio:    env("S3_REGIO", "garage"),
		Bucket:   env("S3_BUCKET", "documenten"),
		Sleutel:  moet("S3_SLEUTEL"),
		Geheim:   moet("S3_GEHEIM"),
		HTTP:     &http.Client{Timeout: 0},
	}

	switch opdracht {
	case "spiegel", "terugspiegelen":
		if len(os.Args) != 3 {
			log.Fatalf("gebruik: dg-documenten %s <map>", opdracht)
		}
		var fout error
		if opdracht == "spiegel" {
			fout = spiegel(ctx, reg, opslag, os.Args[2])
		} else {
			fout = terugspiegelen(ctx, reg, opslag, os.Args[2])
		}
		if fout != nil {
			log.Fatal(fout)
		}
		return
	case "", "dienst":
	default:
		log.Fatalf("onbekende opdracht %q", opdracht)
	}

	// Wachten op de database en de opslag (bij het starten van alles tegelijk).
	for i := 0; ; i++ {
		err = migreer(ctx, db)
		if err == nil {
			break
		}
		if i == 60 {
			log.Fatalf("database: %v", err)
		}
		time.Sleep(2 * time.Second)
	}
	for i := 0; ; i++ {
		err = opslag.Klaar(ctx)
		if err == nil {
			break
		}
		if i == 60 {
			log.Fatalf("opslag: %v", err)
		}
		time.Sleep(2 * time.Second)
	}

	maxMB, _ := strconv.Atoi(env("MAX_MB", "500"))
	basis := strings.TrimRight(moet("DOC_BASIS_URL"), "/")
	app := &App{
		Reg:    reg,
		Opslag: opslag,
		OIDC: &OIDC{
			Uitgever:     moet("OIDC_UITGEVER"),
			Intern:       env("OIDC_INTERN", moet("OIDC_UITGEVER")),
			ClientID:     env("OIDC_CLIENT_ID", "documenten"),
			ClientGeheim: moet("OIDC_CLIENT_GEHEIM"),
			TerugURL:     basis + "/oidc/terug",
			HTTP:         &http.Client{Timeout: 15 * time.Second},
		},
		Basis:      basis,
		Geheim:     []byte(moet("SESSIE_GEHEIM")),
		MaxGrootte: int64(maxMB) << 20,
		Projecten:  env("REDMINE_PUBLIEK", ""),
	}
	if g := os.Getenv("DOC_BERICHT_GEHEIM"); g != "" {
		app.Berichten = &WordPress{URL: env("WORDPRESS_URL", "http://wordpress"), Host: moet("WORDPRESS_HOST"), Geheim: g,
			HTTP: &http.Client{Timeout: 60 * time.Second}}
	}
	if u := os.Getenv("REDMINE_URL"); u != "" {
		rm := &Redmine{URL: u, Sleutel: moet("REDMINE_API_KEY"), HTTP: &http.Client{Timeout: 30 * time.Second},
			Rollen: leesRollen(env("DOC_ROLLEN", standaardRollen))}
		interval, _ := time.ParseDuration(env("REDMINE_INTERVAL", "5m"))
		go rm.SynchroniseerSteeds(ctx, reg, interval)
	}

	srv := &http.Server{
		Addr:              env("ADRES", ":8080"),
		Handler:           app.Routes(),
		ReadHeaderTimeout: 20 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	go func() {
		<-ctx.Done()
		sctx, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		srv.Shutdown(sctx)
	}()
	log.Printf("dg-documenten op %s (%s)", srv.Addr, basis)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// spiegel haalt elk object dat het register kent en dat nog niet in de map
// staat. Objecten veranderen nooit, dus wat er staat, blijft staan.
func spiegel(ctx context.Context, reg *Register, opslag *S3, doel string) error {
	objecten, err := reg.Objecten(ctx)
	if err != nil {
		return err
	}
	nieuw, totaal := 0, int64(0)
	for sleutel, v := range objecten {
		pad := filepath.Join(doel, filepath.FromSlash(sleutel))
		if st, err := os.Stat(pad); err == nil && st.Size() == v.Grootte {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(pad), 0o700); err != nil {
			return err
		}
		body, _, err := opslag.Haal(ctx, sleutel)
		if err != nil {
			return fmt.Errorf("%s: %w", sleutel, err)
		}
		tmp := pad + ".deel"
		f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			body.Close()
			return err
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(f, h), body)
		body.Close()
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(tmp)
			return fmt.Errorf("%s: %w", sleutel, err)
		}
		if hex.EncodeToString(h.Sum(nil)) != v.SHA256 {
			os.Remove(tmp)
			return fmt.Errorf("%s: controlegetal klopt niet; de opslag is beschadigd", sleutel)
		}
		if err := os.Rename(tmp, pad); err != nil {
			return err
		}
		nieuw++
		totaal += n
	}
	log.Printf("spiegel: %d objecten in het register, %d nieuw gekopieerd (%d bytes)", len(objecten), nieuw, totaal)
	return nil
}

// terugspiegelen zet objecten die het register kent maar die in de opslag
// ontbreken, terug uit de spiegel.
func terugspiegelen(ctx context.Context, reg *Register, opslag *S3, bron string) error {
	if err := opslag.Klaar(ctx); err != nil {
		return err
	}
	objecten, err := reg.Objecten(ctx)
	if err != nil {
		return err
	}
	aanwezig, err := opslag.Lijst(ctx, "objecten/")
	if err != nil {
		return err
	}
	terug, ontbreekt := 0, 0
	for sleutel, v := range objecten {
		if g, ok := aanwezig[sleutel]; ok && g == v.Grootte {
			continue
		}
		f, err := os.Open(filepath.Join(bron, filepath.FromSlash(sleutel)))
		if err != nil {
			log.Printf("ontbreekt ook in de spiegel: %s", sleutel)
			ontbreekt++
			continue
		}
		h := sha256.New()
		n, err := io.Copy(h, f)
		if err == nil && hex.EncodeToString(h.Sum(nil)) != v.SHA256 {
			err = fmt.Errorf("controlegetal klopt niet")
		}
		if err == nil {
			err = opslag.Zet(ctx, sleutel, f, n, v.SHA256, "application/octet-stream")
		}
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", sleutel, err)
		}
		terug++
	}
	log.Printf("terugspiegelen: %d objecten teruggezet, %d niet te vinden", terug, ontbreekt)
	if ontbreekt > 0 {
		return fmt.Errorf("%d objecten ontbreken in de opslag en in de spiegel", ontbreekt)
	}
	return nil
}
