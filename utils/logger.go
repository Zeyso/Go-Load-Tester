package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel definiert die Logging-Level
type LogLevel int

const (
	// LogLevelDebug ist für detaillierte Debug-Informationen
	LogLevelDebug LogLevel = iota
	// LogLevelInfo ist für allgemeine Informationen
	LogLevelInfo
	// LogLevelWarning ist für Warnungen
	LogLevelWarning
	// LogLevelError ist für Fehler
	LogLevelError
	// LogLevelFatal ist für kritische Fehler
	LogLevelFatal
)

// Logger ist die Hauptschnittstelle für das Logging
type Logger struct {
	level      LogLevel
	writer     io.Writer
	fileWriter io.WriteCloser
	mu         sync.Mutex
	isColored  bool
}

var (
	// DefaultLogger ist der Standard-Logger
	DefaultLogger *Logger
	// Standardfarben für verschiedene Log-Level
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
)

func init() {
	// Initialisierung des Standard-Loggers
	DefaultLogger = NewLogger(LogLevelInfo)
}

// NewLogger erstellt einen neuen Logger mit dem angegebenen Level
func NewLogger(level LogLevel) *Logger {
	return &Logger{
		level:     level,
		writer:    os.Stdout,
		isColored: true,
	}
}

// SetLevel setzt das Log-Level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// EnableFileLogging aktiviert das Logging in eine Datei
func (l *Logger) EnableFileLogging(directory string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Stelle sicher, dass das Verzeichnis existiert
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}

	// Erstelle Dateinamen basierend auf Datum und Uhrzeit
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(directory, fmt.Sprintf("minecraft_flooder_%s.log", timestamp))

	// Öffne Logdatei
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	// Schließe die vorherige Datei, falls vorhanden
	if l.fileWriter != nil {
		l.fileWriter.Close()
	}

	l.fileWriter = file

	// Setze Writer auf MultiWriter für Konsole und Datei
	l.writer = io.MultiWriter(os.Stdout, file)
	return nil
}

// DisableFileLogging deaktiviert das Logging in eine Datei
func (l *Logger) DisableFileLogging() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fileWriter != nil {
		l.fileWriter.Close()
		l.fileWriter = nil
	}

	l.writer = os.Stdout
}

// SetColored aktiviert oder deaktiviert farbiges Logging
func (l *Logger) SetColored(colored bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.isColored = colored
}

// logWithLevel schreibt einen Log-Eintrag mit dem angegebenen Level
func (l *Logger) logWithLevel(level LogLevel, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Formatiere Nachricht
	formattedMsg := msg
	if len(args) > 0 {
		formattedMsg = fmt.Sprintf(msg, args...)
	}

	// Zeitstempel
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// Log-Level als String
	levelStr := logLevelToString(level)

	// Farbcode basierend auf Level
	colorCode := ""
	if l.isColored {
		colorCode = getColorForLevel(level)
	}

	// Schreibe formatierte Nachricht
	logLine := fmt.Sprintf("%s%s [%s] %s%s\n",
		colorCode, timestamp, levelStr, formattedMsg, getResetColor(l.isColored))

	fmt.Fprint(l.writer, logLine)
}

// Debug schreibt eine Debug-Nachricht
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.logWithLevel(LogLevelDebug, msg, args...)
}

// Info schreibt eine Info-Nachricht
func (l *Logger) Info(msg string, args ...interface{}) {
	l.logWithLevel(LogLevelInfo, msg, args...)
}

// Warning schreibt eine Warnungs-Nachricht
func (l *Logger) Warning(msg string, args ...interface{}) {
	l.logWithLevel(LogLevelWarning, msg, args...)
}

// Error schreibt eine Fehler-Nachricht
func (l *Logger) Error(msg string, args ...interface{}) {
	l.logWithLevel(LogLevelError, msg, args...)
}

// Fatal schreibt eine kritische Fehler-Nachricht und beendet das Programm
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.logWithLevel(LogLevelFatal, msg, args...)
	os.Exit(1)
}

// LogError protokolliert einen Fehler basierend auf seinem Typ
func (l *Logger) LogError(err error) {
	if err == nil {
		return
	}

	// Spezialbehandlung für FlooderError
	if fe, ok := err.(*FlooderError); ok {
		level := LogLevelError
		switch fe.Level {
		case ErrorLevelInfo:
			level = LogLevelInfo
		case ErrorLevelWarning:
			level = LogLevelWarning
		case ErrorLevelError:
			level = LogLevelError
		case ErrorLevelFatal:
			level = LogLevelFatal
		}

		contextStr := fe.GetContextAsString()
		if contextStr != "" {
			l.logWithLevel(level, "%s [Kontext: %s]", fe.Error(), contextStr)
		} else {
			l.logWithLevel(level, "%s", fe.Error())
		}

		// Bei Debug-Level auch Stack-Trace ausgeben
		if l.level <= LogLevelDebug {
			l.Debug("Stack Trace:\n%s", fe.GetStack())
		}
		return
	}

	// Spezialbehandlung für ProxyError
	if pe, ok := err.(*ProxyError); ok {
		l.Error("Proxy-Fehler: %s (Proxy: %s, Protokoll: %s)",
			pe.Message, pe.ProxyAddr, pe.Protocol)
		if pe.Err != nil {
			l.Debug("Ursprünglicher Fehler: %v", pe.Err)
		}
		return
	}

	// Standard-Fehlerbehandlung
	if IsConnectionError(err) {
		l.Warning("Verbindungsfehler: %v", err)
	} else if IsTimeoutError(err) {
		l.Warning("Timeout: %v", err)
	} else {
		l.Error("Fehler: %v", err)
	}
}

// Close schließt den Logger und alle offenen Ressourcen
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.fileWriter != nil {
		l.fileWriter.Close()
		l.fileWriter = nil
	}
}

// Hilfsfunktionen

// logLevelToString konvertiert LogLevel zu einem lesbaren String
func logLevelToString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarning:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// getColorForLevel gibt den Farbcode für das angegebene Level zurück
func getColorForLevel(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return colorBlue
	case LogLevelInfo:
		return colorGreen
	case LogLevelWarning:
		return colorYellow
	case LogLevelError:
		return colorRed
	case LogLevelFatal:
		return colorPurple
	default:
		return ""
	}
}

// getResetColor gibt den Reset-Farbcode zurück wenn farbiges Logging aktiv ist
func getResetColor(isColored bool) string {
	if isColored {
		return colorReset
	}
	return ""
}

// Global-Funktionen für einfachen Zugriff auf den DefaultLogger

// Debug schreibt eine Debug-Nachricht mit dem DefaultLogger
func Debug(msg string, args ...interface{}) {
	DefaultLogger.Debug(msg, args...)
}

// Info schreibt eine Info-Nachricht mit dem DefaultLogger
func Info(msg string, args ...interface{}) {
	DefaultLogger.Info(msg, args...)
}

// Warning schreibt eine Warnungs-Nachricht mit dem DefaultLogger
func Warning(msg string, args ...interface{}) {
	DefaultLogger.Warning(msg, args...)
}

// Error schreibt eine Fehler-Nachricht mit dem DefaultLogger
func Error(msg string, args ...interface{}) {
	DefaultLogger.Error(msg, args...)
}

// Fatal schreibt eine kritische Fehler-Nachricht mit dem DefaultLogger und beendet das Programm
func Fatal(msg string, args ...interface{}) {
	DefaultLogger.Fatal(msg, args...)
}

// LogError protokolliert einen Fehler mit dem DefaultLogger
func LogError(err error) {
	DefaultLogger.LogError(err)
}

// SetGlobalLogLevel setzt das Log-Level des DefaultLogger
func SetGlobalLogLevel(level LogLevel) {
	DefaultLogger.SetLevel(level)
}

// EnableGlobalFileLogging aktiviert das Logging in eine Datei für den DefaultLogger
func EnableGlobalFileLogging(directory string) error {
	return DefaultLogger.EnableFileLogging(directory)
}
