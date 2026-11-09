// Package session stores private, immutable review snapshots outside the repo.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

type Store struct{ Dir string }

var validID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Default() (Store, error) {
	if dir := os.Getenv("RGD_STATE_DIR"); dir != "" {
		return Store{Dir: dir}, nil
	}
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return Store{Dir: filepath.Join(dir, "rgd", "sessions")}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Store{}, err
	}
	return Store{Dir: filepath.Join(home, ".local", "state", "rgd", "sessions")}, nil
}

func (s Store) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid review ID: expected a full 64-character hexadecimal ID")
	}
	return filepath.Join(s.Dir, id+".json"), nil
}

// Save publishes a complete file with a hard link, so concurrent writers cannot
// expose partial JSON or overwrite an existing immutable snapshot.
func (s Store) Save(value review.Session) (review.Session, error) {
	path, err := s.path(value.ID)
	if err != nil {
		return review.Session{}, err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return review.Session{}, err
	}
	tmp, err := os.CreateTemp(s.Dir, ".snapshot-*")
	if err != nil {
		return review.Session{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := json.NewEncoder(tmp).Encode(value); err != nil {
		return review.Session{}, err
	}
	if err := tmp.Sync(); err != nil {
		return review.Session{}, err
	}
	if err := tmp.Close(); err != nil {
		return review.Session{}, err
	}
	if err := os.Link(tmp.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return review.Session{}, err
	}
	return s.Load(value.ID)
}

func (s Store) Load(id string) (review.Session, error) {
	path, err := s.path(id)
	if err != nil {
		return review.Session{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return review.Session{}, err
	}
	var value review.Session
	if err := json.Unmarshal(data, &value); err != nil {
		return review.Session{}, fmt.Errorf("read review %s: %w", id, err)
	}
	if value.SchemaVersion != review.SchemaVersion {
		return review.Session{}, fmt.Errorf("unsupported review schema version %d", value.SchemaVersion)
	}
	expected := review.New(value.Snapshot.Comparison, value.Snapshot.Patch)
	if value.ID != id || value.Snapshot.ID != expected.Snapshot.ID || value.ID != value.Snapshot.ID {
		return review.Session{}, fmt.Errorf("review %s failed snapshot integrity validation", id)
	}
	return value, nil
}

func (s Store) List() ([]review.Session, error) {
	values := []review.Session{}
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		value, err := s.Load(id)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].CreatedAt.Equal(values[j].CreatedAt) {
			return values[i].ID < values[j].ID
		}
		return values[i].CreatedAt.After(values[j].CreatedAt)
	})
	return values, nil
}
