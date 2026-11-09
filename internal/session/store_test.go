package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

func sample() review.Session {
	return review.New(review.Comparison{Repository: "/project", Target: "@", Base: "@-"}, "diff --git a/a b/a\n")
}

func TestSaveIsImmutableAndConcurrent(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "sessions")}
	value := sample()
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if _, err := store.Save(value); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	loaded, err := store.Load(value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != value {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	repeated := review.New(value.Snapshot.Comparison, value.Snapshot.Patch)
	saved, err := store.Save(repeated)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CreatedAt != value.CreatedAt {
		t.Fatal("repeated capture overwrote original creation time")
	}
	values, err := store.List()
	if err != nil || len(values) != 1 {
		t.Fatalf("expected one saved review, got %d, %v", len(values), err)
	}
	entries, err := os.ReadDir(store.Dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v, %v", entries, err)
	}
	info, err := os.Stat(filepath.Join(store.Dir, value.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot permissions: %v", info.Mode())
	}
}

func TestEmptyListAndInvalidIDs(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "missing")}
	values, err := store.List()
	if err != nil || values == nil || len(values) != 0 {
		t.Fatalf("expected empty non-nil list, got %v, %v", values, err)
	}
	for _, id := range []string{"../escape", "", "abc", "/absolute"} {
		if _, err := store.Load(id); err == nil {
			t.Fatalf("accepted invalid ID %q", id)
		}
	}
}

func TestRejectCorruptionAndFutureSchema(t *testing.T) {
	for _, kind := range []string{"content", "schema"} {
		t.Run(kind, func(t *testing.T) {
			store := Store{Dir: t.TempDir()}
			value := sample()
			if kind == "content" {
				value.Snapshot.Patch = "tampered"
			} else {
				value.SchemaVersion++
			}
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(store.Dir, value.ID+".json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(value.ID); err == nil {
				t.Fatal("accepted corrupt/unsupported snapshot")
			}
		})
	}
}
