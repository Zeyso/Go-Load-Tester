package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func parseProxyURL(proxyURL string) (ProxyConfig, error) {
	if proxyURL == "" {
		return ProxyConfig{}, fmt.Errorf("leere proxy-url")
	}

	u, err := url.Parse(proxyURL)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültige proxy-url: %w", err)
	}

	supportedSchemes := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !supportedSchemes[u.Scheme] {
		return ProxyConfig{}, fmt.Errorf("nicht unterstütztes protokoll: %s", u.Scheme)
	}

	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return ProxyConfig{}, fmt.Errorf("ungültiges host:port format: %w", err)
	}

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}

	return ProxyConfig{
		Protocol: u.Scheme,
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Type:     "rotating",
	}, nil
}

func parseProxyList(proxyText string) ([]ProxyConfig, error) {
	lines := strings.Split(proxyText, "\n")
	var proxies []ProxyConfig

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "://") {
			proxy, err := parseProxyURL(line)
			if err != nil {
				continue
			}
			proxies = append(proxies, proxy)
		} else {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				proxies = append(proxies, ProxyConfig{
					Protocol: "socks5",
					Host:     parts[0],
					Port:     parts[1],
					Type:     "rotating",
				})
			}
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("keine gültigen proxys gefunden")
	}

	return proxies, nil
}

func loadProxiesFromFile(filename string) ([]ProxyConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("fehler beim lesen der datei: %w", err)
	}

	if strings.HasSuffix(filename, ".json") {
		var config struct {
			Proxies []ProxyConfig `json:"proxies"`
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("fehler beim parsen der json-datei: %w", err)
		}
		return config.Proxies, nil
	}

	return parseProxyList(string(data))
}

func saveProxiesToFile(proxies []ProxyConfig, filename string) error {
	config := struct {
		Proxies []ProxyConfig `json:"proxies"`
	}{
		Proxies: proxies,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("fehler beim erstellen der json-daten: %w", err)
	}

	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("fehler beim speichern der datei: %w", err)
	}

	return nil
}

func validateProxy(proxy ProxyConfig) error {
	if proxy.Host == "" {
		return fmt.Errorf("host ist erforderlich")
	}

	portNum, err := strconv.Atoi(proxy.Port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("ungültiger port: %s", proxy.Port)
	}

	validProtocols := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !validProtocols[proxy.Protocol] {
		return fmt.Errorf("ungültiges protokoll: %s", proxy.Protocol)
	}

	return nil
}
