package main

// claude-board CLI. The server is a long-running relay; a global Claude Code
// PostToolUse hook (a plain curl) pipes every tool call to it. This CLI manages
// the hook, the background server process, and an optional macOS autostart.

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "0.1.0"
const defaultPort = 8787

// --- paths --------------------------------------------------------------------
func home() string { h, _ := os.UserHomeDir(); return h }
func claudeDir() string        { return filepath.Join(home(), ".claude") }
func settingsPath() string     { return filepath.Join(claudeDir(), "settings.json") }
func stateDir() string         { return filepath.Join(home(), ".claude-board") }
func pidFile() string          { return filepath.Join(stateDir(), "server.pid") }
func logFile() string          { return filepath.Join(stateDir(), "server.log") }
func launchAgentsDir() string  { return filepath.Join(home(), "Library", "LaunchAgents") }
func plistPath() string        { return filepath.Join(launchAgentsDir(), "com.claude-board.server.plist") }

func boardURL(port int) string {
	return fmt.Sprintf("http://%s:%d/?k=%s", lanIP(), port, ensureToken())
}

// --- process ------------------------------------------------------------------
func isRunning(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func waitUp(port int) bool {
	for i := 0; i < 60; i++ {
		if isRunning(port) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return isRunning(port)
}

func startServer(port int) error {
	if isRunning(port) {
		fmt.Printf("already running — %s\n", boardURL(port))
		return nil
	}
	if launchdManaged() { // launchd owns it — start the managed job
		if err := launchdKickstart(false); err != nil {
			return err
		}
		if waitUp(port) {
			fmt.Printf("started (autostart) — open %s on your iPad\n", boardURL(port))
			return nil
		}
		return fmt.Errorf("failed to start; see %s", logFile())
	}
	if err := os.MkdirAll(stateDir(), 0o755); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(logFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "run", "--port", strconv.Itoa(port))
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = os.WriteFile(pidFile(), []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	if waitUp(port) {
		fmt.Printf("started (pid %d) — open %s on your iPad\n", cmd.Process.Pid, boardURL(port))
		return nil
	}
	return fmt.Errorf("failed to start; see %s", logFile())
}

func readPid() int {
	b, err := os.ReadFile(pidFile())
	if err != nil {
		return 0
	}
	p, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return p
}

func stopServer(port int) {
	if launchdManaged() { // launchd owns it — signal via launchctl
		if err := launchdKill(); err != nil {
			fmt.Println(err)
		}
		for i := 0; i < 40 && isRunning(port); i++ {
			time.Sleep(50 * time.Millisecond)
		}
		if isRunning(port) {
			fmt.Println("still running (autostart may respawn on login)")
		} else {
			fmt.Println("stopped")
		}
		return
	}
	pid := readPid()
	killed := false
	if pid > 0 {
		if syscall.Kill(pid, syscall.SIGTERM) == nil {
			killed = true
		}
	}
	_ = os.Remove(pidFile())
	if !killed && isRunning(port) {
		fmt.Printf("running but no pid on file — kill manually: lsof -ti :%d | xargs kill\n", port)
		return
	}
	if killed {
		fmt.Println("stopped")
	} else {
		fmt.Println("not running")
	}
}

func restartServer(port int) error {
	if launchdManaged() { // kill + start the managed job in one shot
		if err := launchdKickstart(true); err != nil {
			return err
		}
		waitUp(port)
		fmt.Printf("restarted — %s\n", boardURL(port))
		return nil
	}
	stopServer(port)
	time.Sleep(300 * time.Millisecond)
	return startServer(port)
}

// --- commands -----------------------------------------------------------------
func cmdSetup(port int, boot bool) error {
	if err := installHook(port); err != nil {
		return err
	}
	fmt.Println("hook installed in " + settingsPath())
	if isRunning(port) { // replace any prior/old server on this port
		stopServer(port)
		time.Sleep(300 * time.Millisecond)
	}
	if boot {
		if err := enableBoot(port); err != nil {
			return err
		}
		waitUp(port)
	}
	if !isRunning(port) {
		if err := startServer(port); err != nil {
			return err
		}
	}
	fmt.Println()
	fmt.Println("done. On your iPad (same Wi-Fi) open:")
	fmt.Printf("    %s\n", boardURL(port))
	fmt.Println("Then restart Claude Code so the hook loads.")
	return nil
}

func cmdTeardown(port int) error {
	_ = disableBoot()
	stopServer(port)
	n, err := uninstallHook()
	if err != nil {
		return err
	}
	fmt.Printf("removed %d board hook(s) from %s\n", n, settingsPath())
	fmt.Println("torn down. Restart Claude Code.")
	return nil
}

func cmdStatus(port int) {
	if isRunning(port) {
		fmt.Printf("server:    running %s\n", boardURL(port))
	} else {
		fmt.Println("server:    stopped")
	}
	fmt.Printf("hook:      %s\n", tern(hookInstalled(), "installed", "not installed"))
	fmt.Printf("autostart: %s\n", tern(bootEnabled(), "enabled", "disabled"))
	fmt.Printf("auth:      token%s\n", tern(challengeConfigured(), " + quiz", " only"))
}

// cmdChallenge scaffolds / removes the optional tappable quiz gate.
func cmdChallenge(sub string) error {
	switch sub {
	case "off", "disable", "remove":
		if _, err := os.Stat(challengePath()); err != nil {
			fmt.Println("quiz gate not configured")
			return nil
		}
		if err := os.Remove(challengePath()); err != nil {
			return err
		}
		fmt.Println("quiz gate removed (token-only). Restart the server: claude-board restart")
		return nil
	default: // init / scaffold
		if _, err := os.Stat(challengePath()); err == nil {
			fmt.Printf("edit your questions at %s\n", challengePath())
			fmt.Println("then restart the server: claude-board restart")
			return nil
		}
		if err := os.MkdirAll(stateDir(), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(challengePath(), []byte(exampleChallenge), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote an example quiz to %s\n", challengePath())
		fmt.Println("edit it, then restart the server: claude-board restart")
		fmt.Println("note: a multiple-choice quiz is guessable — convenience, not real security.")
		return nil
	}
}

func tern(b bool, a, c string) string {
	if b {
		return a
	}
	return c
}

// --- arg parsing --------------------------------------------------------------
const usage = `claude-board ` + version + ` — live board of Claude Code tool outputs.

usage: claude-board [--port N] <command>

commands:
  setup            install hook + autostart + start (recommended)
  teardown         remove hook + autostart + stop server
  start            start the server in the background
  stop             stop the background server
  restart          restart the background server
  status           show server / hook / autostart state
  run              run the server in the foreground
  url              print the iPad URL
  install-hook     add the PostToolUse hook to settings.json
  uninstall-hook   remove the hook from settings.json
  enable-boot      start on login (macOS LaunchAgent)
  disable-boot     stop starting on login
  challenge        scaffold an optional tappable quiz gate (challenge off removes it)

options:
  --port N         port (default 8787, or $CLAUDE_BOARD_PORT)
  --no-boot        with setup: skip the macOS autostart
  --version
`

func envPort() int {
	if v := os.Getenv("CLAUDE_BOARD_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			return p
		}
	}
	return defaultPort
}

func main() {
	port := envPort()
	boot := true
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--port" && i+1 < len(args):
			port, _ = strconv.Atoi(args[i+1])
			i++
		case strings.HasPrefix(a, "--port="):
			port, _ = strconv.Atoi(a[len("--port="):])
		case a == "--no-boot":
			boot = false
		case a == "--version" || a == "-v":
			fmt.Println("claude-board " + version)
			return
		case a == "-h" || a == "--help":
			fmt.Print(usage)
			return
		default:
			rest = append(rest, a)
		}
	}
	cmd := ""
	if len(rest) > 0 {
		cmd = rest[0]
	}

	var err error
	switch cmd {
	case "setup":
		err = cmdSetup(port, boot)
	case "teardown":
		err = cmdTeardown(port)
	case "start":
		err = startServer(port)
	case "stop":
		stopServer(port)
	case "restart":
		err = restartServer(port)
	case "status":
		cmdStatus(port)
	case "run":
		err = serve(port)
	case "url":
		fmt.Println(boardURL(port))
	case "install-hook":
		if err = installHook(port); err == nil {
			fmt.Println("hook installed in " + settingsPath())
		}
	case "uninstall-hook":
		var n int
		if n, err = uninstallHook(); err == nil {
			fmt.Printf("removed %d board hook(s) from %s\n", n, settingsPath())
		}
	case "enable-boot":
		err = enableBoot(port)
	case "disable-boot":
		err = disableBoot()
	case "challenge":
		sub := ""
		if len(rest) > 1 {
			sub = rest[1]
		}
		err = cmdChallenge(sub)
	case "", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
