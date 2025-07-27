package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type ProxyScrapeResponse struct {
	Proxies []ProxyScrapeProxy `json:"proxies"`
}

type ProxyScrapeProxy struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Country  string `json:"country"`
	SSL      string `json:"ssl"`
}

type AlternativeProxyScrapeResponse struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    []AlternativeProxyData `json:"data"`
}

type AlternativeProxyData struct {
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
}

// Config-Struktur für verschiedene JSON-Formate
type ConfigFile struct {
	Proxies []ProxyConfig `json:"proxies,omitempty"`
}

func configureProxyScrape() []ProxyConfig {
	fmt.Println("\nPROXYSCRAPE DOWNLOAD")
	fmt.Println(strings.Repeat("-", 40))

	// API-Optionen anzeigen
	fmt.Println("Verfügbare Optionen:")
	fmt.Println("1. Alle Protokolle (SOCKS4, SOCKS5, HTTP)")
	fmt.Println("2. Nur SOCKS5")
	fmt.Println("3. Nur HTTP/HTTPS")
	fmt.Println("4. Manuelle API-URL eingeben")

	choice := promptUser("Wähle Option (1-4) [1]: ")

	var apiURL string
	var preferredProtocol string

	switch choice {
	case "2":
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=socks5&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
		preferredProtocol = "socks5"
	case "3":
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
		preferredProtocol = "http"
	case "4":
		apiURL = promptUser("API-URL eingeben: ")
		if apiURL == "" {
			fmt.Println("Keine URL eingegeben")
			return []ProxyConfig{}
		}
	default:
		apiURL = "https://api.proxyscrape.com/v4/free-proxy-list/get?request=displayproxies&protocol=all&timeout=10000&country=all&ssl=all&anonymity=all&skip=0&limit=1000"
	}

	fmt.Printf("API URL: %s\n", apiURL)
	fmt.Println("Lade Proxys herunter...")

	proxies, err := downloadProxiesFromAPI(apiURL, preferredProtocol)
	if err != nil {
		fmt.Printf("Fehler beim Download: %v\n", err)

		// Fallback: Versuche alternative Quellen
		fmt.Println("Versuche alternative Proxy-Quellen...")
		proxies = tryAlternativeSources()
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys gefunden")
		return []ProxyConfig{}
	}

	fmt.Printf("%d Proxys erfolgreich geladen\n", len(proxies))

	// Protokoll-Statistiken anzeigen
	protocolCount := make(map[string]int)
	for _, p := range proxies {
		protocolCount[p.Protocol]++
	}

	fmt.Println("Geladene Protokolle:")
	for protocol, count := range protocolCount {
		fmt.Printf("  %s: %d Proxys\n", strings.ToUpper(protocol), count)
	}

	// Proxys testen?
	testChoice := promptUser("Möchten Sie die Proxys testen? (j/n) [n]: ")
	if strings.ToLower(testChoice) == "j" {
		fmt.Println("Teste Proxys...")
		working, _, _ := testProxiesParallel(proxies, 50)
		fmt.Printf("Funktionsfähige Proxys: %d von %d\n", len(working), len(proxies))
		proxies = working
	}

	// Automatisch speichern mit Duplikatsprüfung
	configFile := "proxy_config.json"
	newProxies, err := saveProxiesToFileWithDuplicateCheck(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Konnte Proxys nicht speichern: %v\n", err)
	} else {
		fmt.Printf("%d neue Proxys in %s gespeichert (%d Duplikate übersprungen)\n",
			newProxies, configFile, len(proxies)-newProxies)
	}

	return proxies
}

func saveProxiesToFileWithDuplicateCheck(newProxies []ProxyConfig, filename string) (int, error) {
	// Existierende Proxys laden
	existingProxies, err := loadExistingProxies(filename)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("fehler beim Laden existierender Proxys: %w", err)
	}

	// Set für schnelle Duplikatsprüfung erstellen
	existingSet := make(map[string]bool)
	for _, proxy := range existingProxies {
		key := fmt.Sprintf("%s:%s:%s", proxy.Host, proxy.Port, proxy.Protocol)
		existingSet[key] = true
	}

	// Nur neue Proxys hinzufügen
	addedCount := 0

	for _, proxy := range newProxies {
		key := fmt.Sprintf("%s:%s:%s", proxy.Host, proxy.Port, proxy.Protocol)
		if !existingSet[key] {
			existingProxies = append(existingProxies, proxy)
			existingSet[key] = true
			addedCount++
		}
	}

	// Alle Proxys speichern
	err = saveProxiesToFile(existingProxies, filename)
	if err != nil {
		return 0, err
	}

	return addedCount, nil
}

func loadExistingProxies(filename string) ([]ProxyConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return []ProxyConfig{}, err
	}

	// Versuche verschiedene JSON-Formate
	var proxies []ProxyConfig

	// Format 1: Direktes Array
	err = json.Unmarshal(data, &proxies)
	if err == nil {
		return proxies, nil
	}

	// Format 2: Objekt mit "proxies" Field
	var config ConfigFile
	err = json.Unmarshal(data, &config)
	if err == nil && config.Proxies != nil {
		return config.Proxies, nil
	}

	// Format 3: Objekt mit beliebigen Feldern - versuche alle Werte zu extrahieren
	var configMap map[string]interface{}
	err = json.Unmarshal(data, &configMap)
	if err == nil {
		// Suche nach Array-Feldern die ProxyConfig enthalten könnten
		for _, value := range configMap {
			if arr, ok := value.([]interface{}); ok {
				// Versuche das Array als ProxyConfig zu parsen
				arrBytes, _ := json.Marshal(arr)
				var tempProxies []ProxyConfig
				if json.Unmarshal(arrBytes, &tempProxies) == nil && len(tempProxies) > 0 {
					return tempProxies, nil
				}
			}
		}
	}

	return []ProxyConfig{}, fmt.Errorf("unbekanntes JSON-Format in Datei %s", filename)
}

func downloadProxiesFromAPI(apiURL, preferredProtocol string) ([]ProxyConfig, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("HTTP-Request fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP-Status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Antwort lesen fehlgeschlagen: %w", err)
	}

	// Verschiedene Antwortformate versuchen
	proxies, err := parseProxyResponse(body, preferredProtocol)
	if err != nil {
		// Fallback: Als einfache Textliste versuchen
		return parseProxyTextList(string(body), preferredProtocol)
	}

	return proxies, nil
}

func parseProxyResponse(data []byte, preferredProtocol string) ([]ProxyConfig, error) {
	// Versuche zuerst das Standard ProxyScrape Format
	var response ProxyScrapeResponse
	err := json.Unmarshal(data, &response)
	if err == nil && len(response.Proxies) > 0 {
		return convertProxyScrapeProxies(response.Proxies, preferredProtocol), nil
	}

	// Versuche alternatives Format
	var altResponse AlternativeProxyScrapeResponse
	err = json.Unmarshal(data, &altResponse)
	if err == nil && len(altResponse.Data) > 0 {
		return convertAlternativeProxies(altResponse.Data, preferredProtocol), nil
	}

	// Versuche als Array
	var proxyArray []ProxyScrapeProxy
	err = json.Unmarshal(data, &proxyArray)
	if err == nil && len(proxyArray) > 0 {
		return convertProxyScrapeProxies(proxyArray, preferredProtocol), nil
	}

	return nil, fmt.Errorf("JSON-Parsing fehlgeschlagen: %w", err)
}

func parseProxyTextList(text, preferredProtocol string) ([]ProxyConfig, error) {
	var proxies []ProxyConfig
	scanner := bufio.NewScanner(strings.NewReader(text))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Verschiedene Formate versuchen
		if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				host := parts[0]
				port := parts[1]

				// Port validieren
				if portNum, err := strconv.Atoi(port); err == nil && portNum > 0 && portNum <= 65535 {
					protocol := preferredProtocol
					if protocol == "" {
						protocol = "socks5" // Standard
					}

					proxy := ProxyConfig{
						Host:     host,
						Port:     port,
						Protocol: protocol,
						Type:     "rotating",
					}

					proxies = append(proxies, proxy)
				}
			}
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("keine gültigen Proxys in Textformat gefunden")
	}

	return proxies, nil
}

func convertProxyScrapeProxies(scraped []ProxyScrapeProxy, preferredProtocol string) []ProxyConfig {
	var proxies []ProxyConfig

	for _, p := range scraped {
		protocol := strings.ToLower(p.Protocol)

		// Protokoll normalisieren
		switch protocol {
		case "socks4", "socks5", "http", "https":
			// Gültige Protokolle
		default:
			if preferredProtocol != "" {
				protocol = preferredProtocol
			} else {
				protocol = "socks5" // Standard
			}
		}

		proxy := ProxyConfig{
			Host:     p.IP,
			Port:     strconv.Itoa(p.Port),
			Protocol: protocol,
			Type:     "rotating",
		}

		proxies = append(proxies, proxy)
	}

	return proxies
}

func convertAlternativeProxies(data []AlternativeProxyData, preferredProtocol string) []ProxyConfig {
	var proxies []ProxyConfig

	for _, p := range data {
		protocol := strings.ToLower(p.Protocol)

		// Protokoll validieren
		switch protocol {
		case "socks4", "socks5", "http", "https":
			// OK
		default:
			if preferredProtocol != "" {
				protocol = preferredProtocol
			} else {
				protocol = "socks5"
			}
		}

		proxy := ProxyConfig{
			Host:     p.IP,
			Port:     p.Port,
			Protocol: protocol,
			Type:     "rotating",
		}

		proxies = append(proxies, proxy)
	}

	return proxies
}

func tryAlternativeSources() []ProxyConfig {
	sources := []string{
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks5.txt",
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/socks4.txt",
		"https://raw.githubusercontent.com/TheSpeedX/PROXY-List/master/http.txt",
	}

	var allProxies []ProxyConfig

	for i, url := range sources {
		fmt.Printf("Versuche Quelle %d/%d...\n", i+1, len(sources))

		protocol := "socks5"
		if strings.Contains(url, "socks4") {
			protocol = "socks4"
		} else if strings.Contains(url, "http") {
			protocol = "http"
		}

		proxies, err := downloadSimpleProxyList(url, protocol)
		if err != nil {
			fmt.Printf("Quelle %d fehlgeschlagen: %v\n", i+1, err)
			continue
		}

		fmt.Printf("Quelle %d: %d Proxys geladen\n", i+1, len(proxies))
		allProxies = append(allProxies, proxies...)
	}

	return allProxies
}

func downloadSimpleProxyList(url, protocol string) ([]ProxyConfig, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseProxyTextList(string(body), protocol)
}
