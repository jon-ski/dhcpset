package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func (m listenModel) View() string {
	style := lipgloss.NewStyle().
		Margin(1, 1).
		Width(60)

	var s string
	for i, item := range m.list {
		selStr := " "
		itemStr := item.hwaddr.String() + " " + fmt.Sprintf("%x", item.xid) + "\n"
		if i == m.selection {
			selStr = ">"
		}
		s += selStr + " " + itemStr
	}
	s += "\n" + m.spinner.View() + " Listening for MAC discover packets..."

	return style.Render(s) + "\n"
}

type discoverInfoSelection discoverInfo

func selectMacCommand(value discoverInfo) tea.Cmd {
	return func() tea.Msg {
		return discoverInfoSelection(value)
	}
}
