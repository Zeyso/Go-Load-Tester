package utils

import (
	"fmt"
	"runtime"
	"strings"
)

// ErrorLevel definiert die Schweregrade von Fehlern
type ErrorLevel int

const (
	// ErrorLevelInfo kennzeichnet Informationen
	ErrorLevelInfo ErrorLevel = iota
	// ErrorLevelWarning kennzeichnet Warnungen
	ErrorLevelWarning
	// ErrorLevelError kennzeichnet Fehler
	ErrorLevelError
	// ErrorLevelFatal kennzeichnet kritische Fehler
	ErrorLevelFatal
)

// FlooderError repräsentiert einen erweiterten Fehler mit Kontext
type FlooderError struct {
	Err       error
	Level     ErrorLevel
	Message   string
	Context   map[string]interface{}
	StackInfo string
}

// NewFlooderError erzeugt einen neuen FlooderError
func NewFlooderError(err error, level ErrorLevel, message string) *FlooderError {
	stackBuf := make([]byte, 4096)
	stackSize := runtime.Stack(stackBuf, false)

	return &FlooderError{
		Err:       err,
		Level:     level,
		Message:   message,
		Context:   make(map[string]interface{}),
		StackInfo: string(stackBuf[:stackSize]),
	}
}

// WithContext fügt dem Fehler Kontextinformationen hinzu
func (fe *FlooderError) WithContext(key string, value interface{}) *FlooderError {
	fe.Context[key] = value
	return fe
}

// Error gibt die Fehlermeldung zurück (implementiert error interface)
func (fe *FlooderError) Error() string {
	if fe.Err != nil {
		return fmt.Sprintf("%s: %v", fe.Message, fe.Err)
	}
	return fe.Message
}

// Unwrap gibt den Original-Fehler zurück (für errors.Is/As Kompatibilität)
func (fe *FlooderError) Unwrap() error {
	return fe.Err
}

// GetLevel gibt den Schweregrad des Fehlers zurück
func (fe *FlooderError) GetLevel() ErrorLevel {
	return fe.Level
}

// GetStack gibt die Stack-Trace-Informationen zurück
func (fe *FlooderError) GetStack() string {
	return fe.StackInfo
}

// GetContextAsString gibt alle Kontextinformationen als String zurück
func (fe *FlooderError) GetContextAsString() string {
	if len(fe.Context) == 0 {
		return ""
	}

	contextParts := make([]string, 0, len(fe.Context))
	for k, v := range fe.Context {
		contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
	}

	return strings.Join(contextParts, ", ")
}

// ProxyError ist ein spezieller Fehler für Proxy-Verbindungsprobleme
type ProxyError struct {
	ProxyAddr string
	Protocol  string
	Message   string
	Err       error
}

func (pe *ProxyError) Error() string {
	return pe.Message
}

// NewProxyError erzeugt einen neuen ProxyError
func NewProxyError(proxyAddr, protocol, message string, err error) *ProxyError {
	return &ProxyError{
		ProxyAddr: proxyAddr,
		Protocol:  protocol,
		Message:   message,
		Err:       err,
	}
}

// IsConnectionError prüft, ob ein Fehler ein Verbindungsfehler ist
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "connection") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "refused") ||
		strings.Contains(errMsg, "reset by peer")
}

// IsTimeoutError prüft, ob ein Fehler ein Timeout ist
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "deadline exceeded")
}

// ErrorLevelToString konvertiert ErrorLevel zu einem lesbaren String
func ErrorLevelToString(level ErrorLevel) string {
	switch level {
	case ErrorLevelInfo:
		return "INFO"
	case ErrorLevelWarning:
		return "WARN"
	case ErrorLevelError:
		return "ERROR"
	case ErrorLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}
