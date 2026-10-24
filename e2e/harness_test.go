//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// gwtBinary is the gwt executable built once for the end-to-end tests.
var gwtBinary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gwt-e2e-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	gwtBinary = filepath.Join(dir, "gwt")
	build := exec.Command("go", "build", "-o", gwtBinary, "github.com/douglasgomes98/gwt/cmd/gwt")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "build gwt:", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// waitTimeout bounds every wait for the screen or the process to settle.
const waitTimeout = 10 * time.Second

// terminal runs gwt inside a pseudo-terminal and keeps a virtual screen of
// its output, so tests send the same bytes a real terminal sends and assert
// on what a user would see.
type terminal struct {
	t      *testing.T
	pty    *os.File
	cmd    *exec.Cmd
	mu     sync.Mutex
	screen *vt.Emulator
	exited chan struct{}
}

// startTUI launches the gwt TUI in dir with an isolated HOME, sized w×h, and
// waits until the worktree status has finished loading.
func startTUI(t *testing.T, dir string, w, h int) *terminal {
	t.Helper()
	home := t.TempDir()
	cmd := exec.Command(gwtBinary)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)}) // #nosec G115 -- small test sizes.
	if err != nil {
		t.Fatal(err)
	}
	term := &terminal{t: t, pty: f, cmd: cmd, screen: vt.NewEmulator(w, h), exited: make(chan struct{})}
	go term.answerQueries()
	go term.readOutput()
	go func() {
		_ = cmd.Wait()
		close(term.exited)
	}()
	t.Cleanup(term.close)
	term.waitFor(func(s string) bool {
		_, focused := term.Focused()
		return focused != "" && strings.Contains(s, " worktrees") && !strings.Contains(s, "checking status")
	})
	return term
}

// answerQueries forwards the emulator's replies (device attributes, cursor
// position, …) back to gwt, as a real terminal would.
func (term *terminal) answerQueries() {
	buf := make([]byte, 1024)
	for {
		n, err := term.screen.Read(buf)
		if n > 0 {
			_, _ = term.pty.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (term *terminal) readOutput() {
	buf := make([]byte, 32*1024)
	for {
		n, err := term.pty.Read(buf)
		if n > 0 {
			term.mu.Lock()
			_, _ = term.screen.Write(buf[:n])
			term.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (term *terminal) close() {
	if term.cmd.Process != nil {
		_ = term.cmd.Process.Kill()
	}
	<-term.exited
	_ = term.pty.Close()
	_ = term.screen.Close()
}

// Screen returns the visible text, one line per row.
func (term *terminal) Screen() []string {
	term.mu.Lock()
	defer term.mu.Unlock()
	return strings.Split(term.screen.String(), "\n")
}

func (term *terminal) waitFor(done func(screen string) bool) {
	term.t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		screen := strings.Join(term.Screen(), "\n")
		if done(screen) {
			return
		}
		if time.Now().After(deadline) {
			term.t.Fatalf("screen did not settle:\n%s", screen)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// send writes input and waits until the screen has redrawn in response.
func (term *terminal) send(input string, settled func(screen string) bool) {
	term.t.Helper()
	if _, err := term.pty.Write([]byte(input)); err != nil {
		term.t.Fatal(err)
	}
	term.waitFor(settled)
}

var keySequences = map[string]string{
	"up": "\x1b[A", "down": "\x1b[B", "pgup": "\x1b[5~", "pgdown": "\x1b[6~",
	"home": "\x1b[H", "end": "\x1b[F", "enter": "\r", "space": " ", "esc": "\x1b",
	"ctrl+d": "\x04", "ctrl+u": "\x15",
}

// Press sends a named key and waits for settled.
func (term *terminal) Press(key string, settled func(screen string) bool) {
	term.t.Helper()
	seq, ok := keySequences[key]
	if !ok {
		seq = key
	}
	term.send(seq, settled)
}

// Click sends an SGR left-button press and release at the 0-based cell x, y.
func (term *terminal) Click(x, y int, settled func(screen string) bool) {
	term.t.Helper()
	if y < 0 {
		term.t.Fatalf("click target is not on screen:\n%s", strings.Join(term.Screen(), "\n"))
	}
	term.send(fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1), settled)
}

// ClickText clicks the first cell of the first on-screen occurrence of text.
func (term *terminal) ClickText(text string, settled func(screen string) bool) {
	term.t.Helper()
	for y, line := range term.Screen() {
		if i := strings.Index(line, text); i >= 0 {
			term.Click(len([]rune(line[:i])), y, settled)
			return
		}
	}
	term.t.Fatalf("%q not on screen:\n%s", text, strings.Join(term.Screen(), "\n"))
}

// Wheel scrolls the mouse wheel down (or up) once over the cell x, y.
func (term *terminal) Wheel(down bool, x, y int, settled func(screen string) bool) {
	term.t.Helper()
	button := 64
	if down {
		button = 65
	}
	term.send(fmt.Sprintf("\x1b[<%d;%d;%dM", button, x+1, y+1), settled)
}

// Resize changes the terminal size and waits for settled.
func (term *terminal) Resize(w, h int, settled func(screen string) bool) {
	term.t.Helper()
	term.mu.Lock()
	term.screen.Resize(w, h)
	term.mu.Unlock()
	if err := pty.Setsize(term.pty, &pty.Winsize{Cols: uint16(w), Rows: uint16(h)}); err != nil { // #nosec G115 -- small test sizes.
		term.t.Fatal(err)
	}
	term.waitFor(settled)
}

// WaitExit waits for gwt to exit on its own.
func (term *terminal) WaitExit() {
	term.t.Helper()
	select {
	case <-term.exited:
	case <-time.After(waitTimeout):
		term.t.Fatalf("gwt did not exit:\n%s", strings.Join(term.Screen(), "\n"))
	}
}

// Focused returns the row holding the › focus marker, or -1.
func (term *terminal) Focused() (int, string) {
	for y, line := range term.Screen() {
		if strings.HasPrefix(line, "›") {
			return y, line
		}
	}
	return -1, ""
}

// Row returns the screen row whose text contains every part, or -1.
func (term *terminal) Row(parts ...string) int {
	for y, line := range term.Screen() {
		if containsAll(line, parts...) {
			return y
		}
	}
	return -1
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}

// shows reports a settled condition: every part appears on one screen line.
func shows(parts ...string) func(string) bool {
	return func(screen string) bool {
		for line := range strings.SplitSeq(screen, "\n") {
			if containsAll(line, parts...) {
				return true
			}
		}
		return false
	}
}

// focusOn reports a settled condition: the focused row contains every part.
func focusOn(parts ...string) func(string) bool {
	return func(screen string) bool {
		for line := range strings.SplitSeq(screen, "\n") {
			if strings.HasPrefix(line, "›") {
				return containsAll(line, parts...)
			}
		}
		return false
	}
}

// group creates a project group with n primary repositories (repo01…) and,
// for each feature branch, worktrees in the given repositories. It returns
// the group directory.
func group(t *testing.T, n int, features map[string][]int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 1; i <= n; i++ {
		repo := filepath.Join(dir, fmt.Sprintf("repo%02d", i))
		git(t, dir, "init", "-q", "-b", "main", repo)
		git(t, repo, "-c", "user.name=e2e", "-c", "user.email=e2e@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	}
	for branch, repos := range features {
		for _, i := range repos {
			repo := filepath.Join(dir, fmt.Sprintf("repo%02d", i))
			git(t, repo, "worktree", "add", "-q", "-b", branch, repo+"."+branch)
		}
	}
	return dir
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("git %v: %s", args, out)
		}
		t.Fatal(err)
	}
	return string(out)
}
