package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNotesRoundTrip(t *testing.T) {
	for name, text := range map[string]string{
		"empty":                  "",
		"unicode and whitespace": "  Grüße aus Köln! ÄÖÜ äöü ß 世界 🙂  \n\n  indented  \n\n",
		"no final newline":       "first\nlast  ",
		"CRLF":                   "first\r\nsecond\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "void", "notes.md")
			for _, value := range []string{"previous note", text} {
				if err := writeNotes(path, value); err != nil {
					t.Fatal(err)
				}
				got, err := readNotes(path)
				if err != nil || got != value {
					t.Fatalf("readNotes = %q, %v; want %q", got, err, value)
				}
			}
			for path, mode := range map[string]os.FileMode{filepath.Dir(path): 0700, path: 0600} {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != mode {
					t.Errorf("%s mode = %o; want %o", path, info.Mode().Perm(), mode)
				}
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 1 || entries[0].Name() != "notes.md" {
				t.Fatalf("unexpected temporary files: %v, %v", entries, err)
			}
		})
	}
}

func TestReadMissingNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "notes.md")
	got, err := readNotes(path)
	if err != nil || got != "" {
		t.Fatalf("missing notes = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("reading should not create the directory: %v", err)
	}
}

func TestFailedReplacementCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.md")
	// A non-empty directory reliably prevents replacement, even as root.
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "keep"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeNotes(path, "replacement"); err == nil {
		t.Fatal("expected rename to fail")
	}
	got, err := os.ReadFile(filepath.Join(path, "keep"))
	if err != nil || string(got) != "original" {
		t.Fatalf("original changed: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file not cleaned up: %v, %v", entries, err)
	}
}
