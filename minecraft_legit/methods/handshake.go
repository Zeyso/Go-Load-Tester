package methods

import (
	"net"
)

type HandshakeMethod struct {
	protocolVersion int32
}

func NewHandshakeMethod(protocolVersion int32) *HandshakeMethod {
	if protocolVersion <= 0 {
		protocolVersion = 763 // Default: 1.20.1
	}

	return &HandshakeMethod{
		protocolVersion: protocolVersion,
	}
}

func (h *HandshakeMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Verwende den übergebenen protocolVersion oder den gespeicherten
	if protocolVersion <= 0 {
		protocolVersion = h.protocolVersion
	}

	// Nur Handshake für Status (State 1)
	handshake := createHandshakePacket(host, port, 1, protocolVersion)

	_, err := conn.Write(handshake)
	return err
}
