package agentskill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAgentDirectories(t *testing.T) {
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("PATH", t.TempDir())
	if err := os.MkdirAll(filepath.Join(config, "opencode"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	agents, err := Detect()
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 || !agents[0].Detected || agents[1].Detected || !agents[2].Detected {
		t.Fatalf("bad detection: %+v", agents)
	}
}

func TestDetectCLIWithoutConfiguration(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	agents, err := Detect()
	if err != nil {
		t.Fatal(err)
	}
	if agents[0].Detected || !agents[1].Detected || agents[2].Detected {
		t.Fatalf("bad CLI detection: %+v", agents)
	}
}
