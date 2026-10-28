package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

//go:embed tmux.conf
var workspaceConfig string

type workspace struct {
	tmux, server, session    string
	binary, directory, shell string
}

func runWorkspace() error {
	if os.Getenv("TMUX") != "" {
		return fmt.Errorf("Void is already inside tmux. Detach first (Ctrl+B, then d), then start Void from a terminal outside tmux")
	}
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux is required. Install tmux (for example: brew install tmux on macOS or sudo apt install tmux on Debian/Ubuntu), then run Void again")
	}
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running Void binary: %w", err)
	}
	directory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find the working directory: %w", err)
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	shell, err = exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("the user shell is unavailable; set SHELL to an executable shell path: %w", err)
	}
	w := workspace{tmux: tmux, server: "void", session: "void", binary: binary, directory: directory, shell: shell}
	if err := w.ensure(); err != nil {
		return err
	}
	// Attaching hands the terminal directly to tmux: no key interception here.
	cmd := w.command("attach-session", "-t", "="+w.session)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		// Keep the workspace alive even if attaching fails.
		return fmt.Errorf("attach to Void (the workspace is kept; retry from an interactive terminal): %w", err)
	}
	return nil
}

func (w workspace) command(args ...string) *exec.Cmd {
	return exec.Command(w.tmux, append([]string{"-L", w.server}, args...)...)
}

func (w workspace) output(args ...string) (string, error) {
	output, err := w.command(args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", args[0], err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (w workspace) exists() bool {
	_, err := w.output("has-session", "-t", "="+w.session)
	return err == nil
}

// ensure creates at most one sidebar: only the launcher which successfully
// creates the session is allowed to split it or remove it on setup failure.
func (w workspace) ensure() (err error) {
	if w.exists() {
		return nil
	}
	// tmux may report pane creation success even when exec later fails.
	for _, program := range []string{w.binary, w.shell} {
		if _, err := exec.LookPath(program); err != nil {
			return fmt.Errorf("workspace program is unavailable (%s): %w", program, err)
		}
	}
	config, err := os.CreateTemp("", "void-tmux-*.conf")
	if err != nil {
		return fmt.Errorf("create Void tmux configuration: %w", err)
	}
	defer os.Remove(config.Name())
	if _, err := config.WriteString(workspaceConfig); err != nil {
		config.Close()
		return err
	}
	if err := config.Close(); err != nil {
		return err
	}
	// Two command arguments tell tmux to exec directly, without shell quoting.
	panes, err := w.output("-f", config.Name(), "new-session", "-d", "-P", "-F", "#{session_id} #{pane_id}",
		"-s", w.session, "-n", "workspace", "-c", w.directory, "-x", "120", "-y", "32", w.shell, "-i")
	if err != nil {
		// Another launcher may have won the atomic new-session race.
		if w.exists() {
			return nil
		}
		return err
	}
	ids := strings.Fields(panes)
	if len(ids) != 2 {
		return fmt.Errorf("tmux returned unexpected session/pane IDs: %q", panes)
	}
	sessionID, shellPane := ids[0], ids[1]
	defer func() {
		if err != nil {
			if _, cleanupErr := w.output("kill-session", "-t", sessionID); cleanupErr != nil {
				err = fmt.Errorf("%w; cleanup failed: %v", err, cleanupErr)
			}
		}
	}()
	// Also apply the config if this private server was already running without
	// a workspace. Never load or change the user's normal tmux configuration.
	if _, err = w.output("source-file", config.Name()); err != nil {
		return err
	}
	sidebarPane, err := w.output("split-window", "-h", "-b", "-l", "36", "-P", "-F", "#{pane_id}",
		"-t", shellPane, "-c", w.directory, w.binary, "--sidebar")
	if err != nil {
		return err
	}
	// Restore the sidebar width when a client attaches or resizes its terminal.
	for _, hook := range []string{"client-attached", "client-resized"} {
		if _, err = w.output("set-hook", "-t", sessionID, hook, "resize-pane -t "+sidebarPane+" -x 36"); err != nil {
			return err
		}
	}
	_, err = w.output("select-pane", "-t", shellPane)
	return err
}
