"""Wiki.js (de kennisbank) inrichten voor Digitaal GemeenGoed.

Draait in een kleine Python-container naast de wiki (compose.yml, service
wiki-inrichten, profiel gereedschap) bij elke bijwerken.sh. Opnieuw draaien kan
zonder schade: het zet alles weer zoals hieronder staat.

- de eerste keer: de installatie afronden (noodaccount beheer@<domein>);
- titel, adres, alleen inloggen via Keycloak (het formulier voor het
  noodaccount staat niet op het inlogscherm);
- groepen met dezelfde naam als de groepen in Keycloak: /netwerk/gebruikers
  (alle leden: lezen en schrijven), /site/redactie (ook verplaatsen en
  verwijderen) en /site/beheer (alles). Bij elke login neemt de wiki de
  groepen over uit Keycloak;
- gasten (niet ingelogd) mogen niets: de kennisbank is alleen voor leden;
- Nederlands;
- als er een sleutel is: de pagina's ook als Markdown in de GitHub-repo
  (KENNISBANK_REPO), in beide richtingen, elke vijf minuten.

Alleen de standaardbibliotheek; geheimen komen uit de omgeving en worden niet
gelogd.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request

DOMEIN = os.environ["DOMEIN"]
WIKI = os.environ.get("WIKI_INTERN", "http://wiki:3000").rstrip("/")
BASIS = os.environ.get("WIKI_BASIS_URL") or f"https://kennis.{DOMEIN}"
KC_PUBLIEK = os.environ.get("KC_BASIS_URL") or f"https://aanmelden.{DOMEIN}"
KC_INTERN = os.environ.get("KEYCLOAK_INTERN", "http://keycloak:8080").rstrip("/")
REALM = "gemeenschap"
BEHEER = f"beheer@{DOMEIN}"
WACHTWOORD = os.environ["WIKI_BEHEER_WACHTWOORD"]
CLIENT_SECRET = os.environ["WIKI_CLIENT_SECRET"]
REPO = os.environ.get("KENNISBANK_REPO", "")
SLEUTEL = os.environ.get("KENNISBANK_SLEUTEL", "/geheimen/kennisbank")
# De sleutel van de strategie bepaalt het terugkeeradres: /login/dgg/callback.
STRATEGIE = "dgg"


def meld(tekst):
    print(f"  {tekst}", flush=True)


def stop(tekst):
    print(f"  FOUT: {tekst}", file=sys.stderr, flush=True)
    sys.exit(1)


def vraag(pad, data=None, jwt=None, tijd=30):
    kop = {"Content-Type": "application/json"}
    if jwt:
        kop["Authorization"] = f"Bearer {jwt}"
    body = json.dumps(data).encode() if data is not None else None
    req = urllib.request.Request(WIKI + pad, data=body, headers=kop, method="POST" if body else "GET")
    with urllib.request.urlopen(req, timeout=tijd) as r:
        tekst = r.read().decode()
    return json.loads(tekst) if tekst.strip().startswith("{") else tekst


def gq(query, jwt, variabelen=None):
    antwoord = vraag("/graphql", {"query": query, "variables": variabelen or {}}, jwt)
    if antwoord.get("errors"):
        stop("GraphQL: " + "; ".join(e.get("message", "?") for e in antwoord["errors"]))
    return antwoord["data"]


def resultaat(blok, wat):
    r = blok.get("responseResult") or {}
    if not r.get("succeeded"):
        stop(f"{wat}: {r.get('message', 'mislukt')}")


def waarde(v):
    """Instellingen gaan als JSON {"v": waarde} naar de wiki."""
    return json.dumps({"v": v})


class GeenDoorverwijzing(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


ZONDER_DOORVERWIJZING = urllib.request.build_opener(GeenDoorverwijzing)


def wacht_op_wiki():
    """Wacht tot de wiki antwoordt; geeft True als de installatie nog moet."""
    for _ in range(60):
        try:
            with ZONDER_DOORVERWIJZING.open(WIKI + "/", timeout=5) as r:
                return "<title>Wiki.js Setup" in r.read().decode(errors="replace")
        except urllib.error.HTTPError as e:
            # Een doorverwijzing (naar het inloggen) of een andere status: de wiki draait.
            if e.code < 500:
                return False
        except (urllib.error.URLError, OSError):
            pass
        time.sleep(3)
    stop("de wiki antwoordt niet")


# --- Installatie afronden (alleen de eerste keer) ---------------------------
if wacht_op_wiki():
    meld("Wiki.js installeren")
    vraag("/finalize", {
        "adminEmail": BEHEER,
        "adminPassword": WACHTWOORD,
        "adminPasswordConfirm": WACHTWOORD,
        "siteUrl": BASIS,
        "telemetry": False,
    }, tijd=180)
    time.sleep(5)
    for _ in range(60):
        if not wacht_op_wiki():
            break
        time.sleep(3)

# --- Inloggen als noodaccount -------------------------------------------------
login = gq(
    'mutation($u: String!, $p: String!) { authentication { login(username: $u, password: $p, strategy: "local")'
    " { responseResult { succeeded message } jwt } } }",
    None,
    {"u": BEHEER, "p": WACHTWOORD},
)["authentication"]["login"]
resultaat(login, "inloggen als noodaccount")
JWT = login["jwt"]

# --- Site ----------------------------------------------------------------------
r = gq(
    """mutation($host: String!) { site { updateConfig(
        host: $host, title: "Kennisbank · Digitaal GemeenGoed",
        description: "Kennis van en voor gemeenten over open source en Common Ground",
        company: "Stichting Digitaal GemeenGoed", contentLicense: "",
        robots: ["noindex", "nofollow"],
        logoUrl: "/_assets/dgg/icoon.png",
        authAutoLogin: true, authHideLocal: true,
        featurePageRatings: false, featurePageComments: true, featurePersonalWikis: false,
        securityTrustProxy: true, securityOpenRedirect: true, securityIframe: true,
        securityReferrerPolicy: true, securityHSTS: false, uploadScanSVG: true
    ) { responseResult { succeeded message } } } }""",
    JWT,
    {"host": BASIS},
)["site"]["updateConfig"]
resultaat(r, "site-instellingen")
meld(f"Site: {BASIS}, alleen inloggen via Keycloak")

# --- Uiterlijk: de kleuren van de site ----------------------------------------------
# Met "body" ervoor: zwaarder dan de kleuren die Vuetify zelf in de pagina zet.
CSS = """
body .nav-header, body .nav-header .v-toolbar__content { background-color: #0a2654 !important; }
body .v-application .primary { background-color: #14478f !important; border-color: #14478f !important; }
body .v-application .primary--text { color: #14478f !important; caret-color: #14478f !important; }
body .v-application .blue--text { color: #14478f !important; }
body .v-application .contents a { color: #1a56a8; }
body .nav-header-title .title { font-weight: 800 !important; letter-spacing: -.01em; }
"""
resultaat(gq("""mutation($css: String!) { theming { setConfig(theme: "default", iconset: "mdi", darkMode: false,
               tocPosition: "left", injectCSS: $css, injectHead: "", injectBody: "") { responseResult { succeeded message } } } }""",
             JWT, {"css": CSS})["theming"]["setConfig"], "uiterlijk")

# --- Groepen ---------------------------------------------------------------
LEZEN = ["read:pages", "read:assets", "read:comments", "read:source", "read:history"]
SCHRIJVEN = LEZEN + ["write:pages", "write:assets", "write:comments"]
REDACTIE = SCHRIJVEN + ["manage:pages", "delete:pages", "manage:assets", "manage:comments", "write:styles"]
BEHEER_RECHTEN = REDACTIE + [
    "write:scripts", "read:users", "write:users", "manage:users", "write:groups", "manage:groups",
    "manage:navigation", "manage:theme", "manage:api", "manage:system",
]
GROEPEN = {
    "/netwerk/gebruikers": SCHRIJVEN,
    "/site/redactie": REDACTIE,
    "/site/beheer": BEHEER_RECHTEN,
}


def regel(rechten):
    return [{"id": "dgg", "deny": False, "match": "START", "roles": rechten, "path": "", "locales": []}]


bestaand = {g["name"]: g for g in gq("{ groups { list { id name isSystem } } }", JWT)["groups"]["list"]}
UPDATE_GROEP = """mutation($id: Int!, $naam: String!, $rechten: [String]!, $regels: [PageRuleInput]!) {
  groups { update(id: $id, name: $naam, redirectOnLogin: "/", permissions: $rechten, pageRules: $regels)
  { responseResult { succeeded message } } } }"""
ids = {}
for naam, rechten in GROEPEN.items():
    if naam not in bestaand:
        nieuw = gq('mutation($n: String!) { groups { create(name: $n) { responseResult { succeeded message } group { id } } } }',
                   JWT, {"n": naam})["groups"]["create"]
        resultaat(nieuw, f"groep {naam} aanmaken")
        ids[naam] = nieuw["group"]["id"]
    else:
        ids[naam] = bestaand[naam]["id"]
    resultaat(gq(UPDATE_GROEP, JWT, {"id": ids[naam], "naam": naam, "rechten": rechten, "regels": regel(rechten)})["groups"]["update"],
              f"groep {naam}")
# Gasten (groep 2): niets. De kennisbank is alleen voor leden.
resultaat(gq(UPDATE_GROEP, JWT, {"id": 2, "naam": "Guests", "rechten": [], "regels": []})["groups"]["update"], "gasten")
meld("Groepen: " + ", ".join(GROEPEN) + "; gasten zien niets")

# --- Inloggen via Keycloak ---------------------------------------------------
oidc = f"{KC_PUBLIEK}/realms/{REALM}/protocol/openid-connect"
oidc_intern = f"{KC_INTERN}/realms/{REALM}/protocol/openid-connect"
keycloak = {
    "authorizationURL": f"{oidc}/auth",
    # Token en gebruikersgegevens rechtstreeks bij Keycloak, niet via internet.
    "tokenURL": f"{oidc_intern}/token",
    "userInfoURL": f"{oidc_intern}/userinfo",
    "issuer": f"{KC_PUBLIEK}/realms/{REALM}",
    "clientId": "wiki",
    "clientSecret": CLIENT_SECRET,
    "acrValues": "",
    "emailClaim": "email",
    "displayNameClaim": "name",
    "pictureClaim": "picture",
    "groupsClaim": "groups",
    "mapGroups": True,
    "skipUserProfile": False,
    "logoutURL": f"{oidc}/logout?client_id=wiki&post_logout_redirect_uri={BASIS}/",
}
r = gq(
    """mutation($s: [AuthenticationStrategyInput]!) { authentication { updateStrategies(strategies: $s)
       { responseResult { succeeded message } } } }""",
    JWT,
    {"s": [
        {"key": STRATEGIE, "strategyKey": "oidc", "displayName": "Digitaal GemeenGoed", "order": 0, "isEnabled": True,
         "config": [{"key": k, "value": waarde(v)} for k, v in keycloak.items()],
         "selfRegistration": True, "domainWhitelist": [], "autoEnrollGroups": [ids["/netwerk/gebruikers"]]},
        # Het noodaccount: alleen voor dit script en in nood (niet op het inlogscherm).
        {"key": "local", "strategyKey": "local", "displayName": "Noodaccount", "order": 1, "isEnabled": True,
         "config": [], "selfRegistration": False, "domainWhitelist": [], "autoEnrollGroups": []},
    ]},
)["authentication"]["updateStrategies"]
resultaat(r, "inloggen via Keycloak")
meld(f"Inloggen via Keycloak (terugkeeradres {BASIS}/login/{STRATEGIE}/callback)")

# --- Nederlands ----------------------------------------------------------------
# De vertaling komt van de vertaaldienst van Wiki.js (internet nodig).
try:
    gq('mutation { localization { downloadLocale(locale: "nl") { responseResult { succeeded message } } } }', JWT)
    talen = [t["code"] for t in gq("{ localization { locales { code isInstalled } } }", JWT)["localization"]["locales"] if t["isInstalled"]]
    # Alleen een taal kiezen die er echt is: anders kan niemand meer een pagina maken.
    gekozen = "nl" if "nl" in talen else "en"
    gq(f'mutation {{ localization {{ updateLocale(locale: "{gekozen}", autoUpdate: true, namespacing: false, namespaces: []) '
       "{ responseResult { succeeded message } } } }", JWT)
    taal = gq("{ localization { config { locale } } }", JWT)["localization"]["config"]["locale"]
    meld("Taal: Nederlands" if taal == "nl" else "Taal: Nederlands ophalen lukte niet; de wiki blijft voorlopig Engels")
except SystemExit:
    meld("Taal: Nederlands ophalen lukte niet; de wiki blijft voorlopig Engels")

# --- Startpagina (alleen als er nog geen is) ---------------------------------------
taal = gq("{ localization { config { locale } } }", JWT)["localization"]["config"]["locale"]
paden = {(x["path"], x["locale"]) for x in gq("{ pages { list(limit: 5000) { path locale } } }", JWT)["pages"]["list"]}
if ("home", taal) not in paden:
    START = """# Kennisbank

Welkom in de kennisbank van **Digitaal GemeenGoed**: kennis van en voor gemeenten over open source, Common Ground en de projecten van het netwerk.

Iedereen die lid is, kan hier lezen en schrijven. Klik op het potlood rechtsonder om een pagina te wijzigen, of op *Nieuwe pagina* bovenaan om er een te beginnen.

## Waar je wat vindt

- **Projecten**: per project een eigen deel, met de werkwijze, besluiten en handleidingen.
- **Implementatie**: ervaringen en handleidingen bij invoering en beheer.
- **Architectuur**: de modellen staan op [architectuur.%s](https://architectuur.%s); hier de uitleg erbij.

## Spelregels

- Schrijf voor een collega bij een andere gemeente: geen afkortingen zonder uitleg.
- Geen persoonsgegevens, wachtwoorden of de technische inrichting van een gemeente (netwerk, beveiliging).
- Elke wijziging wordt bewaard; je kunt altijd terug naar een eerdere versie.
""" % (DOMEIN, DOMEIN)
    r = gq("""mutation($c: String!, $l: String!) { pages { create(content: $c, description: "Startpagina van de kennisbank",
             editor: "markdown", isPublished: true, isPrivate: false, locale: $l, path: "home", tags: [], title: "Kennisbank")
             { responseResult { succeeded message } } } }""", JWT, {"c": START, "l": taal})["pages"]["create"]
    resultaat(r, "startpagina")
    meld("Startpagina gemaakt")

# --- Pagina's ook in GitHub -----------------------------------------------------
if REPO and os.path.isfile(SLEUTEL):
    git = {
        "authType": "ssh",
        "repoUrl": REPO,
        "branch": "main",
        # De inhoud van de sleutel: de wiki zet hem zelf met de juiste rechten neer.
        "sshPrivateKeyMode": "contents",
        "sshPrivateKeyPath": "",
        "sshPrivateKeyContent": open(SLEUTEL, encoding="ascii").read(),
        "verifySSL": True,
        "basicUsername": "",
        "basicPassword": "",
        "defaultEmail": f"kennisbank@{DOMEIN}",
        "defaultName": "Kennisbank Digitaal GemeenGoed",
        "localRepoPath": "./data/repo",
        "alwaysNamespace": False,
        "gitBinaryPath": "",
    }
    r = gq(
        """mutation($t: [StorageTargetInput]!) { storage { updateTargets(targets: $t) { responseResult { succeeded message } } } }""",
        JWT,
        {"t": [{"key": "git", "isEnabled": True, "mode": "sync", "syncInterval": "PT5M",
                "config": [{"key": k, "value": waarde(v)} for k, v in git.items()]}]},
    )["storage"]["updateTargets"]
    resultaat(r, "Git-opslag")
    meld(f"Pagina's ook in {REPO} (elke vijf minuten, beide richtingen)")
else:
    meld("Git-opslag nog niet: KENNISBANK_REPO of de sleutel ontbreekt (zie README)")

meld("Kennisbank ingericht")
