package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func parseProxyURL(proxyURL string) (ProxyConfig, error) {
	if proxyURL == "" {
		return ProxyConfig{}, fmt.Errorf("leere proxy-URL")
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültige URL: %w", err)
	}

	supportedSchemes := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !supportedSchemes[u.Scheme] {
		return ProxyConfig{}, fmt.Errorf("nicht unterstütztes Schema: %s", u.Scheme)
	}

	if u.Host == "" {
		return ProxyConfig{}, fmt.Errorf("fehlender host in URL")
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültiger host:port: %w", err)
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return ProxyConfig{}, fmt.Errorf("ungültiger port: %s", port)
	}

	proxy := ProxyConfig{
		Host:     host,
		Port:     port,
		Protocol: u.Scheme,
		Type:     "rotating",
	}

	if u.User != nil {
		proxy.Username = u.User.Username()
		proxy.Password, _ = u.User.Password()
	}

	return proxy, nil
}

func parseProxyList(input string) ([]ProxyConfig, error) {
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("keine proxy-daten eingegeben")
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
		return proxies, fmt.Errorf("fehler beim parsen von %d zeilen:\n%s", len(errors), strings.Join(errors, "\n"))
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("keine gültigen proxys gefunden")
	}

	return proxies, nil
}

func loadProxiesFromFile(filename string) ([]ProxyConfig, error) {
	if filename == "" {
		return nil, fmt.Errorf("kein dateiname angegeben")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("datei existiert nicht: %s", filename)
	}

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("fehler beim lesen der datei: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("datei ist leer")
	}

	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		var config Config
		err := json.Unmarshal(data, &config)
		if err != nil {
			return nil, fmt.Errorf("fehler beim parsen der JSON-datei: %w", err)
		}
		return config.Proxies, nil
	}

	return parseProxyList(content)
}

func saveProxiesToFile(proxies []ProxyConfig, filename string) error {
	if filename == "" {
		return fmt.Errorf("kein dateiname angegeben")
	}

	if len(proxies) == 0 {
		return fmt.Errorf("keine proxys zum speichern")
	}

	config := Config{Proxies: proxies}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("fehler beim erstellen der JSON-daten: %w", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("fehler beim schreiben der datei: %w", err)
	}

	return nil
}
