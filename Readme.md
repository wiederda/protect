# protect.exe

Kommandozeilentool für Windows, das **geschützte Werte (z. B. Passwörter) in die Registry eines anderen Benutzers schreibt**, zum Beispiel für ein Dienstkonto.

Du meldest dich als Administrator an, übergibst die Zugangsdaten des Zielkontos, und das Tool erledigt den Rest. Ein Abmelden oder eine zweite Eingabeaufforderung unter dem Zielkonto ist nicht nötig.

Die Werte werden mit der Windows-Datenschutz-Schnittstelle (DPAPI) im Namen des Zielbenutzers verschlüsselt und unter dessen `HKEY_CURRENT_USER` abgelegt. Lesen und entschlüsseln kann sie danach nur dieser Benutzer auf diesem Rechner. Der Administrator kann es nicht.

## Voraussetzungen

- Windows
- Eingabeaufforderung oder PowerShell **als Administrator** gestartet
- Benutzername und Passwort des Zielkontos
- Das Zielkonto darf sich auf diesem Rechner anmelden (lokal oder per Richtlinie erlaubt)

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

### Zugangsdaten des Zielbenutzers

Die Zugangsdaten werden über Umgebungsvariablen übergeben:

| Variable | Beschreibung |
|---|---|
| `T_USER` | Benutzername (Pflicht) |
| `T_DOMAIN` | Domäne. Bei lokalen Konten `.` oder der Rechnername. Ohne Angabe wird `.` verwendet |
| `T_PASS` | Passwort (optional). Ist die Variable nicht gesetzt, fragt das Tool das Passwort **verdeckt** ab, es erscheint nichts auf dem Bildschirm |

### Beispiel

Für das lokale Dienstkonto `test`, drei Passwörter unter `Software\MeineAnwendung`:

```
set T_USER=test
set T_DOMAIN=.
protect.exe "Software\MeineAnwendung" pass1=abc pass2=def pass3=ghi
Passwort für .\test:
OK: 3 Wert(e) geschrieben
```

Danach hat `test` in seinem `HKEY_CURRENT_USER\Software\MeineAnwendung` die Werte `pass1`, `pass2` und `pass3`. Gespeichert sind sie verschlüsselt (Base64-kodiert als Zeichenfolge), nicht im Klartext.

Für ein Domänenkonto setzt du `T_DOMAIN` auf den Domänennamen, z. B. `set T_DOMAIN=FIRMA`.

### Exitcodes

| Code | Bedeutung |
|---|---|
| 0 | Alles geschrieben |
| 1 | Fehler bei der Ausführung (Anmeldung, Profil, Verschlüsselung oder Registry) |
| 2 | Falsche oder fehlende Argumente |

## Fehlerbehebung

| Meldung | Mögliche Ursache |
|---|---|
| `LogonUser: ...` | Benutzername, Domäne oder Passwort falsch, oder dem Konto ist die Anmeldung auf diesem Rechner verboten (Richtlinie "Lokal anmelden verweigern") |
| `LoadUserProfile (als Admin gestartet?)` | Das Tool läuft nicht mit Administratorrechten |
| `stdin ist keine Konsole, bitte T_PASS setzen` | Die Eingabe wird umgeleitet oder gepiped. Dann `T_PASS` setzen |
| `Wert '...': leerer Wert` | Bei `name=wert` fehlt der Wert. Leere Werte werden nicht geschrieben |
| `Ungültiges Argument Nr. n` | Das n-te Argument nach dem Key-Pfad hat kein `=`. Das Argument selbst wird absichtlich nicht ausgegeben, es könnte ein Passwort sein |

## Sicherheitshinweise

- Werte als Kommandozeilenargumente sind für andere Prozesse und in der Befehlshistorie der Shell sichtbar.
- Umgebungsvariablen sind für Prozesse desselben Benutzers lesbar. Wenn du `T_PASS` nutzt, leere sie danach mit `set T_PASS=`. Interaktiv ist die verdeckte Abfrage die bessere Wahl.
- Die geschützten Werte sind an Benutzer und Rechner gebunden. Auf einen anderen Rechner kopiert, lassen sie sich nicht entschlüsseln.

## Werte in der eigenen Anwendung lesen

Die Anwendung muss unter dem Zielkonto laufen. Sie liest den Registry-Wert als Zeichenfolge, dekodiert ihn aus Base64 und übergibt ihn an die Windows-Funktion `CryptUnprotectData`. Der Rückgabewert ist der ursprüngliche Klartext.