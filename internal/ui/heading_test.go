package ui

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/term"
)

var _ term.File = (*headingLayer)(nil)

func TestHeadingImageDimensionsAndFallback(t *testing.T) {
	data, cols, rows, err := headingPNG("A readable heading", 60)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if cols > 60 || rows < 2 || img.Bounds().Dx() != cols*12 || img.Bounds().Dy() != rows*24 {
		t.Fatalf("incorrect heading geometry: %d x %d, %v", cols, rows, img.Bounds())
	}
	if _, _, _, err := headingPNG("", 60); err == nil {
		t.Fatal("empty heading accepted")
	}
	t.Setenv("RGD_GRAPHICS", "0")
	layer := newHeadingLayer(&bytes.Buffer{})
	if layer.Prepare("Fallback", 40, 1, 1) != 0 {
		t.Fatal("graphics override ignored")
	}
}

func TestHeadingLayerLifecycle(t *testing.T) {
	t.Setenv("RGD_GRAPHICS", "1")
	var out bytes.Buffer
	layer := newHeadingLayer(&out)
	if layer.Prepare("First heading", 50, 35, 6) == 0 {
		t.Fatal("heading was not rendered")
	}
	if _, err := layer.Write([]byte("frame")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a=t,f=100") || !strings.Contains(out.String(), "\x1b[6;35H") {
		t.Fatal("missing upload/placement")
	}
	out.Reset()
	if _, err := layer.Write([]byte("next frame")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "a=t,f=100") {
		t.Fatal("unchanged image was retransmitted")
	}
	layer.Prepare("Second heading", 50, 35, 6)
	out.Reset()
	if _, err := layer.Write([]byte("changed frame")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a=d,d=I") || !strings.Contains(out.String(), "a=t,f=100") {
		t.Fatal("old heading not replaced")
	}
	out.Reset()
	if _, err := layer.Write([]byte("\x1b[?1049l")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a=d,d=I") || strings.Contains(out.String(), "a=p") {
		t.Fatal("heading survived alternate-screen cleanup")
	}
	layer.Close()
}
