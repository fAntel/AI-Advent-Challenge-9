package agent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

var errProtected = errors.New("protected path")

const maxFileBytes = 256 << 10
const maxToolBytes = 32 << 10
const maxEntries = 20000

// ProjectFS resolves symlinks itself and opens every component with O_NOFOLLOW.
// Checking an absolute path and subsequently opening it would permit link swaps.
// Directory handles are pinned until the operation completes.
type ProjectFS struct {
	root             *os.File
	path, state      string
	runtime          []string
	extraReadBlocked []string
	readOnly         bool
}

func OpenProjectFS(root, state string, runtimePaths ...string) (*ProjectFS, error) {
	canonical, err := ProjectRoot(root)
	if err != nil {
		return nil, err
	}
	if canonical != root {
		return nil, fmt.Errorf("project directory changed; create a new session")
	}
	if state != "" {
		state, _ = filepath.Abs(state)
		if real, err := filepath.EvalSymlinks(state); err == nil {
			state = real
		}
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	fs := &ProjectFS{root: os.NewFile(uintptr(fd), root), path: root, state: state}
	for _, path := range runtimePaths {
		if path != "" {
			absolute, _ := filepath.Abs(path)
			if real, e := filepath.EvalSymlinks(absolute); e == nil {
				absolute = real
			}
			fs.runtime = append(fs.runtime, absolute)
		}
	}
	if fs.protected(nil) {
		fs.Close()
		return nil, fmt.Errorf("project overlaps daemon state")
	}
	return fs, nil
}
func (f *ProjectFS) Close() error { return f.root.Close() }
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func (f *ProjectFS) protected(parts []string) bool {
	for _, p := range parts {
		if strings.EqualFold(p, ".git") {
			return true
		}
	}
	full := filepath.Join(append([]string{f.path}, parts...)...)
	for _, blocked := range f.extraReadBlocked {
		if blocked != "" && within(strings.ToLower(blocked), strings.ToLower(full)) {
			return true
		}
	}
	return f.state != "" && within(strings.ToLower(f.state), strings.ToLower(full))
}
func safeName(path string) error {
	if path == "" || filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return fmt.Errorf("use a project-relative path")
	}
	for _, p := range strings.Split(filepath.ToSlash(path), "/") {
		if p == ".." || strings.EqualFold(p, ".git") {
			return fmt.Errorf("forbidden path component: %s", p)
		}
	}
	return nil
}

// parent returns an owned parent descriptor and a resolved basename. Symlink
// targets are processed relative to the containing directory, never the daemon cwd.
func (f *ProjectFS) parent(path string) (*os.File, string, string, error) {
	if err := safeName(path); err != nil {
		return nil, "", "", err
	}
	start, err := unix.Dup(int(f.root.Fd()))
	if err != nil {
		return nil, "", "", err
	}
	stack := []int{start}
	parts := []string{}
	defer func() {
		for _, fd := range stack {
			unix.Close(fd)
		}
	}()
	queue := strings.Split(filepath.ToSlash(path), "/")
	links := 0
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			if len(stack) == 1 {
				return nil, "", "", fmt.Errorf("symlink escapes project")
			}
			unix.Close(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
			parts = parts[:len(parts)-1]
			continue
		}
		candidate := append(append([]string{}, parts...), p)
		if f.protected(candidate) {
			return nil, "", "", errProtected
		}
		fd := stack[len(stack)-1]
		var st unix.Stat_t
		err := unix.Fstatat(fd, p, &st, unix.AT_SYMLINK_NOFOLLOW)
		if err == nil && st.Mode&unix.S_IFMT == unix.S_IFLNK {
			links++
			if links > 40 {
				return nil, "", "", fmt.Errorf("symlink cycle or excessive depth")
			}
			buf := make([]byte, 4096)
			n, e := unix.Readlinkat(fd, p, buf)
			if e != nil || n == len(buf) {
				return nil, "", "", fmt.Errorf("cannot resolve symlink")
			}
			target := string(buf[:n])
			if filepath.IsAbs(target) {
				if !within(f.path, target) {
					return nil, "", "", fmt.Errorf("symlink escapes project")
				}
				target, _ = filepath.Rel(f.path, target)
				for len(stack) > 1 {
					unix.Close(stack[len(stack)-1])
					stack = stack[:len(stack)-1]
				}
				parts = nil
			}
			queue = append(strings.Split(filepath.ToSlash(target), "/"), queue...)
			continue
		}
		if len(queue) == 0 {
			if err != nil && err != unix.ENOENT {
				return nil, "", "", err
			}
			dup, e := unix.Dup(fd)
			if e != nil {
				return nil, "", "", e
			}
			return os.NewFile(uintptr(dup), "parent"), p, filepath.Join(candidate...), nil
		}
		if err != nil {
			return nil, "", "", err
		}
		next, e := unix.Openat(fd, p, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return nil, "", "", e
		}
		stack = append(stack, next)
		parts = candidate
	}
	dup, e := unix.Dup(stack[len(stack)-1])
	if e != nil {
		return nil, "", "", e
	}
	return os.NewFile(uintptr(dup), "parent"), ".", filepath.Join(parts...), nil
}
func (f *ProjectFS) open(path string, directory bool) (*os.File, string, error) {
	parent, name, resolved, err := f.parent(path)
	if err != nil {
		return nil, "", err
	}
	defer parent.Close()
	flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Openat(int(parent.Fd()), name, flags, 0)
	if err != nil {
		return nil, "", err
	}
	file := os.NewFile(uintptr(fd), resolved)
	info, err := file.Stat()
	if err != nil || (!directory && !info.Mode().IsRegular()) {
		file.Close()
		return nil, "", fmt.Errorf("not a regular file")
	}
	return file, resolved, nil
}
func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
func readText(file *os.File) (string, error) {
	b, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxFileBytes {
		return "", fmt.Errorf("file exceeds %d bytes", maxFileBytes)
	}
	if !utf8.Valid(b) || strings.ContainsRune(string(b), 0) {
		return "", fmt.Errorf("not UTF-8 text")
	}
	return string(b), nil
}
func (f *ProjectFS) Read(path string) (string, string, error) {
	file, resolved, err := f.open(path, false)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	text, err := readText(file)
	return text, resolved, err
}

var skippedDirs = map[string]bool{".git": true, ".venv": true, "node_modules": true, "__pycache__": true, "data": true}

func (f *ProjectFS) Files(ctx context.Context) ([]string, error) {
	files := []string{}
	seenFiles := map[string]bool{}
	visited := map[string]bool{}
	count := 0
	var walk func(string) error
	walk = func(path string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.Count(path, string(filepath.Separator)) > 128 {
			return fmt.Errorf("directory nesting exceeds 128")
		}
		dir, resolved, err := f.open(path, true)
		if err != nil {
			return err
		}
		defer dir.Close()
		if visited[resolved] {
			return nil
		}
		visited[resolved] = true
		entries, err := dir.ReadDir(maxEntries + 1)
		if err != nil && err != io.EOF {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			count++
			if count > maxEntries {
				return fmt.Errorf("project exceeds %d entries", maxEntries)
			}
			if skippedDirs[strings.ToLower(entry.Name())] {
				continue
			}
			child := filepath.Join(resolved, entry.Name())
			if entry.IsDir() {
				if err := walk(child); err != nil && !errors.Is(err, errProtected) {
					return err
				}
				continue
			}
			file, filePath, err := f.open(child, false)
			if err == nil {
				file.Close()
				if !seenFiles[filePath] {
					files = append(files, filePath)
					seenFiles[filePath] = true
				}
				continue
			}
			// Safe directory symlinks are visited once; protected and escaping links skipped.
			if entry.Type()&os.ModeSymlink != 0 {
				if err := walk(child); err != nil && ctx.Err() != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk("."); err != nil {
		return nil, err
	}
	if count > maxEntries {
		return nil, fmt.Errorf("project exceeds %d entries", maxEntries)
	}
	sort.Strings(files)
	return files, nil
}

type Change struct {
	Path       string `json:"path"`
	BeforeHash string `json:"before_hash"`
	AfterHash  string `json:"after_hash"`
	Diff       string `json:"diff"`
	Status     string `json:"status"`
}

// Whole-file unified hunks are deliberately simple, deterministic, and applicable.
func unifiedDiff(path, before, after string, created bool) string {
	if before == after && !created {
		return ""
	}
	old := path
	if created {
		old = "/dev/null"
	} else {
		old = "a/" + path
	}
	lines := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.SplitAfter(s, "\n")
	}
	a, b := lines(before), lines(after)
	if len(a) > 0 && a[len(a)-1] == "" {
		a = a[:len(a)-1]
	}
	if len(b) > 0 && b[len(b)-1] == "" {
		b = b[:len(b)-1]
	}
	startA, startB := 1, 1
	if len(a) == 0 {
		startA = 0
	}
	if len(b) == 0 {
		startB = 0
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ b/%s\n@@ -%d,%d +%d,%d @@\n", old, path, startA, len(a), startB, len(b))
	for i, ls := range [][]string{a, b} {
		prefix := "-"
		if i == 1 {
			prefix = "+"
		}
		for _, line := range ls {
			out.WriteString(prefix + line)
			if !strings.HasSuffix(line, "\n") {
				out.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return out.String()
}
func (f *ProjectFS) Write(ctx context.Context, path, content, expected string, create bool, beforeWrite func(Change) error) (Change, error) {
	if f.readOnly {
		return Change{}, fmt.Errorf("filesystem is read-only")
	}
	var change Change
	if len(content) > maxFileBytes || !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return change, fmt.Errorf("content must be UTF-8 text within %d bytes", maxFileBytes)
	}
	parent, name, resolved, err := f.parent(path)
	if err != nil {
		return change, err
	}
	defer parent.Close()
	for _, runtimePath := range f.runtime {
		if strings.EqualFold(filepath.Join(f.path, resolved), runtimePath) {
			return change, fmt.Errorf("cannot edit an active runtime helper")
		}
	}
	if name == "." {
		return change, fmt.Errorf("cannot replace directory")
	}
	readCurrent := func() (string, os.FileMode, error) {
		fd, e := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if e != nil {
			return "", 0, e
		}
		file := os.NewFile(uintptr(fd), name)
		defer file.Close()
		info, e := file.Stat()
		if e != nil {
			return "", 0, e
		}
		if !info.Mode().IsRegular() {
			return "", 0, fmt.Errorf("not a regular file")
		}
		text, e := readText(file)
		return text, info.Mode().Perm(), e
	}
	before, mode, err := readCurrent()
	if create {
		if !os.IsNotExist(err) {
			return change, fmt.Errorf("create requires a nonexistent file")
		}
		mode = 0644
		before = ""
	} else {
		if err != nil {
			return change, err
		}
		if expected == "" || expected != hashText(before) {
			return change, fmt.Errorf("file changed or missing expected_hash; read it again")
		}
	}
	change = Change{Path: resolved, BeforeHash: hashText(before), AfterHash: hashText(content), Diff: unifiedDiff(resolved, before, content, create), Status: "intent"}
	if create {
		change.BeforeHash = ""
	}
	if err = beforeWrite(change); err != nil {
		return change, err
	}
	if err = ctx.Err(); err != nil {
		return change, err
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		return change, err
	}
	temp := ".advent-write-" + hex.EncodeToString(random)
	fd, err := unix.Openat(int(parent.Fd()), temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return change, err
	}
	defer unix.Unlinkat(int(parent.Fd()), temp, 0)
	file := os.NewFile(uintptr(fd), temp)
	_, err = file.WriteString(content)
	if err == nil {
		err = file.Chmod(mode)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return change, err
	}
	if err = ctx.Err(); err != nil {
		return change, err
	}
	if create {
		// linkat provides atomic no-clobber publication; the temporary inode is new.
		err = unix.Linkat(int(parent.Fd()), temp, int(parent.Fd()), name, 0)
	} else {
		current, _, e := readCurrent()
		if e != nil {
			return change, e
		}
		if hashText(current) != expected {
			return change, fmt.Errorf("file changed during write; read it again")
		}
		err = unix.Renameat(int(parent.Fd()), temp, int(parent.Fd()), name)
	}
	if err != nil {
		return change, err
	}
	change.Status = "saved"
	return change, nil
}
