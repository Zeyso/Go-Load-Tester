package methods

import (
	"net"
)

type RealMotdRequestMethod struct{}

func NewRealMotdRequestMethod() *RealMotdRequestMethod {
	return &RealMotdRequestMethod{}
}

func (r *RealMotdRequestMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Handshake-Paket für Status (State 1)
	handshake := createHandshakePacket(host, port, 1, protocolVersion)
	if err := sendPacket(conn, handshake); err != nil {
		return err
	}

	// Status Request
	statusRequest := []byte{0x01, 0x00}
	return sendPacket(conn, statusRequest)
}
