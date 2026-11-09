package ui

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/ncphillips/rex-diff-guide/internal/review"
)

const patch = `diff --git a/first.go b/first.go
--- a/first.go
+++ b/first.go
@@ -1 +1 @@
-old
+new
diff --git a/second.go b/second.go
--- a/second.go
+++ b/second.go
@@ -1 +1 @@
-before
+after
`

func fixture() model {
	value := review.New(review.Comparison{Repository: "/repo", BaseID: "aaaa", TargetID: "bbbb"}, patch)
	return newModel(value, lipgloss.NewRenderer(io.Discard))
}

func key(m model, value string) model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)})
	return updated.(model)
}

func TestFileAndHunkNavigation(t *testing.T) {
	m := fixture()
	if len(m.files) != 2 || m.files[0].name != "first.go" || m.files[1].name != "second.go" || len(m.hunks) != 2 {
		t.Fatalf("unexpected patch index: %+v, %v", m.files, m.hunks)
	}
	m = key(m, "]")
	if m.offset != 6 || m.currentFile() != 1 {
		t.Fatalf("next file did not navigate: %+v", m)
	}
	m = key(m, "[")
	m = key(m, "n")
	if m.offset != 3 {
		t.Fatalf("next hunk offset: %d", m.offset)
	}
	m = key(m, "n")
	if m.offset != 9 {
		t.Fatalf("second hunk offset: %d", m.offset)
	}
	m = key(m, "N")
	if m.offset != 3 {
		t.Fatalf("previous hunk offset: %d", m.offset)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = key(updated.(model), "j")
	if !m.fileFocus || m.currentFile() != 1 {
		t.Fatal("file focus did not navigate")
	}
	_, quit := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quit == nil {
		t.Fatal("q did not quit")
	}
}

func TestScrollingAndResize(t *testing.T) {
	m := fixture()
	for range 50 {
		m = key(m, "j")
	}
	if m.offset != len(m.lines)-1 {
		t.Fatal("scroll exceeded patch")
	}
	m = key(m, "g")
	m = key(m, "k")
	if m.offset != 0 {
		t.Fatal("scroll moved before patch")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 100, Height: 30}, {Width: 40, Height: 10}, {Width: 10, Height: 3}} {
		updated, _ := m.Update(size)
		m = updated.(model)
		view := ansi.Strip(m.View())
		lines := strings.Split(view, "\n")
		if len(lines) > size.Height {
			t.Fatalf("render exceeds height: %d > %d", len(lines), size.Height)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > size.Width {
				t.Fatalf("render exceeds width: %q", line)
			}
		}
	}
}

func TestEmptyPatchAndUnsafeText(t *testing.T) {
	m := newModel(review.New(review.Comparison{}, ""), lipgloss.NewRenderer(io.Discard))
	for _, value := range []string{"j", "k", "]", "[", "n", "N", "G", "l", "h"} {
		m = key(m, value)
	}
	if !strings.Contains(m.View(), "No changes") {
		t.Fatal("empty comparison missing explanation")
	}
	if got := safeText("\x1b]52;c;payload\a\tcode\r"); strings.ContainsAny(got, "\x1b\a\t\r") {
		t.Fatalf("unsafe controls were rendered: %q", got)
	}
}

func TestDeletionAndMetadataOnlyFiles(t *testing.T) {
	value := review.New(review.Comparison{}, "diff --git a/gone b/gone\n--- a/gone\n+++ /dev/null\n@@ -1 +0,0 @@\n-old\ndiff --git a/executable b/executable\nold mode 100644\nnew mode 100755\n")
	m := newModel(value, lipgloss.NewRenderer(io.Discard))
	if len(m.files) != 2 || m.files[0].name != "gone" || !strings.Contains(m.files[1].name, "executable") {
		t.Fatalf("lost deletion or metadata-only change: %+v", m.files)
	}
	if !strings.Contains(strings.Join(m.lines, "\n"), "new mode 100755") {
		t.Fatal("metadata lost")
	}
}

func TestGuideArrivalNavigationAndResize(t *testing.T) {
	m := fixture()
	guide := &review.Guide{SchemaVersion: 1, SnapshotID: m.review.ID, Steps: []review.Step{
		{Title: "Second file first", Explanation: "Explain the behavior before the supporting changes.", Hunks: []string{m.changes[1].ID}},
		{Title: "Supporting change", Explanation: "Now review the supporting changes.", Hunks: []string{m.changes[0].ID}},
	}}
	updated, _ := m.Update(guideMsg{guide: guide})
	m = updated.(model)
	if m.offset != 9 || !strings.Contains(m.View(), "Second file first") {
		t.Fatal("guide did not load/jump to first step")
	}
	m = key(m, "}")
	if m.step != 1 || m.offset != 3 {
		t.Fatal("step navigation did not follow semantic order")
	}
	m = key(m, "{")
	if m.step != 0 || m.offset != 9 {
		t.Fatal("previous step failed")
	}
	m.offset = 4
	updated, _ = m.Update(guideMsg{guide: guide})
	m = updated.(model)
	if m.offset != 4 {
		t.Fatal("live reload disrupted navigation")
	}
	for height := 6; height < 30; height++ {
		m.height = height
		if len(strings.Split(m.View(), "\n")) > height {
			t.Fatalf("guide exceeds height %d", height)
		}
	}
	m.guide.Steps = m.guide.Steps[:1]
	if len(m.uncovered()) != 1 {
		t.Fatal("uncovered change was hidden")
	}
	m = key(m, "u")
	if m.offset != 3 {
		t.Fatal("cannot navigate to uncovered change")
	}
}

func TestClickableWalkthroughOutline(t *testing.T) {
	m := fixture()
	m.width, m.height = 120, 35
	guide := &review.Guide{SchemaVersion: 1, SnapshotID: m.review.ID, Steps: []review.Step{
		{Title: "Behavior first", Explanation: "Read the behavior.", Hunks: []string{m.changes[1].ID}},
		{Title: "Supporting change", Explanation: "Read supporting code.", Hunks: []string{m.changes[0].ID}},
	}}
	updated, _ := m.Update(guideMsg{guide: guide})
	m = updated.(model)
	rows := m.sidebarRows()
	if !strings.Contains(m.View(), "WALKTHROUGH") {
		t.Fatal("outline is still a file list")
	}
	for i, row := range rows {
		if row.file != 0 {
			continue
		}
		updated, _ = m.Update(tea.MouseMsg{X: 8, Y: 3 + i, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
		m = updated.(model)
		if m.offset != 0 || m.step != 1 {
			t.Fatalf("file click did not select step and scroll to file: %+v", m)
		}
		break
	}
	updated, _ = m.Update(tea.MouseMsg{X: 8, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m = updated.(model)
	if m.step != 0 || m.offset != 9 {
		t.Fatal("step click did not jump to first referenced hunk")
	}
	before := m.offset
	updated, _ = m.Update(tea.MouseMsg{X: 8, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	if updated.(model).offset != before {
		t.Fatal("mouse release triggered navigation")
	}
}

func TestLongExplanationCanBeReadWithoutLosingDiffPosition(t *testing.T) {
	m := fixture()
	m.width, m.height = 80, 24
	guide := &review.Guide{SchemaVersion: 1, SnapshotID: m.review.ID, Steps: []review.Step{{Title: "Readable prose", Explanation: strings.Repeat("Keep words intact and give prose a readable measure. ", 80), Hunks: []string{m.changes[0].ID}}}}
	updated, _ := m.Update(guideMsg{guide: guide})
	m = updated.(model)
	if !strings.Contains(m.View(), "full explanation") {
		t.Fatal("clipped prose has no affordance")
	}
	offset := m.offset
	m = key(m, "e")
	m = key(m, "j")
	if !m.reading || m.proseOffset != 1 || m.offset != offset {
		t.Fatal("explanation scrolling altered diff position")
	}
	m = key(m, "e")
	if m.reading || m.offset != offset {
		t.Fatal("returning from prose lost diff position")
	}
}
