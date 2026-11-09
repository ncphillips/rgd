package agentskill

import (
	"os"
	"os/exec"
	"path/filepath"
)

type Agent struct {
	ID        string
	Name      string
	Directory string
	Detected  bool
}

func Detect() ([]Agent, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	agents := []Agent{{ID: "opencode", Name: "OpenCode"}, {ID: "claude", Name: "Claude Code"}, {ID: "codex", Name: "Codex"}}
	for i := range agents {
		agent := &agents[i]
		agent.Directory, err = Directory(agent.ID)
		if err != nil {
			return nil, err
		}
		config := filepath.Dir(filepath.Dir(agent.Directory))
		if agent.ID == "codex" {
			config = filepath.Join(home, ".codex")
		}
		if info, err := os.Stat(config); err == nil && info.IsDir() {
			agent.Detected = true
		}
		if _, err := exec.LookPath(agent.ID); err == nil {
			agent.Detected = true
		}
	}
	return agents, nil
}
