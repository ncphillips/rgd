package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ncphillips/rex-diff-guide/internal/agentskill"
)

func pickerKey(p skillPicker, value string) (skillPicker, tea.Cmd) {
	message := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
	if value == "enter" {
		message = tea.KeyMsg{Type: tea.KeyEnter}
	}
	if value == "esc" {
		message = tea.KeyMsg{Type: tea.KeyEsc}
	}
	updated, command := p.Update(message)
	return updated.(skillPicker), command
}

func TestSkillPickerRequiresExplicitConfirmation(t *testing.T) {
	p := newSkillPicker([]agentskill.Agent{{ID: "opencode", Detected: true}, {ID: "claude"}, {ID: "codex", Detected: true}}, false)
	if p.confirmed || !p.selected[0] || p.selected[1] || !p.selected[2] {
		t.Fatal("incorrect detection defaults")
	}
	p, _ = pickerKey(p, "j")
	p, _ = pickerKey(p, " ")
	if !p.selected[1] || p.confirmed {
		t.Fatal("checkbox toggle installed without confirmation")
	}
	p, command := pickerKey(p, "enter")
	if !p.confirmed || command == nil {
		t.Fatal("enter did not confirm selection")
	}
}

func TestSkillPickerCancelAndEmptySelection(t *testing.T) {
	p := newSkillPicker([]agentskill.Agent{{ID: "opencode"}, {ID: "claude"}, {ID: "codex"}}, true)
	p, command := pickerKey(p, "enter")
	if p.confirmed || command != nil {
		t.Fatal("empty selection was accepted")
	}
	p, _ = pickerKey(p, "a")
	for _, selected := range p.selected {
		if !selected {
			t.Fatal("select-all failed")
		}
	}
	p, _ = pickerKey(p, "a")
	for _, selected := range p.selected {
		if selected {
			t.Fatal("deselect-all failed")
		}
	}
	p, command = pickerKey(p, "esc")
	if p.confirmed || !p.canceled || command == nil {
		t.Fatal("cancel did not exit without installing")
	}
}
