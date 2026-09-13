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

func (s Store) Save(session *Session) error {
	if err := os.MkdirAll(s.Dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(s.Dir, 0700)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.Dir, ".session-*.tmp")
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
	return os.Rename(name, s.path(session.ID))
}

func (s Store) Load(id string) (*Session, error) {
	if !validID(id) {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(s.path(id))
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
	return os.Remove(s.path(id))
}
func (s Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	result := []Session{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		ss, err := s.Load(id)
		if err == nil {
			result = append(result, *ss)
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
		}
	}
	return nil
}
func (s Store) path(id string) string { return filepath.Join(s.Dir, id+".json") }
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
