package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func notesPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config directory: %w", err)
	}
	return filepath.Join(dir, "void", "notes.md"), nil
}

func readNotes(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeNotes replaces the note only after the temporary file is fully written.
// Keeping both files in the same directory makes the rename atomic.
func writeNotes(path, text string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".notes-*") // CreateTemp uses mode 0600.
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
