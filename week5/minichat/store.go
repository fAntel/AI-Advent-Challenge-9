// Package minichat implements the standalone, session-local RAG conversation.
package minichat

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"golang.org/x/sys/unix"
	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	path string
}

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}
func Open(path string) (*Store, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version > 1 {
		db.Close()
		return nil, fmt.Errorf("unsupported chat database version %d", version)
	}
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
 CREATE TABLE IF NOT EXISTS mini_sessions (id TEXT PRIMARY KEY, created TEXT NOT NULL, updated TEXT NOT NULL, data TEXT NOT NULL CHECK(json_valid(data)));
 PRAGMA user_version=1;`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, path: path}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Create(settings Settings) (*Session, error) {
	now := time.Now().UTC()
	v := &Session{Version: 1, ID: ID(), Created: now, Updated: now, Settings: settings, Memory: Memory{Constraints: map[string]Fact{}, Definitions: map[string]Fact{}}}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec("INSERT INTO mini_sessions(id,created,updated,data) VALUES(?,?,?,?)", v.ID, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), string(data))
	return v, err
}
func (s *Store) Load(id string) (*Session, error) {
	var data string
	err := s.db.QueryRow("SELECT data FROM mini_sessions WHERE id=?", id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	var v Session
	if err = json.Unmarshal([]byte(data), &v); err != nil {
		return nil, err
	}
	if v.Version != 1 {
		return nil, fmt.Errorf("unsupported mini-chat session version %d", v.Version)
	}
	return &v, nil
}
func (s *Store) Save(v *Session) error {
	v.Updated = time.Now().UTC()
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	result, err := s.db.Exec("UPDATE mini_sessions SET updated=?,data=? WHERE id=?", v.Updated.Format(time.RFC3339Nano), string(data), v.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return errors.New("session disappeared")
	}
	return err
}
func (s *Store) List() ([]Session, error) {
	rows, err := s.db.Query("SELECT data FROM mini_sessions ORDER BY updated DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var v Session
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// The OS releases the lock even if a process is killed. Hold across remote calls,
// without holding a SQLite write transaction or blocking other sessions.
func (s *Store) lock(id string) (func(), error) {
	if !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(id) {
		return nil, errors.New("invalid session ID")
	}
	f, err := os.OpenFile(s.path+"."+id+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("session is busy in another process")
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}
