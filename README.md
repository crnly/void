# Void

A minimal terminal workspace with a notes editor and a 25-minute focus timer,
built with Bubble Tea v2, Bubbles v2, and Lip Gloss v2.

## Run

Install Go 1.26.3 or newer, then run from this directory:

```sh
go run .
```

Start typing immediately. Enter adds a new line; Unicode text (including umlauts)
and pasting with your terminal's paste shortcut are supported. Use the arrow keys
or Page Up / Page Down to navigate and scroll through longer notes. The editor
adapts when the terminal is resized. Small terminals omit headings and padding;
the timer, storage status, and help are shown as space permits.

Press **Ctrl+S** to save or **Ctrl+C** to save and quit. Ordinary letters such as
`q` are always text. The status shows **Saved**, **Unsaved changes**, or an error.

## Focus timer

The timer starts stopped at **Focus  25:00** below the notes field.

- **Tab** switches keyboard focus between Notes and Timer. The active area uses
  a subtle blue accent, and the help line changes with the focus.
- With Timer focused, **Space** starts, pauses, or resumes the countdown;
  **R** (or **r**) resets to 25:00 and stops it.
- **Esc** returns to Notes. Letters and spaces are ordinary text in Notes.
- **Ctrl+S** and **Ctrl+C** retain their save and save-and-quit behavior in either area.

The timer continues while you edit notes. At **00:00**, it stops and displays
**Session complete**. Reset it before starting another session. Remaining time
is measured against a deadline, so delayed screen updates do not extend the
session. Paused time is excluded. The timer is kept only for the current session;
restarting Void restores 25:00.

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

The editor and focus routing live in `main.go`; `storage.go` handles local files,
and `timer.go` handles the countdown.

- **Model** holds the textarea, terminal dimensions, storage path, last saved
  text, storage errors, and timer state.
- **Init** starts cursor blinking; the textarea is focused during setup.
- **Update** handles saving, quitting, focus, timer ticks, and terminal resizing.
  Editing events go to the focused textarea.
- **View** renders the headings, textarea, timer, storage status, and help text.

tmux integration is outside this milestone. Avoid editing the same
note in multiple Void instances at once: the last successful save wins.

## Check

```sh
gofmt -d *.go
go test ./...
go build -o /tmp/void .
```

The build command writes the executable outside the repository.
Storage tests use temporary directories and never access personal notes.
Timer tests use controlled timestamps without real waiting. Each scheduled tick
has a unique ID, so stale or duplicate ticks cannot create extra tick loops.
