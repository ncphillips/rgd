package session

import (
	"testing"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

func TestGuidePublicationPreservesSnapshot(t *testing.T) {
	store := Store{Dir: t.TempDir()}
	value := sample()
	if _, err := store.Save(value); err != nil {
		t.Fatal(err)
	}
	if guide, err := store.LoadGuide(value); err != nil || guide != nil {
		t.Fatalf("expected no guide: %v, %v", guide, err)
	}
	guide := review.Guide{SchemaVersion: 1, SnapshotID: value.Snapshot.ID, Steps: []review.Step{{Title: "Start here", Explanation: "Metadata-only change.", Hunks: []string{review.Changes(value.Snapshot.Patch)[0].ID}}}}
	if err := store.SaveGuide(value.ID, guide); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadGuide(value)
	if err != nil || loaded.Steps[0].Title != "Start here" {
		t.Fatalf("guide missing: %v, %v", loaded, err)
	}
	guide.SnapshotID = "stale"
	if err := store.SaveGuide(value.ID, guide); err == nil {
		t.Fatal("accepted stale guide")
	}
	loaded, err = store.LoadGuide(value)
	if err != nil || loaded.SnapshotID != value.ID {
		t.Fatal("invalid submission replaced valid guide")
	}
	values, err := store.List()
	if err != nil || len(values) != 1 || values[0] != value {
		t.Fatalf("guide modified snapshot/list: %v, %v", values, err)
	}
	guide.SnapshotID = value.ID
	guide.Steps[0].Title = "Updated"
	if err := store.SaveGuide(value.ID, guide); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.LoadGuide(value)
	if err != nil || loaded.Steps[0].Title != "Updated" {
		t.Fatal("guide update was not published")
	}
}
