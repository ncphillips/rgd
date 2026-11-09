package jj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (context.Context, string, func(...string) string) {
	t.Helper()
	if _, err := exec.LookPath("jj"); err != nil {
		t.Skip("jj is not installed")
	}
	dir := t.TempDir()
	config := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(config, []byte("[user]\nname = 'Test'\nemail = 'test@example.com'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JJ_CONFIG", config)
	ctx := context.Background()
	run := func(args ...string) string {
		t.Helper()
		out, err := command(ctx, dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out)
	}
	run("git", "init", ".")
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "example.txt"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("initial\n")
	run("commit", "-m", "initial")
	run("bookmark", "create", "main", "-r", "@-")
	write("initial\nfirst change\n")
	run("commit", "-m", "first change")
	run("bookmark", "create", "feature", "-r", "@-")
	write("initial\nfirst change\nsecond change\n")
	return ctx, dir, run
}

func TestComparisonDefaultsAndExplicitOrder(t *testing.T) {
	ctx, dir, run := fixture(t)
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	current := run("log", "-r", "@", "--no-graph", "-T", "commit_id")
	previous := run("log", "-r", "@-", "--no-graph", "-T", "commit_id")
	main := run("log", "-r", "main", "--no-graph", "-T", "commit_id")
	for _, tc := range []struct {
		name, base, target, mode, added string
		args                            []string
	}{
		{"current", previous, current, "change", "+second change", nil},
		{"previous", main, previous, "change", "+first change", []string{"@-"}},
		{"bookmark", main, previous, "bookmark", "+first change", []string{"feature"}},
		{"explicit", main, current, "explicit", "+second change", []string{"@", "main"}},
		{"reverse", current, previous, "explicit", "-second change", []string{"@-", "@"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := r.Capture(ctx, tc.args)
			if err != nil {
				t.Fatal(err)
			}
			c := value.Snapshot.Comparison
			if c.BaseID != tc.base || c.TargetID != tc.target || c.Mode != tc.mode {
				t.Fatalf("unexpected comparison: %+v", c)
			}
			if !strings.Contains(value.Snapshot.Patch, tc.added) {
				t.Fatalf("patch does not contain %q: %s", tc.added, value.Snapshot.Patch)
			}
		})
	}
}

func TestPinnedOperationIgnoresLaterChanges(t *testing.T) {
	ctx, dir, _ := fixture(t)
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	before, err := r.Capture(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "example.txt"), []byte("later edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	newRepo, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	after, err := r.Capture(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := newRepo.Capture(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before.ID != after.ID || before.ID == latest.ID {
		t.Fatal("pinned snapshot changed, or new edits were not captured")
	}
}

func TestRejectAmbiguousTargetAndMissingMain(t *testing.T) {
	ctx, dir, run := fixture(t)
	run("bookmark", "delete", "main")
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Capture(ctx, []string{"@ | @-"}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected ambiguous target error, got %v", err)
	}
	if _, err := r.Capture(ctx, []string{"feature"}); err == nil || !strings.Contains(err.Error(), "main bookmark") {
		t.Fatalf("expected missing main error, got %v", err)
	}
	if _, err := r.Capture(ctx, []string{"feature", "root()"}); err != nil {
		t.Fatalf("explicit base should work without main: %v", err)
	}
}

func TestMergeRequiresExplicitBase(t *testing.T) {
	ctx, dir, run := fixture(t)
	run("new", "main", "-m", "other branch")
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("new", "feature", "@", "-m", "merge")
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Capture(ctx, nil); err == nil || !strings.Contains(err.Error(), "explicit BASE") {
		t.Fatalf("expected merge base error, got %v", err)
	}
	if _, err := r.Capture(ctx, []string{"@", "main"}); err != nil {
		t.Fatal(err)
	}
}

func TestBookmarkUsesCommonAncestorNotMainTip(t *testing.T) {
	ctx, dir, run := fixture(t)
	ancestor := run("log", "-r", "main", "--no-graph", "-T", "commit_id")
	run("new", "main", "-m", "main diverges")
	if err := os.WriteFile(filepath.Join(dir, "main-only.txt"), []byte("not part of feature\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("bookmark", "set", "main", "-r", "@")
	r, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	value, err := r.Capture(ctx, []string{"feature"})
	if err != nil {
		t.Fatal(err)
	}
	if value.Snapshot.Comparison.BaseID != ancestor || strings.Contains(value.Snapshot.Patch, "main-only") {
		t.Fatalf("included unrelated changes from main: %+v", value.Snapshot)
	}
}
