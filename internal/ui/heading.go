package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand/v2"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Like rex-md, render large headings as images, not enlarged terminal cells.
// A fixed header placement keeps code selectable and avoids exposing image
// escapes to Bubble Tea's text diffing renderer. All writes share one lock.
type headingLayer struct {
	output   io.Writer
	mu       sync.Mutex
	enabled  bool
	stopped  bool
	id       uint32
	desired  *headingImage
	uploaded string
}

type headingImage struct {
	key              string
	data             []byte
	cols, rows, x, y int
	clipped          bool
}

func newHeadingLayer(output io.Writer) *headingLayer {
	enabled := os.Getenv("REX_SESSION") != "" || os.Getenv("TERM_PROGRAM") == "ghostty" || os.Getenv("TERM") == "xterm-kitty"
	if setting := os.Getenv("RGD_GRAPHICS"); setting != "" {
		enabled = setting == "1"
	}
	return &headingLayer{output: output, enabled: enabled, id: 1 + rand.Uint32N(0xfffffe)}
}

// Preserve the complete terminal-file interface so Bubble Tea receives resize
// events and knows the screen width when clearing trailing text during redraws.
func (h *headingLayer) Fd() uintptr {
	if file, ok := h.output.(interface{ Fd() uintptr }); ok {
		return file.Fd()
	}
	return ^uintptr(0)
}

func (h *headingLayer) Read(data []byte) (int, error) {
	if input, ok := h.output.(io.Reader); ok {
		return input.Read(data)
	}
	return 0, io.EOF
}

func (h *headingLayer) Prepare(title string, cols, x, y int) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.enabled || cols < 16 {
		return 0
	}
	key := fmt.Sprintf("%s/%d/%d/%d", title, cols, x, y)
	if h.desired != nil && h.desired.key == key {
		return h.desired.rows
	}
	data, width, rows, err := headingPNG(title, cols)
	if err != nil {
		h.desired = nil
		return 0
	}
	h.desired = &headingImage{key: key, data: data, cols: width, rows: rows, x: x, y: y}
	return rows
}

func (h *headingLayer) Hide() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.desired = nil
}

func (h *headingLayer) delete() error {
	if h.uploaded == "" {
		return nil
	}
	_, err := fmt.Fprintf(h.output, "\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", h.id)
	h.uploaded = ""
	return err
}

func (h *headingLayer) Write(data []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if bytes.Contains(data, []byte("\x1b[?1049l")) {
		h.stopped = true
	}
	if h.stopped || h.desired == nil || h.desired.key != h.uploaded {
		if err := h.delete(); err != nil {
			return 0, err
		}
	}
	n, err := h.output.Write(data)
	if err != nil || h.stopped || h.desired == nil {
		return n, err
	}
	image := h.desired
	if h.uploaded != image.key {
		if _, err := io.WriteString(h.output, transmit(h.id, image.data)); err != nil {
			return n, err
		}
		h.uploaded = image.key
	}
	// Save/restore cursor; Kitty C=1 prevents the placement from moving it.
	_, err = fmt.Fprintf(h.output, "\x1b7\x1b[%d;%dH\x1b_Ga=p,i=%d,p=1,c=%d,r=%d,C=1,z=1,q=2\x1b\\\x1b8", image.y, image.x, h.id, image.cols, image.rows)
	return n, err
}

func (h *headingLayer) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	return h.delete()
}

func transmit(id uint32, png []byte) string {
	encoded := base64.StdEncoding.EncodeToString(png)
	var b strings.Builder
	for start := 0; start < len(encoded); start += 4096 {
		end := min(start+4096, len(encoded))
		more := 0
		if end < len(encoded) {
			more = 1
		}
		if start == 0 {
			fmt.Fprintf(&b, "\x1b_Ga=t,f=100,i=%d,q=2,m=%d;%s\x1b\\", id, more, encoded[start:end])
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, encoded[start:end])
		}
	}
	return b.String()
}

func headingPNG(title string, maxCols int) ([]byte, int, int, error) {
	// Render at a consistent 1:2 cell aspect, then place in cell units. This
	// supersampling does not depend on terminal pixel-size reporting or DPI.
	const cellW, cellH = 12, 24
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, 0, 0, err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(cellH) * 1.24, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, 0, 0, err
	}
	defer face.Close()
	var lines []string
	line := ""
	for _, word := range strings.Fields(title) {
		next := strings.TrimSpace(line + " " + word)
		if line != "" && font.MeasureString(face, next).Ceil() > maxCols*cellW {
			lines = append(lines, line)
			line = word
		} else {
			line = next
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	if len(lines) == 0 || len(lines) > 3 {
		return nil, 0, 0, fmt.Errorf("heading cannot fit")
	}
	width := 0
	for _, line := range lines {
		width = max(width, font.MeasureString(face, line).Ceil())
	}
	cols := min(maxCols, max(1, (width+cellW-1)/cellW))
	rows := 2 * len(lines)
	img := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	metrics := face.Metrics()
	textHeight := (metrics.Ascent + metrics.Descent).Ceil()
	drawer := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{R: 241, G: 245, B: 249, A: 255}), Face: face}
	for i, line := range lines {
		drawer.Dot = fixed.P(0, i*cellH*2+(cellH*2-textHeight)/2+metrics.Ascent.Ceil())
		drawer.DrawString(line)
	}
	var b bytes.Buffer
	err = png.Encode(&b, img)
	return b.Bytes(), cols, rows, err
}
