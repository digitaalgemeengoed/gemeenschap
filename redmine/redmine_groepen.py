"""
Groepen van Keycloak naar Redmine.

Keycloak (realm "gemeenschap") is de bron voor de rol in het netwerk. Dit
script neemt elke paar minuten de groepen onder /netwerk over als groepen in
Redmine ("Netwerk: Members" …), met dezelfde leden. Rollen per project geef je
in Redmine aan die groepen. Wie in /site/beheer staat, wordt beheerder in
Redmine; het lokale noodaccount "admin" blijft altijd beheerder.

Iemand die nog nooit in Redmine heeft ingelogd, heeft daar nog geen account;
die komt bij de volgende ronde na zijn eerste login mee.

Omgeving:
  KEYCLOAK_URL          http://keycloak:8080
  KEYCLOAK_REALM        gemeenschap
  SYNC_CLIENT_ID        redmine-sync
  SYNC_CLIENT_SECRET    (uit .env)
  REDMINE_URL           http://redmine:3000
  REDMINE_API_KEY       API-sleutel van het noodaccount admin
  INTERVAL              seconden tussen twee rondes (600); 0 = één keer
"""

import json
import os
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


def main():
    kc, rm = Keycloak(), Redmine()
    interval = int(os.environ.get("INTERVAL", "600"))
    while True:
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
