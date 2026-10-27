# Void

A minimal terminal workspace. This first milestone is a single, focused notes
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

Press **Ctrl+C** to quit. Ordinary letters such as `q` are always text.
**Notes are not saved yet** and are lost when you quit.

## Code

The application lives in `main.go`:

- **Model** holds the textarea and terminal dimensions.
- **Init** starts cursor blinking; the textarea is focused during setup.
- **Update** handles Ctrl+C and terminal resizing, then forwards events to the
  textarea for editing, pasting, navigation, and cursor blinking.
- **View** renders the headings, textarea, and help text in the alternate screen.

Persistence, timers, and tmux integration are outside this milestone.

## Check

```sh
gofmt -d main.go
go build -o /tmp/void .
```

The build command writes the executable outside the repository.
