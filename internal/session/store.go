package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func NewStore(dir string) *Store {
	return &Store{path: filepath.Join(dir, "sessions.json")}
}

func (s *Store) Save(sessions map[string]*Session) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("creating store dir: %w", err)
	}

	data, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sessions: %w", err)
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Load() (map[string]*Session, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*Session), nil
		}
		return nil, fmt.Errorf("reading sessions file: %w", err)
	}

	var sessions map[string]*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parsing sessions file: %w", err)
	}

	if sessions == nil {
		sessions = make(map[string]*Session)
	}
	return sessions, nil
}
