package main

// Optional macOS autostart via a per-user LaunchAgent, so the board server
// comes back after login. RunAtLoad only (no KeepAlive) so a manual `stop`
// isn't fought by launchd.

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

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
