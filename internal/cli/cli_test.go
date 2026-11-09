package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncphillips/rex-diff-guide/internal/review"
	"github.com/ncphillips/rex-diff-guide/internal/session"
)

func TestListAndInspectWithoutRepository(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RGD_STATE_DIR", dir)
	t.Chdir(t.TempDir())
	value := review.New(review.Comparison{BaseID: strings.Repeat("a", 40), TargetID: strings.Repeat("b", 40)}, "complete patch\n")
	if _, err := (session.Store{Dir: dir}).Save(value); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"list", "--json"}, {"inspect", "--json", value.ID}, {"inspect", value.ID}} {
		var out, stderr bytes.Buffer
		if err := Run(context.Background(), args, &out, &stderr); err != nil {
			t.Fatal(err)
		}
		if args[1] == "--json" {
			if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), value.ID) {
				t.Fatalf("invalid machine output: %s", &out)
			}
		} else if !strings.Contains(out.String(), "complete patch") {
			t.Fatalf("patch missing: %s", &out)
		}
	}
}

func TestGuideCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RGD_STATE_DIR", dir)
	store := session.Store{Dir: dir}
	value := review.New(review.Comparison{}, "diff --git a/a b/a\nold mode 100644\nnew mode 100755\n")
	if _, err := store.Save(value); err != nil {
		t.Fatal(err)
	}
	guide := review.Guide{SchemaVersion: 1, SnapshotID: value.ID, Steps: []review.Step{{Title: "Permissions", Explanation: "Make the file executable.", Hunks: []string{review.Changes(value.Snapshot.Patch)[0].ID}}}}
	data, err := json.Marshal(guide)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "guide.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"guide", "set", value.ID, "--file", path}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"inspect", "--json", value.ID}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	var inspected struct {
		Changes []review.Change `json:"changes"`
		Guide   *review.Guide   `json:"guide"`
	}
	if err := json.Unmarshal(out.Bytes(), &inspected); err != nil {
		t.Fatal(err)
	}
	if len(inspected.Changes) != 1 || inspected.Guide == nil || inspected.Guide.Steps[0].Title != "Permissions" {
		t.Fatalf("agent inspection missing guide/index: %s", &out)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"open", "--plain", value.ID}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "new mode") {
		t.Fatal("saved snapshot did not reopen")
	}
}

func TestUsageAndArguments(t *testing.T) {
	for _, tc := range []struct {
		args []string
		fail bool
	}{
		{[]string{"--help"}, false},
		{[]string{"list", "extra"}, true},
		{[]string{"inspect"}, true},
		{[]string{"open"}, true},
		{[]string{"guide", "set"}, true},
		{[]string{"--plain", "--json"}, true},
		{[]string{"a", "b", "c"}, true},
		{[]string{"--unknown"}, true},
	} {
		var out, stderr bytes.Buffer
		err := Run(context.Background(), tc.args, &out, &stderr)
		if (err != nil) != tc.fail {
			t.Errorf("args %v: unexpected error %v", tc.args, err)
		}
	}
}
