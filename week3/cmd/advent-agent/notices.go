package main

import (
	agent "advent-agent"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type lineEditor struct {
	mu     sync.Mutex
	active bool
	prompt string
	input  []rune
}

func (e *lineEditor) display(n agent.Notice) {
	message := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, n.Text)
	if len(message) > 500 {
		message = message[:500] + "…"
	}
	info, err := os.Stdout.Stat()
	tty := err == nil && info.Mode()&os.ModeCharDevice != 0
	_, noColor := os.LookupEnv("NO_COLOR")
	if tty && !noColor && os.Getenv("TERM") != "dumb" {
		message = "\x1b[90m" + message + "\x1b[0m"
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active && tty {
		fmt.Print("\r\x1b[2K")
	}
	fmt.Println(message)
	if e.active {
		fmt.Print(e.prompt, string(e.input))
	}
}
func (e *lineEditor) read(reader *bufio.Reader, prompt string) (string, error) {
	info, err := os.Stdin.Stat()
	tty := err == nil && info.Mode()&os.ModeCharDevice != 0
	if !tty {
		fmt.Print(prompt)
		return reader.ReadString('\n')
	}
	old, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	if err != nil {
		fmt.Print(prompt)
		return reader.ReadString('\n')
	}
	next := *old
	next.Lflag &^= unix.ICANON | unix.ECHO
	next.Cc[unix.VMIN] = 1
	next.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, &next); err != nil {
		fmt.Print(prompt)
		return reader.ReadString('\n')
	}
	defer unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, old)
	e.mu.Lock()
	e.active = true
	e.prompt = prompt
	e.input = nil
	fmt.Print(prompt)
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.active = false; e.mu.Unlock() }()
	for {
		b, _, err := reader.ReadRune()
		if err != nil {
			return "", err
		}
		e.mu.Lock()
		switch b {
		case '\r', '\n':
			line := string(e.input)
			fmt.Println()
			e.mu.Unlock()
			return line + "\n", nil
		case 4:
			if len(e.input) == 0 {
				e.mu.Unlock()
				return "", os.ErrClosed
			}
		case 8, 127:
			if len(e.input) > 0 {
				e.input = e.input[:len(e.input)-1]
				fmt.Print("\b \b")
			}
		case 3:
			e.mu.Unlock()
			return "", context.Canceled
		case 27: // Ignore terminal escape sequences such as arrow keys.
			e.mu.Unlock()
			_, _, _ = reader.ReadRune()
			_, _, _ = reader.ReadRune()
			continue
		default:
			if b >= 32 && b != 127 {
				e.input = append(e.input, b)
				fmt.Printf("%c", b)
			}
		}
		e.mu.Unlock()
	}
}
func streamNotices(api *agent.APIClient, editor *lineEditor) func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			body, err := api.Events(ctx)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
					continue
				}
			}
			scanner := bufio.NewScanner(body)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				var n agent.Notice
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &n) == nil {
					editor.display(n)
				}
			}
			body.Close()
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
	}()
	return cancel
}
