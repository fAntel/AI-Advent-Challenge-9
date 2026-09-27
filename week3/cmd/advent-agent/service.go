package main

import (
	agent "advent-agent"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const serviceLabel = "dev.aiadvent.advent-agent"

func plistEscape(v string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(v)
}
func servicePlist(binary, stderr string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>` + serviceLabel + `</string>
<key>ProgramArguments</key><array><string>` + plistEscape(binary) + `</string></array>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><true/>
<key>StandardErrorPath</key><string>` + plistEscape(stderr) + `</string>
</dict></plist>
`
}
func launchctl(args ...string) error {
	cmd := exec.Command("launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
func runService(cfg agent.Config, args []string) {
	if len(args) == 0 {
		fatal(errors.New("usage: advent-agent service install|start|status|stop|uninstall|key set"))
	}
	if args[0] == "key" {
		if len(args) != 2 || args[1] != "set" {
			fatal(errors.New("usage: advent-agent service key set"))
		}
		fmt.Fprint(os.Stderr, "DeepSeek API key: ")
		old, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
		if err == nil {
			next := *old
			next.Lflag &^= unix.ECHO
			fatalIf(unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, &next))
			defer unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, old)
		}
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && err != io.EOF {
			fatal(err)
		}
		fmt.Fprintln(os.Stderr)
		fatalIf(agent.SetKeychainDeepSeekKey(strings.TrimSpace(line)))
		fmt.Fprintln(os.Stderr, "Key stored in macOS Keychain.")
		return
	}
	home, err := os.UserHomeDir()
	fatalIf(err)
	path := filepath.Join(home, "Library", "LaunchAgents", serviceLabel+".plist")
	target := fmt.Sprintf("gui/%d/%s", os.Getuid(), serviceLabel)
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	switch args[0] {
	case "install":
		if _, err := agent.KeychainDeepSeekKey(); err != nil {
			fatal(errors.New("store the API key first with `advent-agent service key set`"))
		}
		binary, err := findDaemon()
		fatalIf(err)
		binary, err = filepath.Abs(binary)
		fatalIf(err)
		if _, err := os.Stat(filepath.Join(filepath.Dir(binary), "budget-mcp")); err != nil {
			fatal(errors.New("build budget-mcp next to advent-agentd first"))
		}
		fatalIf(os.MkdirAll(filepath.Dir(path), 0700))
		fatalIf(os.MkdirAll(cfg.StateDir, 0700))
		_ = launchctl("bootout", target)
		client := agent.NewAPIClient(cfg.Daemon.SocketPath, 5*time.Second)
		_ = client.Shutdown()
		for i := 0; i < 50; i++ {
			if _, err := client.List(); errors.Is(err, agent.ErrDaemonUnavailable) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		fatalIf(os.WriteFile(path, []byte(servicePlist(binary, filepath.Join(cfg.StateDir, "launchd.stderr.log"))), 0600))
		fatalIf(launchctl("bootstrap", domain, path))
		fmt.Fprintln(os.Stderr, "LaunchAgent installed and started:", path)
	case "start":
		if _, err := os.Stat(path); err != nil {
			fatal(errors.New("LaunchAgent is not installed"))
		}
		if err := launchctl("print", target); err == nil {
			fatalIf(launchctl("kickstart", "-k", target))
		} else {
			fatalIf(launchctl("bootstrap", domain, path))
		}
	case "status":
		fatalIf(launchctl("print", target))
	case "stop":
		fatalIf(launchctl("bootout", target))
	case "uninstall":
		_ = launchctl("bootout", target)
		fatalIf(os.Remove(path))
	default:
		fatal(errors.New("usage: advent-agent service install|start|status|stop|uninstall|key set"))
	}
}
