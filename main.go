package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/jon-ski/dhcpset/pkg/dhcp"
)

func chooseInterface() (net.Interface, error) {
	interfaces, err := dhcp.GetInterfaces()
	if err != nil {
		return net.Interface{}, fmt.Errorf("failed to get interfaces: %w", err)
	}

	// If no interfaces, return an error
	if len(interfaces) == 0 {
		return net.Interface{}, errors.New("no valid interfaces found")
	}

	// If only one interface, return it
	if len(interfaces) == 1 {
		return interfaces[0], nil
	}

	var selection = 0
	var options []huh.Option[int]
	// Otherwise, prompt the user to choose an interface
	for i := range interfaces {
		options = append(options, huh.NewOption(interfaces[i].Name, i))
	}
	form := huh.NewSelect[int]().
		Title("Choose an interface").
		Options(options...).
		Value(&selection)

	err = form.Run()
	if err != nil {
		return net.Interface{}, fmt.Errorf("failed to choose interface: %w", err)
	}

	return interfaces[selection], nil
}

type config struct {
	iface net.Interface
	addr  net.IP
}

func chooseIP(iface net.Interface) (net.IP, error) {
	ipList, err := dhcp.GetIPs(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to get IP addresses: %w", err)
	}

	// If no IPs, return an error
	if len(ipList) == 0 {
		return nil, errors.New("no valid IP addresses found")
	}

	// If only one IP, return it
	if len(ipList) == 1 {
		return ipList[0], nil
	}

	var selection = 0
	var options []huh.Option[int]
	// Otherwise, prompt the user to choose an IP
	for i := range ipList {
		options = append(options, huh.NewOption(ipList[i].String(), i))
	}
	form := huh.NewSelect[int]().
		Title("Choose an IP address").
		Options(options...).
		Value(&selection)

	err = form.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to choose IP address: %w", err)
	}

	return ipList[selection], nil
}

func chooseConfig() (c config, err error) {
	// Choose an interface
	c.iface, err = chooseInterface()
	if err != nil {
		return c, fmt.Errorf("failed to choose interface: %w", err)
	}

	// Choose an IP Address
	c.addr, err = chooseIP(c.iface)
	if err != nil {
		return c, fmt.Errorf("failed to choose IP address: %w", err)
	}

	return c, nil
}

func main() {

	f, err := tea.LogToFile("debug.log", "dhcpset")
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	handler := log.New(f)
	handler.SetOutput(f)
	handler.SetLevel(log.DebugLevel)
	handler.SetReportTimestamp(true)
	handler.SetReportCaller(true)
	log.SetDefault(handler)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	log.Default().SetLevel(log.DebugLevel)
	defer f.Close()

	// Setup
	log.Debug("starting setup form")
	cfg, err := chooseConfig()
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("using interface %v with IP %v", cfg.iface.Name, cfg.addr)

	// Create a listener
	log.Debug("creating dhcp server")
	s, err := dhcp.NewServer(cfg.addr.String())
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	defer s.Close()

	// Listen for packets
	log.Debug("setting up listener")
	err = s.Listen()
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// model
	m := newModel(cfg, s)

	log.Debug("listening for discover packets")
	m.discoverChan = sniffMacs(s, m.stopChan)

	// Run the UI
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	if err != nil {
		log.Fatalf("failed to run program: %v", err)
	}
}

type discoverInfo struct {
	hwaddr net.HardwareAddr
	xid    uint32
	tstamp time.Time
}

func newDiscoverInfo(hwaddr net.HardwareAddr, xid uint32) discoverInfo {
	return discoverInfo{
		hwaddr: hwaddr,
		xid:    xid,
		tstamp: time.Now(),
	}
}

func sniffMacs(s *dhcp.Server, stop chan struct{}) chan discoverInfo {
	info := make(chan discoverInfo)
	go func() {
		for {
			// if running, continue. If stopped, break
			select {
			case <-stop:
				log.Debug("stopping MAC sniffing")
				return
			default:
			}

			mac, xid, err := s.SniffMac()
			if err != nil {
				log.Errorf("failed to sniff MAC: %v", err)
				continue
			}
			log.Debugf("new MAC: %v", mac)
			info <- newDiscoverInfo(mac, xid)

			// // Test Code
			// log.Debug("sending test MAC")
			// macs <- net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
			// time.Sleep(5 * time.Second)
		}
	}()
	return info
}

type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding

	Help key.Binding

	Quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		k.ShortHelp(),
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("↑/k", "move up"),
	),

	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "move down"),
	),

	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "Select"),
	),

	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "Toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "Quit"),
	),
}
