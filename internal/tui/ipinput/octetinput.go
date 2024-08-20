package ipinput

import (
	"fmt"
	"math"
	"strconv"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon-ski/dhcpset/internal/styles"
)

type OctetInput struct {
	input             textinput.Model
	FocusedForeground lipgloss.Color
}

func NewOctetInput() OctetInput {
	input := textinput.New()
	input.Placeholder = "0"
	// input.PlaceholderStyle = input.PlaceholderStyle.Width(3)
	// input.TextStyle = input.TextStyle.Align(lipgloss.Right).Width(3)
	input.Cursor.SetChar("")
	input.Cursor.SetMode(cursor.CursorHide)
	input.CharLimit = 3
	// input.Width = 4
	input.Prompt = ""
	input.Validate = octetValidator

	return OctetInput{
		input:             input,
		FocusedForeground: styles.Primary(),
	}
}

func (o *OctetInput) Focus() {
	o.input.Focus()
	o.input.SetCursor(len(o.input.Value()))
}

func (o *OctetInput) Blur() {
	o.input.Blur()
}

func (o *OctetInput) Focused() bool {
	return o.input.Focused()
}

func (o *OctetInput) Value() uint8 {
	v, err := strconv.ParseUint(o.input.Value(), 10, 8)
	if err != nil {
		return 0
	}
	if v > math.MaxUint8 {
		v = math.MaxUint8
	}
	return uint8(v)
}

func (o OctetInput) Init() tea.Cmd {
	return textinput.Blink
}

func (o OctetInput) Update(msg tea.Msg) (OctetInput, tea.Cmd) {
	var cmd tea.Cmd
	o.input, cmd = o.input.Update(msg)
	return o, cmd
}

var defaultStyle = lipgloss.NewStyle().UnderlineSpaces(true)

var placeholderStyle = defaultStyle.Faint(true)

func focusedStyle(st lipgloss.Style) lipgloss.Style {
	return st.Bold(true)
}

func (o OctetInput) View() string {
	// Format the value so it has leading underscores
	// value := fmt.Sprintf("%03d", o.Value())
	value := fmt.Sprintf("%3d", o.Value())
	if o.Empty() {
		value = "   "
	}

	// return lipgloss.NewStyle().Foreground(styles.Primary()).Width(4).Align(lipgloss.Right).Render(o.input.View())
	style := defaultStyle
	if o.Empty() {
		style = placeholderStyle
	}
	if o.Focused() {
		style = focusedStyle(style)
		style = style.Foreground(o.FocusedForeground)
	}
	return style.Render(value)
	// return o.input.View()
}

func (o OctetInput) Empty() bool {
	return o.input.Value() == ""
}

func (o OctetInput) Done() bool {
	// If len == 3, we're done
	return len(o.input.Value()) == 3
}

func octetValidator(s string) error {
	// An octet should be a number between 0 and 255
	o, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("octet is invalid")
	}

	if o < 0 || o > 255 {
		return fmt.Errorf("octet is invalid")
	}

	return nil
}

type octetMsg string

func (o octetMsg) String() string {
	return string(o)
}

func (o OctetInput) Cmd(s string) tea.Cmd {
	return func() tea.Msg {
		return octetMsg(s)
	}
}

func (o OctetInput) IsValid() bool {
	if o.input.Err != nil {
		return false
	}
	if o.Empty() {
		return false
	}
	return true
}
