# Void

A minimal tmux workspace: a notes editor and 25-minute focus timer on the left,
and a real interactive shell on the right. The sidebar uses Bubble Tea v2,
Bubbles v2, and Lip Gloss v2.

## Run

Requires Go 1.26.3 or newer, tmux, and an interactive terminal on macOS or Linux.
Tested with tmux 3.8. Install tmux with `brew install tmux` on macOS or
`sudo apt install tmux` on Debian/Ubuntu. Then run from this directory:

```sh
go run .
```

The shell has keyboard focus initially. It uses `$SHELL` (or `/bin/sh` when unset)
and starts in the directory from which Void was launched. The sidebar is about
36 columns wide, with a muted blue-grey divider and a short tmux help line.

- **Ctrl+B, then Left/Right** switches between the sidebar and shell.
- **Ctrl+B, then d** detaches. The workspace, shell programs, and timer keep running.
- Run **`go run .`** again to reconnect to the existing Void workspace, without
  starting a second editor. Reconnecting preserves the existing shell directory.
- Tab and Ctrl+C work normally in the shell; Void does not intercept shell input.

If already inside tmux, Void asks you to detach first instead of nesting servers.
Void uses its own `tmux -L void` server and the embedded `tmux.conf`, without
loading `~/.tmux.conf` or changing normal tmux sessions. See the
[tmux manual](https://man.openbsd.org/tmux) for server and pane commands.

## Notes sidebar

Switch to the left pane to edit notes. Enter adds a new line; Unicode text
(including umlauts) and pasting with your terminal's paste shortcut are supported. Use the arrow keys
or Page Up / Page Down to navigate and scroll through longer notes. The editor
adapts when the terminal is resized. Small terminals omit headings and padding;
the timer, storage errors, and help are shown as space permits.

Notes are saved locally after a **500 ms writing pause** and restored on the
next start. In the sidebar, press **Ctrl+C** to exit; Void saves the latest text
before closing, even if the writing pause has not elapsed. **Ctrl+C there does not stop the shell or any other programs.** The sidebar
pane closes after a successful save; the remaining shell stays open. Reconnecting
attaches to that same workspace and does not respawn the sidebar. To start a fresh
two-pane workspace, finish any shell programs, exit the remaining shell, then
start Void again. When all panes have exited, tmux removes the session.

Ordinary letters such as `q` are always text. Saving needs no manual action;
only storage errors appear in the interface. The help wraps to fit the sidebar.

`go run . --sidebar` is the internal editor-only mode and bypasses the workspace
launcher. Normal use should go through `go run .` to avoid opening the shared note
file in multiple editors.

## Focus timer

The timer starts stopped at **Focus  25:00** below the notes field.

- **Tab** switches keyboard focus between Notes and Timer. The active area uses
  a subtle blue accent, and the help line changes with the focus.
- With Timer focused, **Space** starts, pauses, or resumes the countdown;
  **R** (or **r**) resets to 25:00 and stops it.
- **Esc** returns to Notes. Letters and spaces are ordinary text in Notes.
- **Ctrl+C** saves the latest notes and quits from either area. Autosave works
  while the timer has focus too.

The timer continues while you edit notes. At **00:00**, it stops and displays
**Session complete**. Reset it before starting another session. Remaining time
is measured against a deadline, so delayed screen updates do not extend the
session. Paused time is excluded. A fresh sidebar process restores 25:00;
detaching and reconnecting keeps the running process and its timer state.

## Local storage

Void loads one shared note from `void/notes.md` inside Go's `os.UserConfigDir()`:
typically `~/Library/Application Support/void/notes.md` on macOS,
`$XDG_CONFIG_HOME/void/notes.md` (or `~/.config/void/notes.md`) on Linux, and
`%AppData%/void/notes.md` on Windows. A missing file opens an empty editor.

Saving writes and syncs a temporary file in the same directory, then atomically
renames it over `notes.md`. The directory uses mode `0700` and the file `0600`
(Unix permissions). Unicode, spaces, blank lines, and trailing newlines are kept.

If saving fails, Void stays open with your text intact and displays an English
error. Fix the cause; the next edit retries automatically after the writing pause,
or press Ctrl+C to retry the final save and exit. A failed final save keeps the
sidebar open. If loading fails, saving is
disabled for that session to protect existing notes. Fix the cause and restart
Void. Saving is also disabled if the textarea cannot represent the loaded file
without changing its characters.

## Code

`workspace.go` launches and reconnects to tmux using structured process arguments.
The current Void executable starts the sidebar with `--sidebar`; no extra build
or binary copy is needed. Paths with spaces are passed directly to tmux.
Only a newly created session is removed if setup fails. An existing workspace
is kept, including when attaching fails.

The editor, autosave, and focus routing live in `main.go`; `storage.go` handles
local files, and `timer.go` handles the countdown. Autosave timeouts have unique
IDs, so obsolete or duplicate timeouts cannot save. File writes run sequentially
in the update loop using the current text; an older write cannot finish after a
newer one. Normal shutdown (Ctrl+C, SIGINT, or SIGTERM) flushes pending changes
before exiting. Forced termination cannot flush edits still within the writing
pause.

- **Model** holds the textarea, terminal dimensions, storage path, last saved
  text, storage errors, and timer state.
- **Init** starts cursor blinking; the textarea is focused during setup.
- **Update** handles autosave, final saving on quit, focus, timer ticks, and resizing.
  Editing events go to the focused textarea.
- **View** renders the headings, textarea, timer, storage errors, and help text.

Avoid starting multiple editor-only instances against the same note file:
the last successful save wins.

## Check

```sh
gofmt -d *.go
go test ./...
go build -o /tmp/void .
```

The build command writes the executable outside the repository.
Storage tests use temporary directories and never access personal notes.
Autosave tests deliver timeout messages directly without waiting; they check
stale messages, shutdown flushing, restoration, and recovery from storage errors.
Timer tests use controlled timestamps without real waiting. Each scheduled tick
has a unique ID, so stale or duplicate ticks cannot create extra tick loops.
Workspace integration tests use unique `void-test-*` tmux servers and a harmless
sidebar fixture, never the real `void` server or personal notes. They check layout,
working directories with spaces, shell focus, reconnects, concurrent starts, and
cleanup on setup errors. These tests are skipped if tmux is unavailable.
