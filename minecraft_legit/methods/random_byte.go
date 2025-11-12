package methods

import (
	"crypto/rand"
	"net"
)

type RandomByteMethod struct{}

func NewRandomByteMethod() *RandomByteMethod {
	return &RandomByteMethod{}
}

func (r *RandomByteMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// Generiere zufällige Byte-Länge zwischen 5 und 65539
	var lengthBytes [2]byte
	rand.Read(lengthBytes[:])
	length := 5 + int(lengthBytes[0])<<8 + int(lengthBytes[1])

	if length > 65539 {
		length = 65539
	}

	// Generiere zufällige Bytes
	randomData := make([]byte, length)
	_, err := rand.Read(randomData)
	if err != nil {
		return err
	}

	// Sende zufällige Daten
	_, err = conn.Write(randomData)
	return err
}
