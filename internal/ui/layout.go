package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m model) documentX() int {
	if left := m.sidebarWidth(); left > 0 {
		return left + 4
	}
	return 3
}

func (m model) documentWidth() int { return max(1, m.width-m.documentX()-3) }

func (m model) fullExplanation() []string {
	if m.guide == nil {
		return nil
	}
	var lines []string
	for _, paragraph := range strings.Split(m.guide.Steps[m.step].Explanation, "\n") {
		lines = append(lines, strings.Split(ansi.Wrap(safeText(paragraph), min(76, m.documentWidth()), ""), "\n")...)
	}
	return lines
}

func (m model) introLines() []string {
	if m.guide == nil || m.bodyHeight() < 10 {
		return nil
	}
	step := m.guide.Steps[m.step]
	muted, accent := m.style(secondaryColor), m.style(accentColor)
	rows := []string{muted.Render(fmt.Sprintf("STEP %02d / %02d", m.step+1, len(m.guide.Steps))), ""}
	title := safeText(step.Title)
	imageRows := 0
	clipped := false
	if m.heading != nil && !m.reading && m.bodyHeight() >= 18 {
		var paragraphs []string
		for _, paragraph := range strings.Split(step.Explanation, "\n") {
			paragraphs = append(paragraphs, safeText(paragraph))
		}
		imageRows, clipped = m.heading.PrepareGuide(title, strings.Join(paragraphs, "\n"), min(76, m.documentWidth()), m.documentX()+1, 6, m.bodyHeight()-13)
	}
	if imageRows > 0 {
		for range imageRows {
			rows = append(rows, "")
		}
		caption := "e: selectable explanation"
		if clipped {
			caption = "Explanation continues · e to read all"
		}
		rows = append(rows, "", muted.Render(ansi.Truncate(caption, m.documentWidth(), "…")))
	} else {
		for _, line := range strings.Split(ansi.Wrap(title, min(90, m.documentWidth()), ""), "\n") {
			rows = append(rows, accent.Bold(true).Render(line))
		}
		rows = append(rows, "")
		prose := m.fullExplanation()
		// Keep some code visible. Long explanations remain accessible with e.
		limit := max(1, m.bodyHeight()-len(rows)-6)
		if len(prose) > limit {
			prose = append([]string{}, prose[:limit]...)
			prose[limit-1] = ansi.Truncate(prose[limit-1], max(1, m.documentWidth()-26), "") + " … e: full explanation"
		}
		for _, line := range prose {
			rows = append(rows, m.style(textColor).Render(line))
		}
	}
	rows = append(rows, "", muted.Render(strings.Repeat("─", min(76, m.documentWidth()))), "")
	if len(m.files) > 0 {
		rows = append(rows, accent.Render("DIFF  "+safeText(m.files[m.currentFile()].name)), "")
	}
	return rows[:min(len(rows), max(0, m.bodyHeight()-2))]
}

func (m model) View() string {
	if m.width < 24 || m.height < 6 {
		if m.heading != nil {
			m.heading.Hide()
		}
		return ansi.Truncate("Resize terminal; q quits", max(1, m.width), "")
	}
	accent, muted := m.style(accentColor), m.style(secondaryColor)
	c := m.review.Snapshot.Comparison
	header := fmt.Sprintf(" rgd   %s   ·   %.12s → %.12s", safeText(filepath.Base(c.Repository)), c.BaseID, c.TargetID)
	rows := []string{accent.Bold(true).Render(ansi.Truncate(header, m.width, "…")), ""}
	left := m.sidebarWidth()
	label := "FILES"
	if m.guide != nil {
		label = "WALKTHROUGH"
	}
	if m.fileFocus {
		label += " •"
	}
	status := "No guide yet · browsing the complete diff"
	if m.guide != nil {
		status = fmt.Sprintf("%d steps   ·   %d other changes   ·   {/} navigate", len(m.guide.Steps), len(m.uncovered()))
	}
	if m.guideError != "" {
		status = "Guide unavailable: " + safeText(m.guideError)
	}
	if len(m.lines) == 0 {
		status = "No changes in this comparison"
	}
	section := ""
	if left > 0 {
		section = muted.Render(fit(" "+label, left) + "│")
	}
	section += strings.Repeat(" ", m.documentX()-ansi.StringWidth(section)) + muted.Render(ansi.Truncate(status, m.documentWidth(), "…"))
	rows = append(rows, section)
	intro := m.introLines()
	if m.heading != nil && (m.guide == nil || m.reading || m.bodyHeight() < 18) {
		m.heading.Hide()
	}
	sidebar := m.sidebarRows()
	for row := 0; row < m.bodyHeight(); row++ {
		var b strings.Builder
		if left > 0 {
			index := m.sidebarTop + row
			text := ""
			if index < len(sidebar) {
				text = sidebar[index].text
			}
			text = fit(text, left)
			if index == m.sidebarCursor && index < len(sidebar) {
				b.WriteString(m.style(textColor).Background(lipgloss.Color(selectionColor)).Bold(true).Render(text))
			} else if index < len(sidebar) && sidebar[index].heading {
				if sidebar[index].step == m.step {
					b.WriteString(m.style(textColor).Bold(true).Render(text))
				} else {
					b.WriteString(accent.Render(text))
				}
			} else {
				b.WriteString(muted.Render(text))
			}
			b.WriteString(muted.Render("│"))
		}
		b.WriteString(strings.Repeat(" ", m.documentX()-ansi.StringWidth(b.String())))
		if m.reading {
			prose := m.fullExplanation()
			if at := row + m.proseOffset; at < len(prose) {
				b.WriteString(m.style(textColor).Render(prose[at]))
			}
		} else if row < len(intro) {
			b.WriteString(intro[row])
		} else {
			index := m.offset + row - len(intro)
			line := ""
			if index < len(m.lines) {
				line = m.lines[index]
			}
			visible := ansi.Cut(line, m.horizontal, m.horizontal+m.documentWidth())
			switch {
			case strings.HasPrefix(line, "diff --git"), strings.HasPrefix(line, "@@"):
				visible = accent.Bold(true).Render(visible)
			case strings.HasPrefix(line, "+"):
				visible = m.style(addedColor).Render(visible)
			case strings.HasPrefix(line, "-"):
				visible = m.style(removedColor).Render(visible)
			default:
				visible = m.style(textColor).Render(visible)
			}
			b.WriteString(visible)
		}
		rows = append(rows, b.String())
	}
	help := " Click to jump · wheel scroll · Tab outline · {/} step · n/N hunk · e explanation · q quit"
	if m.reading {
		help = " Full explanation · j/k or wheel scroll · e return to diff · q quit"
	}
	rows = append(rows, muted.Render(ansi.Truncate(help, m.width, "…")))
	return strings.Join(rows, "\n")
}

func fit(value string, width int) string {
	value = ansi.Truncate(value, width, "…")
	return value + strings.Repeat(" ", max(0, width-ansi.StringWidth(value)))
}
