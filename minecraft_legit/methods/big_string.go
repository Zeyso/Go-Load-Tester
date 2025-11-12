package methods

import (
	"crypto/rand"
	"net"
)

type BigStringMethod struct {
	stringLength int
	randomString string
}

func NewBigStringMethod(length int) *BigStringMethod {
	if length <= 0 {
		length = 25555
	}

	method := &BigStringMethod{
		stringLength: length,
	}
	method.generateRandomString()
	return method
}

func (b *BigStringMethod) generateRandomString() {
	bytes := make([]byte, b.stringLength)
	for i := 0; i < b.stringLength; i++ {
		randByte := make([]byte, 1)
		rand.Read(randByte)
		bytes[i] = byte((int(randByte[0]) % 125) + 1)
	}
	b.randomString = string(bytes)
}

func (b *BigStringMethod) Execute(conn net.Conn, host string, port uint16, protocolVersion int32) error {
	// UTF-8 String schreiben (2 Bytes Länge + String)
	length := len(b.randomString)
	packet := make([]byte, 2+length)

	// String-Länge als Big-Endian Short
	packet[0] = byte(length >> 8)
	packet[1] = byte(length)

	// String-Daten
	copy(packet[2:], b.randomString)

	_, err := conn.Write(packet)
	if err != nil {
		return err
	}

	return nil
}
