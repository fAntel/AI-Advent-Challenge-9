package agent

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	ini "gopkg.in/ini.v1"
)

const (
	MemoryWorking    = "working"
	MemoryPreference = "preference"
	MemorySolution   = "solution"
	MemoryKnowledge  = "knowledge"
)

var memoryKeyRE = regexp.MustCompile(`^[a-z0-9._-]+$`)

type MemoryStorage interface {
	Load(path string, allowedSections ...string) (map[string]map[string]string, error)
	Save(path string, values map[string]map[string]string) error
}
type INIMemoryStorage struct{}

func normalizeMemoryKey(key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" || !memoryKeyRE.MatchString(key) || strings.ContainsAny(key, "[]=") {
		return "", fmt.Errorf("invalid memory key %q", key)
	}
	return key, nil
}
func validateMemoryValue(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("memory value is empty")
	}
	if !utf8.ValidString(value) {
		return errors.New("memory value is not valid UTF-8")
	}
	for _, r := range value {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			return errors.New("memory value must be single-line text without control characters")
		}
	}
	return nil
}

func (INIMemoryStorage) Load(path string, allowedSections ...string) (map[string]map[string]string, error) {
	want := map[string]bool{}
	out := map[string]map[string]string{}
	for _, s := range allowedSections {
		want[s] = true
		out[s] = map[string]string{}
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	cfg, err := ini.LoadSources(ini.LoadOptions{Insensitive: false, AllowBooleanKeys: false, AllowShadows: true, IgnoreInlineComment: true}, data)
	if err != nil {
		return nil, fmt.Errorf("parse memory file %q: %w", path, err)
	}
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == ini.DEFAULT_SECTION && len(section.Keys()) == 0 {
			continue
		}
		if !want[name] {
			return nil, fmt.Errorf("parse memory file %q: unexpected section %q", path, name)
		}
		for _, key := range section.Keys() {
			normalized, e := normalizeMemoryKey(key.Name())
			if e != nil {
				return nil, fmt.Errorf("parse memory file %q: %w", path, e)
			}
			if len(key.ValueWithShadows()) > 1 {
				return nil, fmt.Errorf("parse memory file %q: duplicate key %q", path, key.Name())
			}
			if _, exists := out[name][normalized]; exists {
				return nil, fmt.Errorf("parse memory file %q: duplicate normalized key %q", path, normalized)
			}
			value := key.Value()
			if e := validateMemoryValue(value); e != nil {
				return nil, fmt.Errorf("parse memory file %q: key %q: %w", path, key.Name(), e)
			}
			out[name][normalized] = value
		}
	}
	return out, nil
}

func (INIMemoryStorage) Save(path string, values map[string]map[string]string) error {
	for section, entries := range values {
		switch section {
		case "working", "preferences", "solutions", "knowledge", "invariants":
		default:
			return fmt.Errorf("invalid memory section %q", section)
		}
		for key, value := range entries {
			normalized, err := normalizeMemoryKey(key)
			if err != nil {
				return err
			}
			if normalized != key {
				return fmt.Errorf("memory key %q is not normalized", key)
			}
			if err := validateMemoryValue(value); err != nil {
				return fmt.Errorf("memory key %q: %w", key, err)
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Dir(path), 0700)
	var sections []string
	for section := range values {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	var b bytes.Buffer
	for i, section := range sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[%s]\n", section)
		keys := make([]string, 0, len(values[section]))
		for key := range values[section] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s = %s\n", key, values[section][key])
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memory-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(b.Bytes())
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}

type MemoryManager struct {
	Root    string
	Storage MemoryStorage
}

func (m MemoryManager) EnsureProject(profile, project, path string) error {
	dir := filepath.Join(m.profileRoot(profile), "projects", project)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0700)
	target := filepath.Join(dir, "project.json")
	if data, err := os.ReadFile(target); err == nil {
		var saved struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(data, &saved) != nil {
			return fmt.Errorf("decode project metadata %q", target)
		}
		if saved.Path != path {
			return fmt.Errorf("project id collision for %q", project)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, _ := json.MarshalIndent(struct {
		Path string `json:"path"`
	}{path}, "", "  ")
	data = append(data, '\n')
	tmp, err := os.CreateTemp(dir, ".project-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, target)
}

func (m MemoryManager) storage() MemoryStorage {
	if m.Storage == nil {
		return INIMemoryStorage{}
	}
	return m.Storage
}
func (m MemoryManager) profileRoot(profile string) string {
	return filepath.Join(m.Root, "profiles", profile)
}
func (m MemoryManager) LongTermPath(profile string) string {
	return filepath.Join(m.profileRoot(profile), "long-term.ini")
}
func (m MemoryManager) WorkingPath(profile, project string) string {
	return filepath.Join(m.profileRoot(profile), "projects", project, "working.ini")
}
func (m MemoryManager) InvariantsPath(profile, project string) string {
	return filepath.Join(m.profileRoot(profile), "projects", project, "invariants.ini")
}

func (m MemoryManager) Invariants(profile, project string) (map[string]string, error) {
	values, err := m.storage().Load(m.InvariantsPath(profile, project), "invariants")
	if err != nil {
		return nil, err
	}
	return values["invariants"], nil
}

func (m MemoryManager) MutateInvariant(profile, project, action, key, value string) error {
	key, err := normalizeMemoryKey(key)
	if err != nil {
		return fmt.Errorf("invalid invariant key: %w", err)
	}
	if action == "create" || action == "edit" {
		if err := validateMemoryValue(value); err != nil {
			return fmt.Errorf("invalid invariant rule: %w", err)
		}
	}
	path := m.InvariantsPath(profile, project)
	data, err := m.storage().Load(path, "invariants")
	if err != nil {
		return err
	}
	_, exists := data["invariants"][key]
	switch action {
	case "create":
		if exists {
			return fmt.Errorf("invariant %q already exists", key)
		}
		data["invariants"][key] = value
	case "edit":
		if !exists {
			return fmt.Errorf("invariant %q does not exist", key)
		}
		data["invariants"][key] = value
	case "delete":
		if !exists {
			return fmt.Errorf("invariant %q does not exist", key)
		}
		delete(data["invariants"], key)
	default:
		return fmt.Errorf("unknown invariant action %q", action)
	}
	return m.storage().Save(path, data)
}
func memoryLocation(scope string) (fileKind, section string, err error) {
	switch scope {
	case MemoryWorking:
		return "working", "working", nil
	case MemoryPreference:
		return "long", "preferences", nil
	case MemorySolution:
		return "long", "solutions", nil
	case MemoryKnowledge:
		return "long", "knowledge", nil
	}
	return "", "", fmt.Errorf("unknown memory scope %q", scope)
}
func (m MemoryManager) loadFor(scope, profile, project string) (string, map[string]map[string]string, string, error) {
	kind, section, err := memoryLocation(scope)
	if err != nil {
		return "", nil, "", err
	}
	if kind == "working" {
		path := m.WorkingPath(profile, project)
		v, e := m.storage().Load(path, "working")
		return path, v, section, e
	}
	path := m.LongTermPath(profile)
	v, e := m.storage().Load(path, "preferences", "solutions", "knowledge")
	return path, v, section, e
}

func (m MemoryManager) View(profile, project string) (MemoryView, error) {
	working, err := m.storage().Load(m.WorkingPath(profile, project), "working")
	if err != nil {
		return MemoryView{}, err
	}
	long, err := m.storage().Load(m.LongTermPath(profile), "preferences", "solutions", "knowledge")
	if err != nil {
		return MemoryView{}, err
	}
	return MemoryView{Working: working["working"], Preferences: long["preferences"], Solutions: long["solutions"], Knowledge: long["knowledge"]}, nil
}
func (m MemoryManager) Create(profile, project, scope, key, value string) error {
	return m.mutate(profile, project, "create", scope, key, value, "")
}
func (m MemoryManager) Edit(profile, project, scope, key, value string) error {
	return m.mutate(profile, project, "edit", scope, key, value, "")
}
func (m MemoryManager) Delete(profile, project, scope, key string) error {
	return m.mutate(profile, project, "delete", scope, key, "", "")
}
func (m MemoryManager) Move(profile, project, source, key, destination string) error {
	return m.mutate(profile, project, "move", source, key, "", destination)
}
func (m MemoryManager) mutate(profile, project, action, scope, key, value, destination string) error {
	key, err := normalizeMemoryKey(key)
	if err != nil {
		return err
	}
	if action == "create" || action == "edit" {
		if err := validateMemoryValue(value); err != nil {
			return err
		}
	}
	path, data, section, err := m.loadFor(scope, profile, project)
	if err != nil {
		return err
	}
	current, exists := data[section][key]
	switch action {
	case "create":
		if exists {
			return fmt.Errorf("memory key %q already exists in %s", key, scope)
		}
		data[section][key] = value
		return m.storage().Save(path, data)
	case "edit":
		if !exists {
			return fmt.Errorf("memory key %q does not exist in %s", key, scope)
		}
		data[section][key] = value
		return m.storage().Save(path, data)
	case "delete":
		if !exists {
			return fmt.Errorf("memory key %q does not exist in %s", key, scope)
		}
		delete(data[section], key)
		return m.storage().Save(path, data)
	case "move":
		if !exists {
			return fmt.Errorf("memory key %q does not exist in %s", key, scope)
		}
	}
	destPath, destData, destSection, err := m.loadFor(destination, profile, project)
	if err != nil {
		return err
	}
	if _, exists := destData[destSection][key]; exists {
		return fmt.Errorf("memory key %q already exists in %s", key, destination)
	}
	if destPath == path {
		delete(data[section], key)
		data[destSection][key] = current
		return m.storage().Save(path, data)
	}
	destData[destSection][key] = current
	if err := m.storage().Save(destPath, destData); err != nil {
		return err
	}
	delete(data[section], key)
	if err := m.storage().Save(path, data); err != nil {
		delete(destData[destSection], key)
		_ = m.storage().Save(destPath, destData)
		return err
	}
	return nil
}

func ProjectIdentity(dir string) (string, string, error) {
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		canonical, err = filepath.Abs(dir)
	}
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command("git", "-C", canonical, "rev-parse", "--show-toplevel")
	if output, e := cmd.Output(); e == nil {
		if root := strings.TrimSpace(string(output)); root != "" {
			canonical, err = filepath.EvalSymlinks(root)
			if err != nil {
				canonical = root
			}
		}
	}
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:16]), canonical, nil
}

func formatINIContext(sectionValues map[string]map[string]string) string {
	var b bytes.Buffer
	sections := make([]string, 0, len(sectionValues))
	for section := range sectionValues {
		if len(sectionValues[section]) > 0 {
			sections = append(sections, section)
		}
	}
	sort.Strings(sections)
	for i, section := range sections {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "[%s]\n", section)
		keys := make([]string, 0, len(sectionValues[section]))
		for key := range sectionValues[section] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s = %s\n", key, sectionValues[section][key])
		}
	}
	return strings.TrimSpace(b.String())
}
