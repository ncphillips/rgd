package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/ncphillips/rex-diff-guide/internal/agentskill"
	"github.com/ncphillips/rex-diff-guide/skills"
)

const skillUsage = `Usage:
  rgd skill show                         Print the bundled agent workflow
  rgd skill setup [--force]                Choose agents in a checkbox picker
  rgd skill status [--agent AGENT]
  rgd skill remove [--agent AGENT]

AGENT is opencode (default), claude, or codex for status/remove.
Setup detects agents, preselects matches, and always asks before installing.
It requires an interactive terminal and installs only SKILL.md;
it does not alter agent configuration, hooks, permissions, or other skills.
--force explicitly replaces an unmanaged/modified regular SKILL.md on setup.
`

func skillCommand(ctx context.Context, args []string, out, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, skillUsage)
		return err
	}
	command := args[0]
	if command == "show" {
		if len(args) != 1 {
			return fmt.Errorf("usage: rgd skill show")
		}
		_, err := io.WriteString(out, skills.Guide)
		return err
	}
	if command != "setup" && command != "status" && command != "remove" {
		return fmt.Errorf("unknown skill command %q\n%s", command, skillUsage)
	}
	flags := flag.NewFlagSet("rgd skill "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, skillUsage) }
	agent := "opencode"
	var force bool
	if command == "setup" {
		flags.BoolVar(&force, "force", false, "replace a modified or unmanaged regular skill file")
	} else {
		flags.StringVar(&agent, "agent", "opencode", "agent: opencode, claude, or codex")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%s takes no positional arguments", command)
	}
	if command == "setup" {
		return setupSkills(ctx, out, force)
	}
	dir, err := agentskill.Directory(agent)
	if err != nil {
		return err
	}
	var result string
	switch command {
	case "status":
		result, err = agentskill.Status(dir)
	case "remove":
		result, err = agentskill.Remove(dir)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "%s: %s\n%s\n", agent, result, filepath.Join(dir, "SKILL.md"))
	return err
}
