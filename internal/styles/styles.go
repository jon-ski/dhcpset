package styles

import "github.com/charmbracelet/lipgloss"

func Right(lg lipgloss.Style) lipgloss.Style {
	return lg.Align(lipgloss.Right)
}

func Left(lg lipgloss.Style) lipgloss.Style {
	return lg.Align(lipgloss.Left)
}

func Center(lg lipgloss.Style) lipgloss.Style {
	return lg.Align(lipgloss.Center)
}

func Disabled(lg lipgloss.Style) lipgloss.Style {
	return lg.Foreground(lipgloss.Color("#767676"))
}

func Default() lipgloss.Style {
	return lipgloss.NewStyle()
}

var light = lipgloss.Color("#b2bec3")
var primary = lipgloss.Color("#0984e3")
var secondary = lipgloss.Color("#74b9ff")
var success = lipgloss.Color("#00b894")
var warning = lipgloss.Color("#fdcb6e")
var danger = lipgloss.Color("#d63031")

func Light() lipgloss.Color     { return light }
func Primary() lipgloss.Color   { return primary }
func Secondary() lipgloss.Color { return secondary }
func Success() lipgloss.Color   { return success }
func Warning() lipgloss.Color   { return warning }
func Danger() lipgloss.Color    { return danger }
