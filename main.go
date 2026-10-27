package main

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	notesHelp = "Tab timer · Ctrl+S save · Ctrl+C save & quit"
	timerHelp = "Space start/pause · R reset · Tab/Esc notes · Ctrl+S save · Ctrl+C save & quit"
)

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
	notes     textarea.Model
	width     int
	height    int
	path      string
	savedText string
	loadErr   error
	saveErr   error
	timer     focusTimer
	now       func() time.Time
}

func initialModel(path string, pathErr error) model {
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
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	styles.Cursor.Color = accent
	notes.SetStyles(styles)
	notes.Focus()

	m := model{notes: notes, path: path, loadErr: pathErr, timer: newFocusTimer(), now: time.Now}
	if m.loadErr == nil {
		m.savedText, m.loadErr = readNotes(path)
		if m.loadErr == nil {
			m.notes.SetValue(m.savedText)
			// Never overwrite content that the textarea would silently normalize.
			if m.notes.Value() != m.savedText {
				m.loadErr = fmt.Errorf("the file contains characters the editor cannot preserve")
			}
		}
	}
	return m
}

func (m model) status() string {
	if m.loadErr != nil {
		return "Could not load notes; saving is disabled to protect the file: " + m.loadErr.Error()
	}
	if m.saveErr != nil {
		return "Could not save notes: " + m.saveErr.Error()
	}
	if m.notes.Value() != m.savedText {
		return "Unsaved changes"
	}
	return "Saved"
}

func (m *model) save() bool {
	if m.loadErr != nil {
		return false
	}
	m.saveErr = writeNotes(m.path, m.notes.Value())
	if m.saveErr != nil {
		return false
	}
	m.savedText = m.notes.Value()
	return true
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

// Update routes keys to the focused area; saving and quitting work everywhere.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+s" || msg.String() == "ctrl+c" {
			if m.save() && msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "tab":
			if m.notes.Focused() {
				m.notes.Blur()
				return m, nil
			}
			cmd := m.notes.Focus()
			return m, cmd
		case "esc":
			if !m.notes.Focused() {
				cmd := m.notes.Focus()
				return m, cmd
			}
			return m, nil
		}
		if !m.notes.Focused() {
			switch msg.String() {
			case "space":
				cmd := m.timer.toggle(m.now())
				return m, cmd
			case "r", "R":
				m.timer.reset()
			}
			return m, nil
		}
	case timerTickMsg:
		cmd := m.timer.tick(msg.id, m.now())
		return m, cmd
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.compact() {
			m.notes.SetWidth(max(1, m.width))
			// Keep timer, status, and help visible as space permits.
			rows := min(3, max(0, m.height-1))
			m.notes.SetHeight(max(1, m.height-rows))
		} else {
			m.notes.SetWidth(m.width - pageStyle.GetHorizontalFrameSize())
			// Two headings, timer, status, help, and the page padding.
			m.notes.SetHeight(m.height - 5 - pageStyle.GetVerticalFrameSize())
		}
	}

	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	return m, cmd
}

// Small terminals omit the headings and padding.
func (m model) compact() bool {
	return m.width < 9 || m.height < 8
}

func (m model) help() string {
	if m.notes.Focused() {
		return notesHelp
	}
	return timerHelp
}

func (m model) View() tea.View {
	content := m.notes.View()
	notesStyle, timerStyle := helpStyle, sectionStyle
	if m.notes.Focused() {
		notesStyle, timerStyle = sectionStyle, helpStyle
	}
	if m.compact() {
		line := helpStyle.MaxWidth(max(1, m.width))
		if m.height > 1 {
			content += "\n" + timerStyle.MaxWidth(max(1, m.width)).Render(m.timer.view())
		}
		if m.height > 2 {
			content += "\n" + line.Render(m.status())
		}
		if m.height > 3 {
			content += "\n" + line.Render(m.help())
		}
	} else {
		width := m.width - pageStyle.GetHorizontalFrameSize()
		content = pageStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render("Void"),
			notesStyle.Render("Notes"),
			content,
			timerStyle.MaxWidth(width).Render(m.timer.view()),
			helpStyle.MaxWidth(width).Render(m.status()),
			helpStyle.MaxWidth(width).Render(m.help()),
		))
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func main() {
	path, pathErr := notesPath()
	if _, err := tea.NewProgram(initialModel(path, pathErr)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not start Void:", err)
		os.Exit(1)
	}
}
