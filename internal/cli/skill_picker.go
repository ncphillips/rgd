package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"github.com/ncphillips/rex-diff-guide/internal/agentskill"
)

type skillPicker struct {
	agents              []agentskill.Agent
	selected            []bool
	cursor              int
	confirmed, canceled bool
	force               bool
}

func newSkillPicker(agents []agentskill.Agent, force bool) skillPicker {
	selected := make([]bool, len(agents))
	for i, agent := range agents {
		selected[i] = agent.Detected
	}
	return skillPicker{agents: agents, selected: selected, force: force}
}

func (p skillPicker) Init() tea.Cmd { return nil }

func (p skillPicker) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			p.canceled = true
			return p, tea.Quit
		case "up", "k":
			p.cursor = max(0, p.cursor-1)
		case "down", "j":
			p.cursor = min(len(p.agents)-1, p.cursor+1)
		case " ":
			p.selected[p.cursor] = !p.selected[p.cursor]
		case "a":
			all := true
			for _, selected := range p.selected {
				all = all && selected
			}
			for i := range p.selected {
				p.selected[i] = !all
			}
		case "enter":
			for _, selected := range p.selected {
				if selected {
					p.confirmed = true
					return p, tea.Quit
				}
			}
		}
	}
	return p, nil
}

func (p skillPicker) View() string {
	if p.canceled {
		return "Skill setup canceled.\n"
	}
	if p.confirmed {
		return "Installing selected skills…\n"
	}
	var b strings.Builder
	b.WriteString("Install the rgd agent skill\n\nDetected agents are preselected. Choose where to install:\n\n")
	for i, agent := range p.agents {
		cursor, checkbox := " ", " "
		if i == p.cursor {
			cursor = "›"
		}
		if p.selected[i] {
			checkbox = "x"
		}
		detection := "not detected"
		if agent.Detected {
			detection = "detected"
		}
		fmt.Fprintf(&b, " %s [%s] %s (%s)\n         %s\n", cursor, checkbox, agent.Name, detection, agent.Directory)
	}
	if p.force {
		b.WriteString("\n--force: selected existing skill files may be replaced.\n")
	}
	b.WriteString("\n↑/↓ move · Space toggle · a all/none · Enter install · Esc cancel\n")
	return b.String()
}

func setupSkills(ctx context.Context, out io.Writer, force bool) error {
	terminal, ok := out.(*os.File)
	if !ok || !term.IsTerminal(terminal.Fd()) || !term.IsTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("skill setup requires an interactive terminal to select agents and confirm installation; run rgd skill setup in your terminal")
	}
	agents, err := agentskill.Detect()
	if err != nil {
		return err
	}
	result, err := tea.NewProgram(newSkillPicker(agents, force), tea.WithContext(ctx), tea.WithInput(os.Stdin), tea.WithOutput(out)).Run()
	if err != nil {
		return err
	}
	picker := result.(skillPicker)
	if !picker.confirmed || picker.canceled {
		return nil
	}
	for i, agent := range agents {
		if !picker.selected[i] {
			continue
		}
		state, err := agentskill.Setup(agent.Directory, force)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s: %s\n%s\n", agent.Name, state, agent.Directory); err != nil {
			return err
		}
	}
	return nil
}
