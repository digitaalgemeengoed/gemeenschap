"""
Groepen van Keycloak naar Redmine.

Keycloak (realm "gemeenschap") is de bron voor de rol in het netwerk. Dit
script neemt elke paar minuten de groepen onder /netwerk over als groepen in
Redmine ("Netwerk: Members" …), met dezelfde leden. Rollen per project geef je
in Redmine aan die groepen. Wie in /site/beheer staat, wordt beheerder in
Redmine; het lokale noodaccount "admin" blijft altijd beheerder.

Iemand die nog nooit in Redmine heeft ingelogd, heeft daar nog geen account;
die komt bij de volgende ronde na zijn eerste login mee.

Projectvoorstellen (project "projectvoorstellen", zie inrichten.rb): wie een
voorstel indiende, wordt volger van zijn voorstel (zo krijgt hij de reacties
van de kerngroep). Heeft hij nog geen account in Redmine, dan maakt dit script
het aan (met naam uit Keycloak); bij zijn eerste login via Keycloak wordt het
op e-mailadres gekoppeld. Een voorstel op "Goedgekeurd" wordt een project:
met de mapnaam als identifier, de netwerkgroepen zoals bij de andere
projecten, de indiener als projectleider; daarna staat het voorstel op
"Gestart". Is de mapnaam bezet, dan gaat het terug naar "In bespreking".

Omgeving:
  KEYCLOAK_URL          http://keycloak:8080
  KEYCLOAK_REALM        gemeenschap
  SYNC_CLIENT_ID        redmine-sync
  SYNC_CLIENT_SECRET    (uit .env)
  REDMINE_URL           http://redmine:3000
  REDMINE_API_KEY       API-sleutel van het noodaccount admin
  INTERVAL              seconden tussen twee rondes (600); 0 = één keer
  DOCUMENTEN_URL        https://documenten.<domein> (voor de link in het voorstel)
"""

import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

NETWERK = {
    "gebruikers": "Netwerk: Gebruikers",
    "contributors": "Netwerk: Contributors",
    "members": "Netwerk: Members",
    "kerngroep": "Netwerk: Kerngroep",
}
BEHEER = "/site/beheer"
NOODACCOUNT = "admin"

# Projectvoorstellen
VOORSTELLEN = "projectvoorstellen"
VOORSTEL_TRACKER = "Projectvoorstel"
ROL_PER_GROEP = {"Netwerk: Gebruikers": "Gebruiker", "Netwerk: Contributors": "Contributor",
                 "Netwerk: Members": "Member", "Netwerk: Kerngroep": "Kerngroep"}
MODULES = ["issue_tracking", "boards", "news", "files", "calendar", "gantt", "roadmap"]
MAPNAAM = re.compile(r"^[a-z][a-z0-9-]{2,39}$")
# Ook in wordpress/dg-projectvoorstel.php: namen die Documenten of Redmine zelf gebruikt.
GERESERVEERD = {"cooperatie", "projectvoorstellen", "nieuw", "new", "mijn", "beheer"}
GERESERVEERD_BEGIN = ("werkgroep-", "mijn-", "project-")


def log(*a):
    print(time.strftime("%Y-%m-%d %H:%M:%S"), *a, flush=True)


def vraag(methode, url, kop=None, lichaam=None, vorm="json"):
    data = None
    kop = dict(kop or {})
    if lichaam is not None:
        if vorm == "form":
            data = urllib.parse.urlencode(lichaam).encode()
            kop["Content-Type"] = "application/x-www-form-urlencoded"
        else:
            data = json.dumps(lichaam).encode()
            kop["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=kop, method=methode)
    with urllib.request.urlopen(req, timeout=30) as r:
        tekst = r.read().decode() or "null"
        return json.loads(tekst) if tekst.strip() else None


class Keycloak:
    def __init__(self):
        self.basis = os.environ["KEYCLOAK_URL"].rstrip("/")
        self.realm = os.environ.get("KEYCLOAK_REALM", "gemeenschap")

    def token(self):
        t = vraag(
            "POST",
            f"{self.basis}/realms/{self.realm}/protocol/openid-connect/token",
            lichaam={
                "grant_type": "client_credentials",
                "client_id": os.environ.get("SYNC_CLIENT_ID", "redmine-sync"),
                "client_secret": os.environ["SYNC_CLIENT_SECRET"],
            },
            vorm="form",
        )
        self.kop = {"Authorization": f"Bearer {t['access_token']}"}

    def get(self, pad):
        return vraag("GET", f"{self.basis}/admin/realms/{self.realm}{pad}", self.kop)

    def persoon(self, mail):
        """Het actieve account met dit e-mailadres, of None."""
        lijst = self.get("/users?exact=true&briefRepresentation=true&email=" + urllib.parse.quote(mail))
        for u in lijst or []:
            if (u.get("email") or "").lower() == mail and u.get("enabled", True):
                return u
        return None

    def groep(self, pad):
        return self.get("/group-by-path" + urllib.parse.quote(pad))

    def leden(self, groep_id):
        """E-mailadressen (klein) van de leden, in delen van 100."""
        uit, eerste = set(), 0
        while True:
            deel = self.get(f"/groups/{groep_id}/members?first={eerste}&max=100&briefRepresentation=true")
            uit |= {u["email"].lower() for u in deel if u.get("email") and u.get("enabled", True)}
            if len(deel) < 100:
                return uit
            eerste += 100


class Redmine:
    def __init__(self):
        self.basis = os.environ["REDMINE_URL"].rstrip("/")
        self.kop = {"X-Redmine-API-Key": os.environ["REDMINE_API_KEY"]}

    def get(self, pad):
        return vraag("GET", self.basis + pad, self.kop)

    def gebruikers(self):
        """Per e-mailadres (klein): id, login en of het een beheerder is. Ook geblokkeerde accounts."""
        uit, offset = {}, 0
        while True:
            deel = self.get(f"/users.json?status=&limit=100&offset={offset}")
            for u in deel["users"]:
                if u.get("mail"):
                    uit[u["mail"].lower()] = u
            offset += 100
            if offset >= deel.get("total_count", 0):
                return uit

    def groepen(self):
        return {g["name"]: g["id"] for g in self.get("/groups.json")["groups"]}

    def leden(self, groep_id):
        g = self.get(f"/groups/{groep_id}.json?include=users")["group"]
        return {u["id"] for u in g.get("users", [])}

    def maak_groep(self, naam, ids):
        return vraag("POST", self.basis + "/groups.json", self.kop, {"group": {"name": naam, "user_ids": sorted(ids)}})["group"]["id"]

    def zet_leden(self, groep_id, ids):
        vraag("PUT", f"{self.basis}/groups/{groep_id}.json", self.kop, {"group": {"user_ids": sorted(ids)}})

    def zet_beheerder(self, user_id, aan):
        vraag("PUT", f"{self.basis}/users/{user_id}.json", self.kop, {"user": {"admin": aan}})

    def zend(self, methode, pad, lichaam=None):
        return vraag(methode, self.basis + pad, self.kop, lichaam)

    def issues(self, project, status):
        uit, offset = [], 0
        while True:
            deel = self.get(f"/issues.json?project_id={project}&status_id={status}&limit=100&offset={offset}")
            uit += deel["issues"]
            offset += 100
            if offset >= deel.get("total_count", 0):
                return uit

    def project(self, identifier):
        try:
            return self.get(f"/projects/{identifier}.json")["project"]
        except urllib.error.HTTPError as e:
            if e.code == 404:
                return None
            raise


def ronde(kc, rm):
    kc.token()
    accounts = rm.gebruikers()
    bestaand = rm.groepen()
    for sleutel, naam in NETWERK.items():
        g = kc.groep(f"/netwerk/{sleutel}")
        mails = kc.leden(g["id"])
        ids = {accounts[m]["id"] for m in mails if m in accounts}
        if naam not in bestaand:
            rm.maak_groep(naam, ids)
            log(f"Groep '{naam}' aangemaakt met {len(ids)} leden")
            continue
        huidig = rm.leden(bestaand[naam])
        if huidig != ids:
            rm.zet_leden(bestaand[naam], ids)
            log(f"Groep '{naam}': {len(ids - huidig)} erbij, {len(huidig - ids)} eraf")
    beheer = kc.leden(kc.groep(BEHEER)["id"])
    for mail, u in accounts.items():
        moet = mail in beheer or u.get("login") == NOODACCOUNT
        if bool(u.get("admin")) != moet:
            rm.zet_beheerder(u["id"], moet)
            log(f"{u.get('login')}: beheerder {'aan' if moet else 'uit'}")


def veld(issue, naam):
    for f in issue.get("custom_fields", []):
        if f.get("name") == naam:
            return (f.get("value") or "").strip()
    return ""


def mapnaam_goed(m):
    return bool(MAPNAAM.match(m)) and m not in GERESERVEERD and not m.startswith(GERESERVEERD_BEGIN)


def account(kc, rm, accounts, mail):
    """Het Redmine-account van dit e-mailadres; zo nodig aangemaakt (naam uit Keycloak)."""
    mail = mail.lower()
    if not mail:
        return None
    if mail in accounts:
        return accounts[mail]
    p = kc.persoon(mail)
    if not p:
        return None
    lokaal = mail.split("@")[0]
    u = rm.zend("POST", "/users.json", {"user": {
        "login": mail, "mail": mail, "firstname": p.get("firstName") or lokaal, "lastname": p.get("lastName") or "-",
        "generate_password": True, "mail_notification": "only_my_events", "language": "nl"}})["user"]
    accounts[mail] = u
    log(f"Account voor {mail} aangemaakt (indiener van een projectvoorstel)")
    return u


def voorstellen(kc, rm):
    if not rm.project(VOORSTELLEN):
        return  # nog niet ingericht (inrichten.rb)
    statussen = {s["name"]: s["id"] for s in rm.get("/issue_statuses.json")["issue_statuses"]}
    accounts = rm.gebruikers()

    # De indiener volgt zijn voorstel.
    for issue in rm.issues(VOORSTELLEN, "open"):
        u = account(kc, rm, accounts, veld(issue, "Indiener"))
        if not u:
            continue
        volgers = {w["id"] for w in rm.get(f"/issues/{issue['id']}.json?include=watchers")["issue"].get("watchers", [])}
        if u["id"] not in volgers and u["id"] != issue.get("author", {}).get("id"):
            rm.zend("POST", f"/issues/{issue['id']}/watchers.json", {"user_id": u["id"]})

    if "Goedgekeurd" not in statussen or "Gestart" not in statussen:
        return
    for issue in rm.issues(VOORSTELLEN, statussen["Goedgekeurd"]):
        start(kc, rm, accounts, statussen, issue)


def start(kc, rm, accounts, statussen, issue):
    nr, naam, mapnaam = issue["id"], issue["subject"].strip(), veld(issue, "Mapnaam").lower()
    merk = f"Gestart vanuit voorstel #{nr}."

    def terug(uitleg):
        rm.zend("PUT", f"/issues/{nr}.json", {"issue": {"status_id": statussen.get("In bespreking"), "notes": uitleg}})
        log(f"Voorstel #{nr}: terug naar In bespreking ({uitleg})")

    if not mapnaam_goed(mapnaam):
        return terug(f"De mapnaam *{mapnaam}* kan niet: 3 tot 40 kleine letters, cijfers en streepjes, "
                     "beginnend met een letter, en niet een naam die het platform zelf gebruikt. "
                     "Pas het veld Mapnaam aan en zet het voorstel opnieuw op Goedgekeurd.")
    bestaand = rm.project(mapnaam)
    if bestaand and merk not in (bestaand.get("description") or ""):
        return terug(f"De mapnaam *{mapnaam}* is al in gebruik. Kies een andere in het veld Mapnaam "
                     "en zet het voorstel opnieuw op Goedgekeurd.")
    if not bestaand:
        trackers = [t["id"] for t in rm.get("/trackers.json")["trackers"] if t["name"] != VOORSTEL_TRACKER]
        project = {"name": naam[:255], "identifier": mapnaam, "is_public": True,
                   "description": f"{issue.get('description') or ''}\n\n{merk}".strip(),
                   "enabled_module_names": MODULES, "tracker_ids": trackers}
        fase = [f["id"] for f in rm.get("/custom_fields.json")["custom_fields"]
                if f.get("name") == "Fase" and f.get("customized_type") == "project"]
        if fase:
            project["custom_fields"] = [{"id": fase[0], "value": "Verkenning"}]
        try:
            bestaand = rm.zend("POST", "/projects.json", {"project": project})["project"]
        except urllib.error.HTTPError as e:
            if e.code != 422:
                raise
            fouten = "; ".join(json.loads(e.read().decode() or "{}").get("errors", [])) or "onbekende fout"
            return terug(f"Redmine kon het project niet aanmaken: {fouten}. Pas het voorstel aan en zet het opnieuw op Goedgekeurd.")
        log(f"Voorstel #{nr}: project {mapnaam} aangemaakt")

    rollen = {r["name"]: r["id"] for r in rm.get("/roles.json")["roles"]}
    leden = rm.get(f"/projects/{bestaand['id']}/memberships.json?limit=100")["memberships"]
    al = {(m.get("group") or m.get("user") or {}).get("id") for m in leden}
    groepen = rm.groepen()
    for groep, rol in ROL_PER_GROEP.items():
        if groep in groepen and groepen[groep] not in al and rol in rollen:
            rm.zend("POST", f"/projects/{bestaand['id']}/memberships.json",
                    {"membership": {"user_id": groepen[groep], "role_ids": [rollen[rol]]}})
    leider = account(kc, rm, accounts, veld(issue, "Indiener"))
    if leider and leider["id"] not in al and "Projectleider" in rollen:
        rm.zend("POST", f"/projects/{bestaand['id']}/memberships.json",
                {"membership": {"user_id": leider["id"], "role_ids": [rollen["Projectleider"]]}})

    docs = os.environ.get("DOCUMENTEN_URL", "").rstrip("/")
    uitleg = [f"Het project is gestart: project:{mapnaam}"]
    if docs:
        uitleg.append(f"De ruimte in Documenten volgt binnen een paar minuten: {docs}/r/{mapnaam}")
    uitleg.append("De mapnaam ligt nu vast; de naam van het project kan de projectleider in Redmine nog veranderen.")
    if not leider:
        uitleg.append("Let op: de indiener heeft geen actief account; voeg zelf een projectleider toe.")
    rm.zend("PUT", f"/issues/{nr}.json", {"issue": {"status_id": statussen["Gestart"], "notes": "\n\n".join(uitleg)}})
    log(f"Voorstel #{nr}: gestart als {mapnaam}")


def main():
    kc, rm = Keycloak(), Redmine()
    interval = int(os.environ.get("INTERVAL", "600"))
    while True:
        try:
            kc.token()
            voorstellen(kc, rm)
        except (urllib.error.URLError, KeyError, ValueError) as e:
            log("Projectvoorstellen mislukt:", e)
        try:
            ronde(kc, rm)
        except (urllib.error.URLError, KeyError, ValueError) as e:
            log("Ronde mislukt:", e)
            if interval == 0:
                sys.exit(1)
        if interval == 0:
            return
        time.sleep(interval)


if __name__ == "__main__":
    main()
