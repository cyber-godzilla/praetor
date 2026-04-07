package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorOrange   = lipgloss.Color("#e8a838")
	colorGreen    = lipgloss.Color("#55cc55")
	colorRed      = lipgloss.Color("#cc4444")
	colorBlue     = lipgloss.Color("#88aaff")
	colorDim      = lipgloss.Color("#888888")
	colorBG       = lipgloss.Color("#0a0a14")
	colorBorder   = lipgloss.Color("#444444")
	colorBarEmpty = lipgloss.Color("#333333")

	activeTabStyle = lipgloss.NewStyle().
			Background(colorOrange).Foreground(lipgloss.Color("#0a0a14")).Bold(true).Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(colorDim).Padding(0, 1)
	tabBarStyle      = lipgloss.NewStyle().
				BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorBorder)
	statusBarStyle = lipgloss.NewStyle().
			BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorBorder)
	sidebarStyle = lipgloss.NewStyle().
			BorderLeft(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorBorder)
	inputStyle = lipgloss.NewStyle().
			BorderTop(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(colorBorder)
	promptStyle = lipgloss.NewStyle().Foreground(colorOrange)
)
