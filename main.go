package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	notesHelp = "Tab timer · Ctrl+C quit"
	timerHelp = "Space start/pause · R reset · Tab/Esc notes · Ctrl+C quit"
)

var (
	accent       = lipgloss.Color("#88AABB")
	muted        = lipgloss.Color("#788896")
	titleStyle   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sectionStyle = lipgloss.NewStyle().Foreground(accent)
	helpStyle    = lipgloss.NewStyle().Foreground(muted)
	pageStyle    = lipgloss.NewStyle().Padding(1, 2)
)

const autosaveDelay = 500 * time.Millisecond

type autosaveMsg struct{ id uint64 }
type quitRequestMsg struct{}

// model holds the editor state and the current terminal size.
type model struct {
	notes     textarea.Model
	width     int
	height    int
	path      string
	savedText string
	saveID    uint64
	quitting  bool
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
	return ""
}

func (m *model) save() bool {
	if m.loadErr != nil {
		return false
	}
	if m.notes.Value() == m.savedText && m.saveErr == nil {
		return true
	}
	// Writes run only in Update, never in asynchronous commands. This serializes
	// autosave and the final save so an older write cannot finish after a newer one.
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

func (m *model) quit() tea.Cmd {
	m.saveID++ // Invalidate every pending autosave before the final write.
	if !m.save() {
		m.resizeNotes()
		return nil
	}
	m.quitting = true
	return tea.Quit
}

// Bubble Tea consumes quit/signal messages before Update. Route them through
// the same final save as Ctrl+C, and allow quitting only after it succeeds.
func saveBeforeQuit(current tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.QuitMsg, tea.InterruptMsg:
		if !current.(model).quitting {
			return quitRequestMsg{}
		}
	}
	return msg
}

// Update routes keys to the focused area; quitting works everywhere.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, nil // Do not accept edits between the final save and tea.Quit.
	}
	switch msg := msg.(type) {
	case quitRequestMsg:
		cmd := m.quit()
		return m, cmd
	case autosaveMsg:
		if msg.id != m.saveID || m.quitting {
			return m, nil
		}
		m.saveID++ // A delayed or duplicate message may save at most once.
		m.save()
		m.resizeNotes()
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			cmd := m.quit()
			return m, cmd
		}
		switch msg.String() {
		case "tab":
			if m.notes.Focused() {
				m.notes.Blur()
				m.resizeNotes()
				return m, nil
			}
			cmd := m.notes.Focus()
			m.resizeNotes()
			return m, cmd
		case "esc":
			if !m.notes.Focused() {
				cmd := m.notes.Focus()
				m.resizeNotes()
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
		m.resizeNotes()
	}

	before := m.notes.Value()
	var cmd tea.Cmd
	m.notes, cmd = m.notes.Update(msg)
	if m.notes.Value() != before {
		m.saveID++
		id := m.saveID
		if m.loadErr == nil {
			return m, tea.Batch(cmd, tea.Tick(autosaveDelay, func(time.Time) tea.Msg {
				return autosaveMsg{id: id}
			}))
		}
	}
	return m, cmd
}

func (m *model) resizeNotes() {
	statusRows := 0
	if m.status() != "" {
		statusRows = 1
	}
	if m.compact() {
		m.notes.SetWidth(max(1, m.width))
		// Keep timer, errors, and help visible as space permits.
		rows := min(2+statusRows, max(0, m.height-1))
		m.notes.SetHeight(max(1, m.height-rows))
	} else {
		m.notes.SetWidth(m.width - pageStyle.GetHorizontalFrameSize())
		// Headings, timer, optional error, wrapped help, and page padding.
		m.notes.SetHeight(m.height - 3 - statusRows - lipgloss.Height(m.helpView()) - pageStyle.GetVerticalFrameSize())
	}
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

func (m model) helpView() string {
	width := max(1, m.width-pageStyle.GetHorizontalFrameSize())
	// At least one editor row must remain, even on a short terminal.
	return helpStyle.Width(width).MaxWidth(width).MaxHeight(max(1, m.height-7)).Render(m.help())
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
		helpRow := 2
		if m.status() != "" && m.height > 2 {
			content += "\n" + line.Render(m.status())
			helpRow++
		}
		if m.height > helpRow {
			content += "\n" + line.Render(m.help())
		}
	} else {
		width := m.width - pageStyle.GetHorizontalFrameSize()
		rows := []string{
			titleStyle.Render("Void"),
			notesStyle.Render("Notes"),
			content,
			timerStyle.MaxWidth(width).Render(m.timer.view()),
		}
		if m.status() != "" {
			rows = append(rows, helpStyle.MaxWidth(width).Render(m.status()))
		}
		rows = append(rows, m.helpView())
		content = pageStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func runSidebar() error {
	path, pathErr := notesPath()
	_, err := tea.NewProgram(initialModel(path, pathErr), tea.WithFilter(saveBeforeQuit)).Run()
	return err
}

func main() {
	sidebar := flag.Bool("sidebar", false, "run only the notes editor and focus timer")
	flag.Parse()
	var err error
	if *sidebar {
		err = runSidebar()
	} else {
		err = runWorkspace()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not start Void:", err)
		os.Exit(1)
	}
}
