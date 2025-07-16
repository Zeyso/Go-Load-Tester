package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type ProxyConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}

type Config struct {
	Proxies []ProxyConfig `json:"proxies"`
}

type LoadTester struct {
	proxies         []ProxyConfig
	targetServer    string
	targetHost      string
	targetPort      uint16
	concurrency     int
	requestsPerSec  int
	duration        time.Duration
	useRotating     bool
	currentProxyIdx int64

	// Statistiken
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	avgResponseTime int64

	// Error Tracking
	errorStats map[string]int64
	errorMutex sync.RWMutex
}

type ProxyError struct {
	ProxyAddr string
	Protocol  string
	Message   string
	Err       error
}

type ProxyTestResult struct {
	Proxy   ProxyConfig
	Success bool
	Error   error
}

func (pe *ProxyError) Error() string {
	return fmt.Sprintf("Proxy %s (%s): %s - %v", pe.ProxyAddr, pe.Protocol, pe.Message, pe.Err)
}

func NewLoadTester(targetServer string, concurrency, requestsPerSec int, duration time.Duration) (*LoadTester, error) {
	host, portStr, err := net.SplitHostPort(targetServer)
	if err != nil {
		return nil, fmt.Errorf("ungültiges server-format '%s': %w", targetServer, err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("ungültiger port '%s': %w", portStr, err)
	}

	return &LoadTester{
		targetServer:   targetServer,
		targetHost:     host,
		targetPort:     uint16(port),
		concurrency:    concurrency,
		requestsPerSec: requestsPerSec,
		duration:       duration,
		errorStats:     make(map[string]int64),
	}, nil
}

func (lt *LoadTester) trackError(errorType string) {
	lt.errorMutex.Lock()
	defer lt.errorMutex.Unlock()
	lt.errorStats[errorType]++
}

func (lt *LoadTester) getErrorStats() map[string]int64 {
	lt.errorMutex.RLock()
	defer lt.errorMutex.RUnlock()
	stats := make(map[string]int64)
	for k, v := range lt.errorStats {
		stats[k] = v
	}
	return stats
}

func (lt *LoadTester) AddProxy(host, port, username, password, proxyType, protocol string) error {
	if host == "" {
		return fmt.Errorf("proxy host darf nicht leer sein")
	}

	if port == "" {
		return fmt.Errorf("proxy port darf nicht leer sein")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("ungültiger port %s: muss zwischen 1-65535 sein", port)
	}

	validProtocols := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !validProtocols[protocol] {
		return fmt.Errorf("ungültiges protokoll %s: unterstützt sind socks5, socks4, http, https", protocol)
	}

	lt.proxies = append(lt.proxies, ProxyConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Type:     proxyType,
		Protocol: protocol,
	})
	return nil
}

func (lt *LoadTester) getNextProxy() ProxyConfig {
	if len(lt.proxies) == 0 {
		return ProxyConfig{}
	}

	if lt.useRotating {
		idx := atomic.AddInt64(&lt.currentProxyIdx, 1)
		return lt.proxies[int(idx-1)%len(lt.proxies)]
	}

	// Statisch - verwende ersten Proxy
	return lt.proxies[0]
}

func parseProxyURL(proxyURL string) (ProxyConfig, error) {
	if proxyURL == "" {
		return ProxyConfig{}, fmt.Errorf("proxy URL darf nicht leer sein")
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültige URL '%s': %w", proxyURL, err)
	}

	supportedSchemes := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !supportedSchemes[u.Scheme] {
		return ProxyConfig{}, fmt.Errorf("nicht unterstütztes protokoll '%s' in URL '%s'. Verfügbar: socks5, socks4, http, https", u.Scheme, proxyURL)
	}

	if u.Host == "" {
		return ProxyConfig{}, fmt.Errorf("host fehlt in URL '%s'", proxyURL)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültige host:port kombination '%s' in URL '%s': %w", u.Host, proxyURL, err)
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return ProxyConfig{}, fmt.Errorf("ungültiger port '%s' in URL '%s': muss zwischen 1-65535 sein", port, proxyURL)
	}

	proxy := ProxyConfig{
		Host:     host,
		Port:     port,
		Type:     "static",
		Protocol: u.Scheme,
	}

	if u.User != nil {
		proxy.Username = u.User.Username()
		if password, hasPassword := u.User.Password(); hasPassword {
			proxy.Password = password
		}
	}

	return proxy, nil
}

func parseProxyList(input string) ([]ProxyConfig, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("proxy-liste ist leer")
	}

	var proxies []ProxyConfig
	lines := strings.Split(input, "\n")
	var errors []string

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		proxy, err := parseProxyURL(line)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Zeile %d: %v", i+1, err))
			continue
		}

		proxies = append(proxies, proxy)
	}

	if len(errors) > 0 {
		errorMsg := fmt.Sprintf("Fehler beim Parsen von %d Zeilen:\n%s", len(errors), strings.Join(errors, "\n"))
		if len(proxies) == 0 {
			return nil, fmt.Errorf(errorMsg)
		}
		fmt.Printf("Warnung: %s\n", errorMsg)
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("keine gültigen proxys gefunden")
	}

	return proxies, nil
}

func loadProxiesFromFile(filename string) ([]ProxyConfig, error) {
	if filename == "" {
		return nil, fmt.Errorf("dateiname darf nicht leer sein")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("datei '%s' existiert nicht", filename)
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("fehler beim lesen der datei '%s': %w", filename, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("datei '%s' ist leer", filename)
	}

	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		var config Config
		err = json.Unmarshal(data, &config)
		if err != nil {
			return nil, fmt.Errorf("fehler beim parsen der JSON-datei '%s': %w", filename, err)
		}
		return config.Proxies, nil
	} else {
		proxies, err := parseProxyList(content)
		if err != nil {
			return nil, fmt.Errorf("fehler beim parsen der proxy-liste '%s': %w", filename, err)
		}
		return proxies, nil
	}
}

func saveProxiesToFile(proxies []ProxyConfig, filename string) error {
	if filename == "" {
		return fmt.Errorf("dateiname darf nicht leer sein")
	}

	if len(proxies) == 0 {
		return fmt.Errorf("keine proxys zum speichern vorhanden")
	}

	config := Config{Proxies: proxies}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("fehler beim erstellen der JSON-daten: %w", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("fehler beim speichern der datei '%s': %w", filename, err)
	}

	return nil
}

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

	// Verfügbare Typen sammeln
	typeCount := make(map[string]int)
	for _, p := range proxies {
		if p.Type == "" {
			p.Type = "static"
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
		fmt.Printf("%d. Nur %s Proxys verwenden\n", i+2, strings.Title(pType))
	}

	choice := promptUser(fmt.Sprintf("Wähle Option (1-%d) [1]: ", len(types)+1))

	choiceNum, err := strconv.Atoi(choice)
	if err != nil || choiceNum < 1 || choiceNum > len(types)+1 {
		choiceNum = 1
	}

	if choiceNum == 1 {
		fmt.Println("Verwende alle Proxy-Typen")
		return proxies
	}

	selectedType := types[choiceNum-2]
	fmt.Printf("Verwende nur %s Proxys\n", strings.Title(selectedType))

	var filtered []ProxyConfig
	for _, p := range proxies {
		if p.Type == selectedType || (p.Type == "" && selectedType == "static") {
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
		fmt.Println("Verwende statischen Modus")
		return false
	default:
		fmt.Println("Verwende Rotating-Modus")
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
	fmt.Println("7. Ohne Proxys fortfahren")

	choice := promptUser("Wähle eine Option (1-7): ")

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
		fmt.Println("Teste ohne Proxys (direkte Verbindung)")
		return []ProxyConfig{}, false
	default:
		fmt.Println("Ungültige Auswahl, verwende direkte Verbindung")
		return []ProxyConfig{}, false
	}

	proxies = selectProxyTypes(proxies)
	useRotating := false

	if len(proxies) > 0 {
		useRotating = selectRotatingMode()
	}

	return proxies, useRotating
}

func testProxiesParallel(proxies []ProxyConfig, maxWorkers int) ([]ProxyConfig, []ProxyConfig, []string) {
	if len(proxies) == 0 {
		return []ProxyConfig{}, []ProxyConfig{}, []string{}
	}

	// Anzahl der Worker bestimmen
	workers := maxWorkers
	if len(proxies) < workers {
		workers = len(proxies)
	}

	// Channels für die Kommunikation
	proxyQueue := make(chan ProxyConfig, len(proxies))
	resultQueue := make(chan ProxyTestResult, len(proxies))
	var wg sync.WaitGroup

	// Worker starten
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for proxy := range proxyQueue {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				err := testSingleProxy(ctx, proxy)
				cancel()

				resultQueue <- ProxyTestResult{
					Proxy:   proxy,
					Success: err == nil,
					Error:   err,
				}
			}
		}()
	}

	// Proxys in die Queue einreihen
	for _, proxy := range proxies {
		proxyQueue <- proxy
	}
	close(proxyQueue)

	// Worker abwarten
	go func() {
		wg.Wait()
		close(resultQueue)
	}()

	// Ergebnisse sammeln
	var working []ProxyConfig
	var failed []ProxyConfig
	var errors []string
	var completed int

	for result := range resultQueue {
		completed++

		// Live-Progress anzeigen
		fmt.Printf("\r✅ Test %d/%d abgeschlossen...", completed, len(proxies))

		if result.Success {
			working = append(working, result.Proxy)
		} else {
			failed = append(failed, result.Proxy)
			errors = append(errors, fmt.Sprintf("%s:%s - %v",
				result.Proxy.Host, result.Proxy.Port, result.Error))
		}
	}

	fmt.Printf("\r%s\n", strings.Repeat(" ", 50)) // Clear progress line
	return working, failed, errors
}

func testAndManageProxies() []ProxyConfig {
	configFile := "proxy_config.json"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Keine gespeicherte Konfiguration gefunden: %s\n", configFile)
		return []ProxyConfig{}
	}

	proxies, err := loadProxiesFromFile(configFile)
	if err != nil {
		fmt.Printf("Fehler beim Laden der gespeicherten Proxys: %v\n", err)
		return []ProxyConfig{}
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys in der gespeicherten Konfiguration gefunden")
		return []ProxyConfig{}
	}

	fmt.Printf("%d gespeicherte Proxys gefunden\n", len(proxies))

	// Worker-Anzahl bestimmen
	workers := 20
	if len(proxies) < workers {
		workers = len(proxies)
	}

	fmt.Printf("Teste alle Proxys mit %d parallelen Verbindungen...\n", workers)

	working, failed, errors := testProxiesParallel(proxies, workers)

	fmt.Printf("\nTESTERGEBNISSE\n")
	fmt.Println(strings.Repeat("-", 20))
	fmt.Printf("Funktionierende Proxys: %d\n", len(working))
	fmt.Printf("Fehlerhafte Proxys: %d\n", len(failed))

	if len(proxies) > 0 {
		fmt.Printf("Erfolgsrate: %.1f%%\n",
			float64(len(working))/float64(len(proxies))*100)
	}

	if len(failed) > 0 {
		fmt.Println("\nFEHLERHAFTE PROXYS:")
		for _, err := range errors {
			fmt.Printf("  %s\n", err)
		}

		removeChoice := promptUser("\nFehlerhafte Proxys aus Konfiguration entfernen? (j/n) [j]: ")
		if removeChoice == "" || strings.ToLower(removeChoice)[0] == 'j' {
			if len(working) > 0 {
				err := saveProxiesToFile(working, configFile)
				if err != nil {
					fmt.Printf("Fehler beim Speichern der bereinigten Liste: %v\n", err)
				} else {
					fmt.Printf("✅ %d fehlerhafte Proxys entfernt, %d funktionierende Proxys gespeichert\n",
						len(failed), len(working))
				}
				return working
			} else {
				fmt.Println("⚠️ Alle Proxys fehlerhaft - Konfiguration nicht geändert")
			}
		}
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

		var host string
		for {
			host = promptUser("Host/IP: ")
			if host != "" {
				break
			}
			fmt.Println("Fehler: Host darf nicht leer sein")
		}

		var port string
		for {
			port = promptUser("Port: ")
			if portNum, err := strconv.Atoi(port); err == nil && portNum > 0 && portNum <= 65535 {
				break
			}
			fmt.Println("Fehler: Port muss zwischen 1-65535 sein")
		}

		protocol := selectProxyProtocol()
		username := promptUser("Username (optional, Enter = leer): ")

		var password string
		if username != "" {
			password = promptUser("Password: ")
		}

		fmt.Println("\nProxy Typ:")
		fmt.Println("1. Static (fester Proxy)")
		fmt.Println("2. Rotating (rotierender Proxy)")
		typeChoice := promptUser("Wähle Typ (1-2) [1]: ")

		proxyType := "static"
		if typeChoice == "2" {
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

		fmt.Printf("\nProxy hinzugefügt:\n")
		fmt.Printf("   Adresse: %s://%s:%s\n", protocol, host, port)
		if username != "" {
			fmt.Printf("   Authentifizierung: %s:***\n", username)
		}
		fmt.Printf("   Typ: %s\n", proxyType)

		fmt.Println("\nWeiteren Proxy hinzufügen?")
		fmt.Println("1. Ja, weiteren Proxy eingeben")
		fmt.Println("2. Nein, mit aktueller Liste fortfahren")
		fmt.Println("3. Liste testen und speichern")

		continueChoice := promptUser("Wähle Option (1-3) [2]: ")

		switch continueChoice {
		case "1":
			continue
		case "3":
			return saveAndTestProxies(proxies)
		default:
			break
		}
		break
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys konfiguriert")
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
			fmt.Printf("Warnung: Speichern fehlgeschlagen: %v\n", err)
		} else {
			fmt.Printf("✅ Konfiguration gespeichert: %s\n", configFile)
		}
	}

	return proxies
}

func saveAndTestProxies(proxies []ProxyConfig) []ProxyConfig {
	fmt.Println("\nPROXY LISTE TESTEN UND SPEICHERN")
	fmt.Println(strings.Repeat("-", 40))

	if len(proxies) == 0 {
		fmt.Println("Fehler: Keine Proxys zum Speichern vorhanden")
		return []ProxyConfig{}
	}

	configFile := "proxy_config.json"
	err := saveProxiesToFile(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Speichern fehlgeschlagen: %v\n", err)
	} else {
		fmt.Printf("✅ Konfiguration gespeichert: %s\n", configFile)
	}

	testChoice := promptUser("Proxys testen? (j/n) [j]: ")
	if testChoice == "" || strings.ToLower(testChoice)[0] == 'j' {
		return testProxies(proxies)
	}

	return proxies
}

func testProxies(proxies []ProxyConfig) []ProxyConfig {
	if len(proxies) == 0 {
		fmt.Println("Fehler: Keine Proxys zum Testen vorhanden")
		return []ProxyConfig{}
	}

	fmt.Println("\nPROXY VERBINDUNGSTEST")
	fmt.Println(strings.Repeat("-", 35))

	// Worker-Anzahl bestimmen
	workers := 20
	if len(proxies) < workers {
		workers = len(proxies)
	}

	fmt.Printf("Teste %d Proxys mit %d parallelen Verbindungen...\n", len(proxies), workers)

	working, failed, errors := testProxiesParallel(proxies, workers)

	fmt.Printf("\nTESTERGEBNISSE\n")
	fmt.Println(strings.Repeat("-", 20))
	fmt.Printf("Funktionierende Proxys: %d\n", len(working))
	fmt.Printf("Fehlerhafte Proxys: %d\n", len(failed))

	if len(proxies) > 0 {
		fmt.Printf("Erfolgsrate: %.1f%%\n",
			float64(len(working))/float64(len(proxies))*100)
	}

	if len(failed) > 0 {
		fmt.Println("\nFEHLERHAFTE PROXYS:")
		for _, err := range errors {
			fmt.Printf("  %s\n", err)
		}

		removeChoice := promptUser("\nFehlerhafte Proxys entfernen? (j/n) [n]: ")
		if strings.ToLower(removeChoice)[0] == 'j' {
			return working
		}
	}

	return proxies
}

func testSingleProxy(ctx context.Context, proxy ProxyConfig) error {
	switch proxy.Protocol {
	case "socks5":
		return testSocks5Proxy(ctx, proxy)
	case "socks4":
		return testSocks4Proxy(ctx, proxy)
	case "http", "https":
		return testHTTPProxy(ctx, proxy)
	default:
		return fmt.Errorf("unbekanntes Protokoll: %s", proxy.Protocol)
	}
}

func testSocks5Proxy(ctx context.Context, proxyConf ProxyConfig) error {
	var auth *proxy.Auth
	if proxyConf.Username != "" {
		auth = &proxy.Auth{
			User:     proxyConf.Username,
			Password: proxyConf.Password,
		}
	}

	proxyAddr := net.JoinHostPort(proxyConf.Host, proxyConf.Port)

	conn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyAddr,
			Protocol:  "socks5",
			Message:   "Direkte Verbindung zum Proxy fehlgeschlagen",
			Err:       err,
		}
	}
	conn.Close()

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyAddr,
			Protocol:  "socks5",
			Message:   "SOCKS5-Dialer konnte nicht erstellt werden",
			Err:       err,
		}
	}

	conn, err = dialWithTimeout(ctx, dialer, "tcp", "8.8.8.8:53")
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyAddr,
			Protocol:  "socks5",
			Message:   "Verbindung über Proxy fehlgeschlagen",
			Err:       err,
		}
	}
	defer conn.Close()

	return nil
}

func testSocks4Proxy(ctx context.Context, proxy ProxyConfig) error {
	err := testSocks5Proxy(ctx, proxy)
	if err != nil {
		if pe, ok := err.(*ProxyError); ok {
			pe.Protocol = "socks4"
		}
	}
	return err
}

func testHTTPProxy(ctx context.Context, proxy ProxyConfig) error {
	proxyURL := &url.URL{
		Scheme: proxy.Protocol,
		Host:   net.JoinHostPort(proxy.Host, proxy.Port),
	}

	if proxy.Username != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	conn, err := net.DialTimeout("tcp", proxyURL.Host, 10*time.Second)
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyURL.Host,
			Protocol:  proxy.Protocol,
			Message:   "Direkte Verbindung zum Proxy fehlgeschlagen",
			Err:       err,
		}
	}
	conn.Close()

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyURL.Host,
			Protocol:  proxy.Protocol,
			Message:   "HTTP-Request über Proxy fehlgeschlagen",
			Err:       err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ProxyError{
			ProxyAddr: proxyURL.Host,
			Protocol:  proxy.Protocol,
			Message:   fmt.Sprintf("HTTP-Fehler: Status %d", resp.StatusCode),
			Err:       nil,
		}
	}

	return nil
}

func dialWithTimeout(ctx context.Context, dialer proxy.Dialer, network, address string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}

	ch := make(chan result, 1)

	go func() {
		defer close(ch)
		conn, err := dialer.Dial(network, address)
		select {
		case ch <- result{conn, err}:
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
		}
	}()

	select {
	case res := <-ch:
		return res.conn, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("verbindungs-timeout: %w", ctx.Err())
	}
}

func useSavedProxies() []ProxyConfig {
	configFile := "proxy_config.json"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Keine gespeicherte Konfiguration gefunden: %s\n", configFile)
		fmt.Println("Tipp: Verwende Option 1, 3 oder 5 um Proxys zu konfigurieren")
		return []ProxyConfig{}
	}

	proxies, err := loadProxiesFromFile(configFile)
	if err != nil {
		fmt.Printf("Fehler beim Laden der gespeicherten Proxys: %v\n", err)
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
	for _, p := range proxies {
		if p.Protocol == "" {
			p.Protocol = "socks5"
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
			proxies[i].Type = "static"
		}
	}

	fmt.Printf("%d Proxys erfolgreich geladen\n", len(proxies))

	// Automatisch in proxy_config.json speichern
	configFile := "proxy_config.json"
	err = saveProxiesToFile(proxies, configFile)
	if err != nil {
		fmt.Printf("Warnung: Konnte Proxys nicht in %s speichern: %v\n", configFile, err)
	} else {
		fmt.Printf("✅ Proxys automatisch in %s gespeichert\n", configFile)
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
		fmt.Printf("✅ Konfiguration gespeichert: %s\n", configFile)
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
			fmt.Println("Fehler: Format muss host:port oder host:port:username:password sein")
			continue
		}

		if portNum, err := strconv.Atoi(parts[1]); err != nil || portNum < 1 || portNum > 65535 {
			fmt.Printf("Fehler: Ungültiger Port '%s'\n", parts[1])
			continue
		}

		proxy := ProxyConfig{
			Host:     parts[0],
			Port:     parts[1],
			Type:     "rotating",
			Protocol: protocol,
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
		fmt.Printf("✅ %d Rotating Proxys gespeichert: %s\n", len(proxies), configFile)
	}

	return proxies
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
			return "", 0, 0, 0, fmt.Errorf("ungültige Worker-Anzahl '%s': %w", concurrencyStr, err)
		} else if c < 1 || c > 10000 {
			return "", 0, 0, 0, fmt.Errorf("worker-anzahl '%d' muss zwischen 1-10000 sein", c)
		} else {
			concurrency = c
		}
	}

	rpsStr := promptUser("Requests pro Sekunde [50]: ")
	rps := 50
	if rpsStr != "" {
		if r, err := strconv.Atoi(rpsStr); err != nil {
			return "", 0, 0, 0, fmt.Errorf("ungültige RPS '%s': %w", rpsStr, err)
		} else if r < 1 || r > 10000 {
			return "", 0, 0, 0, fmt.Errorf("rps '%d' muss zwischen 1-10000 sein", r)
		} else {
			rps = r
		}
	}

	durationStr := promptUser("Testdauer in Sekunden [30]: ")
	duration := 30 * time.Second
	if durationStr != "" {
		if d, err := strconv.Atoi(durationStr); err != nil {
			return "", 0, 0, 0, fmt.Errorf("ungültige Dauer '%s': %w", durationStr, err)
		} else if d < 1 || d > 3600 {
			return "", 0, 0, 0, fmt.Errorf("dauer '%d' muss zwischen 1-3600 sekunden sein", d)
		} else {
			duration = time.Duration(d) * time.Second
		}
	}

	return server, concurrency, rps, duration, nil
}

func (lt *LoadTester) Start() {
	fmt.Printf("Starte Lasttest gegen %s\n", lt.targetServer)
	fmt.Printf("Konfiguration: %d Worker, %d Req/s, Dauer: %v\n",
		lt.concurrency, lt.requestsPerSec, lt.duration)
	fmt.Printf("Verfügbare Proxys: %d\n", len(lt.proxies))

	if len(lt.proxies) > 0 {
		protocolCount := make(map[string]int)
		typeCount := make(map[string]int)
		for _, p := range lt.proxies {
			protocolCount[p.Protocol]++
			typeCount[p.Type]++
		}
		fmt.Print("Protokolle: ")
		for protocol, count := range protocolCount {
			fmt.Printf("%s(%d) ", strings.ToUpper(protocol), count)
		}
		fmt.Print(" | Typen: ")
		for pType, count := range typeCount {
			fmt.Printf("%s(%d) ", strings.Title(pType), count)
		}
		fmt.Printf("\nModus: ")
		if lt.useRotating {
			fmt.Println("Rotating (wechselnde Proxys)")
		} else {
			fmt.Println("Statisch (erster Proxy)")
		}
	}

	fmt.Println(strings.Repeat("=", 60))

	ctx, cancel := context.WithTimeout(context.Background(), lt.duration)
	defer cancel()

	rateLimiter := time.NewTicker(time.Second / time.Duration(lt.requestsPerSec))
	defer rateLimiter.Stop()

	var wg sync.WaitGroup
	requestChan := make(chan struct{}, lt.concurrency*2)

	for i := 0; i < lt.concurrency; i++ {
		wg.Add(1)
		go lt.worker(ctx, &wg, requestChan, i)
	}

	go func() {
		defer close(requestChan)
		for {
			select {
			case <-ctx.Done():
				return
			case <-rateLimiter.C:
				select {
				case requestChan <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()

	go lt.printLiveStats(ctx, statsTicker.C)

	wg.Wait()
	lt.printFinalStats()
}

func (lt *LoadTester) worker(ctx context.Context, wg *sync.WaitGroup, requestChan <-chan struct{}, workerID int) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-requestChan:
			if !ok {
				return
			}

			if len(lt.proxies) > 0 {
				proxyConfig := lt.getNextProxy()
				lt.performRequest(ctx, proxyConfig)
			} else {
				lt.performDirectRequest(ctx)
			}
		}
	}
}

func (lt *LoadTester) performRequest(ctx context.Context, proxyConfig ProxyConfig) {
	atomic.AddInt64(&lt.totalRequests, 1)

	start := time.Now()
	err := lt.connectViaProxy(ctx, proxyConfig)
	duration := time.Since(start)

	atomic.AddInt64(&lt.avgResponseTime, duration.Milliseconds())

	if err != nil {
		atomic.AddInt64(&lt.failedRequests, 1)
		lt.trackError(categorizeError(err))
	} else {
		atomic.AddInt64(&lt.successRequests, 1)
	}
}

func categorizeError(err error) string {
	if err == nil {
		return "unknown"
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "timeout"):
		return "timeout"
	case strings.Contains(errStr, "connection refused"):
		return "connection_refused"
	case strings.Contains(errStr, "no route to host"):
		return "no_route"
	case strings.Contains(errStr, "network unreachable"):
		return "network_unreachable"
	case strings.Contains(errStr, "authentication"):
		return "auth_failed"
	case strings.Contains(errStr, "proxy"):
		return "proxy_error"
	case strings.Contains(errStr, "dns"):
		return "dns_error"
	default:
		return "other"
	}
}

func (lt *LoadTester) performDirectRequest(ctx context.Context) {
	atomic.AddInt64(&lt.totalRequests, 1)

	start := time.Now()
	err := lt.connectDirect(ctx)
	duration := time.Since(start)

	atomic.AddInt64(&lt.avgResponseTime, duration.Milliseconds())

	if err != nil {
		atomic.AddInt64(&lt.failedRequests, 1)
		lt.trackError(categorizeError(err))
	} else {
		atomic.AddInt64(&lt.successRequests, 1)
	}
}

func (lt *LoadTester) connectViaProxy(ctx context.Context, proxyConfig ProxyConfig) error {
	switch proxyConfig.Protocol {
	case "socks5":
		return lt.connectViaSocks5(ctx, proxyConfig)
	case "socks4":
		return lt.connectViaSocks4(ctx, proxyConfig)
	case "http", "https":
		return lt.connectViaHTTP(ctx, proxyConfig)
	default:
		return fmt.Errorf("nicht unterstütztes Protokoll: %s", proxyConfig.Protocol)
	}
}

func (lt *LoadTester) connectViaSocks5(ctx context.Context, proxyConfig ProxyConfig) error {
	var auth *proxy.Auth
	if proxyConfig.Username != "" {
		auth = &proxy.Auth{
			User:     proxyConfig.Username,
			Password: proxyConfig.Password,
		}
	}

	proxyAddr := net.JoinHostPort(proxyConfig.Host, proxyConfig.Port)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyAddr,
			Protocol:  "socks5",
			Message:   "SOCKS5-Dialer konnte nicht erstellt werden",
			Err:       err,
		}
	}

	conn, err := lt.dialWithContext(ctx, dialer, "tcp", lt.targetServer)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Einfacher TCP-Ping (wie in deinem Beispiel)
	return nil
}

func (lt *LoadTester) connectViaSocks4(ctx context.Context, proxyConfig ProxyConfig) error {
	var auth *proxy.Auth
	if proxyConfig.Username != "" {
		auth = &proxy.Auth{
			User:     proxyConfig.Username,
			Password: proxyConfig.Password,
		}
	}

	proxyAddr := net.JoinHostPort(proxyConfig.Host, proxyConfig.Port)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyAddr,
			Protocol:  "socks4",
			Message:   "SOCKS4-Dialer konnte nicht erstellt werden",
			Err:       err,
		}
	}

	conn, err := lt.dialWithContext(ctx, dialer, "tcp", lt.targetServer)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Einfacher TCP-Ping
	return nil
}

func (lt *LoadTester) connectViaHTTP(ctx context.Context, proxyConfig ProxyConfig) error {
	proxyURL := &url.URL{
		Scheme: proxyConfig.Protocol,
		Host:   net.JoinHostPort(proxyConfig.Host, proxyConfig.Port),
	}

	if proxyConfig.Username != "" {
		proxyURL.User = url.UserPassword(proxyConfig.Username, proxyConfig.Password)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 5 * time.Second,
		}).DialContext,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		return &ProxyError{
			ProxyAddr: proxyURL.Host,
			Protocol:  proxyConfig.Protocol,
			Message:   "HTTP-Request über Proxy fehlgeschlagen",
			Err:       err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ProxyError{
			ProxyAddr: proxyURL.Host,
			Protocol:  proxyConfig.Protocol,
			Message:   fmt.Sprintf("HTTP-Fehler: Status %d", resp.StatusCode),
			Err:       nil,
		}
	}

	return nil
}

func (lt *LoadTester) connectDirect(ctx context.Context) error {
	var d net.Dialer
	d.Timeout = 5 * time.Second

	conn, err := d.DialContext(ctx, "tcp", lt.targetServer)
	if err != nil {
		return fmt.Errorf("direkte Verbindung zu %s fehlgeschlagen: %w", lt.targetServer, err)
	}
	defer conn.Close()

	// Einfacher TCP-Ping
	return nil
}

func (lt *LoadTester) dialWithContext(ctx context.Context, dialer proxy.Dialer, network, address string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}

	ch := make(chan result, 1)

	go func() {
		defer close(ch)
		conn, err := dialer.Dial(network, address)
		select {
		case ch <- result{conn, err}:
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
		}
	}()

	select {
	case res := <-ch:
		return res.conn, res.err
	case <-ctx.Done():
		return nil, fmt.Errorf("verbindungs-timeout zu %s: %w", address, ctx.Err())
	}
}

func (lt *LoadTester) printLiveStats(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			total := atomic.LoadInt64(&lt.totalRequests)
			success := atomic.LoadInt64(&lt.successRequests)
			failed := atomic.LoadInt64(&lt.failedRequests)

			if total > 0 {
				successRate := float64(success) / float64(total) * 100
				fmt.Printf("\rRequests: %d | Erfolg: %d (%.1f%%) | Fehler: %d",
					total, success, successRate, failed)
			}
		}
	}
}

func (lt *LoadTester) printFinalStats() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("FINALE STATISTIKEN")
	fmt.Println(strings.Repeat("=", 60))

	total := atomic.LoadInt64(&lt.totalRequests)
	success := atomic.LoadInt64(&lt.successRequests)
	failed := atomic.LoadInt64(&lt.failedRequests)
	avgRT := atomic.LoadInt64(&lt.avgResponseTime)

	if total > 0 {
		successRate := float64(success) / float64(total) * 100
		failRate := float64(failed) / float64(total) * 100
		avgResponseTime := float64(avgRT) / float64(total)
		requestsPerSec := float64(total) / lt.duration.Seconds()

		fmt.Printf("Zielserver: %s\n", lt.targetServer)
		fmt.Printf("Testdauer: %v\n", lt.duration)
		fmt.Printf("Gesamt Requests: %d\n", total)
		fmt.Printf("Erfolgreiche Requests: %d (%.2f%%)\n", success, successRate)
		fmt.Printf("Fehlgeschlagene Requests: %d (%.2f%%)\n", failed, failRate)
		fmt.Printf("Requests/Sekunde: %.2f\n", requestsPerSec)
		fmt.Printf("Durchschnittliche Antwortzeit: %.0fms\n", avgResponseTime)

		errorStats := lt.getErrorStats()
		if len(errorStats) > 0 {
			fmt.Println("\nFEHLER-VERTEILUNG:")
			for errorType, count := range errorStats {
				percentage := float64(count) / float64(failed) * 100
				fmt.Printf("  %s: %d (%.1f%%)\n", errorType, count, percentage)
			}
		}
	} else {
		fmt.Println("Keine Requests durchgeführt")
	}
}

func main() {
	fmt.Println("SERVER LASTTEST")
	fmt.Println(strings.Repeat("=", 60))

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
			err := tester.AddProxy(p.Host, p.Port, p.Username, p.Password, p.Type, p.Protocol)
			if err != nil {
				addErrors = append(addErrors, fmt.Sprintf("Proxy %s:%s - %v", p.Host, p.Port, err))
			}
		}

		if len(addErrors) > 0 {
			fmt.Printf("Warnung: %d Proxys konnten nicht hinzugefügt werden:\n", len(addErrors))
			for _, errMsg := range addErrors {
				fmt.Printf("  %s\n", errMsg)
			}
		}

		if len(tester.proxies) == 0 {
			fmt.Println("Fehler: Keine gültigen Proxys verfügbar")
			fmt.Println("Fahre mit direkter Verbindung fort...")
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	tester.Start()
}
