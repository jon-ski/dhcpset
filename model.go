package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon-ski/dhcpset/internal/logging"
	"github.com/jon-ski/dhcpset/pkg/dhcp"
)

type window struct {
	width  int
	height int
}

type state int

const (
	state_listen state = iota
	state_form
	state_set
)

type model struct {
	cfg              config
	server           *dhcp.Server
	dhcpService      *dhcp.Service
	discoverChan     chan discoverInfo
	selectedDiscover discoverInfo
	stopChan         chan struct{}
	progressChan     chan dhcp.ProgressUpdate

	lModel listenModel

	ipsetter IPSetter

	// Program state
	// 0 = listening for packets
	// 1 = New IP Address form
	// 2 = Setting IP Address (unused)
	state state

	// For the UI
	// help
	keys keyMap
	help help.Model

	window window
}

func newModel(cfg config, server *dhcp.Server) model {
	// Create DHCP service with logging callback
	dhcpService := dhcp.NewService(server)

	return model{
		cfg:         cfg,
		server:      server,
		dhcpService: dhcpService,

		stopChan:     make(chan struct{}),
		progressChan: make(chan dhcp.ProgressUpdate, 10),

		lModel:   newListenModel(),
		ipsetter: NewIPSetter(),

		keys: keys,
		help: help.New(),
	}
}

func (m model) getMac() tea.Cmd {
	return func() tea.Msg {
		select {
		case mac := <-m.discoverChan:
			return mac
		case <-time.After(100 * time.Millisecond):
			// Return nil if no message available to prevent blocking
			return nil
		}
	}
}

func (m model) getProgress() tea.Cmd {
	return func() tea.Msg {
		select {
		case progress := <-m.progressChan:
			return progress
		case <-time.After(50 * time.Millisecond):
			// Return nil if no progress update available
			return nil
		}
	}
}

// bubbletea init function
func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.lModel.Init(),
		m.ipsetter.Init(),
	)
}

func (m model) UpdateMACListener(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case discoverInfoSelection:
		logging.LogUIEvent("mac_selected", "listener", map[string]interface{}{
			"mac": msg.hwaddr.String(),
			"xid": msg.xid,
		})
		m.state = state_form
		m.selectedDiscover = discoverInfo(msg)
		m.ipsetter.SetHwAddr(m.selectedDiscover.hwaddr)
		m.ipsetter.SetTXID(m.selectedDiscover.xid)
		go func() {
			select {
			case m.stopChan <- struct{}{}:
				logging.Debug("stop_signal_sent", "reason", "device_selected")
			case <-time.After(1 * time.Second):
				logging.Warn("stop_channel_timeout", "channel_full", true)
			}
		}()
		return m, cmd
	case discoverInfo:
		logging.LogUIEvent("discover_received", "listener", map[string]interface{}{
			"mac": msg.hwaddr.String(),
			"xid": msg.xid,
		})
		for i := range m.lModel.list {
			if m.lModel.list[i].hwaddr.String() == msg.hwaddr.String() {
				m.lModel.list[i] = msg
				return m, m.getMac()
			}
		}
		m.lModel.list = append(m.lModel.list, msg)
		return m, m.getMac()
	}
	m.lModel, cmd = m.lModel.Update(msg)
	return m, tea.Batch(cmd, m.getMac())
}

func (m model) UpdateIPInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case SetIPRequest:
		logging.LogUIEvent("ip_set_requested", "ipsetter", map[string]interface{}{
			"mac": msg.MAC.String(),
			"ip":  msg.IP.String(),
			"xid": msg.XID,
		})

		// Set up logging callback for the service
		m.dhcpService.SetLogger(func(logMsg string) {
			m.ipsetter.Log(logMsg)
		})

		// Set up progress callback for the service
		m.dhcpService.SetProgressCallback(func(progress dhcp.ProgressUpdate) {
			select {
			case m.progressChan <- progress:
			default:
				// Channel full, skip this update and log overflow
				logging.Warn("progress_channel_overflow", "dropped_updates", 1, "step", progress.Step)
			}
		})

		m.ipsetter.pendLog.Item(NewSetIPLogMsg(fmt.Sprintf("Starting IP assignment for %v", msg.MAC)))
		return m, tea.Batch(
			m.getProgress(),
			func() tea.Msg {
				result := m.dhcpService.SetIPAddress(dhcp.SetIPRequest{
					IP:  msg.IP,
					MAC: msg.MAC,
					XID: msg.XID,
				})
				return SetIPResult{
					err:     result.Error,
					Message: result.Message,
				}
			},
		)

	case dhcp.ProgressUpdate:
		// Handle progress updates
		m.ipsetter, cmd = m.ipsetter.Update(msg)
		return m, tea.Batch(cmd, m.getProgress())
	}

	m.ipsetter, cmd = m.ipsetter.Update(msg)
	return m, cmd
}

func (m model) updateWindow(msg tea.Msg) (model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.window.width = msg.Width
		m.window.height = msg.Height
	}

	return m, cmd
}

// bubbletea update function
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m, cmd = m.updateWindow(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			logging.Info("user_requested_quit", "key", msg.String())
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	switch m.state {
	case state_listen:
		return m.UpdateMACListener(msg)

	case state_form:
		return m.UpdateIPInput(msg)
	}

	return m, cmd
}

const _ = `
██████╗ ██╗  ██╗ ██████╗██████╗ ███████╗███████╗████████╗
██╔══██╗██║  ██║██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝
██║  ██║███████║██║     ██████╔╝███████╗█████╗     ██║   
██║  ██║██╔══██║██║     ██╔═══╝ ╚════██║██╔══╝     ██║   
██████╔╝██║  ██║╚██████╗██║     ███████║███████╗   ██║   
╚═════╝ ╚═╝  ╚═╝ ╚═════╝╚═╝     ╚══════╝╚══════╝   ╚═╝   
`
const titleText = "DHCPSET"
const subtitleText = "Single device DHCP IP setter"

var titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
var subtitleStyle = lipgloss.NewStyle().Faint(true)

var hline = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, true, false)
var listenStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())

func (m model) View() string {
	header := titleStyle.Render(titleText) + "\n"
	header += subtitleStyle.Render(subtitleText)
	header += hline.Width(m.window.width).Render("") + "\n"

	var s string
	switch m.state {
	case state_listen:
		listenText := m.lModel.View()
		listenText = listenStyle.Width(m.window.width - 2).
			Render(listenText)
		s += lipgloss.PlaceHorizontal(
			m.window.width, lipgloss.Center, listenText,
		)
		s += "\n"

	case state_form:
		s += m.ipsetter.View() + "\n\n"
	}

	// Help view
	s += "\n"
	helpView := lipgloss.PlaceHorizontal(m.window.width, lipgloss.Left, m.help.View(m.keys))

	return header + s + helpView + "\n"
}
