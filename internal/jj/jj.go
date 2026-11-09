// Package jj resolves comparisons against one pinned Jujutsu operation.
package jj

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ncphillips/rex-diff-guide/internal/review"
)

type Repository struct {
	Root      string
	operation string
}

func command(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "jj", append([]string{"--no-pager", "--color=never"}, args...)...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("jj %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func Open(ctx context.Context, dir string) (*Repository, error) {
	root, err := command(ctx, dir, "root")
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	// Snapshot working-copy edits before pinning the operation used for every
	// subsequent query. Later edits cannot change this comparison halfway through.
	if _, err := command(ctx, root, "log", "-r", "@", "--no-graph", "-T", "commit_id"); err != nil {
		return nil, err
	}
	op, err := command(ctx, root, "op", "log", "--ignore-working-copy", "--limit", "1", "--no-graph", "-T", "id")
	if err != nil {
		return nil, err
	}
	return &Repository{Root: root, operation: strings.TrimSpace(op)}, nil
}

func (r *Repository) run(ctx context.Context, args ...string) (string, error) {
	return command(ctx, r.Root, append([]string{"--at-operation=" + r.operation}, args...)...)
}

func (r *Repository) one(ctx context.Context, expression, label string) (string, error) {
	out, err := r.run(ctx, "log", "--no-graph", "-r", expression, "-T", `commit_id ++ "\n"`)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	ids := strings.Fields(out)
	if len(ids) != 1 {
		return "", fmt.Errorf("%s must resolve to exactly one revision (got %d); specify an explicit target/base", label, len(ids))
	}
	return ids[0], nil
}

func (r *Repository) isBookmark(ctx context.Context, name string) (bool, error) {
	// Compare exact names rather than treating user input as a bookmark glob.
	out, err := r.run(ctx, "bookmark", "list", "-T", `name ++ "\n"`)
	if err != nil {
		return false, err
	}
	for _, candidate := range strings.Split(out, "\n") {
		if candidate == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) Capture(ctx context.Context, args []string) (review.Session, error) {
	if len(args) > 2 {
		return review.Session{}, fmt.Errorf("expected [TARGET [BASE]]")
	}
	target := "@"
	if len(args) > 0 {
		target = args[0]
	}
	targetID, err := r.one(ctx, target, "target")
	if err != nil {
		return review.Session{}, err
	}
	base, mode := "parents("+targetID+")", "change"
	if len(args) == 2 {
		base, mode = args[1], "explicit"
	} else if len(args) == 1 {
		bookmark, err := r.isBookmark(ctx, target)
		if err != nil {
			return review.Session{}, err
		}
		if bookmark {
			mainID, err := r.one(ctx, `bookmarks(exact:"main")`, "main bookmark (or supply BASE)")
			if err != nil {
				return review.Session{}, err
			}
			base = "heads(::" + targetID + " & ::" + mainID + ")"
			mode = "bookmark"
		}
	}
	baseID, err := r.one(ctx, base, "base (merge changes require an explicit BASE)")
	if err != nil {
		return review.Session{}, err
	}
	patch, err := r.run(ctx, "diff", "--from", baseID, "--to", targetID, "--git", "--context", strconv.Itoa(3))
	if err != nil {
		return review.Session{}, err
	}
	return review.New(review.Comparison{
		Repository: r.Root, Target: target, Base: base,
		TargetID: targetID, BaseID: baseID, Mode: mode,
	}, patch), nil
}
