package stun2

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"log"
	"net"
	"sync"
	"time"
)

const (
	// STUN Message Types
	MsgTypeBindingRequest  = 0x0001
	MsgTypeBindingResponse = 0x0101

	// STUN Attribute Types
	AttrMappedAddress    = 0x0001
	AttrChangedAddress   = 0x0005
	AttrXorMappedAddress = 0x0020
	AttrFingerprint      = 0x8028

	// Address Family
	FamilyIPv4 = 0x01
	FamilyIPv6 = 0x02

	// STUN Magic Cookie (RFC 5389)
	MagicCookie = 0x2112A442

	// Fingerprint XOR value
	FingerprintXor = 0x5354554e
)

var (
	magicCookieBytes = []byte{0x21, 0x12, 0xA4, 0x42}
)

type txnState struct {
	primaryAddr   *net.UDPAddr
	secondaryAddr *net.UDPAddr
	firstSeen     time.Time
}

// StunServer represents a STUN server instance
type StunServer struct {
	primaryAddr   string
	secondaryAddr string
	primaryConn   *net.UDPConn
	secondaryConn *net.UDPConn
	wg            sync.WaitGroup
	stopCh        chan struct{}

	mu            sync.Mutex
	txns          map[string]*txnState
	primaryPort   int
	secondaryPort int
}

// StartSvr starts the STUN server on the specified IP and port
// It listens on two ports: port (primary) and port+1 (secondary)
func StartSvr(ip string, port int) {
	server := &StunServer{
		primaryAddr:   net.JoinHostPort(ip, itoa(port)),
		secondaryAddr: net.JoinHostPort(ip, itoa(port+1)),
		stopCh:        make(chan struct{}),
		txns:          make(map[string]*txnState),
		primaryPort:   port,
		secondaryPort: port + 1,
	}

	var err error

	// Start primary UDP listener
	server.primaryConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		log.Fatalf("Failed to start primary UDP listener on %d: %v", port, err)
	}
	log.Printf("STUN server primary listener started on %d", port)

	// Start secondary UDP listener
	server.secondaryConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: port + 1})
	if err != nil {
		log.Fatalf("Failed to start secondary UDP listener on %d: %v", port+1, err)
	}
	log.Printf("STUN server secondary listener started on %d", port+1)

	// Start handlers
	server.wg.Add(2)
	go server.handleConnection(server.primaryConn, server.secondaryAddr)
	go server.handleConnection(server.secondaryConn, server.primaryAddr)

	// Wait for all handlers to finish
	server.wg.Wait()
}

// handleConnection handles incoming STUN requests on a UDP connection
func (s *StunServer) handleConnection(conn *net.UDPConn, changedAddr string) {
	defer s.wg.Done()
	defer conn.Close()

	buf := make([]byte, 1024)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Error reading from UDP: %v", err)
			continue
		}

		if n < 20 {
			log.Printf("Received packet too short: %d bytes", n)
			continue
		}

		go s.handleStunRequest(conn, remoteAddr, buf[:n], changedAddr)
	}
}

// handleStunRequest processes a STUN request and sends a response
func (s *StunServer) handleStunRequest(conn *net.UDPConn, remoteAddr *net.UDPAddr, data []byte, changedAddr string) {
	// Parse STUN message header
	// Bytes 0-1: Message Type
	// Bytes 2-3: Message Length
	// Bytes 4-7: Magic Cookie
	// Bytes 8-19: Transaction ID

	msgType := binary.BigEndian.Uint16(data[0:2])

	// Verify Magic Cookie (RFC 5389)
	if !bytes.Equal(data[4:8], magicCookieBytes) {
		log.Printf("Invalid magic cookie from %s", remoteAddr.String())
		return
	}

	transactionID := data[8:20]

	// Track requests by Transaction ID across primary/secondary ports within 3 seconds
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		log.Printf("Unexpected local address type: %T", conn.LocalAddr())
		return
	}
	localPort := localAddr.Port

	now := time.Now()

	s.mu.Lock()
	// Simple expiration of old entries
	for k, v := range s.txns {
		if now.Sub(v.firstSeen) > 3*time.Second {
			delete(s.txns, k)
		}
	}

	tidKey := string(transactionID)
	state, exists := s.txns[tidKey]
	if !exists || now.Sub(state.firstSeen) > 3*time.Second {
		state = &txnState{
			firstSeen: now,
		}
		s.txns[tidKey] = state
	}

	// Only correlate packets that arrive on the configured primary/secondary ports
	if localPort == s.primaryPort {
		state.primaryAddr = remoteAddr
	} else if localPort == s.secondaryPort {
		state.secondaryAddr = remoteAddr
	}

	if state.primaryAddr != nil && state.secondaryAddr != nil && now.Sub(state.firstSeen) <= 3*time.Second {
		natType := "NAT1-NAT3"
		if !addrEqual(state.primaryAddr, state.secondaryAddr) {
			natType = "NAT4"
		}

		log.Printf("primary(%d)->%s secondary(%d)->%s => %s",
			s.primaryPort, state.primaryAddr.String(),
			s.secondaryPort, state.secondaryAddr.String(),
			natType,
		)

		delete(s.txns, tidKey)
	}
	s.mu.Unlock()

	// Only handle Binding Request
	if msgType != MsgTypeBindingRequest {
		log.Printf("Unsupported message type 0x%04x from %s", msgType, remoteAddr.String())
		return
	}

	// Build response
	response := s.buildBindingResponse(remoteAddr, transactionID, changedAddr)

	// Send response
	_, err := conn.WriteToUDP(response, remoteAddr)
	if err != nil {
		log.Printf("Error sending response to %s: %v", remoteAddr.String(), err)
		return
	}
}

// buildBindingResponse constructs a STUN Binding Response message
func (s *StunServer) buildBindingResponse(remoteAddr *net.UDPAddr, transactionID []byte, changedAddr string) []byte {
	var buf bytes.Buffer

	// Build attributes first to calculate total length
	mappedAddr := buildMappedAddress(remoteAddr)
	xorMappedAddr := buildXorMappedAddress(remoteAddr, transactionID)
	changedAddrAttr := buildChangedAddress(changedAddr)

	// Calculate attributes length (without fingerprint)
	attrsLen := len(mappedAddr) + len(xorMappedAddr) + len(changedAddrAttr)

	// Write STUN header
	// Message Type: Binding Response (0x0101)
	buf.Write([]byte{0x01, 0x01})

	// Message Length (will be updated after adding fingerprint)
	// Fingerprint adds 8 bytes (4 type+length + 4 value)
	totalLen := attrsLen + 8
	lenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBytes, uint16(totalLen))
	buf.Write(lenBytes)

	// Magic Cookie
	buf.Write(magicCookieBytes)

	// Transaction ID
	buf.Write(transactionID)

	// Write attributes
	buf.Write(mappedAddr)
	buf.Write(xorMappedAddr)
	buf.Write(changedAddrAttr)

	// Calculate and add FINGERPRINT
	fingerprint := calcFingerprint(buf.Bytes())
	buf.Write(buildFingerprint(fingerprint))

	return buf.Bytes()
}

// buildMappedAddress creates a MAPPED-ADDRESS attribute
func buildMappedAddress(addr *net.UDPAddr) []byte {
	var buf bytes.Buffer

	// Attribute Type: MAPPED-ADDRESS (0x0001)
	buf.Write([]byte{0x00, 0x01})

	ip4 := addr.IP.To4()
	if ip4 != nil {
		// Attribute Length: 8 bytes for IPv4
		buf.Write([]byte{0x00, 0x08})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv4
		buf.WriteByte(FamilyIPv4)
		// Port
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(addr.Port))
		buf.Write(portBytes)
		// Address
		buf.Write(ip4)
	} else {
		// Attribute Length: 20 bytes for IPv6
		buf.Write([]byte{0x00, 0x14})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv6
		buf.WriteByte(FamilyIPv6)
		// Port
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(addr.Port))
		buf.Write(portBytes)
		// Address
		buf.Write(addr.IP.To16())
	}

	return buf.Bytes()
}

// buildXorMappedAddress creates an XOR-MAPPED-ADDRESS attribute
func buildXorMappedAddress(addr *net.UDPAddr, transactionID []byte) []byte {
	var buf bytes.Buffer

	// Attribute Type: XOR-MAPPED-ADDRESS (0x0020)
	buf.Write([]byte{0x00, 0x20})

	ip4 := addr.IP.To4()
	if ip4 != nil {
		// Attribute Length: 8 bytes for IPv4
		buf.Write([]byte{0x00, 0x08})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv4
		buf.WriteByte(FamilyIPv4)

		// X-Port: Port XOR'd with most significant 16 bits of magic cookie
		xPort := uint16(addr.Port) ^ 0x2112
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, xPort)
		buf.Write(portBytes)

		// X-Address: IP XOR'd with magic cookie
		xAddr := make([]byte, 4)
		for i := 0; i < 4; i++ {
			xAddr[i] = ip4[i] ^ magicCookieBytes[i]
		}
		buf.Write(xAddr)
	} else {
		// Attribute Length: 20 bytes for IPv6
		buf.Write([]byte{0x00, 0x14})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv6
		buf.WriteByte(FamilyIPv6)

		// X-Port: Port XOR'd with most significant 16 bits of magic cookie
		xPort := uint16(addr.Port) ^ 0x2112
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, xPort)
		buf.Write(portBytes)

		// X-Address: IP XOR'd with magic cookie + transaction ID
		ip16 := addr.IP.To16()
		xAddr := make([]byte, 16)
		for i := 0; i < 4; i++ {
			xAddr[i] = ip16[i] ^ magicCookieBytes[i]
		}
		for i := 4; i < 16; i++ {
			xAddr[i] = ip16[i] ^ transactionID[i-4]
		}
		buf.Write(xAddr)
	}

	return buf.Bytes()
}

// buildChangedAddress creates a CHANGED-ADDRESS attribute
func buildChangedAddress(addr string) []byte {
	var buf bytes.Buffer

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("Invalid changed address: %s", addr)
		return nil
	}

	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	ip := net.ParseIP(host)
	if ip == nil {
		log.Printf("Invalid IP in changed address: %s", host)
		return nil
	}

	// Attribute Type: CHANGED-ADDRESS (0x0005)
	buf.Write([]byte{0x00, 0x05})

	ip4 := ip.To4()
	if ip4 != nil {
		// Attribute Length: 8 bytes for IPv4
		buf.Write([]byte{0x00, 0x08})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv4
		buf.WriteByte(FamilyIPv4)
		// Port
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(port))
		buf.Write(portBytes)
		// Address
		buf.Write(ip4)
	} else {
		// Attribute Length: 20 bytes for IPv6
		buf.Write([]byte{0x00, 0x14})
		// Reserved byte
		buf.WriteByte(0x00)
		// Family: IPv6
		buf.WriteByte(FamilyIPv6)
		// Port
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(port))
		buf.Write(portBytes)
		// Address
		buf.Write(ip.To16())
	}

	return buf.Bytes()
}

// buildFingerprint creates a FINGERPRINT attribute
func buildFingerprint(crc uint32) []byte {
	var buf bytes.Buffer

	// Attribute Type: FINGERPRINT (0x8028)
	buf.Write([]byte{0x80, 0x28})
	// Attribute Length: 4 bytes
	buf.Write([]byte{0x00, 0x04})
	// CRC-32 value
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc)
	buf.Write(crcBytes)

	return buf.Bytes()
}

// calcFingerprint calculates the STUN fingerprint (CRC-32 XOR 0x5354554e)
func calcFingerprint(data []byte) uint32 {
	return crc32.ChecksumIEEE(data) ^ FingerprintXor
}

func addrEqual(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return false
	}
	return a.IP.Equal(b.IP) && a.Port == b.Port
}

// itoa converts an integer to a string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}

// parseAddressAttr parses MAPPED-ADDRESS or XOR-MAPPED-ADDRESS attribute value
func parseAddressAttr(data []byte, xor bool, transactionID []byte) (string, int, error) {
	if len(data) < 4 {
		return "", 0, nil
	}

	family := data[1]
	portBytes := data[2:4]
	var ip []byte

	switch family {
	case FamilyIPv4:
		if len(data) < 8 {
			return "", 0, nil
		}
		ip = make([]byte, 4)
		copy(ip, data[4:8])
	case FamilyIPv6:
		if len(data) < 20 {
			return "", 0, nil
		}
		ip = make([]byte, 16)
		copy(ip, data[4:20])
	default:
		return "", 0, nil
	}

	port := binary.BigEndian.Uint16(portBytes)

	if xor {
		// XOR port with magic cookie high 16 bits
		port ^= 0x2112
		// XOR IP with magic cookie
		for i := 0; i < 4 && i < len(ip); i++ {
			ip[i] ^= magicCookieBytes[i]
		}
		// For IPv6, also XOR with transaction ID
		if family == FamilyIPv6 && transactionID != nil {
			for i := 4; i < 16 && i-4 < len(transactionID); i++ {
				ip[i] ^= transactionID[i-4]
			}
		}
	}

	return net.IP(ip).String(), int(port), nil
}
