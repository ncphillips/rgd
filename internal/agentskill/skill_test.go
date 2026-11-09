package agentskill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectory(t *testing.T) {
	home := t.TempDir()
	config := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	for agent, want := range map[string]string{
		"opencode": filepath.Join(config, "opencode", "skills", "rgd"),
		"claude":   filepath.Join(home, ".claude", "skills", "rgd"),
		"codex":    filepath.Join(home, ".agents", "skills", "rgd"),
	} {
		got, err := Directory(agent)
		if err != nil || got != want {
			t.Fatalf("%s: %s, %v", agent, got, err)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	got, err := Directory("opencode")
	if err != nil || got != filepath.Join(home, ".config", "opencode", "skills", "rgd") {
		t.Fatalf("bad config fallback: %s, %v", got, err)
	}
	if _, err := Directory("unknown"); err == nil {
		t.Fatal("accepted unknown agent")
	}
}

func TestSetupStatusAndSafeRemoval(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rgd")
	if state, err := Status(dir); err != nil || state != "not installed" {
		t.Fatalf("%s, %v", state, err)
	}
	if state, err := Setup(dir, false); err != nil || state != "installed" {
		t.Fatalf("%s, %v", state, err)
	}
	if state, err := Setup(dir, false); err != nil || state != "already installed" {
		t.Fatalf("%s, %v", state, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\nname: rgd\n") || !strings.Contains(string(data), "rgd skill show") || !unmodified(data) {
		t.Fatalf("invalid skill registration: %s", data)
	}
	if state, err := Status(dir); err != nil || state != "installed" {
		t.Fatalf("%s, %v", state, err)
	}
	keep := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(keep, []byte("user notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if state, err := Remove(dir); err != nil || state != "removed" {
		t.Fatalf("%s, %v", state, err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("user files were removed")
	}
	if state, err := Remove(dir); err != nil || state != "not installed" {
		t.Fatalf("%s, %v", state, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v, %v", entries, err)
	}
}

func TestPreserveUnmanagedAndEditedSkills(t *testing.T) {
	for _, managed := range []bool{false, true} {
		dir := t.TempDir()
		original := []byte("user-authored skill")
		if managed {
			original = append(Stub(), []byte("\nuser edit")...)
		}
		path := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(path, original, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Setup(dir, false); err == nil {
			t.Fatal("overwrote unfamiliar or modified skill")
		}
		if _, err := Remove(dir); err == nil {
			t.Fatal("removed unfamiliar or modified skill")
		}
		current, err := os.ReadFile(path)
		if err != nil || string(current) != string(original) {
			t.Fatal("modified user content")
		}
		if state, err := Status(dir); err != nil || !strings.Contains(state, "modified") {
			t.Fatalf("%s, %v", state, err)
		}
		if _, err := Setup(dir, true); err != nil {
			t.Fatal(err)
		}
		if state, err := Status(dir); err != nil || state != "installed" {
			t.Fatalf("%s, %v", state, err)
		}
	}
}

func TestRefuseSymlinkFilesAndDirectories(t *testing.T) {
	for _, directory := range []bool{false, true} {
		root := t.TempDir()
		dir := filepath.Join(root, "rgd")
		target := filepath.Join(root, "target")
		if directory {
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, dir); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("do not touch"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(dir, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Setup(dir, true); err == nil {
			t.Fatal("followed a symlink even with force")
		}
		if _, err := Remove(dir); err == nil {
			t.Fatal("removed a symlink")
		}
		if directory {
			entries, err := os.ReadDir(target)
			if err != nil || len(entries) != 0 {
				t.Fatal("wrote into symlink destination")
			}
		} else {
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "do not touch" {
				t.Fatal("modified symlink target")
			}
		}
	}
}
