# Herkomst

Overgenomen uit `golang.org/x/net/webdav` (v0.47.0, BSD-3-Clause, zie
LICENSE), zonder de tests. Alleen het importpad van `internal/xml` is
aangepast. Meegeleverd in plaats van als module, zodat de dienst geen
afhankelijkheid van buiten heeft naast de Postgres-driver.

Bijwerken: kopieer dezelfde bestanden uit een nieuwere versie en pas het
importpad opnieuw aan.

## Eigen aanpassingen (webdav.go)

Zodat een bestandsbeheer een begrijpelijke fout toont:

- PUT en MKCOL: `os.ErrPermission` geeft 403 Forbidden (was 404 en 405),
  PUT met `os.ErrInvalid` geeft 400.
- PUT: een fout die `ErrInsufficientStorage` omvat, geeft 507 (was 405).

Bij bijwerken: deze wijzigingen opnieuw aanbrengen (zoek op "Documenten:").
