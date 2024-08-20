package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/jon-ski/dhcpset/internal/styles"
	"github.com/jon-ski/dhcpset/internal/tui/ipinput"
)

type IPSetter struct {
	state   int
	hwaddr  net.HardwareAddr
	txid    uint32
	ipinput ipinput.Model
	result  SetIPResult
	pendLog *list.List
}

func NewIPSetter() IPSetter {
	ipinput := ipinput.New()
	ipinput.Prompt = "IP Address"
	ipinput.Style = ipinput.Style.Border(lipgloss.NormalBorder()).Margin(1).Padding(0, 1)
	ipinput.FocusedForeground = styles.Primary()
	ipinput.Focus()
	return IPSetter{
		state:   0,
		hwaddr:  nil,
		txid:    0,
		ipinput: ipinput,
		pendLog: list.New(),
	}
}

func (m IPSetter) Init() tea.Cmd {
	return tea.Batch(
		m.ipinput.Init(),
	)
}

func (m IPSetter) updateIP(msg tea.Msg) (IPSetter, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case ipinput.Msg:
		switch msg.String() {
		case "done":
			if m.ipinput.IsValid() {
				m.state = 1
				return m, m.SetIP
			}
		}
	}
	m.ipinput, cmd = m.ipinput.Update(msg)
	return m, cmd
}

func (m IPSetter) updatePending(msg tea.Msg) (IPSetter, tea.Cmd) {
	switch msg := msg.(type) {
	case SetIPResult:
		m.result = msg
		// m.state = 2
		m.Log("Press q to quit...")

	case SetIPLogMsg:
		m.pendLog.Item(msg.String())
	}
	return m, nil
}

func (m IPSetter) updateResult(msg tea.Msg) (IPSetter, tea.Cmd) {
	switch msg := msg.(type) {
	case SetIPResult:
		m.result = msg
	}
	return m, nil
}

func (m IPSetter) Update(msg tea.Msg) (IPSetter, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case 0:
		return m.updateIP(msg)
	case 1:
		return m.updatePending(msg)
	case 2:
		return m.updateResult(msg)
	}
	return m, cmd
}

func (m IPSetter) viewInfo() string {
	var s strings.Builder
	s.WriteString("Mac Address: ")
	s.WriteString(
		lipgloss.NewStyle().
			Foreground(styles.Secondary()).
			Render(m.hwaddr.String()),
	)
	s.WriteString("\n")
	return s.String()
}

func (m IPSetter) viewIPInput() string {
	var s strings.Builder
	s.WriteString(m.viewInfo())
	s.WriteString(m.ipinput.View())
	s.WriteString("\n\n")
	return s.String()
}

func (m IPSetter) viewPending() string {
	var s strings.Builder
	s.WriteString(m.viewInfo())
	s.WriteString("Setting IP to: ")
	s.WriteString(
		lipgloss.NewStyle().
			Foreground(styles.Primary()).
			Render(m.ipinput.Value().String()),
	)
	s.WriteString("\n\n")
	s.WriteString(m.pendLog.String())
	// s.WriteString("Sending DHCP Offer...")
	return s.String()
}

func (m IPSetter) viewResult() string {
	var s strings.Builder
	s.WriteString(m.viewInfo())
	s.WriteString("Setting IP to: ")
	s.WriteString(
		lipgloss.NewStyle().
			Foreground(styles.Primary()).
			Render(m.ipinput.Value().String()),
	)
	s.WriteString("\n\n")
	s.WriteString("Result: ")
	switch m.result.err {
	case nil:
		s.WriteString(
			lipgloss.NewStyle().
				Foreground(styles.Success()).
				Render("Success"),
		)
	default:
		s.WriteString(
			lipgloss.NewStyle().
				Foreground(styles.Danger()).
				Render(fmt.Sprintf("Error: %v", m.result.err)),
		)
	}
	return s.String()
}

func (m IPSetter) View() string {
	switch m.state {
	case 0:
		return m.viewIPInput()
	case 1:
		return m.viewPending()
	case 2:
		return m.viewResult()
	}
	return ""
}

func (m *IPSetter) SetHwAddr(hwaddr net.HardwareAddr) {
	m.hwaddr = hwaddr
}

func (m *IPSetter) SetTXID(txid uint32) {
	m.txid = txid
}

func (m *IPSetter) SetIP() tea.Msg {
	return SetIPRequest{
		IP:  m.ipinput.Value(),
		MAC: m.hwaddr,
		XID: m.txid,
	}
}

type SetIPRequest struct {
	IP  net.IP
	MAC net.HardwareAddr
	XID uint32
}

type SetIPResult struct {
	err error
}

type SetIPLogMsg struct {
	tstamp time.Time
	msg    string
}

func NewSetIPLogMsg(msg string) SetIPLogMsg {
	return SetIPLogMsg{
		tstamp: time.Now(),
		msg:    msg,
	}
}

func (m SetIPLogMsg) String() string {
	return fmt.Sprintf(
		"%s | %s",
		m.tstamp.Local().Format("15:04:05"),
		m.msg,
	)
}

func (m *IPSetter) Log(msg string) {
	m.pendLog.Item(NewSetIPLogMsg(msg))
}
