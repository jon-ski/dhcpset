//go:build windows

package dhcp

import (
	"net"

	"github.com/jon-ski/dhcpset/pkg/dhcp/pkt"
)

func listenUDP(ip net.IP) (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr("udp4", ip.To4().String()+":67")
	if err != nil {
		return nil, err
	}
	return net.ListenUDP("udp4", addr)
}

func writeDHCP(conn *net.UDPConn, p *pkt.Pkt) error {
	buf, err := p.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = conn.WriteToUDP(buf, &net.UDPAddr{IP: net.IPv4bcast, Port: 68})
	return err
}
