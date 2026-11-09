package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/ncphillips/rex-diff-guide/internal/review"
)

type sidebarRow struct {
	text    string
	step    int
	file    int
	line    int
	heading bool
}

func (m model) sidebarWidth() int {
	if m.width < 70 {
		return 0
	}
	return min(42, max(28, m.width/4))
}

func (m model) sidebarRows() []sidebarRow {
	var rows []sidebarRow
	addFiles := func(changes []review.Change, step int) {
		seen := map[int]bool{}
		for _, change := range changes {
			for i, f := range m.files {
				end := len(m.lines)
				if i+1 < len(m.files) {
					end = m.files[i+1].start
				}
				if change.Line < f.start || change.Line >= end || seen[i] {
					continue
				}
				seen[i] = true
				rows = append(rows, sidebarRow{text: "    " + safeText(f.name), step: step, file: i, line: f.start})
			}
		}
	}
	if m.guide == nil {
		for i, f := range m.files {
			rows = append(rows, sidebarRow{text: "  " + safeText(f.name), step: -1, file: i, line: f.start})
		}
		return rows
	}
	for i, step := range m.guide.Steps {
		if i > 0 {
			rows = append(rows, sidebarRow{step: -1, file: -1, line: -1})
		}
		title := safeText(step.Title)
		for j, line := range strings.Split(ansi.Wrap(title, max(12, m.sidebarWidth()-7), ""), "\n") {
			prefix := "     "
			if j == 0 {
				prefix = fmt.Sprintf(" %02d  ", i+1)
			}
			rows = append(rows, sidebarRow{text: prefix + line, step: i, file: -1, line: -1, heading: true})
		}
		var changes []review.Change
		for _, id := range step.Hunks {
			for _, change := range m.changes {
				if change.ID == id {
					changes = append(changes, change)
				}
			}
		}
		addFiles(changes, i)
	}
	if uncovered := m.uncovered(); len(uncovered) > 0 {
		rows = append(rows, sidebarRow{step: -1, file: -1, line: -1}, sidebarRow{text: " Other changes", step: -1, file: -1, line: -1})
		addFiles(uncovered, -1)
	}
	return rows
}

func (m *model) activateRow(index int) {
	rows := m.sidebarRows()
	if index < 0 || index >= len(rows) {
		return
	}
	row := rows[index]
	if !row.heading && row.file < 0 {
		return
	}
	m.sidebarCursor = index
	if row.step >= 0 {
		m.step = row.step
	}
	if row.heading {
		m.jumpStep()
	} else {
		m.offset, m.horizontal = row.line, 0
	}
	m.sidebarCursor = index
	m.proseOffset = 0
	m.revealSidebar()
}

func (m *model) moveSidebar(delta int) {
	rows := m.sidebarRows()
	for i := m.sidebarCursor + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].heading || rows[i].file >= 0 {
			m.activateRow(i)
			return
		}
	}
}

func (m *model) revealSidebar() {
	if m.sidebarCursor < m.sidebarTop {
		m.sidebarTop = m.sidebarCursor
	}
	if m.sidebarCursor >= m.sidebarTop+m.bodyHeight() {
		m.sidebarTop = m.sidebarCursor - m.bodyHeight() + 1
	}
	m.sidebarTop = max(0, min(m.sidebarTop, max(0, len(m.sidebarRows())-m.bodyHeight())))
}

func (m *model) syncSidebar() {
	rows := m.sidebarRows()
	for i, row := range rows {
		if row.file == m.currentFile() && (m.guide == nil || row.step == m.step) {
			m.sidebarCursor = i
			m.revealSidebar()
			return
		}
	}
	for i, row := range rows {
		if row.file == m.currentFile() {
			m.sidebarCursor = i
			if row.step >= 0 {
				m.step = row.step
			}
			m.revealSidebar()
			return
		}
	}
}
