package review

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// Change is an addressable hunk, or a whole-file change for metadata/binary diffs.
// Line is a zero-based offset into the immutable patch, not a source line number.
type Change struct {
	ID   string `json:"id"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func Changes(patch string) []Change {
	changes := []Change{}
	lines := strings.Split(strings.TrimSuffix(patch, "\n"), "\n")
	for start := 0; start < len(lines); {
		if !strings.HasPrefix(lines[start], "diff --git ") {
			start++
			continue
		}
		end := start + 1
		for end < len(lines) && !strings.HasPrefix(lines[end], "diff --git ") {
			end++
		}
		name := strings.TrimPrefix(lines[start], "diff --git ")
		var positions []int
		for i := start + 1; i < end; i++ {
			if len(positions) == 0 && strings.HasPrefix(lines[i], "--- a/") {
				name = strings.TrimPrefix(lines[i], "--- a/")
			}
			if len(positions) == 0 && strings.HasPrefix(lines[i], "+++ b/") {
				name = strings.TrimPrefix(lines[i], "+++ b/")
			}
			if strings.HasPrefix(lines[i], "@@ ") {
				positions = append(positions, i)
			}
		}
		if len(positions) == 0 {
			positions = []int{start}
		}
		for i, position := range positions {
			stop := end
			if i+1 < len(positions) {
				stop = positions[i+1]
			}
			digest := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", name, position, strings.Join(lines[position:stop], "\n"))))
			changes = append(changes, Change{ID: fmt.Sprintf("%x", digest), File: name, Line: position})
		}
		start = end
	}
	return changes
}

type Step struct {
	Title       string   `json:"title"`
	Explanation string   `json:"explanation"`
	Hunks       []string `json:"hunks"`
}

type Guide struct {
	SchemaVersion int    `json:"schema_version"`
	SnapshotID    string `json:"snapshot_id"`
	Steps         []Step `json:"steps"`
}

func (g Guide) Validate(value Session) error {
	if g.SchemaVersion != 1 {
		return fmt.Errorf("unsupported guide schema version %d", g.SchemaVersion)
	}
	if g.SnapshotID != value.Snapshot.ID {
		return fmt.Errorf("stale guide: snapshot ID does not match review")
	}
	if len(g.Steps) == 0 {
		return fmt.Errorf("guide must contain at least one step")
	}
	known := map[string]bool{}
	for _, change := range Changes(value.Snapshot.Patch) {
		known[change.ID] = true
	}
	for i, step := range g.Steps {
		if strings.TrimSpace(step.Title) == "" || strings.TrimSpace(step.Explanation) == "" || len(step.Hunks) == 0 {
			return fmt.Errorf("step %d needs a title, explanation, and hunk references", i+1)
		}
		for _, id := range step.Hunks {
			if !known[id] {
				return fmt.Errorf("step %d references unknown hunk %q", i+1, id)
			}
		}
	}
	return nil
}
