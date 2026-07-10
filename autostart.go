package main

// Optional macOS autostart via a per-user LaunchAgent, so the board server
// comes back after login. RunAtLoad only (no KeepAlive) so a manual `stop`
// isn't fought by launchd.

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// When autostart is on, launchd owns the process (no pid file), so start/stop/
// restart must go through launchctl instead of a pid.
func launchdManaged() bool { return bootEnabled() }

func launchdTarget() string {
	return fmt.Sprintf("gui/%d/com.claude-board.server", os.Getuid())
}

func launchdKickstart(restart bool) error {
	args := []string{"kickstart"}
	if restart {
		args = append(args, "-k") // kill the running instance, then start
	}
	args = append(args, launchdTarget())
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kickstart: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func launchdKill() error {
	out, err := exec.Command("launchctl", "kill", "SIGTERM", launchdTarget()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl kill: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func plistXML(exe string, port int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.claude-board.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>run</string>
    <string>--port</string>
    <string>%d</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
</dict></plist>
`, exe, port, logFile(), logFile())
}

func enableBoot(port int) error {
	if runtime.GOOS != "darwin" {
		fmt.Println("autostart is macOS-only; use `claude-board start` instead")
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(launchAgentsDir(), 0o755); err != nil {
		return err
	}
	_ = os.MkdirAll(stateDir(), 0o755)
	if err := os.WriteFile(plistPath(), []byte(plistXML(exe, port)), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "unload", plistPath()).Run()
	out, err := exec.Command("launchctl", "load", "-w", plistPath()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load failed: %s", string(out))
	}
	fmt.Printf("autostart enabled (%s)\n", plistPath())
	return nil
}

func disableBoot() error {
	if _, err := os.Stat(plistPath()); err != nil {
		fmt.Println("autostart not enabled")
		return nil
	}
	_ = exec.Command("launchctl", "unload", "-w", plistPath()).Run()
	_ = os.Remove(plistPath())
	fmt.Println("autostart disabled")
	return nil
}

func bootEnabled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}
