package dhcp

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jon-ski/dhcpset/internal/logging"
	"github.com/jon-ski/dhcpset/pkg/dhcp/pkt"
)

// ErrInvalidIP is returned when an invalid IP string is provided to NewServer.
var ErrInvalidIP = errors.New("invalid IP address")

// ErrNotListening is returned when socket operations are attempted before Listen.
var ErrNotListening = errors.New("server is not listening")

const (
	bootpOpRequest = 0x01 // BOOTP request from client
	bootpOpReply   = 0x02 // BOOTP reply from server

	htypeEthernet = 0x01
)

type Server struct {
	conn *net.UDPConn

	addr net.IP
}

// NewServer creates a new DHCP server bound to the given server IP address.
func NewServer(ipAddr string) (*Server, error) {
	addr := net.ParseIP(ipAddr)
	if addr == nil {
		return nil, ErrInvalidIP
	}
	return &Server{
		addr: addr,
	}, nil
}

// Listen binds a UDP socket ready to read and write DHCP packets.
func (s *Server) Listen() error {
	conn, err := listenUDP(s.addr.To4())
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}
	s.conn = conn
	return nil
}

// Close closes the underlying UDP socket.
func (s *Server) Close() error {
	if s.conn == nil {
		return ErrNotListening
	}

	// Set a deadline for graceful shutdown
	s.conn.SetDeadline(time.Now().Add(1 * time.Second))

	err := s.conn.Close()
	s.conn = nil // Clear the connection to prevent reuse
	return err
}

// Read reads a single DHCP packet from the socket.
func (s *Server) Read() (*pkt.Pkt, error) {
	if s.conn == nil {
		return nil, ErrNotListening
	}

	// Set a reasonable read timeout
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	buf := make([]byte, 1500)
	n, addr, err := s.conn.ReadFromUDP(buf)
	if err != nil {
		logging.Error("failed_to_read_packet", "error", err)
		return nil, err
	}

	logging.Debug("raw_packet_received", "bytes", n, "source", addr.String())

	// Validate packet size
	if n < 240 {
		logging.Error("packet_too_small", "bytes_received", n, "source_addr", addr.String())
		return nil, fmt.Errorf("packet too small: %d bytes", n)
	}

	pkt, err := pkt.NewFromBytes(buf[:n])
	if err != nil {
		logging.Error("failed_to_parse_packet", "error", err, "bytes_received", n, "source_addr", addr.String())
		// Log first few bytes for debugging
		if n > 0 {
			maxBytes := n
			if maxBytes > 16 {
				maxBytes = 16
			}
			firstBytes := make([]byte, maxBytes)
			copy(firstBytes, buf[:maxBytes])
			logging.Debug("packet_first_bytes", "hex", fmt.Sprintf("%x", firstBytes))
		}
		return nil, err
	}

	logging.Debug("packet_received", "bytes", n, "source", addr.String(), "xid", pkt.Header.XID)
	return pkt, nil
}

// SniffMac blocks until a packet arrives and returns the source MAC and XID.
func (s *Server) SniffMac() (net.HardwareAddr, uint32, error) {
	const maxRetries = 3
	const baseBackoff = 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		pkt, err := s.Read()
		if err != nil {
			// Check if connection is closed - don't retry in this case
			if strings.Contains(err.Error(), "use of closed network connection") {
				logging.Debug("connection_closed_during_read", "action", "stopping_sniff")
				return nil, 0, fmt.Errorf("connection closed: %w", err)
			}

			if attempt < maxRetries-1 {
				backoff := time.Duration(attempt+1) * baseBackoff
				logging.Warn("packet_read_failed_retrying", "attempt", attempt+1, "backoff_ms", backoff.Milliseconds(), "error", err)
				time.Sleep(backoff)
				continue
			}
			return nil, 0, fmt.Errorf("failed to read packet after %d attempts: %w", maxRetries, err)
		}

		// Validate packet before processing
		if len(pkt.Header.CHAddr) < 6 {
			logging.Warn("invalid_packet_hwaddr_length", "length", len(pkt.Header.CHAddr))
			continue
		}

		mac := pkt.Header.CHAddr[:6]
		macAddr := net.HardwareAddr(mac)
		logging.LogPacket("sniffed", "discover", macAddr.String(), "", pkt.Header.XID)
		return mac, pkt.Header.XID, nil
	}

	return nil, 0, fmt.Errorf("max retries exceeded")
}

// Write broadcasts the DHCP packet to port 68.
func (s *Server) Write(p *pkt.Pkt) error {
	if s.conn == nil {
		return ErrNotListening
	}

	logging.Debug("writing_packet", "xid", p.Header.XID, "opcode", p.Header.OpCode)
	buf, err := p.MarshalBinary()
	if err != nil {
		logging.Error("failed_to_marshal_packet", "error", err)
		return fmt.Errorf("failed to marshal packet: %w", err)
	}

	dest := &net.UDPAddr{
		Port: 68,
		IP:   net.IPv4bcast,
	}

	_, err = s.conn.WriteToUDP(buf, dest)
	if err != nil {
		logging.Error("failed_to_write_packet", "error", err, "destination", dest.String(), "bytes", len(buf))
		return fmt.Errorf("failed to write packet: %w", err)
	}

	logging.Debug("packet_sent", "bytes", len(buf), "destination", dest.String())
	return nil
}

func (s *Server) newOffer(hwAddr net.HardwareAddr, ip net.IP, xid uint32) *pkt.Pkt {
	req := s.newPkt()
	req.Header.OpCode = bootpOpReply
	req.Header.HType = htypeEthernet
	req.Header.XID = xid
	req.Header.YIAddr = [4]byte(ip.To4())
	req.SetCHAddr(hwAddr)
	req.Options.Add(pkt.NewOptionMessageType(pkt.MessageTypeOffer))
	req.Options.Add(pkt.NewOptionServerID(s.addr.To4()))
	s.addStandardOptions(req)
	return req
}

// Offer sends a DHCPOFFER for the given hardware address and transaction ID.
func (s *Server) Offer(hwAddr net.HardwareAddr, ip net.IP, xid uint32) error {
	p := s.newOffer(hwAddr, ip, xid)
	logging.LogPacket("sending", "offer", hwAddr.String(), ip.String(), xid)
	return s.Write(p)
}

func (s *Server) newAck(hwAddr net.HardwareAddr, ip net.IP, xid uint32) *pkt.Pkt {
	req := s.newPkt()
	req.Header.OpCode = bootpOpReply
	req.Header.HType = htypeEthernet
	req.Header.XID = xid
	req.Header.YIAddr = [4]byte(ip.To4())
	req.Header.SIAddr = [4]byte(s.addr.To4())
	req.SetCHAddr(hwAddr)
	req.Options.Add(pkt.NewOptionMessageType(pkt.MessageTypeAck))
	req.Options.Add(pkt.NewOptionServerID(s.addr.To4()))
	s.addStandardOptions(req)
	return req
}

// WaitRequest blocks until a DHCPREQUEST for the given XID is received.
func (s *Server) WaitRequest(hwAddr net.HardwareAddr, ip net.IP, xid uint32) error {
	logging.Debug("listening_for_request", "mac", hwAddr.String(), "ip", ip.String(), "xid", xid)

	for {
		p, err := s.Read()
		if err != nil {
			logging.Error("failed_to_read_packet", "error", err)
			return fmt.Errorf("failed to read packet: %w", err)
		}

		logging.Debug("received_packet", "opcode", p.Header.OpCode, "received_xid", p.Header.XID)

		// Ensure DHCP Message Type is REQUEST (53=3)
		if p.Header.OpCode == bootpOpRequest && p.Header.XID == xid {
			if t, ok := p.MessageType(); ok && t == pkt.MessageTypeRequest {
				logging.Info("received_matching_request")
				break
			}
			// Not the DHCPREQUEST we expect; keep listening
			logging.Debug("ignoring_non_request_with_matching_xid")
		} else {
			logging.Debug("ignoring_packet", "reason", "xid_mismatch_or_wrong_opcode")
		}
	}
	return nil
}

// Ack sends a DHCPACK for the given hardware address and transaction ID.
func (s *Server) Ack(hwAddr net.HardwareAddr, ip net.IP, xid uint32) error {
	p := s.newAck(hwAddr, ip, xid)
	logging.LogPacket("sending", "ack", hwAddr.String(), ip.String(), xid)
	return s.Write(p)
}

func (s *Server) OfferRequest(hwAddr net.HardwareAddr, ip net.IP, xid uint32) error {
	if err := s.Offer(hwAddr, ip, xid); err != nil {
		return fmt.Errorf("failed to send offer: %w", err)
	}
	logging.Debug("offer sent")
	if err := s.WaitRequest(hwAddr, ip, xid); err != nil {
		return err
	}
	return s.Ack(hwAddr, ip, xid)
}

func (s *Server) newPkt() *pkt.Pkt {
	return &pkt.Pkt{
		Header: pkt.Header{
			OpCode: 0x00,
			HType:  0,
			HLen:   0,
			Hops:   0,
			XID:    0,
			Secs:   0,
			Flags:  0,
			CIAddr: [4]byte{},
			YIAddr: [4]byte{},
			SIAddr: [4]byte(s.addr.To4()),
			GIAddr: [4]byte{},
			CHAddr: [16]byte{},
			SName:  [64]byte{},
			File:   [128]byte{},
			Cookie: [4]byte{0x63, 0x82, 0x53, 0x63},
		},
		Options: pkt.Options{},
	}
}

// addStandardOptions appends common options used in OFFER and ACK.
func (s *Server) addStandardOptions(p *pkt.Pkt) {
	p.Options.Add(pkt.NewOptionSubnetMask(net.IPv4Mask(255, 255, 255, 0)))
	// Default timers suitable for embedded devices
	p.Options.Add(pkt.NewOptionLeaseTime(3600))     // 1 hour
	p.Options.Add(pkt.NewOptionRenewalTime(1800))   // T1 30 min
	p.Options.Add(pkt.NewOptionRebindingTime(3150)) // T2 ~52.5 min
	p.Options.Add(pkt.NewOptionEnd())
}

// ServeAddress returns the UDP address the server is bound to, or "unbound".
func (s *Server) ServeAddress() string {
	if s.conn == nil {
		return "unbound"
	}
	return s.conn.LocalAddr().String()
}

// IsHealthy checks if the UDP connection is still healthy
func (s *Server) IsHealthy() bool {
	if s.conn == nil {
		return false
	}

	// Try to get local address - if this fails, connection is broken
	addr := s.conn.LocalAddr()
	return addr != nil
}

// SetReadDeadline sets a deadline for read operations
func (s *Server) SetReadDeadline(deadline time.Time) error {
	if s.conn == nil {
		return ErrNotListening
	}
	return s.conn.SetReadDeadline(deadline)
}

// SetWriteDeadline sets a deadline for write operations
func (s *Server) SetWriteDeadline(deadline time.Time) error {
	if s.conn == nil {
		return ErrNotListening
	}
	return s.conn.SetWriteDeadline(deadline)
}
