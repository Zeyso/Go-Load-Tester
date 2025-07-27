package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

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
			Protocol:  proxyConfig.Protocol,
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
			Protocol:  proxyConfig.Protocol,
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
		return fmt.Errorf("direkte Verbindung fehlgeschlagen: %w", err)
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
		return nil, fmt.Errorf("Verbindungs-Timeout: %w", ctx.Err())
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
