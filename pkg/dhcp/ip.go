package dhcp

import "net"

func IPv4(b []byte) net.IP {
	if len(b) < 4 {
		return nil
	}
	return net.IPv4(b[0], b[1], b[2], b[3])
}
