package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func useSavedProxies() []ProxyConfig {
	configFile := "proxy_config.json"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Keine gespeicherte Konfiguration gefunden (%s)\n", configFile)
		return []ProxyConfig{}
	}

	proxies, err := loadProxiesFromFile(configFile)
	if err != nil {
		fmt.Printf("Fehler beim Laden der gespeicherten Konfiguration: %v\n", err)
		fmt.Println("Tipp: Überprüfe das Format der Datei oder verwende eine andere Option")
		return []ProxyConfig{}
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys in der gespeicherten Konfiguration gefunden")
		return []ProxyConfig{}
	}

	fmt.Printf("%d gespeicherte Proxys gefunden\n", len(proxies))

	fmt.Println("\nSoll das Protokoll für alle Proxys geändert werden?")
	fmt.Println("1. Ja, neues Protokoll wählen")
	fmt.Println("2. Nein, gespeicherte Protokolle verwenden")

	choice := promptUser("Wähle Option (1-2) [2]: ")

	if choice == "1" {
		protocol := selectProxyProtocol()
		fmt.Printf("Setze alle Proxys auf Protokoll: %s\n", protocol)

		for i := range proxies {
			proxies[i].Protocol = protocol
		}
	}

	protocolCount := make(map[string]int)
	for i, p := range proxies {
		if p.Protocol == "" {
			proxies[i].Protocol = "socks5"
		}
		protocolCount[p.Protocol]++
	}

	fmt.Println("\nProxy Protokoll Verteilung:")
	for protocol, count := range protocolCount {
		fmt.Printf("  %s: %d Proxys\n", strings.ToUpper(protocol), count)
	}

	fmt.Printf("%d Proxys erfolgreich geladen\n", len(proxies))
	return proxies
}

func loadFromFile() []ProxyConfig {
	filename := promptUser("Pfad zur Proxy-Datei eingeben: ")

	if filename == "" {
		fmt.Println("Fehler: Kein Dateiname eingegeben")
		return []ProxyConfig{}
	}

	proxies, err := loadProxiesFromFile(filename)
	if err != nil {
		fmt.Printf("Fehler beim Laden der Datei: %v\n", err)
		return []ProxyConfig{}
	}

	for i := range proxies {
		if proxies[i].Protocol == "" {
			proxies[i].Protocol = "socks5"
		}
		if proxies[i].Type == "" {
			proxies[i].Type = "rotating"
		}
	}

	fmt.Printf("%d Proxys erfolgreich geladen\n", len(proxies))

	configFile := "proxy_config.json"
	err = saveProxiesToFile(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Konnte Proxys nicht in %s speichern: %v\n", configFile, err)
	} else {
		fmt.Printf("Proxys automatisch in %s gespeichert\n", configFile)
	}

	return proxies
}

func configureProxyList() []ProxyConfig {
	fmt.Println("\nPROXY-LISTE EINFÜGEN")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("Füge deine Proxy-Liste im URL Format ein:")
	fmt.Println("Unterstützte Protokolle:")
	fmt.Println("  socks5://proxy.example.com:1080")
	fmt.Println("  socks4://proxy.example.com:1080")
	fmt.Println("  http://proxy.example.com:8080")
	fmt.Println("  https://proxy.example.com:443")
	fmt.Println("  socks5://username:password@proxy.example.com:1080")
	fmt.Println("\nEnde mit einer leeren Zeile:")

	var lines []string
	scanner := bufio.NewScanner(os.Stdin)

	for {
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}

		lines = append(lines, line)
	}

	if len(lines) == 0 {
		fmt.Println("Fehler: Keine Proxys eingegeben")
		return []ProxyConfig{}
	}

	input := strings.Join(lines, "\n")
	proxies, err := parseProxyList(input)
	if err != nil {
		fmt.Printf("Fehler beim Parsen der Proxy-Liste: %v\n", err)
		return []ProxyConfig{}
	}

	fmt.Printf("%d Proxys erfolgreich geparst\n", len(proxies))

	protocolCount := make(map[string]int)
	for _, p := range proxies {
		protocolCount[p.Protocol]++
	}

	fmt.Println("Erkannte Protokolle:")
	for protocol, count := range protocolCount {
		fmt.Printf("  %s: %d Proxys\n", strings.ToUpper(protocol), count)
	}

	configFile := "proxy_config.json"
	err = saveProxiesToFile(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Konnte Konfiguration nicht speichern: %v\n", err)
	} else {
		fmt.Printf("Proxys automatisch in %s gespeichert\n", configFile)
	}

	return proxies
}

func configureRotatingProxy() []ProxyConfig {
	fmt.Println("\nROTATING PROXY KONFIGURATION")
	fmt.Println(strings.Repeat("-", 40))

	protocol := selectProxyProtocol()
	fmt.Printf("Verwende Protokoll: %s\n", strings.ToUpper(protocol))

	fmt.Println("\nGib die Proxy-Daten ein (Format: host:port:username:password)")
	fmt.Println("Für Proxys ohne Auth nur host:port eingeben")
	fmt.Println("Leere Zeile zum Beenden eingeben")

	var proxies []ProxyConfig
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("Proxy #%d: ", len(proxies)+1)
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			break
		}

		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			fmt.Println("Fehler: Format host:port erwartet")
			continue
		}

		if portNum, err := strconv.Atoi(parts[1]); err != nil || portNum < 1 || portNum > 65535 {
			fmt.Printf("Fehler: Ungültiger Port '%s'\n", parts[1])
			continue
		}

		proxy := ProxyConfig{
			Host:     parts[0],
			Port:     parts[1],
			Protocol: protocol,
			Type:     "rotating",
		}

		if len(parts) >= 4 {
			proxy.Username = parts[2]
			proxy.Password = parts[3]
		}

		proxies = append(proxies, proxy)
		fmt.Printf("Proxy hinzugefügt: %s://%s:%s", protocol, proxy.Host, proxy.Port)
		if proxy.Username != "" {
			fmt.Printf(" (mit Auth)")
		}
		fmt.Println()
	}

	if len(proxies) == 0 {
		fmt.Println("Fehler: Keine Proxys eingegeben")
		return []ProxyConfig{}
	}

	configFile := "proxy_config.json"
	err := saveProxiesToFile(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Speichern fehlgeschlagen: %v\n", err)
	} else {
		fmt.Printf("Konfiguration in %s gespeichert\n", configFile)
	}

	return proxies
}

func addProxiesManually() []ProxyConfig {
	fmt.Println("\nMANUELLE PROXY EINGABE")
	fmt.Println(strings.Repeat("-", 40))

	var proxies []ProxyConfig

	for {
		fmt.Printf("\nProxy #%d konfigurieren\n", len(proxies)+1)
		fmt.Println(strings.Repeat("-", 25))

		host := promptUser("Host: ")
		if host == "" {
			break
		}

		port := promptUser("Port: ")
		if port == "" {
			break
		}

		if portNum, err := strconv.Atoi(port); err != nil || portNum < 1 || portNum > 65535 {
			fmt.Printf("Ungültiger Port: %s\n", port)
			continue
		}

		protocol := selectProxyProtocol()

		username := promptUser("Username (optional): ")
		password := ""
		if username != "" {
			password = promptUser("Password: ")
		}

		proxyType := promptUser("Typ (rotating/static) [rotating]: ")
		if proxyType == "" {
			proxyType = "rotating"
		}

		proxy := ProxyConfig{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
			Type:     proxyType,
			Protocol: protocol,
		}

		proxies = append(proxies, proxy)
		fmt.Printf("Proxy hinzugefügt: %s://%s:%s\n", protocol, host, port)

		if promptUser("Weiteren Proxy hinzufügen? (j/n) [n]: ") != "j" {
			break
		}
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys hinzugefügt")
		return []ProxyConfig{}
	}

	fmt.Printf("\nPROXY ZUSAMMENFASSUNG\n")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Printf("Anzahl Proxys: %d\n", len(proxies))

	protocolCount := make(map[string]int)
	typeCount := make(map[string]int)
	authCount := 0

	for _, p := range proxies {
		protocolCount[p.Protocol]++
		typeCount[p.Type]++
		if p.Username != "" {
			authCount++
		}
	}

	fmt.Println("\nProtokolle:")
	for protocol, count := range protocolCount {
		fmt.Printf("  %s: %d\n", strings.ToUpper(protocol), count)
	}

	fmt.Println("\nTypen:")
	for pType, count := range typeCount {
		fmt.Printf("  %s: %d\n", strings.Title(pType), count)
	}

	fmt.Printf("\nMit Authentifizierung: %d\n", authCount)

	saveChoice := promptUser("\nKonfiguration speichern? (j/n) [j]: ")
	if saveChoice == "" || strings.ToLower(saveChoice)[0] == 'j' {
		configFile := "proxy_config.json"
		err := saveProxiesToFile(proxies, configFile)
		if err != nil {
			fmt.Printf("Fehler beim Speichern: %v\n", err)
		} else {
			fmt.Printf("Konfiguration in %s gespeichert\n", configFile)
		}
	}

	return proxies
}
