package main

import (
	"fmt"
	"net"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/jon-ski/dhcpset/internal/styles"
	"github.com/jon-ski/dhcpset/internal/tui/ipinput"
	"github.com/jon-ski/dhcpset/pkg/dhcp"
)

type model struct {
	cfg         config
	server      *dhcp.Server
	macChan     chan net.HardwareAddr
	macSelected net.HardwareAddr
	stopChan    chan struct{}

	lModel listenModel

	ipsetter IPSetter

	// state of program
	// 0 = listening for packets
	// 1 = New IP Address form
	// 2 = Setting IP Address
	state int

	// For the UI
	// help
	keys keyMap
	help help.Model
}

func newModel(cfg config, server *dhcp.Server) model {
	ipinput := ipinput.New()
	ipinput.Prompt = "IP Address"
	ipinput.Style = ipinput.Style.Border(lipgloss.NormalBorder())
	ipinput.FocusedForeground = styles.Primary()
	return model{
		cfg:    cfg,
		server: server,

		stopChan: make(chan struct{}),

		lModel:   newListenModel(),
		ipsetter: NewIPSetter(),

		keys: keys,
		help: help.New(),
	}
}

func (m model) getMac() tea.Cmd {
	return func() tea.Msg {
		mac := <-m.macChan
		return mac
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
	case macSelection:
		log.Debug("msg: macSelection")
		m.state = 1
		m.macSelected = net.HardwareAddr(msg)
		log.Debug("selected MAC: ", m.macSelected)
		log.Debug("sending stop signal")
		m.ipsetter.SetHwAddr(m.macSelected)
		m.ipsetter.SetTXID(0) // TODO: set txid
		go func() {
			m.stopChan <- struct{}{}
		}()
		return m, cmd
	case net.HardwareAddr:
		log.Debug("msg: net.HardwareAddr")
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
		log.Debug("msg: SetIPRequest")
		log.Debug("setting IP: ", "details", msg)
		return m, func() tea.Msg {
			err := m.server.Offer(msg.MAC, msg.IP)
			if err != nil {
				log.Errorf("failed to set IP: %v", err)
				return SetIPResult{fmt.Errorf("failed to set IP: %w", err)}
			}
			return SetIPResult{nil}
		}
		// case SetIPResult:
		// 	log.Debug("msg: SetIPResult")
		// 	switch msg {
		// 	case nil:
		// 		log.Debug("Offer Sent Successfully")
		// 	default:
		// 		log.Errorf("failed to set IP: %v", msg)
		// 	}
	}

	m.ipsetter, cmd = m.ipsetter.Update(msg)
	return m, cmd
}

// bubbletea update function
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	switch m.state {
	case 0:
		return m.UpdateMACListener(msg)

	case 1:
		return m.UpdateIPInput(msg)
	}

	return m, cmd
}

const titleText = `
____  __  ____________  _____ ____________
/ __ \/ / / / ____/ __ \/ ___// ____/_  __/
/ / / / /_/ / /   / /_/ /\__ \/ __/   / /   
/ /_/ / __  / /___/ ____/___/ / /___  / /    
/_____/_/ /_/\____/_/    /____/_____/ /_/     
										   
`

func (m model) View() string {
	header := titleText

	var s string
	switch m.state {
	case 0:
		s += m.lModel.View()

	case 1:
		s += m.ipsetter.View() + "\n\n"
	}

	// Help view
	helpView := m.help.View(m.keys) + "\n"

	return header + s + helpView
}
