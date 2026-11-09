package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

func (s Store) guidePath(id string) (string, error) {
	if _, err := s.path(id); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, "guides", id+".json"), nil
}

func (s Store) LoadGuide(value review.Session) (*review.Guide, error) {
	path, err := s.guidePath(value.ID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var guide review.Guide
	if err := json.Unmarshal(data, &guide); err != nil {
		return nil, err
	}
	if err := guide.Validate(value); err != nil {
		return nil, err
	}
	return &guide, nil
}

// SaveGuide atomically replaces the guide; concurrent valid submissions are
// last-writer-wins. Immutable snapshots themselves are never modified.
func (s Store) SaveGuide(id string, guide review.Guide) error {
	value, err := s.Load(id)
	if err != nil {
		return err
	}
	if err := guide.Validate(value); err != nil {
		return err
	}
	path, err := s.guidePath(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".guide-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := json.NewEncoder(tmp).Encode(guide); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
