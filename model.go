package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/jon-ski/dhcpset/internal/styles"
	"github.com/jon-ski/dhcpset/internal/tui/ipinput"
	"github.com/jon-ski/dhcpset/pkg/dhcp"
)

type window struct {
	width  int
	height int
}

type model struct {
	cfg              config
	server           *dhcp.Server
	discoverChan     chan discoverInfo
	selectedDiscover discoverInfo
	stopChan         chan struct{}

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

	window window
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
		mac := <-m.discoverChan
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
	case discoverInfoSelection:
		log.Debug("msg: macSelection")
		m.state = 1
		m.selectedDiscover = discoverInfo(msg)
		log.Debug("selected MAC: ", m.selectedDiscover)
		log.Debug("sending stop signal")
		m.ipsetter.SetHwAddr(m.selectedDiscover.hwaddr)
		m.ipsetter.SetTXID(m.selectedDiscover.xid)
		go func() {
			m.stopChan <- struct{}{}
		}()
		return m, cmd
	case discoverInfo:
		log.Debug("msg: discoverInfo")
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

func (m model) sendOffer(req SetIPRequest) error {
	m.ipsetter.Log("Sending offer")
	err := m.server.Offer(req.MAC, req.IP, req.XID)
	if err != nil {
		err = fmt.Errorf("failed to set IP: %w", err)
		m.ipsetter.Log(err.Error())
		return err
	}
	m.ipsetter.Log("Offer sent successfully")
	return nil
}

func (m model) waitRequest(req SetIPRequest) error {
	m.ipsetter.Log("Listening for request from device")
	err := m.server.WaitRequest(req.MAC, req.IP, req.XID)
	if err != nil {
		err = fmt.Errorf("failed to set IP: %w", err)
		m.ipsetter.Log(err.Error())
		return err
	}
	m.ipsetter.Log("Request received")
	return nil
}

func (m model) sendAck(req SetIPRequest) error {
	m.ipsetter.Log("Sending ACK packet")
	err := m.server.Ack(req.MAC, req.IP, req.XID)
	if err != nil {
		err = fmt.Errorf("failed to set IP: %w", err)
		m.ipsetter.Log(err.Error())
		return err
	}
	m.ipsetter.Log("ACK sent successfully")
	return nil
}

func (m model) UpdateIPInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case SetIPRequest:
		log.Debug("msg: SetIPRequest")
		log.Debug("setting IP: ", "details", msg)
		m.ipsetter.pendLog.Item(NewSetIPLogMsg(fmt.Sprintf("Sending Offer to %v", msg.MAC)))
		return m, func() tea.Msg {
			err := m.sendOffer(msg)
			if err != nil {
				return SetIPResult{err}
			}
			err = m.waitRequest(msg)
			if err != nil {
				return SetIPResult{err}
			}
			err = m.sendAck(msg)
			if err != nil {
				return SetIPResult{err}
			}
			m.ipsetter.Log("IP set successfully")
			return SetIPResult{nil}
		}
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
	case 0:
		listenText := m.lModel.View()
		listenText = listenStyle.Width(m.window.width - 2).
			Render(listenText)
		s += lipgloss.PlaceHorizontal(
			m.window.width, lipgloss.Center, listenText,
		)
		s += "\n"

	case 1:
		s += m.ipsetter.View() + "\n\n"
	}

	// Help view
	s += "\n"
	helpView := lipgloss.PlaceHorizontal(m.window.width, lipgloss.Left, m.help.View(m.keys))

	return header + s + helpView + "\n"
}
