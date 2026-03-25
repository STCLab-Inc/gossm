package tui

import "github.com/charmbracelet/lipgloss"

var (
	activeBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("39")) // blue

	inactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")) // gray

	titleActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("39"))

	titleInactiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("240"))

	cursorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("255"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")) // pink

	dirEntryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	fileEntryStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // green

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")) // red

	loadingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")) // orange

	pathEditStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255"))

	filterStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)
)
