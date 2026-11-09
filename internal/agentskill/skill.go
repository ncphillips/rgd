// Package agentskill installs a runtime-backed skill without editing agent
// configuration or overwriting unrecognized/user-modified skill files.
package agentskill

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ncphillips/rex-diff-guide/skills"
)

const marker = "\n<!-- rgd-managed-skill sha256:"

func Directory(agent string) (string, error) {
	if agent != "opencode" && agent != "claude" && agent != "codex" {
		return "", fmt.Errorf("unknown agent %q (choose opencode, claude, or codex)", agent)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case "claude":
		return filepath.Join(home, ".claude", "skills", "rgd"), nil
	case "codex":
		return filepath.Join(home, ".agents", "skills", "rgd"), nil
	default:
		config := os.Getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(config) {
			config = filepath.Join(home, ".config")
		}
		return filepath.Join(config, "opencode", "skills", "rgd"), nil
	}
}

func Stub() []byte {
	end := strings.Index(skills.Guide[4:], "\n---\n") + 4 + 5
	frontmatter := skills.Guide[:end]
	body := "\n# rgd\n\nRun `rgd skill show` and follow the guide it prints. The installed CLI supplies\nthe workflow and schema for its version, so upgrading rgd updates these\ninstructions without reinstalling the skill.\n\nIf rgd is unavailable, ask the user to install it or fix PATH; do not invent\ncommands or write directly into its session store.\n"
	plain := frontmatter + body
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
	return []byte(frontmatter + marker + digest + " -->\n" + body)
}

func unmodified(data []byte) bool {
	text := string(data)
	start := strings.Index(text, marker)
	if start < 0 {
		return false
	}
	valueStart := start + len(marker)
	end := strings.Index(text[valueStart:], " -->\n")
	if end != 64 {
		return false
	}
	plain := text[:start] + text[valueStart+end+len(" -->\n"):]
	return text[valueStart:valueStart+end] == fmt.Sprintf("%x", sha256.Sum256([]byte(plain)))
}

func checkDirectory(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing skill directory that is a symlink or non-directory: %s", dir)
	}
	return nil
}

func read(dir string) ([]byte, error) {
	if err := checkDirectory(dir); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "SKILL.md")
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing skill file that is a symlink or non-regular file: %s", path)
	}
	return os.ReadFile(path)
}

func Status(dir string) (string, error) {
	data, err := read(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "not installed", nil
	}
	if err != nil {
		return "", err
	}
	if !unmodified(data) {
		return "unmanaged or modified (left untouched)", nil
	}
	if string(data) != string(Stub()) {
		return "installed (run setup to update registration)", nil
	}
	return "installed", nil
}

func Setup(dir string, force bool) (string, error) {
	data, err := read(dir)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if exists && string(data) == string(Stub()) {
		return "already installed", nil
	}
	if exists && !unmodified(data) && !force {
		return "", fmt.Errorf("refusing to overwrite unmanaged or modified %s; review it before using --force", filepath.Join(dir, "SKILL.md"))
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	if err := checkDirectory(dir); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".rgd-skill-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if _, err := tmp.Write(Stub()); err != nil {
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "SKILL.md")
	if !exists {
		if err := os.Link(tmp.Name(), path); err != nil {
			if errors.Is(err, os.ErrExist) {
				current, readErr := read(dir)
				if readErr == nil && string(current) == string(Stub()) {
					return "already installed", nil
				}
			}
			return "", err
		}
	} else if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return "installed", nil
}

func Remove(dir string) (string, error) {
	data, err := read(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "not installed", nil
	}
	if err != nil {
		return "", err
	}
	if !unmodified(data) {
		return "", fmt.Errorf("refusing to remove unmanaged or modified %s", filepath.Join(dir, "SKILL.md"))
	}
	if err := os.Remove(filepath.Join(dir, "SKILL.md")); err != nil {
		return "", err
	}
	// Leave the directory and any supporting/user files alone.
	return "removed", nil
}
