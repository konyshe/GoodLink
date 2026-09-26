package stun2

import (
	"encoding/binary"
	"testing"
)

func TestXorMappedAddressPortAndPadding(t *testing.T) {
	magic := []byte{0x21, 0x12, 0xa4, 0x42}
	txid := make([]byte, 12)

	// SOFTWARE (0x8022) length 5 -> padded to 8, then XOR-MAPPED-ADDRESS
	var attrs []byte
	attrs = append(attrs, 0x80, 0x22, 0x00, 0x05)
	attrs = append(attrs, 'h', 'e', 'l', 'l', 'o', 0, 0, 0)

	attrs = append(attrs, 0x00, 0x20, 0x00, 0x08) // XOR-MAPPED IPv4
	attrs = append(attrs, 0x00, 0x01)               // reserved + family
	xport := uint16(3478) ^ 0x2112
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, xport)
	attrs = append(attrs, portBytes...)
	ip := []byte{1, 2, 3, 4}
	for i := 0; i < 4; i++ {
		attrs = append(attrs, ip[i]^magic[i])
	}

	gotIP, gotPort, err := getStunIpPort4(attrs, len(attrs), 0x0020, magic, txid)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if gotIP != "1.2.3.4" {
		t.Fatalf("ip=%s want 1.2.3.4", gotIP)
	}
	if gotPort != 3478 {
		t.Fatalf("port=%d want 3478", gotPort)
	}
}
