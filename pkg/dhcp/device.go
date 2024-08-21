package dhcp

import (
	"net"

	"github.com/jon-ski/dhcpset/pkg/dhcp/pkt"
)

// DeviceData is a struct that holds the hardware address and IP address of a device
type DeviceData struct {
	HWAddr net.HardwareAddr
	IP     net.IP
}

// RequestData is a struct that holds the data from the server and client
// This data is used for DHCP requests and responses
type RequestData struct {
	ServerData DeviceData
	ClientData DeviceData
	XID        uint32
}

func RequestDataFromPkt(pkt *pkt.Pkt) RequestData {
	return RequestData{
		ServerData: DeviceData{
			HWAddr: HwAddrFromBytes(pkt.Header.CHAddr[:]),
			IP:     IPv4(pkt.Header.CIAddr[:]),
		},
		ClientData: DeviceData{
			HWAddr: HwAddrFromBytes(pkt.Header.SIAddr[:]),
			IP:     IPv4(pkt.Header.SIAddr[:]),
		},
		XID: pkt.Header.XID,
	}
}
