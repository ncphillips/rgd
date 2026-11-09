package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/ncphillips/rex-diff-guide/internal/jj"
	"github.com/ncphillips/rex-diff-guide/internal/review"
	"github.com/ncphillips/rex-diff-guide/internal/session"
	"github.com/ncphillips/rex-diff-guide/internal/ui"
)

const usage = `Usage:
  rgd [TARGET [BASE]]          Open the interactive diff browser
  rgd --plain [TARGET [BASE]]  Print the complete diff
  rgd --json [TARGET [BASE]]   Capture a machine-readable snapshot
  rgd list [--json]            List saved reviews
  rgd inspect [--json] ID      Read a saved review
  rgd open ID                 Reopen an exact saved snapshot in the TUI
  rgd guide set ID --file PATH Publish a snapshot-bound walkthrough
  rgd skill setup             Choose agents and install the skill interactively
  rgd skill show              Print the current agent workflow

TARGET comes first; BASE overrides its default parent or merge-base with main.
Use -- before a revision that collides with a subcommand name.
RGD_STATE_DIR overrides the local session storage directory.
Non-terminal output falls back to plain text.
`

func Run(ctx context.Context, args []string, out, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "skill" {
		return skillCommand(ctx, args[1:], out, stderr)
	}
	if len(args) > 0 && args[0] == "guide" {
		return setGuide(args[1:], out)
	}
	command := "review"
	if len(args) > 0 && (args[0] == "list" || args[0] == "inspect" || args[0] == "open") {
		command, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("rgd", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	asJSON := flags.Bool("json", false, "emit machine-readable JSON")
	plain := flags.Bool("plain", false, "print the diff instead of opening the TUI")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	args = flags.Args()
	if *asJSON && *plain {
		return fmt.Errorf("--json and --plain are mutually exclusive")
	}
	if command == "list" && len(args) != 0 {
		return fmt.Errorf("list takes no positional arguments")
	}
	if (command == "inspect" || command == "open") && len(args) != 1 {
		return fmt.Errorf("%s requires a review ID", command)
	}
	if command == "review" && len(args) > 2 {
		return fmt.Errorf("expected [TARGET [BASE]]")
	}
	store, err := session.Default()
	if err != nil {
		return err
	}
	if command == "list" {
		values, err := store.List()
		if err != nil {
			return err
		}
		if *asJSON {
			return writeJSON(out, values)
		}
		for _, value := range values {
			c := value.Snapshot.Comparison
			if _, err := fmt.Fprintf(out, "%s  %.12s → %.12s  %s\n", value.ID, c.BaseID, c.TargetID, c.Repository); err != nil {
				return err
			}
		}
		return nil
	}
	var value review.Session
	if command == "inspect" || command == "open" {
		value, err = store.Load(args[0])
	} else {
		var dir string
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
		repo, openErr := jj.Open(ctx, dir)
		if openErr != nil {
			return openErr
		}
		value, err = repo.Capture(ctx, args)
		if err == nil {
			value, err = store.Save(value)
		}
	}
	if err != nil {
		return err
	}
	if *asJSON {
		guide, err := store.LoadGuide(value)
		if err != nil {
			return err
		}
		return writeJSON(out, struct {
			review.Session
			Changes []review.Change `json:"changes"`
			Guide   *review.Guide   `json:"guide,omitempty"`
		}{value, review.Changes(value.Snapshot.Patch), guide})
	}
	if command != "inspect" && !*plain {
		if terminal, ok := out.(*os.File); ok && term.IsTerminal(terminal.Fd()) && term.IsTerminal(os.Stdin.Fd()) {
			return ui.Run(ctx, value, os.Stdin, out, func() (*review.Guide, error) { return store.LoadGuide(value) })
		}
	}
	c := value.Snapshot.Comparison
	if _, err := fmt.Fprintf(out, "Review %s\n%.12s → %.12s (%s)\n\n", value.ID, c.BaseID, c.TargetID, c.Mode); err != nil {
		return err
	}
	if value.Snapshot.Patch == "" {
		_, err = fmt.Fprintln(out, "No changes.")
	} else {
		_, err = io.WriteString(out, value.Snapshot.Patch)
	}
	return err
}

func setGuide(args []string, out io.Writer) error {
	if len(args) != 4 || args[0] != "set" || args[2] != "--file" {
		return fmt.Errorf("usage: rgd guide set REVIEW_ID --file PATH")
	}
	data, err := os.ReadFile(args[3])
	if err != nil {
		return err
	}
	var guide review.Guide
	if err := json.Unmarshal(data, &guide); err != nil {
		return err
	}
	store, err := session.Default()
	if err != nil {
		return err
	}
	if err := store.SaveGuide(args[1], guide); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Published %d guide steps for %s\n", len(guide.Steps), args[1])
	return err
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
