package methods

import (
	"bytes"
	"encoding/binary"
	"net"
)

type AuthenticationRequestMethod struct{}

func NewAuthenticationRequestMethod() *AuthenticationRequestMethod {
	return &AuthenticationRequestMethod{}
}

func (a *AuthenticationRequestMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Handshake-Paket für Login (State 2)
	handshake := createHandshakePacket(host, port, 2, protocolVersion)
	if err := sendPacket(conn, handshake); err != nil {
		return err
	}

	// Login Start mit zufälligem Benutzernamen
	username := generateRandomUsername()
	loginStart := createLoginStartPacket(username)
	return sendPacket(conn, loginStart)
}
