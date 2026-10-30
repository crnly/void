package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func press(m model, key string) (model, tea.Cmd) {
	code := 's'
	if key == "ctrl+c" {
		code = 'c'
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl})
	return next.(model), cmd
}

func edit(m model, text string) model {
	next, _ := m.Update(tea.PasteMsg{Content: text})
	return next.(model)
}

func autosave(m model) model {
	next, _ := m.Update(autosaveMsg{id: m.saveID})
	return next.(model)
}

func TestLoadAutosaveAndQuit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "void", "notes.md")
	original := "  Grüße!  \n\nÄÖÜ äöü ß\n"
	if err := writeNotes(path, original); err != nil {
		t.Fatal(err)
	}
	m := initialModel(path, nil)
	if !m.notes.Focused() || m.notes.Value() != original || m.status() != "" {
		t.Fatalf("unexpected initial state: %q, %s", m.notes.Value(), m.status())
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = next.(model)
	if cmd == nil || !strings.Contains(m.notes.Value(), "q") || m.saveID == 0 {
		t.Fatal("typing must edit and schedule autosave")
	}
	if got, _ := readNotes(path); got != original {
		t.Fatal("typing must wait for the writing pause")
	}
	m = autosave(m)
	if got, _ := readNotes(path); got != m.notes.Value() {
		t.Fatal("autosave did not write current text")
	}
	m = edit(m, "\nlast line  \n")
	pending := m.saveID
	m, cmd = press(m, "ctrl+c")
	if cmd == nil {
		t.Fatal("Ctrl+C should quit after saving without waiting for debounce")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit command")
	}
	restored := initialModel(path, nil)
	if restored.notes.Value() != m.notes.Value() || restored.loadErr != nil {
		t.Fatalf("restart lost text: %q, %v", restored.notes.Value(), restored.loadErr)
	}
	next, cmd = m.Update(autosaveMsg{id: pending})
	if cmd != nil || next.(model).saveID != m.saveID {
		t.Fatal("pending autosave must be discarded after final save")
	}
}

func TestAutosaveDiscardsStaleAndDuplicateMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	m := edit(initialModel(path, nil), "old")
	oldID := m.saveID
	m = edit(m, " newest\n")
	newID := m.saveID
	for _, msg := range []autosaveMsg{{oldID}, {newID}, {oldID}, {newID}} {
		next, _ := m.Update(msg)
		m = next.(model)
		got, err := readNotes(path)
		want := m.notes.Value()
		if msg.id == oldID && m.saveID == newID {
			want = "" // The first timeout is stale: no write before the latest pause.
		}
		if err != nil || got != want {
			t.Fatalf("autosave %d wrote %q, want %q (%v)", msg.id, got, want, err)
		}
	}
}

func TestCtrlSDoesNotSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	m := edit(initialModel(path, nil), "pending")
	m, _ = press(m, "ctrl+s")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("Ctrl+S must not manually save")
	}
	m = autosave(m)
	if got, _ := readNotes(path); got != "pending" {
		t.Fatal("Ctrl+S must not interfere with autosave")
	}
}

func TestMissingNotesStartEmpty(t *testing.T) {
	m := initialModel(filepath.Join(t.TempDir(), "void", "notes.md"), nil)
	if m.loadErr != nil || m.notes.Value() != "" {
		t.Fatalf("missing file should open empty: %s", m.status())
	}
}

func TestReadErrorBlocksSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	m := initialModel(path, nil)
	if m.loadErr == nil {
		t.Fatal("reading a directory should fail")
	}
	// Make the path writable after the failed load: saving must still be blocked.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("precious notes"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ctrl+c"} {
		var cmd tea.Cmd
		m, cmd = press(m, key)
		if cmd != nil || !strings.Contains(m.status(), "saving is disabled") {
			t.Fatalf("%s must stay open with a load error: %s", key, m.status())
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "precious notes" {
		t.Fatalf("existing note overwritten: %q, %v", got, err)
	}
}

func TestSaveFailureAndAutomaticRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "void", "notes.md")
	m := edit(initialModel(path, nil), "unsaved Grüße\n")
	if err := os.WriteFile(filepath.Dir(path), []byte("blocks directory"), 0600); err != nil {
		t.Fatal(err)
	}
	m = autosave(m)
	m, cmd := press(m, "ctrl+c")
	if cmd != nil || !strings.Contains(m.status(), "Could not save notes") || m.notes.Value() != "unsaved Grüße\n" {
		t.Fatal("failed autosave and quit must display error, retain text and stay open")
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	m = edit(m, "recovered")
	if m.status() == "" {
		t.Fatal("editing must not hide an unresolved error")
	}
	m = autosave(m)
	if got, err := readNotes(path); err != nil || got != m.notes.Value() || m.status() != "" {
		t.Fatalf("automatic retry failed: %q, %v, %s", got, err, m.status())
	}
}

func TestConfigErrorBlocksSaving(t *testing.T) {
	m := initialModel("", errors.New("config directory is unavailable"))
	m, cmd := press(m, "ctrl+c")
	if cmd != nil || !strings.Contains(m.status(), "config directory is unavailable") {
		t.Fatalf("config error should block saving and quitting: %s", m.status())
	}
}

func TestLoadedTextCannotBeSilentlyNormalized(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	original := "Grüße\twith tabs\r\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	m := initialModel(path, nil)
	// If the component normalizes this input, the original must stay protected.
	if m.notes.Value() != original && m.loadErr == nil {
		t.Fatal("normalized input must disable saving")
	}
	m = autosave(m)
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Fatalf("original characters changed: %q, %v", got, err)
	}
}

func TestResizeWithStorageStatus(t *testing.T) {
	m := initialModel(filepath.Join(t.TempDir(), "notes.md"), nil)
	for _, size := range [][2]int{{80, 24}, {36, 24}, {40, 10}, {9, 8}, {9, 7}, {8, 6}, {1, 1}, {160, 50}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(model)
		view := m.View().Content
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w > size[0] || h > size[1] {
			t.Fatalf("view %dx%d exceeds terminal %dx%d", w, h, size[0], size[1])
		}
		for _, removed := range []string{"Saved", "Unsaved changes", "Ctrl+S"} {
			if strings.Contains(view, removed) {
				t.Fatalf("manual save UI remains: %s", removed)
			}
		}
	}
}

func TestSidebarHelpFits(t *testing.T) {
	m := initialModel(filepath.Join(t.TempDir(), "notes.md"), nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 36, Height: 24})
	m = next.(model)
	view := m.View().Content
	if !strings.Contains(view, notesHelp) {
		t.Fatalf("sidebar help is clipped: %s", view)
	}
	if lipgloss.Width(view) > 36 || lipgloss.Height(view) > 24 {
		t.Fatal("help overflowed the sidebar")
	}
}

func TestNotesStayEditableAfterTabAndEscape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	m := initialModel(path, nil)
	for _, key := range []rune{tea.KeyTab, tea.KeyEscape} {
		next, _ := m.Update(tea.KeyPressMsg{Code: key})
		m = next.(model)
		if !m.notes.Focused() {
			t.Fatal("Tab and Esc must not leave the notes editor")
		}
		for _, r := range "q rR" {
			next, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			m = next.(model)
		}
		m = edit(m, " pasted")
	}
	if m.notes.Value() != "q rR pastedq rR pasted" {
		t.Fatalf("note input was lost: %q", m.notes.Value())
	}
	m = autosave(m)
	if got, err := readNotes(path); err != nil || got != m.notes.Value() {
		t.Fatalf("note input was not saved: %q, %v", got, err)
	}
}

func TestNormalQuitMessagesSaveLatestText(t *testing.T) {
	for _, msg := range []tea.Msg{tea.QuitMsg{}, tea.InterruptMsg{}} {
		path := filepath.Join(t.TempDir(), "notes.md")
		m := edit(initialModel(path, nil), "latest before shutdown\n")
		next, cmd := m.Update(saveBeforeQuit(m, msg))
		m = next.(model)
		if cmd == nil {
			t.Fatalf("%T did not flush and quit", msg)
		}
		if _, ok := saveBeforeQuit(m, cmd()).(tea.QuitMsg); !ok {
			t.Fatal("successful final save must allow termination")
		}
		if got, err := readNotes(path); err != nil || got != m.notes.Value() {
			t.Fatalf("shutdown lost text: %q, %v", got, err)
		}
	}
}

func TestAutosaveEmptyNoteAndLoadProtection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	m := edit(initialModel(path, nil), "delete me")
	m = autosave(m)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	m = next.(model)
	// Clear through the textarea's normal delete input.
	for remaining := len(m.notes.Value()); m.notes.Value() != ""; remaining-- {
		if remaining == 0 {
			t.Fatal("delete did not clear note")
		}
		next, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
		m = next.(model)
		if cmd == nil && m.notes.Value() != "" {
			t.Fatal("delete should schedule autosave")
		}
	}
	m = autosave(m)
	if got, err := readNotes(path); err != nil || got != "" {
		t.Fatalf("empty note not saved: %q, %v", got, err)
	}
	m = initialModel(path, errors.New("cannot load"))
	m = edit(m, "protected")
	m = autosave(m)
	if got, _ := readNotes(path); got != "" {
		t.Fatal("autosave bypassed load protection")
	}
}

func TestQuitFailureThroughFilterRetainsText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocked", "notes.md")
	m := edit(initialModel(path, nil), "keep this")
	if err := os.WriteFile(filepath.Dir(path), []byte("block"), 0600); err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(saveBeforeQuit(m, tea.QuitMsg{}))
	m = next.(model)
	if cmd != nil || m.quitting || m.notes.Value() != "keep this" || m.saveErr == nil {
		t.Fatal("normal quit must retain text and show failed save")
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	m, cmd = press(m, "ctrl+c")
	if cmd == nil || m.saveErr != nil {
		t.Fatal("quit must retry successfully after storage is fixed")
	}
	next, _ = m.Update(tea.PasteMsg{Content: "too late"})
	if next.(model).notes.Value() != m.notes.Value() {
		t.Fatal("shutdown must not accept edits after final save")
	}
}

func TestStorageErrorsVisibleAfterResize(t *testing.T) {
	m := initialModel(filepath.Join(t.TempDir(), "notes.md"), nil)
	m.saveErr = errors.New("disk full")
	for _, size := range [][2]int{{80, 24}, {36, 24}, {40, 10}, {9, 8}, {9, 7}, {8, 6}, {1, 1}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(model)
		view := m.View().Content
		if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
			t.Fatalf("error view exceeds terminal %v", size)
		}
		if size[0] >= 36 && !strings.Contains(view, "Could not save notes") {
			t.Fatal("save error must remain visible")
		}
	}
}
