package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func NewLoadTester(targetServer string, concurrency, requestsPerSec int, duration time.Duration) (*LoadTester, error) {
	host, portStr, err := net.SplitHostPort(targetServer)
	if err != nil {
		return nil, fmt.Errorf("ungültiges server-format: %w", err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return nil, fmt.Errorf("ungültiger port: %w", err)
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

func (lt *LoadTester) AddProxy(host, port, protocol, username, password string) error {
	if host == "" {
		return fmt.Errorf("host ist erforderlich")
	}

	if port == "" {
		return fmt.Errorf("port ist erforderlich")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("ungültiger port: %s", port)
	}

	validProtocols := map[string]bool{
		"socks5": true,
		"socks4": true,
		"http":   true,
		"https":  true,
	}

	if !validProtocols[protocol] {
		return fmt.Errorf("ungültiges protokoll: %s", protocol)
	}

	lt.proxies = append(lt.proxies, ProxyConfig{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Protocol: protocol,
		Type:     "rotating",
	})
	return nil
}
func (lt *LoadTester) getNextProxy() ProxyConfig {
	if len(lt.proxies) == 0 {
		return ProxyConfig{}
	}

	if lt.useRotating {
		idx := atomic.AddInt64(&lt.currentProxyIdx, 1) - 1
		return lt.proxies[idx%int64(len(lt.proxies))]
	}

	return lt.proxies[0]
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
			fmt.Println("Rotating Proxys")
		} else {
			fmt.Println("Statischer Proxy")
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
				default:
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
				fmt.Printf("\r[Live] Requests: %d | Success: %d (%.1f%%) | Failed: %d",
					total, success, successRate, failed)
			}
		}
	}
}

func (lt *LoadTester) printFinalStats() {
	total := atomic.LoadInt64(&lt.totalRequests)
	success := atomic.LoadInt64(&lt.successRequests)
	failed := atomic.LoadInt64(&lt.failedRequests)
	avgTime := atomic.LoadInt64(&lt.avgResponseTime)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("FINALE STATISTIKEN")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Gesamt Requests:        %d\n", total)
	fmt.Printf("Erfolgreiche Requests:  %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("Fehlgeschlagene:        %d (%.2f%%)\n", failed, float64(failed)/float64(total)*100)

	if total > 0 {
		fmt.Printf("Durchschn. Antwortzeit: %dms\n", avgTime/total)
	}

	errorStats := lt.getErrorStats()
	if len(errorStats) > 0 {
		fmt.Println("\nFEHLER-ÜBERSICHT:")
		for errType, count := range errorStats {
			percentage := float64(count) / float64(failed) * 100
			fmt.Printf("  %-30s: %d (%.1f%%)\n", errType, count, percentage)
		}
	}
	fmt.Println(strings.Repeat("=", 60))
}
