package pkt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	packetreader "github.com/jon-ski/dhcpset/internal/packet-reader"
)

const (
	MessageTypeOffer        = 2
	MessageTypeRequest      = 3
	MessageTypeAck          = 5
	optMsgType         byte = 53
	optServerID        byte = 54
	optRequestedIP     byte = 50
	optParamReqList    byte = 55
	optLeaseTime       byte = 51
	optRenewalTime     byte = 58
	optRebindingTime   byte = 59
	optSubnetMask      byte = 1
	optRouter          byte = 3
	optDNSServer       byte = 6
	optDomainName      byte = 15
	optEnd             byte = 255
)

var dhcpMagicCookie = []byte{0x63, 0x82, 0x53, 0x63}

var ErrInvalidPacket = errors.New("invalid packet")

// Header represents the BOOTP header
// fixed length
type Header struct {
	OpCode uint8
	HType  uint8
	HLen   uint8     // hardware address length
	Hops   uint8     // used by relay agents
	XID    uint32    // transaction ID
	Secs   uint16    // seconds since client started trying to boot
	Flags  uint16    // flags
	CIAddr [4]byte   // client IP address
	YIAddr [4]byte   // your IP address
	SIAddr [4]byte   // server IP address
	GIAddr [4]byte   // gateway IP address
	CHAddr [16]byte  // client hardware address
	SName  [64]byte  // server host name
	File   [128]byte // boot file name
	Cookie [4]byte   // magic cookie
}

// DHCPOptions represents the DHCP options
// variable length
type Options struct {
	Options []Option
}

func (o *Options) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	for _, opt := range o.Options {
		optBuf, err := opt.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("failed to marshal option: %w", err)
		}
		_, err = buf.Write(optBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to write option data to buffer: %w", err)
		}
	}
	return buf.Bytes(), nil
}

type Option struct {
	Type   byte
	Length byte
	Data   []byte
}

func (o *Option) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 2+len(o.Data))
	buf[0] = o.Type
	buf[1] = o.Length
	copy(buf[2:], o.Data)
	return buf, nil
}

func (o *Options) Decode(r io.Reader) error {
	for {
		var opt Option
		err := opt.Decode(r)
		if err != nil {
			return fmt.Errorf("failed to decode option: %w", err)
		}
		o.Options = append(o.Options, opt)
		if opt.Type == optEnd {
			break
		}
	}
	return nil
}

func (o *Option) Decode(r io.Reader) error {
	// Read code first
	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		return err
	}
	o.Type = code[0]
	// Handle Pad and End which have no length nor data
	if o.Type == 0 { // Pad
		o.Length = 0
		o.Data = nil
		return nil
	}
	if o.Type == optEnd { // End
		o.Length = 0
		o.Data = nil
		return nil
	}
	// Read length then data
	var lb [1]byte
	if _, err := io.ReadFull(r, lb[:]); err != nil {
		return err
	}
	o.Length = lb[0]
	if o.Length == 0 {
		o.Data = nil
		return nil
	}
	o.Data = make([]byte, o.Length)
	_, err := io.ReadFull(r, o.Data)
	return err
}

type Pkt struct {
	Header  Header
	Options Options
}

func NewPkt() *Pkt {
	return &Pkt{}
}

func NewFromBytes(b []byte) (*Pkt, error) {
	pkt := NewPkt()
	err := pkt.UnmarshalBinary(b)
	if err != nil {
		return nil, err
	}
	return pkt, nil
}

func (p *Pkt) UnmarshalBinary(b []byte) error {
	err := binary.Read(packetreader.NewReader(b), binary.BigEndian, &p.Header)
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Decode options
	p.Options.Options = make([]Option, 0)
	err = p.Options.Decode(bytes.NewReader(b[240:]))
	if err != nil {
		return fmt.Errorf("failed to decode options: %w", err)
	}
	return nil
}

func (p *Pkt) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, &p.Header)
	if err != nil {
		return nil, fmt.Errorf("failed to write header: %w", err)
	}

	// Marshal options
	options, err := p.Options.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal options: %w", err)
	}
	_, err = buf.Write(options)
	if err != nil {
		return nil, fmt.Errorf("failed to write options: %w", err)
	}

	return buf.Bytes(), nil
}

func (p *Pkt) PrintMAC() string {
	// format and print the mac address
	b := strings.Builder{}
	for i := 0; i < int(p.Header.HLen); i++ {
		b.WriteString(fmt.Sprintf("%02x", p.Header.CHAddr[i]))
		if i < int(p.Header.HLen)-1 {
			b.WriteString(":")
		}
	}
	return b.String()
}

func (p *Pkt) PrintName() string {
	// Get name from options
	for _, opt := range p.Options.Options {
		if opt.Type == 12 && opt.Length > 0 {
			return string(opt.Data)
		}
	}
	return "unknown"
}

func (p *Pkt) SetCHAddr(addr net.HardwareAddr) {
	// Set the length of the hardware address
	if len(addr) > 16 {
		p.Header.HLen = 16
		addr = addr[:16]
	}
	p.Header.HLen = uint8(len(addr))
	copy(p.Header.CHAddr[:], addr)
}

func (o *Options) Add(opt Option) {
	o.Options = append(o.Options, opt)
}

func NewOptionMessageType(t uint8) Option {
	return Option{
		Type:   53,
		Length: 1,
		Data:   []byte{t},
	}
}

func NewOptionServerID(ip net.IP) Option {
	return Option{
		Type:   54,
		Length: 4,
		Data:   []byte(ip.To4()),
	}
}

func NewOptionSubnetMask(mask net.IPMask) Option {
	return Option{
		Type:   optSubnetMask,
		Length: 4,
		Data:   []byte(mask),
	}
}

func NewOptionEnd() Option {
	return Option{
		Type:   0xff,
		Length: 0,
		Data:   nil,
	}
}

func dhcpOption(code byte, data []byte) Option {
	return Option{
		Type:   code,
		Length: byte(len(data)),
		Data:   data,
	}
}

func NewServerIDOption(ip net.IP) Option {
	return dhcpOption(optServerID, ip.To4())
}

// NewOptionLeaseTime creates option 51 with a 32-bit big-endian seconds value
func NewOptionLeaseTime(seconds uint32) Option {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, seconds)
	return dhcpOption(optLeaseTime, buf)
}

// NewOptionRenewalTime creates option 58 (T1)
func NewOptionRenewalTime(seconds uint32) Option {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, seconds)
	return dhcpOption(optRenewalTime, buf)
}

// NewOptionRebindingTime creates option 59 (T2)
func NewOptionRebindingTime(seconds uint32) Option {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, seconds)
	return dhcpOption(optRebindingTime, buf)
}

// NewOptionRouter creates option 3 with one router
func NewOptionRouter(ip net.IP) Option {
	return dhcpOption(optRouter, ip.To4())
}

// NewOptionDNSServers creates option 6 with one or more IPv4 DNS servers
func NewOptionDNSServers(ips ...net.IP) Option {
	b := make([]byte, 0, 4*len(ips))
	for _, ip := range ips {
		b = append(b, ip.To4()...)
	}
	return dhcpOption(optDNSServer, b)
}

// Get returns all options of a given code
func (o *Options) Get(code byte) []Option {
	var res []Option
	for _, opt := range o.Options {
		if opt.Type == code {
			res = append(res, opt)
		}
	}
	return res
}

// First returns the first option of a given code
func (o *Options) First(code byte) (*Option, bool) {
	for i := range o.Options {
		if o.Options[i].Type == code {
			return &o.Options[i], true
		}
	}
	return nil, false
}

// MessageType extracts DHCP option 53 if present
func (p *Pkt) MessageType() (byte, bool) {
	if opt, ok := p.Options.First(optMsgType); ok && len(opt.Data) == 1 {
		return opt.Data[0], true
	}
	return 0, false
}
