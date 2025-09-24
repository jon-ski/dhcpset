//go:build unix

package dhcp

import (
	"context"
	"net"
	"syscall"

	"github.com/jon-ski/dhcpset/pkg/dhcp/pkt"
)

func listenUDP(ip net.IP) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var ctrlErr error
			err := c.Control(func(fd uintptr) {
				// Allow rebinding and broadcast
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			})
			if err != nil {
				ctrlErr = err
			}
			return ctrlErr
		},
	}
	local := net.JoinHostPort(ip.To4().String(), "67")
	pc, err := lc.ListenPacket(context.Background(), "udp4", local)
	if err != nil {
		return nil, err
	}
	return pc.(*net.UDPConn), nil
}

func writeDHCP(conn *net.UDPConn, p *pkt.Pkt) error {
	// Ensure broadcast is allowed; already set at listen time
	buf, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(buf, &net.UDPAddr{IP: net.IPv4bcast, Port: 68})
	return err
}
