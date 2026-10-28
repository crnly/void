package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func attachAndDetach(t *testing.T, w workspace) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Control mode exercises real attach/detach without requiring a test TTY.
	cmd := exec.CommandContext(ctx, w.tmux, "-L", w.server, "-C", "attach-session", "-t", "="+w.session)
	cmd.Stdin = strings.NewReader("refresh-client -C 100,30\ndetach-client\n")
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "%error") {
		t.Fatalf("attach/detach failed: %v, %s", err, output)
	}
}

func testWorkspace(t *testing.T) workspace {
	t.Helper()
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is not installed; workspace integration tests require tmux")
	}
	root := t.TempDir()
	socketDir, err := os.MkdirTemp(root, "server-")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "work dir ' $(literal)")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	// This harmless fixture replaces the editor: tests never open personal notes.
	binary := filepath.Join(root, "void binary ' with spaces")
	helper := "#!/bin/sh\n[ \"$1\" = --sidebar ] || exit 2\nprintf 'SIDEBAR_READY\\n'\nexec /bin/cat\n"
	if err := os.WriteFile(binary, []byte(helper), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV", "") // Do not load any interactive sh startup file.
	// A unique socket label keeps production Void and normal tmux untouched.
	w := workspace{tmux: tmux, server: "void-test-" + filepath.Base(socketDir), session: "void",
		binary: binary, directory: dir, shell: "/bin/sh"}
	t.Cleanup(func() { w.output("kill-server") })
	return w
}

func waitForPaneText(t *testing.T, w workspace, pane, text string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		var err error
		got, err = w.output("capture-pane", "-p", "-t", pane)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, text) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("pane never displayed %q: %s", text, got)
}

func TestWorkspaceLayoutAndReuse(t *testing.T) {
	w := testWorkspace(t)
	if err := w.ensure(); err != nil {
		t.Fatal(err)
	}
	format := "#{pane_id}|#{pane_left}|#{pane_width}|#{pane_active}|#{pane_current_path}|#{pane_pid}"
	before, err := w.output("list-panes", "-t", "="+w.session, "-F", format)
	if err != nil {
		t.Fatal(err)
	}
	panes := strings.Split(before, "\n")
	if len(panes) != 2 {
		t.Fatalf("expected two panes: %s", before)
	}
	left, right := strings.Split(panes[0], "|"), strings.Split(panes[1], "|")
	if len(left) != 6 || len(right) != 6 || left[1] != "0" || left[2] != "36" || left[3] != "0" || right[1] != "37" || right[3] != "1" {
		t.Fatalf("wrong pane layout/focus: %s", before)
	}
	for _, pane := range [][]string{left, right} {
		// macOS may resolve /var to /private/var in process working directories.
		got, _ := filepath.EvalSymlinks(pane[4])
		want, _ := filepath.EvalSymlinks(w.directory)
		if got != want {
			t.Fatalf("working directory = %q, want %q", got, want)
		}
	}
	waitForPaneText(t, w, left[0], "SIDEBAR_READY")
	// A repeat launch must preserve both process IDs and the existing layout.
	if err := w.ensure(); err != nil {
		t.Fatal(err)
	}
	after, err := w.output("list-panes", "-t", "="+w.session, "-F", format)
	if err != nil || after != before {
		t.Fatalf("existing workspace was modified: %s, %v", after, err)
	}
	attachAndDetach(t, w)
	attachAndDetach(t, w)
	width, err := w.output("display-message", "-p", "-t", left[0], "#{pane_width}")
	if err != nil || width != "36" {
		t.Fatalf("sidebar width after reconnect = %q, %v", width, err)
	}
	for _, binding := range []struct{ key, command string }{
		{"Left", "select-pane -L"}, {"Right", "select-pane -R"}, {"d", "detach-client"},
	} {
		got, err := w.output("list-keys", "-T", "prefix", binding.key)
		if err != nil || !strings.Contains(got, binding.command) {
			t.Fatalf("default binding %s was changed: %s, %v", binding.key, got, err)
		}
	}
	for _, option := range []struct{ name, want string }{
		{"prefix", "C-b"}, {"pane-active-border-style", "fg=#88aabb"},
	} {
		got, err := w.output("show-options", "-gv", option.name)
		if err != nil || !strings.EqualFold(got, option.want) {
			t.Fatalf("%s = %q, %v", option.name, got, err)
		}
	}
	// Ctrl+C interrupts a shell command, not the workspace or sidebar.
	if _, err := w.output("send-keys", "-t", right[0], "sleep 30", "Enter"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.output("send-keys", "-t", right[0], "C-c"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.output("send-keys", "-t", right[0], "printf 'SHELL_%s\\n' STILL_ALIVE", "Enter"); err != nil {
		t.Fatal(err)
	}
	waitForPaneText(t, w, right[0], "SHELL_STILL_ALIVE")
	// Exiting only the sidebar must leave the shell process running.
	if _, err := w.output("kill-pane", "-t", left[0]); err != nil {
		t.Fatal(err)
	}
	remaining, err := w.output("list-panes", "-t", "="+w.session, "-F", "#{pane_id}|#{pane_pid}")
	if err != nil || remaining != right[0]+"|"+right[5] {
		t.Fatalf("sidebar exit affected shell: %s, %v", remaining, err)
	}
	if err := w.ensure(); err != nil {
		t.Fatal(err)
	}
	remainingAfter, _ := w.output("list-panes", "-t", "="+w.session, "-F", "#{pane_id}|#{pane_pid}")
	if remainingAfter != remaining {
		t.Fatal("reconnect must not respawn a closed sidebar")
	}
	attachAndDetach(t, w)
}

func TestWorkspaceConcurrentLaunch(t *testing.T) {
	w := testWorkspace(t)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() { errs <- w.ensure() })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	panes, err := w.output("list-panes", "-t", "="+w.session, "-F", "#{pane_id}")
	if err != nil || len(strings.Fields(panes)) != 2 {
		t.Fatalf("concurrent launch created extra sidebars: %q, %v", panes, err)
	}
}

func TestWorkspaceSetupFailureOnlyRemovesNewSession(t *testing.T) {
	w := testWorkspace(t)
	// Keep another session on the same test server; setup failure must spare it.
	if _, err := w.output("-f", "/dev/null", "new-session", "-d", "-s", "keep", "/bin/sh", "-i"); err != nil {
		t.Fatal(err)
	}
	// Force a split failure after session creation to exercise owned cleanup.
	proxy := filepath.Join(t.TempDir(), "tmux proxy")
	t.Setenv("VOID_TEST_TMUX", w.tmux)
	if err := os.WriteFile(proxy, []byte("#!/bin/sh\ncase \"$*\" in *split-window*) echo 'injected split failure' >&2; exit 1;; esac\nexec \"$VOID_TEST_TMUX\" \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	w.tmux = proxy
	if err := w.ensure(); err == nil {
		t.Fatal("expected sidebar startup to fail")
	}
	if w.exists() {
		t.Fatal("failed new workspace should be removed")
	}
	if _, err := w.output("has-session", "-t", "=keep"); err != nil {
		t.Fatalf("pre-existing session was removed: %v", err)
	}
}

func TestWorkspacePrerequisites(t *testing.T) {
	t.Setenv("TMUX", "already-inside-tmux")
	if err := runWorkspace(); err == nil || !strings.Contains(err.Error(), "already inside tmux") {
		t.Fatalf("expected nesting message, got %v", err)
	}
	t.Setenv("TMUX", "")
	t.Setenv("PATH", t.TempDir())
	if err := runWorkspace(); err == nil || !strings.Contains(err.Error(), "tmux is required") {
		t.Fatalf("expected installation message, got %v", err)
	}
}
