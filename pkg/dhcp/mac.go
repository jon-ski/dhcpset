package dhcp

func HwAddrFromBytes(b []byte) []byte {
	if len(b) < 6 {
		return nil
	}
	return b[:6]
}
