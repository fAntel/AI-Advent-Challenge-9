package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Store struct{ Dir string }

func (s Store) sessionsDir(profile string) string {
	if profile == "" {
		profile = DefaultProfile
	}
	return filepath.Join(s.Dir, "profiles", profile, "sessions")
}

func (s Store) Save(session *Session) error {
	dir := s.sessionsDir(session.Profile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0700)
	sessionDir := filepath.Join(dir, session.ID)
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(sessionDir, 0700)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(sessionDir, ".session-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err == nil {
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
	return os.Rename(name, filepath.Join(sessionDir, "session.json"))
}

func (s Store) Load(id string) (*Session, error) {
	if !validID(id) {
		return nil, os.ErrNotExist
	}
	path, err := s.find(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("decode session %s: %w", id, err)
	}
	return &session, nil
}
func (s Store) Delete(id string) error {
	if !validID(id) {
		return os.ErrNotExist
	}
	path, err := s.find(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return os.Remove(filepath.Dir(path))
}
func (s Store) List() ([]Session, error) {
	return s.ListProfile("")
}
func (s Store) ListProfile(profile string) ([]Session, error) {
	result := []Session{}
	dirs := []string{}
	if profile != "" {
		dirs = append(dirs, s.sessionsDir(profile))
	} else {
		profiles, err := os.ReadDir(filepath.Join(s.Dir, "profiles"))
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		for _, p := range profiles {
			if p.IsDir() {
				dirs = append(dirs, s.sessionsDir(p.Name()))
			}
		}
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() || !validID(e.Name()) {
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(dir, e.Name(), "session.json"))
			if readErr != nil {
				continue
			}
			var ss Session
			if json.Unmarshal(data, &ss) == nil {
				result = append(result, ss)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}
func (s Store) Recover() error {
	sessions, err := s.List()
	if err != nil {
		return err
	}
	for i := range sessions {
		if sessions[i].Operation != nil && sessions[i].Operation.State == "running" {
			sessions[i].Operation.State = "interrupted"
			sessions[i].Operation.Error = "daemon stopped while request was running"
			if err := s.Save(&sessions[i]); err != nil {
				return err
			}
		} else if sessions[i].Operation != nil && sessions[i].Operation.State == "compressing" {
			sessions[i].Operation.State = "completed"
			sessions[i].Operation.Warning = "daemon stopped while history compression was running; uncompressed messages were retained"
			if err := s.Save(&sessions[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s Store) find(id string) (string, error) {
	if !validID(id) {
		return "", os.ErrNotExist
	}
	matches, err := filepath.Glob(filepath.Join(s.Dir, "profiles", "*", "sessions", id, "session.json"))
	if err != nil || len(matches) != 1 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}
func validID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
