# protect.exe

Kommandozeilentool für Windows, das **geschützte Werte (z. B. Passwörter) in die Registry eines Benutzers schreibt**: wahlweise für ein anderes Konto, zum Beispiel ein Dienstkonto, oder für den aktuell angemeldeten Benutzer.

Für ein anderes Konto meldest du dich als Administrator an, übergibst die Zugangsdaten des Zielkontos, und das Tool erledigt den Rest. Ein Abmelden oder eine zweite Eingabeaufforderung unter dem Zielkonto ist nicht nötig. Für den aktuell angemeldeten Benutzer reicht der Aufruf ohne weitere Angaben.

Die Werte werden mit der Windows-Datenschutz-Schnittstelle (DPAPI) im Namen des Zielbenutzers verschlüsselt und unter dessen `HKEY_CURRENT_USER` abgelegt. Lesen und entschlüsseln kann sie danach nur dieser Benutzer auf diesem Rechner. Bei einem anderen Zielkonto kann der Administrator es nicht.

## Voraussetzungen

- Windows
- Für **ein anderes Konto**:
  - Eingabeaufforderung oder PowerShell **als Administrator** gestartet
  - Benutzername und Passwort des Zielkontos
  - Das Zielkonto darf sich auf diesem Rechner anmelden (lokal oder per Richtlinie erlaubt)
- Für den **aktuell angemeldeten Benutzer**: nichts davon. Kein Passwort, keine Administratorrechte

## Installation

Keine. `protect.exe` herunterladen (z. B. von der Releases-Seite) und in einen beliebigen Ordner legen.

## Verwendung

```
protect.exe <Key-Pfad> name=wert [name=wert ...]
protect.exe -h
```

| Argument | Beschreibung |
|---|---|
| `<Key-Pfad>` | Pfad unterhalb von `HKEY_CURRENT_USER` des Zielbenutzers, ohne führenden Backslash, z. B. `Software\MeineAnwendung` |
| `name=wert` | Name des Registry-Werts und der Klartext-Wert. Beliebig viele Paare möglich |

Gibt es einen Wert mit gleichem Namen schon, wird er überschrieben. Andere Werte im selben Key bleiben unberührt. Der Key wird bei Bedarf angelegt.

### Zielbenutzer

Der Zielbenutzer wird über Umgebungsvariablen festgelegt:

| Variable | Beschreibung |
|---|---|
| `T_USER` | Benutzername des Zielkontos |
| `T_DOMAIN` | Domäne. Bei lokalen Konten `.` oder der Rechnername. Ohne Angabe wird die Domäne des aktuell angemeldeten Benutzers verwendet |
| `T_PASS` | Passwort des Zielkontos (optional, nur bei einem anderen Konto nötig). Ist die Variable nicht gesetzt, fragt das Tool das Passwort **verdeckt** ab, es erscheint nichts auf dem Bildschirm |

### Standardwerte

| `T_USER` | `T_DOMAIN` | Ziel | Passwort nötig |
|---|---|---|---|
| gesetzt | gesetzt | genau diese Angabe | ja |
| gesetzt | fehlt | `T_USER` in der Domäne des aktuell angemeldeten Benutzers | ja |
| fehlt | fehlt | der aktuell angemeldete Benutzer | nein |
| fehlt | gesetzt | Fehler (Exitcode 2) | – |

> **Achtung:** Bist du selbst mit einem Domänenkonto angemeldet und das Zielkonto ist ein lokales Konto, setze `T_DOMAIN=.`. Sonst sucht das Tool das Konto in deiner Domäne, und die Anmeldung schlägt fehl.

Mit "aktuell angemeldeter Benutzer" ist der Benutzer gemeint, unter dem der Prozess läuft. Startest du die Konsole über die UAC-Abfrage mit den Zugangsdaten eines anderen Administratorkontos, ist das Ziel dieses Konto und nicht der Benutzer, der am Desktop sitzt.

Das Tool gibt das Ziel am Ende aus (`OK: … geschrieben für DOMÄNE\Benutzer`), sodass ein vergessenes `T_USER` auffällt. Bei einem anderen Konto steht es außerdem in der Passwortabfrage.

### Beispiele

**Anderes Konto:** Für das lokale Dienstkonto `test`, drei Passwörter unter `Software\MeineAnwendung`:

```
set T_USER=test
set T_DOMAIN=.
protect.exe "Software\MeineAnwendung" pass1=abc pass2=def pass3=ghi
Passwort für .\test:
OK: 3 Wert(e) geschrieben für .\test
```

Danach hat `test` in seinem `HKEY_CURRENT_USER\Software\MeineAnwendung` die Werte `pass1`, `pass2` und `pass3`. Gespeichert sind sie verschlüsselt (Base64-kodiert als Zeichenfolge), nicht im Klartext.

Für ein Domänenkonto setzt du `T_DOMAIN` auf den Domänennamen, z. B. `set T_DOMAIN=FIRMA`.

**Aktuell angemeldeter Benutzer:** Beide Variablen weglassen. Es gibt keine Passwortabfrage:

```
protect.exe "Software\MeineAnwendung" pass1=abc
OK: 1 Wert(e) geschrieben für FIRMA\admin
```

### Exitcodes

| Code | Bedeutung |
|---|---|
| 0 | Alles geschrieben |
| 1 | Fehler bei der Ausführung (aktueller Benutzer nicht ermittelbar, Anmeldung, Profil, Verschlüsselung oder Registry) |
| 2 | Falsche oder fehlende Argumente (auch `T_DOMAIN` ohne `T_USER` oder kein Passwort eingegeben) |

## Fehlerbehebung

| Meldung | Mögliche Ursache |
|---|---|
| `LogonUser: ...` | Benutzername, Domäne oder Passwort falsch, oder dem Konto ist die Anmeldung auf diesem Rechner verboten (Richtlinie "Lokal anmelden verweigern"). Bei einem lokalen Zielkonto prüfen, ob `T_DOMAIN=.` gesetzt ist |
| `LoadUserProfile (als Admin gestartet?)` | Das Tool läuft nicht mit Administratorrechten (nötig bei einem anderen Zielkonto) |
| `T_DOMAIN gesetzt, aber T_USER fehlt` | Domäne ohne Benutzer ist nicht erlaubt. `T_USER` setzen oder `T_DOMAIN` entfernen |
| `aktueller Benutzer nicht ermittelbar` | Der angemeldete Benutzer konnte nicht gelesen werden. `T_USER` und `T_DOMAIN` ausdrücklich setzen |
| `kein Passwort eingegeben` | Die Passwortabfrage blieb leer. Passwort eingeben oder `T_PASS` setzen |
| `stdin ist keine Konsole, bitte T_PASS setzen` | Die Eingabe wird umgeleitet oder gepiped. Dann `T_PASS` setzen |
| `Wert '...': leerer Wert` | Bei `name=wert` fehlt der Wert. Leere Werte werden nicht geschrieben |
| `Ungültiges Argument Nr. n` | Das n-te Argument nach dem Key-Pfad hat kein `=`. Das Argument selbst wird absichtlich nicht ausgegeben, es könnte ein Passwort sein |

## Sicherheitshinweise

- Werte als Kommandozeilenargumente sind für andere Prozesse und in der Befehlshistorie der Shell sichtbar.
- Umgebungsvariablen sind für Prozesse desselben Benutzers lesbar. Wenn du `T_PASS` nutzt, leere sie danach mit `set T_PASS=`. Interaktiv ist die verdeckte Abfrage die bessere Wahl.
- Die geschützten Werte sind an Benutzer und Rechner gebunden. Auf einen anderen Rechner kopiert, lassen sie sich nicht entschlüsseln.

## Werte in der eigenen Anwendung lesen

Die Anwendung muss unter dem Zielkonto laufen. Sie liest den Registry-Wert als Zeichenfolge, dekodiert ihn aus Base64 und übergibt ihn an die Windows-Funktion `CryptUnprotectData`. Der Rückgabewert ist der ursprüngliche Klartext.