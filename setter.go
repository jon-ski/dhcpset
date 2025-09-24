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
	"github.com/jon-ski/dhcpset/pkg/dhcp"
)

type IPSetter struct {
	state         int
	hwaddr        net.HardwareAddr
	txid          uint32
	ipinput       ipinput.Model
	result        SetIPResult
	pendLog       *list.List
	progress      dhcp.ProgressUpdate
	timeRemaining time.Duration
}

func NewIPSetter() IPSetter {
	config := ipinput.DefaultConfig()
	config.Prompt = "IP Address"
	config.Style = config.Style.Border(lipgloss.NormalBorder()).Margin(1).Padding(0, 1)
	config.FocusedForeground = styles.Primary()

	ipinput := ipinput.NewWithConfig(config)
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
		m.state = 2 // Move to result state
		m.Log("Press q to quit...")

	case SetIPLogMsg:
		m.pendLog.Item(msg.String())

	case dhcp.ProgressUpdate:
		m.progress = msg
		m.timeRemaining = msg.TimeRemaining
		// Don't add progress messages to the log to avoid duplication
	}
	return m, nil
}

func (m IPSetter) updateResult(msg tea.Msg) (IPSetter, tea.Cmd) {
	switch msg := msg.(type) {
	case SetIPResult:
		m.result = msg
	case tea.KeyMsg:
		switch msg.String() {
		case "r", "R":
			if m.result.err != nil {
				// Reset to input state for retry
				m.state = 0
				m.result = SetIPResult{}
				m.progress = dhcp.ProgressUpdate{}
				m.timeRemaining = 0
				// Clear the pending log for a fresh start
				m.pendLog = list.New()
				m.ipinput.Focus()
			}
		}
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
	s.WriteString(m.renderTips())
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

	// Show current status message prominently
	if m.progress.Message != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(styles.Secondary()).
			Bold(true).
			Margin(1, 0).
			Padding(1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(styles.Secondary())
		s.WriteString(statusStyle.Render(m.progress.Message))
		s.WriteString("\n\n")
	}

	// Show progress bar
	if m.progress.Progress > 0 {
		s.WriteString(m.renderProgressBar())
		s.WriteString("\n\n")
	}

	// Show countdown timer
	if m.timeRemaining > 0 {
		s.WriteString(m.renderCountdown())
		s.WriteString("\n\n")
	}

	// Show any additional log messages (if any)
	logContent := m.pendLog.String()
	if strings.TrimSpace(logContent) != "" {
		s.WriteString("Details:\n")
		s.WriteString(logContent)
	}
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

	// Show user-friendly result message
	if m.result.Message != "" {
		resultStyle := lipgloss.NewStyle().
			Margin(1, 0).
			Padding(1).
			Border(lipgloss.NormalBorder())

		if m.result.err == nil {
			resultStyle = resultStyle.
				Foreground(styles.Success()).
				BorderForeground(styles.Success())
		} else {
			resultStyle = resultStyle.
				Foreground(styles.Danger()).
				BorderForeground(styles.Danger())
		}

		s.WriteString(resultStyle.Render(m.result.Message))
		s.WriteString("\n\n")
	}

	// Show technical details if there's an error
	if m.result.err != nil {
		s.WriteString("Technical details: ")
		s.WriteString(
			lipgloss.NewStyle().
				Foreground(styles.Danger()).
				Render(fmt.Sprintf("%v", m.result.err)),
		)
		s.WriteString("\n\n")
	}

	// Show action buttons
	if m.result.err != nil {
		s.WriteString("Press 'r' to retry or 'q' to quit...")
	} else {
		s.WriteString("Press 'q' to quit...")
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

// Use types from dhcp package
type SetIPRequest = dhcp.SetIPRequest
type SetIPLogMsg = dhcp.SetIPLogMsg

func NewSetIPLogMsg(msg string) SetIPLogMsg {
	return dhcp.NewSetIPLogMsg(msg)
}

type SetIPResult struct {
	err     error
	Message string
}

func (m *IPSetter) Log(msg string) {
	m.pendLog.Item(NewSetIPLogMsg(msg))
}

// renderProgressBar creates a visual progress bar
func (m IPSetter) renderProgressBar() string {
	width := 30
	filled := int(float64(width) * float64(m.progress.Progress) / 100.0)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	progressStyle := lipgloss.NewStyle().
		Foreground(styles.Primary()).
		Bold(true)

	return progressStyle.Render(fmt.Sprintf("[%s] %d%%", bar, m.progress.Progress))
}

// renderCountdown creates a countdown timer display
func (m IPSetter) renderCountdown() string {
	seconds := int(m.timeRemaining.Seconds())

	countdownStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary()).
		Italic(true)

	if seconds > 0 {
		return countdownStyle.Render(fmt.Sprintf("⏰ Time remaining: %d seconds", seconds))
	}
	return ""
}

// renderTips shows helpful tips for the user
func (m IPSetter) renderTips() string {
	tips := []string{
		"💡 Tips:",
		"• Make sure your device is powered on and connected",
		"• The device should be on the same network as this computer",
		"• Most devices respond within 1-2 seconds",
		"• If it takes longer, the device might be busy or slow",
	}

	tipStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary()).
		Margin(1, 0).
		Border(lipgloss.NormalBorder()).
		BorderForeground(styles.Secondary()).
		Padding(1)

	return tipStyle.Render(strings.Join(tips, "\n"))
}
