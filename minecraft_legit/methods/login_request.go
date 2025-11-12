package methods

import (
	"net"
	"sync/atomic"
)

type LoginRequestMethod struct {
	counter int32
}

func NewLoginRequestMethod() *LoginRequestMethod {
	return &LoginRequestMethod{
		counter: 0,
	}
}

func (l *LoginRequestMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Handshake-Paket für Login (State 2)
	handshake := createHandshakePacket(host, port, 2, protocolVersion)
	if err := sendPacket(conn, handshake); err != nil {
		return err
	}

	// Login Request mit inkrementierendem Zähler
	count := atomic.AddInt32(&l.counter, 1)
	username := generateUsernameWithCounter(count)
	loginStart := createLoginStartPacket(username)

	return sendPacket(conn, loginStart)
}

func generateUsernameWithCounter(count int32) string {
	return "User" + string(rune(count))
}
