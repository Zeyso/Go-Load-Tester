package methods

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/binary"
	mathrand "math/rand"
	"net"
	"strconv"
)

func createHandshakePacket(host string, port uint16, nextState int32, protocolVersion int32) []byte {
	var buf bytes.Buffer

	// Packet ID (0x00)
	writeVarInt(&buf, 0)

	// Protocol Version
	writeVarInt(&buf, protocolVersion)

	// Server Address
	writeString(&buf, host)

	// Server Port
	binary.Write(&buf, binary.BigEndian, port)

	// Next State
	writeVarInt(&buf, nextState)

	// Prepend packet length
	packetData := buf.Bytes()
	var result bytes.Buffer
	writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}

func createLoginStartPacket(username string) []byte {
	var buf bytes.Buffer

	// Packet ID (0x00)
	writeVarInt(&buf, 0)

	// Username
	writeString(&buf, username)

	// Prepend packet length
	packetData := buf.Bytes()
	var result bytes.Buffer
	writeVarInt(&result, int32(len(packetData)))
	result.Write(packetData)

	return result.Bytes()
}

func createEncryptionResponsePacket(sharedSecret, verifyToken []byte) []byte {
	var buf bytes.Buffer

	// Packet ID (0x01)
	writeVarInt(&buf, 1)

	// Shared Secret
	writeByteArray(&buf, sharedSecret)

	// Verify Token
	writeByteArray(&buf, verifyToken)

	// Prepend packet length
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

func writeByteArray(buf *bytes.Buffer, data []byte) {
	writeVarInt(buf, int32(len(data)))
	buf.Write(data)
}

func sendPacket(conn net.Conn, packet []byte) error {
	_, err := conn.Write(packet)
	return err
}

func generateRandomUsername() string {
	usernames := []string{
		"Player", "User", "Gamer", "Test", "Random",
		"Guest", "Pro", "Elite", "Ninja", "Shadow",
	}
	base := usernames[mathrand.Intn(len(usernames))]
	return base + strconv.Itoa(mathrand.Intn(9999))
}

func generateRandomBytes(length int) []byte {
	bytes := make([]byte, length)
	cryptorand.Read(bytes)
	return bytes
}

func generateSecureRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := cryptorand.Read(bytes)
	return bytes, err
}
