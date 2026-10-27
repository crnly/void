package main

import (
	"fmt"
	"os"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const help = "Ctrl+C quit · Notes are not saved yet"

var (
	accent       = lipgloss.Color("#88AABB")
	muted        = lipgloss.Color("#788896")
	titleStyle   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sectionStyle = lipgloss.NewStyle().Foreground(accent)
	helpStyle    = lipgloss.NewStyle().Foreground(muted)
	pageStyle    = lipgloss.NewStyle().Padding(1, 2)
)

// model holds the editor state and the current terminal size.
type model struct {
	notes  textarea.Model
	width  int
	height int
}

func initialModel() model {
	notes := textarea.New()
	notes.Placeholder = "Write a note…"
	notes.Prompt = ""
	notes.ShowLineNumbers = false
	notes.CharLimit = 0
	notes.MaxHeight = 0 // Allow notes with more lines than the visible area.
	notes.MaxWidth = 0

	styles := notes.Styles()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(muted)
	styles.Cursor.Color = accent
	notes.SetStyles(styles)
	notes.Focus()

	return model{notes: notes}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles quitting and resizing, then passes events to the textarea.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.compact() {
			m.notes.SetWidth(max(1, m.width))
			m.notes.SetHeight(max(1, m.height))
		} else {
			m.notes.SetWidth(m.width - pageStyle.GetHorizontalFrameSize())
			// Two heading rows, one help row, and the page padding.
			m.notes.SetHeight(m.height - 3 - pageStyle.GetVerticalFrameSize())
		}
	}

	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	return m, cmd
}

// On very small terminals, give all available space to the note itself.
func (m model) compact() bool {
	return m.width < 9 || m.height < 6
}

func (m model) View() tea.View {
	content := m.notes.View()
	if !m.compact() {
		content = pageStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Void"),
			sectionStyle.Render("Notes"),
			content,
			helpStyle.MaxWidth(m.width-pageStyle.GetHorizontalFrameSize()).Render(help),
		))
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not start Void:", err)
		os.Exit(1)
	}
}
