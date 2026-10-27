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

func TestLoadEditSaveAndQuit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "void", "notes.md")
	original := "  Grüße!  \n\nÄÖÜ äöü ß\n"
	if err := writeNotes(path, original); err != nil {
		t.Fatal(err)
	}
	m := initialModel(path, nil)
	if !m.notes.Focused() || m.notes.Value() != original || m.status() != "Saved" {
		t.Fatalf("unexpected initial state: %q, %s", m.notes.Value(), m.status())
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	m = next.(model)
	if m.status() != "Unsaved changes" || !strings.Contains(m.notes.Value(), "q") {
		t.Fatal("ordinary typing must edit the note")
	}
	m, cmd := press(m, "ctrl+s")
	if cmd != nil || m.status() != "Saved" {
		t.Fatalf("Ctrl+S should save and stay open: %s", m.status())
	}
	m.notes.InsertString("\nlast line  \n")
	m, cmd = press(m, "ctrl+c")
	if cmd == nil {
		t.Fatal("Ctrl+C should quit after saving")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected quit command")
	}
	got, err := readNotes(path)
	if err != nil || got != m.notes.Value() {
		t.Fatalf("saved content differs: %q, %v", got, err)
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
	for _, key := range []string{"ctrl+s", "ctrl+c"} {
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

func TestSaveFailureAndRetry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "void", "notes.md")
	m := initialModel(path, nil)
	m.notes.SetValue("unsaved Grüße\n")
	if err := os.WriteFile(filepath.Dir(path), []byte("blocks directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ctrl+s", "ctrl+c"} {
		var cmd tea.Cmd
		m, cmd = press(m, key)
		if cmd != nil || !strings.Contains(m.status(), "Could not save notes") || m.notes.Value() != "unsaved Grüße\n" {
			t.Fatalf("failed %s must retain text and stay open: %s", key, m.status())
		}
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	m, cmd := press(m, "ctrl+s")
	if cmd != nil || m.status() != "Saved" {
		t.Fatalf("retry should succeed: %s", m.status())
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
	m, _ = press(m, "ctrl+s")
	got, err := os.ReadFile(path)
	if err != nil || string(got) != original {
		t.Fatalf("original characters changed: %q, %v", got, err)
	}
}

func TestResizeWithStorageStatus(t *testing.T) {
	m := initialModel(filepath.Join(t.TempDir(), "notes.md"), nil)
	for _, size := range [][2]int{{80, 24}, {40, 10}, {9, 7}, {8, 6}, {1, 1}, {160, 50}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = next.(model)
		view := m.View().Content
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w > size[0] || h > size[1] {
			t.Fatalf("view %dx%d exceeds terminal %dx%d", w, h, size[0], size[1])
		}
		if !m.compact() && (!strings.Contains(view, "Saved") || !strings.Contains(view, "Ctrl+S")) && size[0] >= 40 {
			t.Fatal("status or save shortcut is missing")
		}
	}
}
