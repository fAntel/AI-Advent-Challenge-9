package main

import (
	agent "advent-agent"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	debug := flag.Bool("debug", false, "enable private debug logging")
	stop := flag.Bool("stop", false, "stop the running daemon")
	profile := flag.String("profile", agent.DefaultProfile, "configuration profile")
	flag.Parse()
	cfg, err := agent.LoadProfileConfig(*profile)
	if err != nil {
		fatal(err)
	}
	if *debug {
		cfg.Daemon.Debug = true
	}
	stopRequested := *stop || (flag.NArg() == 1 && flag.Arg(0) == "stop")
	if flag.NArg() > 1 || (flag.NArg() == 1 && flag.Arg(0) != "stop") {
		fatal(fmt.Errorf("usage: advent-agentd [--profile NAME] [--debug] | advent-agentd [--profile NAME] stop"))
	}
	if stopRequested {
		client := agent.NewAPIClient(cfg.Daemon.SocketPath, 5*time.Second)
		stopped, err := stopDaemon(client)
		if err != nil {
			fatal(fmt.Errorf("stop daemon: %w", err))
		}
		if stopped {
			fmt.Fprintln(os.Stderr, "advent-agentd stopped")
		} else {
			fmt.Fprintln(os.Stderr, "advent-agentd is not running")
		}
		return
	}
	key := os.Getenv("DEEPSEEK_API_KEY")
	logger := agent.NewDebugLogger(cfg.Daemon, key)
	logger.Log("daemon.start", "", "", fmt.Sprintf("daemon starting; API key present=%t", key != ""))
	if err := os.MkdirAll(filepath.Dir(cfg.Daemon.SocketPath), 0700); err != nil {
		fatal(err)
	}
	_ = os.Chmod(filepath.Dir(cfg.Daemon.SocketPath), 0700)
	if info, err := os.Lstat(cfg.Daemon.SocketPath); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			fatal(fmt.Errorf("socket path exists and is not a socket: %s", cfg.Daemon.SocketPath))
		}
		if existing, dialErr := net.DialTimeout("unix", cfg.Daemon.SocketPath, 250*time.Millisecond); dialErr == nil {
			_ = existing.Close()
			fatal(fmt.Errorf("advent-agentd is already running at %s", cfg.Daemon.SocketPath))
		}
		_ = os.Remove(cfg.Daemon.SocketPath)
	}
	ln, err := net.Listen("unix", cfg.Daemon.SocketPath)
	if err != nil {
		fatal(err)
	}
	_ = os.Chmod(cfg.Daemon.SocketPath, 0600)
	httpClient := &http.Client{Timeout: time.Duration(cfg.Daemon.RequestTimeout)}
	llm := &agent.DeepSeekClient{HTTP: httpClient, Endpoint: cfg.DeepSeek.Endpoint, APIKey: key, Logger: logger}
	a, err := agent.NewAgent(cfg, llm, logger)
	if err != nil {
		fatal(err)
	}
	shutdownRequested := make(chan struct{}, 1)
	handler := agent.Server{Agent: a, Shutdown: func() {
		select {
		case shutdownRequested <- struct{}{}:
		default:
		}
	}}
	server := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fatal(err)
		}
	}()
	fmt.Fprintf(os.Stderr, "advent-agentd listening on %s\n", cfg.Daemon.SocketPath)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signals:
	case <-shutdownRequested:
	}
	logger.Log("daemon.shutdown", "", "", "graceful shutdown")
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	_ = server.Shutdown(shutdownContext)
	cancelShutdown()
	a.Close()
	_ = os.Remove(cfg.Daemon.SocketPath)
}
func stopDaemon(client *agent.APIClient) (bool, error) {
	err := client.Shutdown()
	if errors.Is(err, agent.ErrDaemonUnavailable) {
		return false, nil
	}
	return err == nil, err
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
