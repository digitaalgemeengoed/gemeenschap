// dg-beheer: het beheerscherm van het platform, voor de beheerders en de
// kerngroep. Leden en rollen en organisaties (in Keycloak), en hoe de server
// ervoor staat. Zie ONTWERP.md.
//
//	dg-beheer                  de dienst (poort 8080)
//	dg-beheer gezond           voor de healthcheck van Docker
//	dg-beheer logboek          het logboek naar stdout (back-up)
//	dg-beheer logboek-terug    het logboek van stdin terugzetten (samenvoegen)
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
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
	lb := &Logboek{Pad: env("LOGBOEK", "/data/logboek.jsonl")}
	switch opdracht {
	case "gezond":
		resp, err := http.Get("http://127.0.0.1" + env("ADRES", ":8080") + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		return
	case "logboek":
		if err := lb.Uitvoer(os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	case "logboek-terug":
		n, err := lb.Terugzetten(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d regel(s) teruggezet\n", n)
		return
	case "", "dienst":
	default:
		log.Fatalf("onbekende opdracht %q", opdracht)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := &http.Client{Timeout: 30 * time.Second}
	uitgever := strings.TrimRight(moet("OIDC_UITGEVER"), "/")
	intern := strings.TrimRight(env("OIDC_INTERN", uitgever), "/")
	basis := strings.TrimRight(moet("BEHEER_BASIS_URL"), "/")
	geheim := sha256.Sum256([]byte(moet("SESSIE_GEHEIM")))
	aanmelden := uitgever
	if i := strings.Index(uitgever, "/realms/"); i > 0 {
		aanmelden = uitgever[:i]
	}
	app := &App{
		KC: &Keycloak{Realm: intern, ClientID: env("KEYCLOAK_CLIENT_ID", "beheer-keycloak"), Geheim: moet("KEYCLOAK_GEHEIM"), HTTP: h},
		OIDC: &OIDC{Uitgever: uitgever, Intern: intern, ClientID: env("OIDC_CLIENT_ID", "beheer"), ClientGeheim: moet("OIDC_CLIENT_GEHEIM"),
			TerugURL: basis + "/oidc/terug", HTTP: h},
		Sessies:   NieuweSessies(),
		Logboek:   lb,
		Basis:     basis,
		Geheim:    geheim[:],
		StatusPad: env("STATUS", "/status/status.json"),
		Links: Links{
			Site:       strings.TrimRight(moet("SITE_URL"), "/"),
			Aanmelden:  aanmelden,
			Projecten:  strings.TrimRight(os.Getenv("PROJECTEN_URL"), "/"),
			Kennisbank: strings.TrimRight(os.Getenv("KENNIS_URL"), "/"),
			Documenten: strings.TrimRight(os.Getenv("DOCUMENTEN_URL"), "/"),
		},
	}
	// Het logboek moet schrijfbaar zijn; anders liever niet starten.
	if f, err := os.OpenFile(lb.Pad, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640); err != nil {
		log.Fatalf("logboek: %v", err)
	} else {
		f.Close()
	}

	srv := &http.Server{Addr: env("ADRES", ":8080"), Handler: app.Routes(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: time.Minute, WriteTimeout: 2 * time.Minute}
	go func() {
		<-ctx.Done()
		sctx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		srv.Shutdown(sctx)
	}()
	log.Printf("dg-beheer op %s (%s)", srv.Addr, basis)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
