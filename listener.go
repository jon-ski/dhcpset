package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/jon-ski/dhcpset/internal/styles"
)

type listenModel struct {
	list      []discoverInfo
	selection int
	value     discoverInfo

	spinner spinner.Model
}

func newListenModel() listenModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return listenModel{
		list:      []discoverInfo{},
		selection: 0,
		value:     discoverInfo{},
		spinner:   s,
	}
}

func (m listenModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
	)
}

func (m listenModel) Update(msg tea.Msg) (listenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "k", "up":
			if m.selection > 0 {
				m.selection--
			}

		case "j", "down":
			if m.selection < len(m.list)-1 {
				m.selection++
			}

		case "enter":
			m.value = m.list[m.selection]
			return m, selectMacCommand(m.value)
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

func (m listenModel) ListEnumerator() list.Enumerator {
	return func(l list.Items, i int) string {
		if m.selection == i {
			return "Use →"
		}
		return ""
	}
}

func (m listenModel) ViewItem(i int) string {
	if i < 0 || i >= len(m.list) {
		return ""
	}
	return fmt.Sprintf(
		"%s %08x",
		m.list[i].hwaddr.String(),
		m.list[i].xid,
	)
}

var listenEnumeratorStyle = lipgloss.NewStyle().
	Foreground(styles.Primary()).
	MarginRight(1)

func (m listenModel) ViewList() string {
	l := list.New()
	for i := range m.list {
		l.Item(m.ViewItem(i))
	}
	l.Enumerator(m.ListEnumerator())
	l.EnumeratorStyle(listenEnumeratorStyle)
	return l.String()
}

func (m listenModel) View() string {
	// style := lipgloss.NewStyle().
	// 	Margin(1, 1).
	// 	Width(60)

	// var s string
	// for i, item := range m.list {
	// 	selStr := " "
	// 	itemStr := item.hwaddr.String() + " " + fmt.Sprintf("%x", item.xid) + "\n"
	// 	if i == m.selection {
	// 		selStr = ">"
	// 	}
	// 	s += selStr + " " + itemStr
	// }

	var s strings.Builder
	items := m.ViewList()
	s.WriteString(items)
	if len(items) != 0 {
		s.WriteString("\n\n")
	}
	s.WriteString(m.spinner.View())
	s.WriteString("Listening for DHCP discover packets...")

	return s.String()
}

type discoverInfoSelection discoverInfo

func selectMacCommand(value discoverInfo) tea.Cmd {
	return func() tea.Msg {
		return discoverInfoSelection(value)
	}
}
