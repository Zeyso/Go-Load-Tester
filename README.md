
# Go Load Tester

Ein leistungsstarker Load-Testing-Tool für Server, der über verschiedene Proxy-Typen arbeiten kann. Das Tool unterstützt TCP-Ping-Tests über SOCKS5, SOCKS4, HTTP und HTTPS Proxys mit flexibler Konfiguration und ausführlicher Statistik.

## Features

### 🔄 Proxy-Unterstützung
- **SOCKS5** mit Authentifizierung
- **SOCKS4** Unterstützung
- **HTTP/HTTPS** Proxy Support
- **Rotating Proxys** für verteilte Last
- **Statische Proxys** für konsistente Tests
- **ProxyScrape Integration** für automatischen Proxy-Download

### 🎮 Minecraft Flooder Modi
Das Tool bietet 29 spezialisierte Minecraft-Angriffsmethoden:

#### Basis-Angriffe
1. **joinFlood** - Standard Join-Spam mit zufälligen Benutzernamen
2. **standardFlood** - Standard Ping-Flood für Server-Listen
3. **legacyMotdAttack** - Legacy Server List Ping (ältere Versionen)
4. **motdFlood** - MOTD-basierte Überlastung

#### Erweiterte Protokoll-Angriffe
5. **bigPacketFlood** - Große Pakete für Memory-Exhaustion
6. **invalidPacketFlood** - Ungültige Pakete für Crash-Tests
7. **invalidDataFlood** - Korrupte Datenstrukturen
8. **multiPacketFlood** - Mehrfach-Paket-Bombardierung
9. **smartFlood** - Intelligente Protokoll-Analyse
10. **randomPacketFlood** - Zufällige Paket-Generation

#### Spezialisierte Bypass-Methoden
11. **botJoinerFlood** - Anti-Bot-System Umgehung
12. **fakeJoinAttack** - Simulierte Join-Versuche
13. **motdKillerAttack** - MOTD-Service Überlastung
14. **ultraJoinAttack** - Ultra-schnelle Join-Versuche
15. **fastJoinAttack** - Schnelle Verbindungsaufbau-Tests
16. **aegisKillerAttack** - Anti-Bot-Bypass mit realistischen Mustern
17. **twoLSBypassAttack** - Two Login States Bypass-Technik

#### Performance-Killer
18. **cpuRipperAttack** - CPU-intensive Paket-Verarbeitung
19. **ramKillerFlood** - Memory-Exhaustion Angriffe
20. **networkFlood** - Netzwerk-Bandbreiten-Sättigung
21. **handshakeFlood** - Handshake-Protokoll Überlastung

#### Low-Level Angriffe
22. **byteAttack** - Raw-Byte-Flooding
23. **rawSocketFlood** - Socket-Level Angriffe
24. **tcpFlood** - TCP-Connection Flooding
25. **connectionFlood** - Verbindungs-Exhaustion

#### Kombinierte Angriffe
26. **destroyerAttack** - Kombiniert mehrere Angriffsvektoren
27. **hybridFlood** - Hybrid aus verschiedenen Techniken
28. **adaptiveFlood** - Adaptive Angriffserkennung
29. **ultimateFlood** - Vollständiger Multi-Vektor-Angriff

### 📊 Test-Modi
- **Direkte Verbindung** ohne Proxy
- **Proxy-basierte Tests** mit automatischer Rotation
- **Konfigurierbare Concurrency** (Worker-Anzahl)
- **Rate Limiting** (Requests pro Sekunde)
- **Zeitbasierte Tests** mit konfigurierbarer Dauer

### 💾 Proxy-Verwaltung
- **Automatisches Speichern** in JSON-Format
- **Import aus Dateien** (JSON oder URL-Format)
- **Proxy-Gesundheitstests** mit Fehlerbereinigung
- **Manuelle Proxy-Konfiguration**
- **Batch-Import** über URL-Listen
- **ProxyScrape API Integration**

### 📈 Statistiken & Monitoring
- **Live-Statistiken** während des Tests
- **Detaillierte Fehleranalyse** nach Kategorien
- **Durchschnittliche Antwortzeiten**
- **Erfolgs-/Fehlerquoten** in Echtzeit
- **Proxy-Protokoll-Verteilung**
- **Minecraft-spezifische Metriken**

## Installation

### Voraussetzungen
- Go 1.19 oder höher
- Internet-Verbindung für Proxy-Tests

### Setup
```bash
git clone <repository-url>
cd go-load-tester
go mod init load-tester
go mod tidy
go build -o load-tester *.go
```

### Dependencies
```bash
go get golang.org/x/net/proxy
```

## Verwendung

### Grundlegende Nutzung
```bash
./load-tester
```

# Interaktive Konfiguration
./load-tester

# Wähle "Minecraft Flooder" aus dem Hauptmenü
# Konfiguriere Zielserver (z.B. hypixel.net:25565)
# Wähle Flood-Typ aus 29 verfügbaren Methoden
# Konfiguriere Worker und Dauer

### Proxy-Konfiguration

#### 1. Aus Datei laden
Unterstützte Formate:
```
# JSON Format
{
  "proxies": [
    {
      "host": "proxy.example.com",
      "port": "1080",
      "username": "user",
      "password": "pass",
      "type": "static",
      "protocol": "socks5"
    }
  ]
}

# URL Format (eine pro Zeile)
socks5://user:pass@proxy1.example.com:1080
socks4://proxy2.example.com:1080
http://user:pass@proxy3.example.com:8080
https://proxy4.example.com:443
```

#### 2. Manuelle Eingabe
```
Proxy #1 konfigurieren
Host: proxy.example.com
Port: 1080
Protokoll: SOCKS5
Username: myuser
Password: mypass
Typ: Rotating
```

#### 3. URL-Listen Import
```bash
# Einfach Proxy-URLs einfügen:
socks5://user:pass@proxy1.com:1080
socks5://user:pass@proxy2.com:1080
http://proxy3.com:8080
# Leere Zeile zum Beenden
```
### 4. ProxyScrape Integration
```bash
# Automatischer Download von Proxys
# Wähle ProxyScrape als Quelle
# Konfiguriere Filter (z.B. nur SOCKS5)
# Proxys werden automatisch getestet und gespeichert
# Automatische Duplikatsprüfung
```
### Server-Konfiguration
```
Zielserver: gommehd.net:80
Worker-Anzahl: 100
Requests/Sekunde: 50
Testdauer: 30 Sekunden
```

### Proxy-Modi

#### Statischer Modus
- Verwendet nur den ersten verfügbaren Proxy
- Konsistente IP-Adresse für alle Requests
- Gut für Debugging und spezifische Tests
- Sollte für Proxys mit Auth verwendet werden

#### Rotating Modus
- Wechselt automatisch zwischen allen Proxys
- Verteilte Last über mehrere IP-Adressen
- Optimal für Load-Testing und Anonymität

## Proxy-Tests

### Automatische Gesundheitstests
```bash
# Beim Start werden Proxys automatisch getestet
Test 1/5: socks5://proxy1.com:1080 ... ✅ OK
Test 2/5: socks5://proxy2.com:1080 ... ❌ Timeout
Test 3/5: http://proxy3.com:8080 ... ✅ OK

Funktionierende Proxys: 2
Fehlerhafte Proxys: 1
Erfolgsrate: 66.7%

Fehlerhafte Proxys entfernen? (j/n) [j]: j
```

### Manuelle Proxy-Tests
Menüoption "6. Gespeicherte Proxys testen":
- Testet alle gespeicherten Proxys
- Zeigt detaillierte Fehlermeldungen
- Option zum Entfernen fehlerhafter Proxys
- Aktualisiert automatisch die Konfiguration

## Ausgabe-Beispiel

### Live-Statistiken
```
============================================================
Requests: 45 | Erfolg: 42 (93.3%) | Fehler: 3 (6.7%)
============================================================
```
```
============================================================
MINECRAFT FLOODER - LIVE STATS
============================================================
Flood-Typ: ultimateFlood
Ziel: hypixel.net:25565
Floods: 1250 | Erfolg: 1180 (94.4%) | Fehler: 70 (5.6%)
Floods/Sekunde: 52.3
Aktive Worker: 500
============================================================
```

### Finale Statistiken
```
============================================================
FINALE STATISTIKEN
============================================================
Zielserver: gommehd.net:80
Testdauer: 30s
Gesamt Requests: 1500
Erfolgreiche Requests: 1420 (94.67%)
Fehlgeschlagene Requests: 80 (5.33%)
Requests/Sekunde: 50.00
Durchschnittliche Antwortzeit: 245ms

FEHLER-VERTEILUNG:
  timeout: 65 (81.2%)
  connection_refused: 10 (12.5%)
  proxy_error: 5 (6.3%)
```
```
============================================================
MINECRAFT FLOODER STATISTIKEN
============================================================
Ziel-Server: 1 Server
Flood-Typ: ultimateFlood
Testdauer: 1m0s
Gesamt Floods: 3000
Erfolgreiche Floods: 2850 (95.00%)
Fehlgeschlagene Floods: 150 (5.00%)
Floods/Sekunde: 50.00

FEHLER-VERTEILUNG:
connection_timeout: 120 (80.0%)
protocol_error: 20 (13.3%)
proxy_error: 10 (6.7%)
============================================================
```
## Konfigurationsdateien

### proxy_config.json
Automatisch erstellte Datei mit allen Proxy-Konfigurationen:
```json
{
  "proxies": [
    {
      "host": "proxy.example.com",
      "port": "1080",
      "username": "user",
      "password": "pass",
      "type": "rotating",
      "protocol": "socks5"
    }
  ]
}
```

## Fehlerbehebung

### Häufige Probleme

#### Proxy-Verbindungsfehler
```
❌ Proxy Ping fehlgeschlagen: timeout
```
**Lösung**: Überprüfen Sie Proxy-Credentials und Erreichbarkeit

#### Authentifizierungsfehler
```
❌ authentication failed
```
**Lösung**: Prüfen Sie Username und Passwort

#### DNS-Probleme
```
❌ dns_error: no such host
```
**Lösung**: Überprüfen Sie Proxy-Hostname und Internet-Verbindung

### Debug-Tipps
1. Testen Sie Proxys einzeln mit Option "6"
2. Verwenden Sie weniger Worker bei Problemen
3. Reduzieren Sie Requests/Sekunde bei Rate-Limits
4. Prüfen Sie Firewall-Einstellungen

## Performance-Optimierung

### Empfohlene Einstellungen

#### Kleine Tests
```
Worker: 10-50
Requests/Sekunde: 10-25
Dauer: 10-30 Sekunden
```

#### Mittlere Tests
```
Worker: 50-200
Requests/Sekunde: 25-100
Dauer: 30-120 Sekunden
```

#### Große Tests
```
Worker: 200-1000
Requests/Sekunde: 100-500
Dauer: 60-300 Sekunden
```

### Systemlimits beachten
- Betriebssystem-Dateideskriptor-Limits
- Netzwerk-Bandbreite
- Proxy-Provider Rate-Limits
- Zielserver-Kapazität

## Lizenz

MIT License - siehe LICENSE Datei für Details.

## Beitragen

1. Fork das Repository
2. Erstelle einen Feature-Branch
3. Committe deine Änderungen
4. Push zum Branch
5. Erstelle einen Pull Request

## Support

Bei Problemen oder Fragen:
- Öffne ein Issue im Repository
- Überprüfe die Troubleshooting-Sektion
- Teste mit verschiedenen Proxy-Typen

---

**Hinweis**: Dieses Tool ist nur für legale Penetration-Tests und Load-Testing eigener Server gedacht. Missbrauch für DDoS-Angriffe ist strengstens untersagt.
```