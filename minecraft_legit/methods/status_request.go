package methods

import (
	"bytes"
	"net"
	"time"
)

type StatusRequestMethod struct{}

func NewStatusRequestMethod() *StatusRequestMethod {
	return &StatusRequestMethod{}
}

func (s *StatusRequestMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Handshake-Paket für Status (State 1)
	handshake := createHandshakePacket(host, port, 1, protocolVersion)
	if err := sendPacket(conn, handshake); err != nil {
		return err
	}

	// Status Request
	statusRequest := []byte{0x01, 0x00}
	if err := sendPacket(conn, statusRequest); err != nil {
		return err
	}

	// Ping Packet
	pingPacket := createPingPacket(time.Now().UnixMilli())
	return sendPacket(conn, pingPacket)
}

func createPingPacket(timestamp int64) []byte {
	var buf bytes.Buffer

	// Packet ID (0x01)
	writeVarInt(&buf, 1)

	// Timestamp als Long (8 Bytes, Big Endian)
	for i := 7; i >= 0; i-- {
		buf.WriteByte(byte(timestamp >> (i * 8)))
	}

	// Prepend packet length
	packetData := buf.Bytes()
	var result bytes.Buffer
	writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}
