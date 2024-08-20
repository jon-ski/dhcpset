package dhcp

import (
	"net"
)

// GetInterfaces retrieves all valid interfaces to use for UDP DHCP messaging.
func GetInterfaces() ([]net.Interface, error) {
	// Get all interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// Filter out loopback and down interfaces
	var validInterfaces []net.Interface
	for _, i := range interfaces {
		if i.Flags&net.FlagLoopback == 0 && i.Flags&net.FlagUp != 0 {
			validInterfaces = append(validInterfaces, i)
		}
	}

	return validInterfaces, nil
}

func GetIPs(iface net.Interface) ([]net.IP, error) {
	// Get all addresses for the interface
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	// Parse the CIDR addresses
	// CIDR is a string representation of an IP address and its associated routing prefix
	var ips []net.IP
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, err
		}
		ips = append(ips, ip)
	}

	// Filter out non-IPv4 addresses
	var ipv4s []net.IP
	for _, ip := range ips {
		if ip.To4() != nil {
			ipv4s = append(ipv4s, ip)
		}
	}

	// Filter out non-dhcp listener valid IPs
	var validIPs []net.IP
	for _, ip := range ipv4s {
		if ip[0] == 169 && ip[1] == 254 {
			continue
		}
		validIPs = append(validIPs, ip)
	}

	return validIPs, nil
}
