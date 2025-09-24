package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/jon-ski/dhcpset/internal/logging"
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
	// Initialize logging
	err := logging.Init()
	if err != nil {
		logging.Fatal("failed to initialize logging", "error", err)
	}
	defer func() {
		if r := recover(); r != nil {
			logging.Error("panic recovered", "panic", r)
		}
	}()

	// Setup
	logging.Info("application_started", "version", "1.0.0")

	cfg, err := chooseConfig()
	if err != nil {
		logging.Error("failed to choose configuration", "error", err)
		logging.Fatal("configuration failed")
	}
	logging.LogNetworkOperation("interface_selected", cfg.iface.Name, cfg.addr.String(), nil)

	// Create a listener
	s, err := dhcp.NewServer(cfg.addr.String())
	if err != nil {
		logging.Error("failed to create DHCP server", "error", err)
		logging.Fatal("server creation failed")
	}
	defer s.Close()

	// Listen for packets
	err = s.Listen()
	if err != nil {
		logging.Error("failed to setup listener", "error", err)
		logging.Fatal("listener setup failed")
	}

	// model
	m := newModel(cfg, s)

	logging.Info("starting_discover_packet_monitoring")
	m.discoverChan = sniffMacs(s, m.stopChan)

	// Run the UI
	p := tea.NewProgram(m, tea.WithAltScreen())
	logging.Info("starting_ui")
	_, err = p.Run()
	if err != nil {
		logging.Error("failed to run program", "error", err)
		logging.Fatal("program execution failed")
	}

	// Ensure proper cleanup
	logging.Info("stopping_discover_packet_monitoring")
	close(m.stopChan)

	// Close the UDP connection to stop any pending reads
	logging.Info("closing_udp_connection")
	s.Close()

	// Give the goroutine a moment to clean up
	time.Sleep(200 * time.Millisecond)

	logging.Info("application_shutdown")
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
	info := make(chan discoverInfo, 10) // Add buffer to prevent blocking
	go func() {
		logging.Info("mac_sniffing_started")
		defer logging.Info("mac_sniffing_stopped")
		defer close(info) // Ensure channel is closed when goroutine exits

		for {
			select {
			case <-stop:
				logging.Info("stopping MAC sniffing")
				return
			default:
				// Call SniffMac directly - it has its own timeout handling
				mac, xid, err := s.SniffMac()
				if err != nil {
					// Check if this is a connection closed error during shutdown
					if strings.Contains(err.Error(), "connection closed") {
						logging.Debug("sniff_mac_connection_closed", "action", "shutting_down")
						return
					}
					logging.Error("failed to sniff MAC", "error", err)
					// Add exponential backoff for network errors
					time.Sleep(100 * time.Millisecond)
					continue
				}

				logging.LogPacket("received", "discover", mac.String(), "", xid)
				select {
				case info <- newDiscoverInfo(mac, xid):
				case <-stop:
					return
				default:
					logging.Warn("discover_info_channel_full", "dropped_packet", mac.String())
				}
			}
		}
	}()
	return info
}

type keyMap struct {
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Retry key.Binding

	Help key.Binding

	Quit key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Retry},
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

	Retry: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "Retry"),
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
