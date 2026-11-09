// Package ui implements the interactive diff browser independently of storage.
package ui

import (
	"context"
	"io"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

type file struct {
	name  string
	start int
}

type model struct {
	review             review.Session
	lines              []string
	files              []file
	hunks              []int
	changes            []review.Change
	width, height      int
	offset, horizontal int
	fileFocus          bool
	renderer           *lipgloss.Renderer
	guide              *review.Guide
	guideError         string
	step               int
	loadGuide          func() (*review.Guide, error)
	sidebarCursor      int
	sidebarTop         int
	reading            bool
	proseOffset        int
	heading            *headingLayer
}

type guideMsg struct {
	guide *review.Guide
	err   error
}

func Run(ctx context.Context, value review.Session, input io.Reader, output io.Writer, loadGuide func() (*review.Guide, error)) error {
	m := newModel(value, lipgloss.NewRenderer(output))
	m.loadGuide = loadGuide
	layer := newHeadingLayer(output)
	m.heading = layer
	defer layer.Close()
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(input),
		tea.WithOutput(layer), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func newModel(value review.Session, renderer *lipgloss.Renderer) model {
	m := model{review: value, width: 80, height: 24, renderer: renderer, changes: review.Changes(value.Snapshot.Patch)}
	patch := strings.TrimSuffix(value.Snapshot.Patch, "\n")
	if patch == "" {
		return m
	}
	inHunk := false
	for _, line := range strings.Split(patch, "\n") {
		index := len(m.lines)
		if strings.HasPrefix(line, "diff --git ") {
			inHunk = false
			m.files = append(m.files, file{name: strings.TrimPrefix(line, "diff --git "), start: index})
		}
		if !inHunk && strings.HasPrefix(line, "+++ ") && len(m.files) > 0 {
			name := strings.TrimPrefix(line, "+++ ")
			if name != "/dev/null" {
				m.files[len(m.files)-1].name = strings.TrimPrefix(name, "b/")
			}
		}
		if !inHunk && strings.HasPrefix(line, "--- ") && len(m.files) > 0 {
			name := strings.TrimPrefix(line, "--- ")
			if name != "/dev/null" {
				m.files[len(m.files)-1].name = strings.TrimPrefix(name, "a/")
			}
		}
		if strings.HasPrefix(line, "@@ ") {
			inHunk = true
			m.hunks = append(m.hunks, index)
		}
		m.lines = append(m.lines, safeText(line))
	}
	return m
}

// Never interpret terminal control sequences from repository contents. Tabs are
// expanded for horizontal scrolling; all other control characters are visible.
func safeText(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case unicode.IsControl(r):
			b.WriteRune('�')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m model) Init() tea.Cmd {
	if m.loadGuide == nil {
		return nil
	}
	return func() tea.Msg { guide, err := m.loadGuide(); return guideMsg{guide, err} }
}

func (m model) bodyHeight() int { return max(1, m.height-4) }

func (m model) diffHeight() int { return max(1, m.bodyHeight()-len(m.introLines())) }

func (m *model) jumpStep() {
	if m.guide == nil {
		return
	}
	step := m.guide.Steps[m.step]
	for _, change := range m.changes {
		if change.ID == step.Hunks[0] {
			m.offset, m.horizontal = change.Line, 0
			m.proseOffset = 0
			m.syncSidebar()
			return
		}
	}
}

func (m model) uncovered() []review.Change {
	covered := map[string]bool{}
	if m.guide != nil {
		for _, step := range m.guide.Steps {
			for _, id := range step.Hunks {
				covered[id] = true
			}
		}
	}
	var changes []review.Change
	for _, change := range m.changes {
		if !covered[change.ID] {
			changes = append(changes, change)
		}
	}
	return changes
}

func (m model) currentFile() int {
	selected := 0
	for i, f := range m.files {
		if f.start > m.offset {
			break
		}
		selected = i
	}
	return selected
}

func (m *model) moveFile(delta int) {
	if len(m.files) == 0 {
		return
	}
	index := max(0, min(len(m.files)-1, m.currentFile()+delta))
	m.offset = m.files[index].start
	m.horizontal = 0
	m.syncSidebar()
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case guideMsg:
		m.guideError = ""
		if msg.err != nil {
			m.guideError = msg.err.Error()
			m.guide = nil
		} else {
			first := m.guide == nil && msg.guide != nil
			m.guide = msg.guide
			if m.guide != nil {
				m.step = min(m.step, len(m.guide.Steps)-1)
				if first {
					m.jumpStep()
				}
			}
		}
		if m.loadGuide != nil {
			return m, tea.Tick(time.Second, func(time.Time) tea.Msg {
				guide, err := m.loadGuide()
				return guideMsg{guide, err}
			})
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.revealSidebar()
	case tea.MouseMsg:
		mouse := tea.MouseEvent(msg)
		inSidebar := m.sidebarWidth() > 0 && mouse.X < m.sidebarWidth()
		if mouse.Button == tea.MouseButtonWheelUp || mouse.Button == tea.MouseButtonWheelDown {
			delta := 3
			if mouse.Button == tea.MouseButtonWheelUp {
				delta = -3
			}
			if inSidebar {
				m.sidebarTop = max(0, min(max(0, len(m.sidebarRows())-m.bodyHeight()), m.sidebarTop+delta))
			} else if m.reading {
				m.proseOffset = max(0, m.proseOffset+delta)
			} else {
				m.offset += delta
			}
		} else if mouse.Button == tea.MouseButtonLeft && mouse.Action == tea.MouseActionPress && inSidebar && mouse.Y >= 3 && mouse.Y < 3+m.bodyHeight() {
			m.activateRow(m.sidebarTop + mouse.Y - 3)
			m.fileFocus = true
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "e":
			if m.guide != nil {
				m.reading = !m.reading
				m.fileFocus = false
			}
			m.proseOffset = 0
		case "}", "{":
			if m.guide != nil {
				delta := 1
				if msg.String() == "{" {
					delta = -1
				}
				m.step = max(0, min(len(m.guide.Steps)-1, m.step+delta))
				m.jumpStep()
				m.fileFocus = false
			}
		case "u":
			changes := m.uncovered()
			if len(changes) > 0 {
				previous := m.offset
				m.offset = changes[0].Line
				for _, change := range changes {
					if change.Line > previous {
						m.offset = change.Line
						break
					}
				}
			}
		case "tab", "shift+tab":
			m.fileFocus = !m.fileFocus
		case "j", "down":
			if m.fileFocus {
				m.moveSidebar(1)
			} else if m.reading {
				m.proseOffset++
			} else {
				m.offset++
			}
		case "k", "up":
			if m.fileFocus {
				m.moveSidebar(-1)
			} else if m.reading {
				m.proseOffset = max(0, m.proseOffset-1)
			} else {
				m.offset--
			}
		case "]":
			m.moveFile(1)
		case "[":
			m.moveFile(-1)
		case "enter":
			m.fileFocus = false
		case "pgdown", "ctrl+f", " ":
			if m.reading {
				m.proseOffset += m.bodyHeight()
			} else {
				m.offset += m.diffHeight()
			}
		case "pgup", "ctrl+b":
			if m.reading {
				m.proseOffset = max(0, m.proseOffset-m.bodyHeight())
			} else {
				m.offset -= m.diffHeight()
			}
		case "ctrl+d":
			if m.reading {
				m.proseOffset += max(1, m.bodyHeight()/2)
			} else {
				m.offset += max(1, m.diffHeight()/2)
			}
		case "ctrl+u":
			if m.reading {
				m.proseOffset = max(0, m.proseOffset-max(1, m.bodyHeight()/2))
			} else {
				m.offset -= max(1, m.diffHeight()/2)
			}
		case "g", "home":
			if m.reading {
				m.proseOffset = 0
			} else {
				m.offset = 0
			}
		case "G", "end":
			if m.reading {
				m.proseOffset = max(0, len(m.fullExplanation())-m.bodyHeight())
			} else {
				m.offset = len(m.lines) - m.diffHeight()
			}
		case "h", "left":
			m.horizontal = max(0, m.horizontal-4)
		case "l", "right":
			m.horizontal += 4
		case "n":
			for _, line := range m.hunks {
				if line > m.offset {
					m.offset = line
					break
				}
			}
		case "N":
			for i := len(m.hunks) - 1; i >= 0; i-- {
				if m.hunks[i] < m.offset {
					m.offset = m.hunks[i]
					break
				}
			}
		}
	}
	// Allow the final file/hunk to be placed at the top, even if it is short.
	m.offset = max(0, min(max(0, len(m.lines)-1), m.offset))
	m.proseOffset = min(m.proseOffset, max(0, len(m.fullExplanation())-m.bodyHeight()))
	return m, nil
}

func (m model) style(color string) lipgloss.Style {
	return m.renderer.NewStyle().Foreground(lipgloss.Color(color))
}
