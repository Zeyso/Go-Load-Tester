package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func promptUser(message string) string {
	fmt.Print(message)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

func selectProxyProtocol() string {
	fmt.Println("\nPROXY PROTOKOLL WÄHLEN")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("1. SOCKS5 (empfohlen)")
	fmt.Println("2. SOCKS4")
	fmt.Println("3. HTTP")
	fmt.Println("4. HTTPS")

	choice := promptUser("Wähle Protokoll (1-4) [1]: ")

	switch choice {
	case "2":
		return "socks4"
	case "3":
		return "http"
	case "4":
		return "https"
	default:
		return "socks5"
	}
}

func selectProxyTypes(proxies []ProxyConfig) []ProxyConfig {
	if len(proxies) == 0 {
		return proxies
	}

	typeCount := make(map[string]int)
	for _, p := range proxies {
		if p.Type == "" {
			p.Type = "rotating"
		}
		typeCount[p.Type]++
	}

	if len(typeCount) <= 1 {
		return proxies
	}

	fmt.Println("\nPROXY TYP AUSWAHL")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("Verfügbare Proxy-Typen:")

	var types []string
	for pType, count := range typeCount {
		types = append(types, pType)
		fmt.Printf("  %s: %d Proxys\n", strings.Title(pType), count)
	}

	fmt.Println("\nOptionen:")
	fmt.Println("1. Alle Proxy-Typen verwenden")
	for i, pType := range types {
		fmt.Printf("%d. Nur %s Proxys\n", i+2, strings.Title(pType))
	}

	choice := promptUser(fmt.Sprintf("Wähle Option (1-%d) [1]: ", len(types)+1))

	choiceNum, err := strconv.Atoi(choice)
	if err != nil || choiceNum < 1 || choiceNum > len(types)+1 {
		choiceNum = 1
	}

	if choiceNum == 1 {
		return proxies
	}

	selectedType := types[choiceNum-2]
	fmt.Printf("Verwende nur %s Proxys\n", strings.Title(selectedType))

	var filtered []ProxyConfig
	for _, p := range proxies {
		if p.Type == selectedType {
			filtered = append(filtered, p)
		}
	}

	fmt.Printf("Gefilterte Proxys: %d von %d\n", len(filtered), len(proxies))
	return filtered
}

func selectRotatingMode() bool {
	fmt.Println("\nPROXY VERWENDUNG")
	fmt.Println(strings.Repeat("-", 30))
	fmt.Println("1. Statische Proxys (verwende ersten Proxy)")
	fmt.Println("2. Rotating Proxys (wechsle zwischen allen Proxys)")

	choice := promptUser("Wähle Modus (1-2) [2]: ")

	switch choice {
	case "1":
		fmt.Println("Verwende statischen Proxy-Modus")
		return false
	default:
		fmt.Println("Verwende Rotating Proxy-Modus")
		return true
	}
}

func configureProxies() ([]ProxyConfig, bool) {
	fmt.Println("PROXY KONFIGURATION")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("1. Proxys aus Datei laden")
	fmt.Println("2. Rotating Proxy konfigurieren")
	fmt.Println("3. Proxy-Liste einfügen (URL Format)")
	fmt.Println("4. Gespeicherte Proxys verwenden")
	fmt.Println("5. Manuell Proxys hinzufügen")
	fmt.Println("6. Gespeicherte Proxys testen")
	fmt.Println("7. ProxyScrape Download")
	fmt.Println("8. Ohne Proxys fortfahren")

	choice := promptUser("Wähle eine Option (1-8): ")

	var proxies []ProxyConfig

	switch choice {
	case "1":
		proxies = loadFromFile()
	case "2":
		proxies = configureRotatingProxy()
	case "3":
		proxies = configureProxyList()
	case "4":
		proxies = useSavedProxies()
	case "5":
		proxies = addProxiesManually()
	case "6":
		proxies = testAndManageProxies()
	case "7":
		proxies = configureProxyScrape()
	case "8":
		fmt.Println("Fahre ohne Proxys fort")
		return []ProxyConfig{}, false
	default:
		fmt.Println("Ungültige Auswahl, fahre ohne Proxys fort")
		return []ProxyConfig{}, false
	}

	proxies = selectProxyTypes(proxies)
	useRotating := false

	if len(proxies) > 0 {
		useRotating = selectRotatingMode()
	}

	return proxies, useRotating
}

func getServerConfig() (string, int, int, time.Duration, error) {
	fmt.Println("\nSERVER KONFIGURATION")
	fmt.Println(strings.Repeat("=", 50))

	server := promptUser("Zielserver (host:port) [gommehd.net:80]: ")
	if server == "" {
		server = "gommehd.net:80"
	}

	if _, _, err := net.SplitHostPort(server); err != nil {
		return "", 0, 0, 0, fmt.Errorf("ungültiges server-format '%s': %w", server, err)
	}

	concurrencyStr := promptUser("Anzahl Worker [100]: ")
	concurrency := 100
	if concurrencyStr != "" {
		if c, err := strconv.Atoi(concurrencyStr); err != nil {
			return "", 0, 0, 0, fmt.Errorf("ungültige worker-anzahl: %s", concurrencyStr)
		} else if c > 0 {
			concurrency = c
		}
	}

	rpsStr := promptUser("Requests pro Sekunde [50]: ")
	rps := 50
	if rpsStr != "" {
		if r, err := strconv.Atoi(rpsStr); err != nil {
			return "", 0, 0, 0, fmt.Errorf("ungültige rps-anzahl: %s", rpsStr)
		} else if r > 0 {
			rps = r
		}
	}

	durationStr := promptUser("Testdauer in Sekunden [30]: ")
	duration := 30 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err != nil {
			return "", 0, 0, 0, fmt.Errorf("ungültige dauer: %s", durationStr)
		} else if d > 0 {
			duration = time.Duration(d) * time.Second
		}
	}

	return server, concurrency, rps, duration, nil
}
