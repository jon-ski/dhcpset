package dhcp

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"

	"github.com/jon-ski/dhcpset/pkg/dhcp/pkt"
)

var ErrInvalidIP = errors.New("invalid IP address")

type Server struct {
	conn *net.UDPConn

	addr net.IP
}

func NewServer(ipAddr string) (*Server, error) {
	addr := net.ParseIP(ipAddr)
	if addr == nil {
		return nil, ErrInvalidIP
	}
	return &Server{
		addr: addr,
	}, nil
}

func (s *Server) Listen() error {
	addr, err := net.ResolveUDPAddr("udp", s.addr.To4().String()+":67")
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}
	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}
	return nil
}

func (l *Server) Close() error {
	return l.conn.Close()
}

func (l *Server) Read() (*pkt.Pkt, error) {
	slog.Debug("reading packet")
	buf := make([]byte, 1500)
	n, _, err := l.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	return pkt.NewFromBytes(buf[:n])
}

func (s *Server) SniffMac() (net.HardwareAddr, error) {
	pkt, err := s.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read packet: %w", err)
	}
	slog.Debug("sniffed MAC address", "mac", pkt.Header.CHAddr[:6])
	return pkt.Header.CHAddr[:6], nil
}

func (l *Server) Write(pkt *pkt.Pkt) error {
	slog.Debug("writing packet", "packet", pkt)
	buf, err := pkt.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal packet: %w", err)
	}
	_, err = l.conn.WriteToUDP(buf, &net.UDPAddr{
		Port: 68,
		IP:   net.IPv4bcast,
	})
	if err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}
	return nil
}

func (s *Server) newOffer(hwAddr net.HardwareAddr, ip net.IP, xid uint32) *pkt.Pkt {
	req := s.newPkt()
	req.Header.OpCode = 0x02
	req.Header.XID = xid
	req.Header.YIAddr = [4]byte(ip.To4())
	copy(req.Header.CHAddr[:], hwAddr)
	// req.Header.CHAddr = [16]byte(hwAddr)
	req.Options.Add(pkt.NewOptionMessageType(pkt.MessageTypeOffer))
	req.Options.Add(pkt.NewOptionServerID(s.addr.To4()))
	req.Options.Add(pkt.NewOptionSubnetMask(net.IPv4Mask(255, 255, 255, 0)))
	req.Options.Add(pkt.NewOptionEnd())
	return req
}

func (l *Server) Offer(hwAddr net.HardwareAddr, ip net.IP) error {
	var xid uint32

	// Sniff until we see the hwAddr, then extract the xid
	slog.Debug("listening for hardware address", slog.String("addr", hwAddr.String()))
	for {
		pkt, err := l.Read()
		if err != nil {
			return fmt.Errorf("failed to read packet: %w", err)
		}
		// Compare the MAC address
		if slices.Equal(pkt.Header.CHAddr[:6], hwAddr) {
			xid = pkt.Header.XID
			slog.Debug("found MAC address", "mac", hwAddr.String(), "xid", xid)
			break
		}
	}
	pkt := l.newOffer(hwAddr, ip, xid)
	slog.Debug("sending offer", "packet", pkt)
	return l.Write(pkt)
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

func (s *Server) ServeAddress() string {
	if s.conn == nil {
		return "unbound"
	}
	return s.conn.LocalAddr().String()
}
