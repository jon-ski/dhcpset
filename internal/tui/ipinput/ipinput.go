package ipinput

import (
	"fmt"
	"net"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type (
	errMsg error
)

// Config holds configuration options for the IP input component
type Config struct {
	Prompt            string
	Style             lipgloss.Style
	FocusedForeground lipgloss.Color
	Placeholder       string
}

// DefaultConfig returns a default configuration for the IP input
func DefaultConfig() Config {
	return Config{
		Prompt:            "",
		Style:             lipgloss.NewStyle(),
		FocusedForeground: lipgloss.Color("205"),
		Placeholder:       "0",
	}
}

type Model struct {
	inputs            []OctetInput
	focused           int
	err               error
	Prompt            string
	Style             lipgloss.Style
	FocusedForeground lipgloss.Color
	isFocused         bool
}

func New() Model {
	return NewWithConfig(DefaultConfig())
}

func NewWithConfig(config Config) Model {
	var inputs []OctetInput = make([]OctetInput, 4)
	for i := range inputs {
		inputs[i] = NewOctetInput()
		inputs[i].input.Placeholder = config.Placeholder
	}

	m := Model{
		inputs:            inputs,
		focused:           0,
		isFocused:         false,
		err:               nil,
		Prompt:            config.Prompt,
		Style:             config.Style,
		FocusedForeground: config.FocusedForeground,
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd = make([]tea.Cmd, len(m.inputs))

	switch msg := msg.(type) {
	case tea.KeyMsg:
		consume := false
		switch msg.String() {
		case "tab", "right", ".":
			m.nextInput()
			consume = true
		case "shift+tab", "left":
			m.prevInput()
			consume = true
		case "enter":
			if m.IsValid() && m.focused == len(m.inputs)-1 {
				m.inputs[m.focused].Blur()
				return m, m.Cmd("done")
			}
			m.nextInput()

		case "backspace":
			if m.inputs[m.focused].Empty() {
				m.prevInput()
				if m.inputs[m.focused].Empty() {
					break
				}
				v := m.inputs[m.focused].input.Value()
				if len(v) > 0 {
					m.inputs[m.focused].input.SetValue(v[:len(v)-1])
				}
			}
		}
		if msg.String()[0] >= '0' && msg.String()[0] <= '9' {
			if m.inputs[m.focused].Done() {
				m.nextInput()
			}
		}

		for i := range m.inputs {
			m.inputs[i].Blur()
		}
		if m.isFocused {
			m.inputs[m.focused].Focus()
		}

		// Do not propagate navigation keys to child inputs to avoid inserting characters like '0'
		if consume {
			for i := range m.inputs {
				m.inputs[i], _ = m.inputs[i].Update(nil)
			}
			return m, nil
		}

	// We handle errors just like any other message
	case errMsg:
		m.err = msg
		return m, nil
	}

	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	promptStr := ""
	if m.Prompt != "" {
		promptStr = m.Prompt + ": "
	}
	for i := range m.inputs {
		m.inputs[i].FocusedForeground = m.FocusedForeground
	}
	inputStr := fmt.Sprintf(
		"%s.%s.%s.%s",
		m.inputs[0].View(),
		m.inputs[1].View(),
		m.inputs[2].View(),
		m.inputs[3].View(),
	)
	style := m.Style
	if m.Focused() {
		style = style.BorderForeground(m.FocusedForeground)
	}

	// contStr := "Continue ->"
	// contStyle := lipgloss.NewStyle().Faint(true).Margin(1).Padding(1).Background(lipgloss.Color(888))
	// if m.IsValid() {
	// 	contStyle = contStyle.Background(styles.Success()).UnsetFaint()
	// }
	// contStr = contStyle.Render(contStr)

	return style.Render(promptStr + inputStr)
}

// nextInput focuses the next input field
func (m *Model) nextInput() {
	if m.focused < len(m.inputs)-1 {
		m.focused++
		return
	}
}

// prevInput focuses the previous input field
func (m *Model) prevInput() {
	if m.focused > 0 {
		m.focused--
		return
	}
}

func (m *Model) IsDone() bool {
	for i := range m.inputs {
		if !m.inputs[i].Done() {
			return false
		}
	}
	return true
}

func (m *Model) IsValid() bool {
	// First check basic octet validation
	for i := range m.inputs {
		if !m.inputs[i].IsValid() {
			return false
		}
	}

	// Additional network-aware validation
	ip := m.Value()
	return m.isNetworkValid(ip)
}

// isNetworkValid performs network-aware validation
func (m *Model) isNetworkValid(ip net.IP) bool {
	// Check if IP is in valid range (not 0.0.0.0, 255.255.255.255, etc.)
	if ip.IsUnspecified() {
		return false
	}

	if ip.IsMulticast() {
		return false
	}

	if ip.IsLoopback() {
		return false
	}

	// Check for broadcast addresses
	if ip.Equal(net.IPv4(255, 255, 255, 255)) {
		return false
	}

	// Check for link-local addresses (169.254.x.x)
	if ip.To4()[0] == 169 && ip.To4()[1] == 254 {
		return false
	}

	// Check for reserved addresses (224.0.0.0 - 239.255.255.255)
	if ip.To4()[0] >= 224 && ip.To4()[0] <= 239 {
		return false
	}

	return true
}

type Msg string

func (m Msg) String() string {
	return string(m)
}

func (m Model) Cmd(s string) tea.Cmd {
	return func() tea.Msg {
		return Msg(s)
	}
}

func (m *Model) Focus() {
	m.isFocused = true
}

func (m *Model) Blur() {
	m.isFocused = false
}

func (m *Model) Focused() bool {
	return m.isFocused
}

func (m *Model) Value() net.IP {
	return net.IPv4(
		byte(m.inputs[0].Value()),
		byte(m.inputs[1].Value()),
		byte(m.inputs[2].Value()),
		byte(m.inputs[3].Value()),
	)
}
