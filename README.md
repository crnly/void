# Void

A minimal terminal workspace with a single, focused notes
editor built with Bubble Tea v2, Bubbles v2, and Lip Gloss v2.

## Run

Install Go 1.26.3 or newer, then run from this directory:

```sh
go run .
```

Start typing immediately. Enter adds a new line; Unicode text (including umlauts)
and pasting with your terminal's paste shortcut are supported. Use the arrow keys
or Page Up / Page Down to navigate and scroll through longer notes. The editor
adapts when the terminal is resized. Very small terminals show only the text field.

Press **Ctrl+S** to save or **Ctrl+C** to save and quit. Ordinary letters such as
`q` are always text. The status shows **Saved**, **Unsaved changes**, or an error.

## Local storage

Void loads one shared note from `void/notes.md` inside Go's `os.UserConfigDir()`:
typically `~/Library/Application Support/void/notes.md` on macOS,
`$XDG_CONFIG_HOME/void/notes.md` (or `~/.config/void/notes.md`) on Linux, and
`%AppData%/void/notes.md` on Windows. A missing file opens an empty editor.

Saving writes and syncs a temporary file in the same directory, then atomically
renames it over `notes.md`. The directory uses mode `0700` and the file `0600`
(Unix permissions). Unicode, spaces, blank lines, and trailing newlines are kept.

If saving fails, Void stays open with your text intact and displays an English
error. Fix the cause and retry with Ctrl+S or Ctrl+C. If loading fails, saving is
disabled for that session to protect existing notes. Fix the cause and restart
Void. Saving is also disabled if the textarea cannot represent the loaded file
without changing its characters.

## Code

The editor lives in `main.go`; `storage.go` handles the path, loading, and atomic
file writes.

- **Model** holds the textarea, terminal dimensions, storage path, last saved
  text, and any storage errors.
- **Init** starts cursor blinking; the textarea is focused during setup.
- **Update** handles saving, quitting, and terminal resizing, then forwards events to the
  textarea for editing, pasting, navigation, and cursor blinking.
- **View** renders the headings, textarea, storage status, and help text in the alternate screen.

Timers and tmux integration are outside this milestone. Avoid editing the same
note in multiple Void instances at once: the last successful save wins.

## Check

```sh
gofmt -d *.go
go test ./...
go build -o /tmp/void .
```

The build command writes the executable outside the repository.
Storage tests use temporary directories and never access personal notes.
