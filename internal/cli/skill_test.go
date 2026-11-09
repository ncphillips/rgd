package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncphillips/rex-diff-guide/internal/agentskill"
)

func TestSkillCommandsOutsideRepository(t *testing.T) {
	home := t.TempDir()
	config := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("RGD_STATE_DIR", filepath.Join(home, "unused-state"))
	t.Chdir(t.TempDir())
	for _, args := range [][]string{
		{"skill", "show"},
		{"skill", "status"},
		{"skill", "status", "--agent", "claude"},
		{"skill", "status", "--agent", "codex"},
	} {
		var out, stderr bytes.Buffer
		if err := Run(context.Background(), args, &out, &stderr); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if out.Len() == 0 {
			t.Fatalf("no output for %v", args)
		}
		if args[1] == "show" && (!strings.Contains(out.String(), "rgd guide set") || !strings.Contains(out.String(), "snapshot_id")) {
			t.Fatal("bundled workflow missing schema or publication command")
		}
	}
	for _, path := range []string{
		filepath.Join(config, "opencode", "skills", "rgd", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "rgd", "SKILL.md"),
		filepath.Join(home, ".agents", "skills", "rgd", "SKILL.md"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("read-only command installed a skill")
		}
	}
	if _, err := os.Stat(filepath.Join(home, "unused-state")); !os.IsNotExist(err) {
		t.Fatal("skill commands touched review storage")
	}
	var out, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"skill", "setup"}, &out, &stderr); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("setup did not ask for confirmation: %v", err)
	}
	if err := Run(context.Background(), []string{"skill", "setup", "--force"}, &out, &stderr); err == nil {
		t.Fatal("force bypassed interactive confirmation")
	}
	if _, err := agentskill.Setup(filepath.Join(config, "opencode", "skills", "rgd"), false); err != nil {
		t.Fatal(err)
	}
	if err := Run(context.Background(), []string{"skill", "remove"}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(config, "opencode", "skills", "rgd", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("skill was not removed")
	}
}

func TestSkillArgumentErrorsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"skill", "unknown"},
		{"skill", "setup", "extra"},
		{"skill", "show", "extra"},
		{"skill", "status", "--agent", "unknown"},
		{"skill", "remove", "--force"},
	} {
		var out, stderr bytes.Buffer
		if err := Run(context.Background(), args, &out, &stderr); err == nil {
			t.Fatalf("accepted invalid args: %v", args)
		}
	}
	for _, args := range [][]string{{"skill"}, {"skill", "--help"}, {"skill", "setup", "--help"}} {
		var out, stderr bytes.Buffer
		if err := Run(context.Background(), args, &out, &stderr); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String()+stderr.String(), "Usage:") {
			t.Fatal("help missing usage")
		}
	}
}
