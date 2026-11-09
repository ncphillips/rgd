package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// PrepareGuide uses a larger proportional face for explanations than the native
// monospace diff. The full explanation also remains available as text with e.
func (h *headingLayer) PrepareGuide(title, prose string, cols, x, y, maxRows int) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.enabled || cols < 16 || maxRows < 5 {
		return 0, false
	}
	key := fmt.Sprintf("guide/%s/%s/%d/%d/%d/%d", title, prose, cols, x, y, maxRows)
	if h.desired != nil && h.desired.key == key {
		return h.desired.rows, h.desired.clipped
	}
	data, rows, clipped, err := guidePNG(title, prose, cols, maxRows)
	if err != nil {
		h.desired = nil
		return 0, false
	}
	h.desired = &headingImage{key: key, data: data, cols: cols, rows: rows, x: x, y: y, clipped: clipped}
	return rows, clipped
}

func guideFace(data []byte, size float64) (font.Face, error) {
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func wrapFont(face font.Face, text string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(paragraph) {
			next := strings.TrimSpace(line + " " + word)
			if line != "" && font.MeasureString(face, next).Ceil() > width {
				lines = append(lines, line)
				line = word
			} else {
				line = next
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func guidePNG(title, prose string, cols, maxRows int) ([]byte, int, bool, error) {
	const cellW, cellH = 12, 24
	titleFace, err := guideFace(gobold.TTF, cellH*1.24)
	if err != nil {
		return nil, 0, false, err
	}
	defer titleFace.Close()
	proseFace, err := guideFace(goregular.TTF, cellH)
	if err != nil {
		return nil, 0, false, err
	}
	defer proseFace.Close()
	titles := wrapFont(titleFace, title, cols*cellW)
	paragraph := wrapFont(proseFace, prose, cols*cellW)
	const proseLineHeight = 36
	start := len(titles)*cellH*2 + cellH/2
	limit := (maxRows*cellH - start) / proseLineHeight
	if limit < 1 {
		return nil, 0, false, fmt.Errorf("guide cannot fit")
	}
	clipped := len(paragraph) > limit
	if clipped {
		paragraph = paragraph[:limit]
	}
	rows := (start + len(paragraph)*proseLineHeight + cellH - 1) / cellH
	img := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH))
	ink := image.NewUniform(color.RGBA{R: 241, G: 245, B: 249, A: 255})
	draw := func(face font.Face, lines []string, top, lineHeight int) {
		metrics := face.Metrics()
		textHeight := (metrics.Ascent + metrics.Descent).Ceil()
		drawer := font.Drawer{Dst: img, Src: ink, Face: face}
		for i, line := range lines {
			drawer.Dot = fixed.P(0, top+i*lineHeight+(lineHeight-textHeight)/2+metrics.Ascent.Ceil())
			drawer.DrawString(line)
		}
	}
	draw(titleFace, titles, 0, cellH*2)
	draw(proseFace, paragraph, start, proseLineHeight)
	var b bytes.Buffer
	err = png.Encode(&b, img)
	return b.Bytes(), rows, clipped, err
}
