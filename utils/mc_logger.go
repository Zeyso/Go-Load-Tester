package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"
)

// FlooderInterface definiert die Schnittstelle für den MinecraftFlooder
type FlooderInterface interface {
	GetServers() []string
	GetProxies() []ProxyConfig
	GetConcurrency() int
	GetDuration() time.Duration
	GetFloodType() string
	GetUseRotating() bool
	GetErrorStats() map[string]int64
	Start()
}

// ProxyConfig repräsentiert die Konfiguration eines Proxys
type ProxyConfig struct {
	Protocol string
	Host     string
	Port     string
	Username string
	Password string
	Type     string
}

// InitLogging initialisiert das Logging-System
func InitLogging(flooder FlooderInterface, level LogLevel, enableFileLogging bool) error {
	// Setze globales Log-Level
	SetGlobalLogLevel(level)

	// Aktiviere Datei-Logging wenn gewünscht
	if enableFileLogging {
		// Erstelle logs-Verzeichnis im aktuellen Arbeitsverzeichnis
		currentDir, err := os.Getwd()
		if err != nil {
			return NewFlooderError(err, ErrorLevelError, "Konnte Arbeitsverzeichnis nicht ermitteln")
		}

		logsDir := filepath.Join(currentDir, "logs")
		if err := EnableGlobalFileLogging(logsDir); err != nil {
			return NewFlooderError(err, ErrorLevelError, "Konnte Datei-Logging nicht aktivieren")
		}

		Info("Datei-Logging aktiviert in: %s", logsDir)
	}

	return nil
}

// StartWithErrorHandling startet den Flooder mit erweiterter Fehlerbehandlung
func StartWithErrorHandling(flooder FlooderInterface) {
	// Setze Panic-Handler
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			Error("PANIC AUFGETRETEN: %v\nStack Trace:\n%s", r, stackTrace)

			// Erstelle Crash-Report
			createCrashReport(flooder, fmt.Sprintf("%v", r), stackTrace)

			// Programm mit Fehlercode beenden
			os.Exit(1)
		}
	}()

	Info("Starte Minecraft Server Flooder mit erweiterter Fehlerbehandlung")
	Info("Ziel-Server: %d Server", len(flooder.GetServers()))
	Info("Konfiguration: %d Worker, Dauer: %v, Typ: %s",
		flooder.GetConcurrency(), flooder.GetDuration(), flooder.GetFloodType())

	if len(flooder.GetProxies()) > 0 {
		Info("Verwende %d Proxys (Rotation: %t)", len(flooder.GetProxies()), flooder.GetUseRotating())
	} else {
		Warning("Keine Proxys konfiguriert, verwende direkte Verbindungen")
	}

	Debug("Floodtyp: %s, Concurrency: %d", flooder.GetFloodType(), flooder.GetConcurrency())

	// Rufe die Original-Start-Methode auf
	flooder.Start()
}

// createCrashReport erstellt einen detaillierten Crash-Report
func createCrashReport(flooder FlooderInterface, errorMsg, stackTrace string) {
	currentDir, err := os.Getwd()
	if err != nil {
		Error("Konnte Arbeitsverzeichnis für Crash-Report nicht ermitteln: %v", err)
		return
	}

	crashDir := filepath.Join(currentDir, "crash-reports")
	if err := os.MkdirAll(crashDir, 0755); err != nil {
		Error("Konnte Crash-Report-Verzeichnis nicht erstellen: %v", err)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(crashDir, fmt.Sprintf("crash_%s.txt", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		Error("Konnte Crash-Report-Datei nicht erstellen: %v", err)
		return
	}
	defer file.Close()

	// Schreibe Crash-Report-Header
	fmt.Fprintf(file, "=== MINECRAFT FLOODER CRASH REPORT ===\n")
	fmt.Fprintf(file, "Zeit: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700"))
	fmt.Fprintf(file, "Fehler: %s\n\n", errorMsg)

	// Schreibe Konfiguration
	fmt.Fprintf(file, "=== KONFIGURATION ===\n")
	fmt.Fprintf(file, "Flood-Typ: %s\n", flooder.GetFloodType())
	fmt.Fprintf(file, "Worker: %d\n", flooder.GetConcurrency())
	fmt.Fprintf(file, "Dauer: %v\n", flooder.GetDuration())
	fmt.Fprintf(file, "Server: %d\n", len(flooder.GetServers()))
	fmt.Fprintf(file, "Proxys: %d (Rotation: %t)\n\n", len(flooder.GetProxies()), flooder.GetUseRotating())

	// Schreibe Stack-Trace
	fmt.Fprintf(file, "=== STACK TRACE ===\n%s\n", stackTrace)

	Info("Crash-Report erstellt: %s", filename)
}

// PrintLiveStatsWithErrorHandling zeigt Live-Statistiken mit zusätzlichen Infos an
func PrintLiveStatsWithErrorHandling(ctx context.Context, ticker <-chan time.Time, flooder FlooderInterface,
	totalRequests *int64, successRequests *int64, failedRequests *int64) {
	lastRequests := int64(0)
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker:
			total := atomic.LoadInt64(totalRequests)
			success := atomic.LoadInt64(successRequests)
			failed := atomic.LoadInt64(failedRequests)

			// Berechne RPS
			currentRPS := total - lastRequests
			lastRequests = total

			// Berechne durchschnittliche RPS
			elapsedSeconds := time.Since(startTime).Seconds()
			avgRPS := float64(total) / elapsedSeconds

			// Berechne Erfolgsrate
			successRate := 0.0
			if total > 0 {
				successRate = float64(success) / float64(total) * 100
			}

			// Zeige Statistiken an
			Info("STATS: %d gesamt | %d erfolg (%.1f%%) | %d fehlg | %d/s aktuell | %.1f/s durchschn.",
				total, success, successRate, failed, currentRPS, avgRPS)

			// Bei Debug-Level auch Fehlerverteilung anzeigen
			if DefaultLogger.level <= LogLevelDebug {
				errorStats := flooder.GetErrorStats()
				if len(errorStats) > 0 {
					errorDetails := make([]string, 0, len(errorStats))
					for errType, count := range errorStats {
						errorDetails = append(errorDetails, fmt.Sprintf("%s:%d", errType, count))
					}
					Debug("Fehlerverteilung: %s", strings.Join(errorDetails, ", "))
				}
			}
		}
	}
}
