package ui

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

func TestGuideImageLayoutAndClipping(t *testing.T) {
	for _, maxRows := range []int{7, 12, 24} {
		data, rows, clipped, err := guidePNG("Readable guide", strings.Repeat("Larger guide text with comfortable line spacing. ", 100), 60, maxRows)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if !clipped || rows > maxRows || img.Bounds().Dy() != rows*24 || img.Bounds().Dx() != 60*12 {
			t.Fatalf("bad narrative geometry: %d rows, clipped=%v", rows, clipped)
		}
	}
	if _, _, _, err := guidePNG("No room", "Explanation", 20, 1); err == nil {
		t.Fatal("accepted an image without room for text")
	}
}

func TestGuideImageAndSelectableFallback(t *testing.T) {
	t.Setenv("RGD_GRAPHICS", "1")
	m := fixture()
	m.width, m.height = 120, 40
	layer := newHeadingLayer(&bytes.Buffer{})
	m.heading = layer
	guide := &review.Guide{SchemaVersion: 1, SnapshotID: m.review.ID, Steps: []review.Step{{Title: "Readable guide", Explanation: "This explanation is larger than the diff text.", Hunks: []string{m.changes[0].ID}}}}
	updated, _ := m.Update(guideMsg{guide: guide})
	m = updated.(model)
	view := m.View()
	if !strings.Contains(view, "selectable explanation") || layer.desired == nil {
		t.Fatal("large guide text not rendered")
	}
	m.fileFocus = true
	m = key(m, "e")
	if m.fileFocus {
		t.Fatal("explanation should receive scroll keys")
	}
	if !strings.Contains(m.View(), "This explanation") || layer.desired != nil {
		t.Fatal("selectable text fallback missing, or image not cleared")
	}
}
