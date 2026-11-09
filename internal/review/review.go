// Package review defines the persisted, UI-independent review snapshot.
package review

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

const SchemaVersion = 1

type Comparison struct {
	Repository string `json:"repository"`
	Target     string `json:"target"`
	Base       string `json:"base"`
	TargetID   string `json:"target_id"`
	BaseID     string `json:"base_id"`
	Mode       string `json:"mode"`
}

// Snapshot retains the entire patch, including metadata-only and binary changes.
// Parsed hunk references and guides will be added in a subsequent milestone.
type Snapshot struct {
	ID         string     `json:"id"`
	Comparison Comparison `json:"comparison"`
	Patch      string     `json:"patch"`
}

type Session struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	Snapshot      Snapshot  `json:"snapshot"`
}

func New(comparison Comparison, patch string) Session {
	snapshot := Snapshot{Comparison: comparison, Patch: patch}
	content, _ := json.Marshal(snapshot)
	digest := sha256.Sum256(content)
	snapshot.ID = hex.EncodeToString(digest[:])
	return Session{
		SchemaVersion: SchemaVersion,
		ID:            snapshot.ID,
		CreatedAt:     time.Now().UTC(),
		Snapshot:      snapshot,
	}
}
