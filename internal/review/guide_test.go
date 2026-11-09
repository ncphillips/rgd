package review

import "testing"

func TestChangeIndexAndGuideValidation(t *testing.T) {
	patch := "diff --git a/a b/a\n--- a/a\n+++ b/a\n@@ -1 +1 @@\n-old\n+new\n@@ -10 +10 @@\n-before\n+after\ndiff --git a/b b/b\nold mode 100644\nnew mode 100755\n"
	value := New(Comparison{}, patch)
	changes := Changes(patch)
	if len(changes) != 3 || changes[0].Line != 3 || changes[1].Line != 6 || changes[2].Line != 9 {
		t.Fatalf("unexpected change index: %+v", changes)
	}
	if changes[0].ID == changes[1].ID || changes[0] != Changes(patch)[0] {
		t.Fatal("change IDs must be distinct and deterministic")
	}
	guide := Guide{SchemaVersion: 1, SnapshotID: value.Snapshot.ID, Steps: []Step{{Title: "Behavior", Explanation: "Partial guides are valid.", Hunks: []string{changes[0].ID}}}}
	if err := guide.Validate(value); err != nil {
		t.Fatal(err)
	}
	guide.SnapshotID = "stale"
	if err := guide.Validate(value); err == nil {
		t.Fatal("accepted stale guide")
	}
	guide.SnapshotID = value.Snapshot.ID
	guide.Steps[0].Hunks = []string{"unknown"}
	if err := guide.Validate(value); err == nil {
		t.Fatal("accepted unknown hunk")
	}
	guide.Steps = nil
	if err := guide.Validate(value); err == nil {
		t.Fatal("accepted empty guide")
	}
}
