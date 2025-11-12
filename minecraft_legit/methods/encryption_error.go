package methods

import (
	"net"
)

type EncryptionErrorMethod struct{}

func NewEncryptionErrorMethod() *EncryptionErrorMethod {
	return &EncryptionErrorMethod{}
}

func (e *EncryptionErrorMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Handshake für Login (State 2)
	handshake := createHandshakePacket(host, port, 2, protocolVersion)
	if err := sendPacket(conn, handshake); err != nil {
		return err
	}

	// Login Start
	username := generateRandomUsername()
	loginStart := createLoginStartPacket(username)
	if err := sendPacket(conn, loginStart); err != nil {
		return err
	}

	// Warte kurz und schließe dann ohne Encryption Response
	// Dies simuliert einen Encryption-Fehler
	return nil
}
