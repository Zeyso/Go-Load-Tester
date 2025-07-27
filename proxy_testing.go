package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

func testProxiesParallel(proxies []ProxyConfig, maxWorkers int) ([]ProxyConfig, []ProxyConfig, []string) {
	if len(proxies) == 0 {
		return []ProxyConfig{}, []ProxyConfig{}, []string{}
	}

	workers := maxWorkers
	if len(proxies) < workers {
		workers = len(proxies)
	}

	proxyQueue := make(chan ProxyConfig, len(proxies))
	resultQueue := make(chan ProxyTestResult, len(proxies))
	var wg sync.WaitGroup

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

	for _, proxy := range proxies {
		proxyQueue <- proxy
	}
	close(proxyQueue)

	go func() {
		wg.Wait()
		close(resultQueue)
	}()

	var working []ProxyConfig
	var failed []ProxyConfig
	var errors []string
	var completed int

	for result := range resultQueue {
		completed++
		progress := float64(completed) / float64(len(proxies)) * 100
		fmt.Printf("\rProgress: %.1f%% (%d/%d)", progress, completed, len(proxies))

		if result.Success {
			working = append(working, result.Proxy)
		} else {
			failed = append(failed, result.Proxy)
			errorMsg := fmt.Sprintf("%s:%s - %v", result.Proxy.Host, result.Proxy.Port, result.Error)
			errors = append(errors, errorMsg)
		}
	}

	fmt.Printf("\r%s\n", strings.Repeat(" ", 50))
	return working, failed, errors
}

func testAndManageProxies() []ProxyConfig {
	configFile := "proxy_config.json"

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Keine gespeicherte Konfiguration gefunden (%s)\n", configFile)
		return []ProxyConfig{}
	}

	proxies, err := loadProxiesFromFile(configFile)
	if err != nil {
		fmt.Printf("Fehler beim Laden der Konfiguration: %v\n", err)
		return []ProxyConfig{}
	}

	if len(proxies) == 0 {
		fmt.Println("Keine Proxys in der Konfiguration gefunden")
		return []ProxyConfig{}
	}

	fmt.Printf("%d gespeicherte Proxys gefunden\n", len(proxies))

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
		successRate := float64(len(working)) / float64(len(proxies)) * 100
		fmt.Printf("Erfolgsrate: %.1f%%\n", successRate)
	}

	if len(failed) > 0 {
		fmt.Println("\nFehlerhafte Proxys:")
		for i, errorMsg := range errors {
			if i < 10 {
				fmt.Printf("  - %s\n", errorMsg)
			}
		}
		if len(errors) > 10 {
			fmt.Printf("  ... und %d weitere\n", len(errors)-10)
		}

		removeChoice := promptUser("\nFehlerhafte Proxys aus der Konfiguration entfernen? (j/n) [j]: ")
		if removeChoice == "" || strings.ToLower(removeChoice)[0] == 'j' {
			if len(working) > 0 {
				err := saveProxiesToFile(working, configFile)
				if err != nil {
					fmt.Printf("Fehler beim Speichern: %v\n", err)
				} else {
					fmt.Printf("Konfiguration aktualisiert. %d funktionierende Proxys gespeichert.\n", len(working))
				}
				return working
			} else {
				fmt.Println("Keine funktionierenden Proxys gefunden. Konfiguration nicht geändert.")
			}
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
		return fmt.Errorf("nicht unterstütztes Protokoll: %s", proxy.Protocol)
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
		return fmt.Errorf("proxy-verbindung fehlgeschlagen: %w", err)
	}
	conn.Close()

	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return fmt.Errorf("socks5-dialer-fehler: %w", err)
	}

	conn, err = dialWithTimeout(ctx, dialer, "tcp", "8.8.8.8:53")
	if err != nil {
		return fmt.Errorf("test-verbindung fehlgeschlagen: %w", err)
	}
	defer conn.Close()

	return nil
}

func testSocks4Proxy(ctx context.Context, proxy ProxyConfig) error {
	err := testSocks5Proxy(ctx, proxy)
	if err != nil {
		return fmt.Errorf("socks4-test fehlgeschlagen: %w", err)
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
		return fmt.Errorf("proxy-verbindung fehlgeschlagen: %w", err)
	}
	conn.Close()

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		return fmt.Errorf("http-test fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("http-fehler: status %d", resp.StatusCode)
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
