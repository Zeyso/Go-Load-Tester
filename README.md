
# Go Load Tester

Ein leistungsstarker Load-Testing-Tool für Server, der über verschiedene Proxy-Typen arbeiten kann. Das Tool unterstützt TCP-Ping-Tests über SOCKS5, SOCKS4, HTTP und HTTPS Proxys mit flexibler Konfiguration und ausführlicher Statistik.

## Features

### 🔄 Proxy-Unterstützung
- **SOCKS5** mit Authentifizierung
- **SOCKS4** Unterstützung
- **HTTP/HTTPS** Proxy Support
- **Rotating Proxys** für verteilte Last
- **Statische Proxys** für konsistente Tests

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

### 📈 Statistiken & Monitoring
- **Live-Statistiken** während des Tests
- **Detaillierte Fehleranalyse** nach Kategorien
- **Durchschnittliche Antwortzeiten**
- **Erfolgs-/Fehlerquoten** in Echtzeit
- **Proxy-Protokoll-Verteilung**

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
go build -o load-tester main.go
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

Das Programm führt Sie durch ein interaktives Menü zur Konfiguration.

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