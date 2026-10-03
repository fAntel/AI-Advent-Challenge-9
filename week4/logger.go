package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DebugLogger struct {
	mu      sync.Mutex
	enabled bool
	path    string
	max     int64
	backups int
	secret  string
}

func NewDebugLogger(cfg DaemonConfig, secret string) *DebugLogger {
	return &DebugLogger{enabled: cfg.Debug, path: cfg.LogPath, max: cfg.LogMaxBytes, backups: cfg.LogBackups, secret: secret}
}

func (l *DebugLogger) Enabled() bool { return l != nil && l.enabled }

func (l *DebugLogger) Redact(value string) string {
	value = strings.ReplaceAll(value, "Authorization: Bearer "+l.secret, "Authorization: Bearer [REDACTED]")
	value = strings.ReplaceAll(value, "Bearer "+l.secret, "Bearer [REDACTED]")
	if l.secret != "" {
		value = strings.ReplaceAll(value, l.secret, "[REDACTED]")
	}
	return value
}

func (l *DebugLogger) Log(event, session, operation, message string) {
	if !l.Enabled() {
		return
	}
	line := fmt.Sprintf("%s level=DEBUG event=%s", time.Now().UTC().Format(time.RFC3339Nano), quote(event))
	if session != "" {
		line += " session=" + quote(session)
	}
	if operation != "" {
		line += " operation=" + quote(operation)
	}
	line += " message=" + quote(l.Redact(message)) + "\n"
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return
	}
	_ = os.Chmod(filepath.Dir(l.path), 0700)
	if info, err := os.Lstat(l.path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return
	}
	if info, err := os.Stat(l.path); err == nil && info.Size()+int64(len(line)) > l.max {
		l.rotate()
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	_ = f.Chmod(0600)
	_, _ = f.WriteString(line)
	_ = f.Close()
}

func quote(s string) string { return fmt.Sprintf("%q", s) }

func (l *DebugLogger) rotate() {
	if l.backups == 0 {
		_ = os.Remove(l.path)
		return
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", l.path, l.backups))
	for i := l.backups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", l.path, i), fmt.Sprintf("%s.%d", l.path, i+1))
	}
	_ = os.Rename(l.path, l.path+".1")
}
