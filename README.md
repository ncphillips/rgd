# rgd — local diff guides

A local diff viewer with an agent-authored walkthrough, built for Rex and the
Kitty graphics protocol. Inspired by Linear's guided reviews.

**Status:** interactive diff browser and agent-authored walkthroughs implemented.

## Development

Requires Go 1.27+ and `jj` on `PATH`.

### Install locally and test another repository

From this checkout, install into a directory on your `PATH`:

```sh
GOBIN="$HOME/.local/bin" go install ./cmd/rgd
```

Then run it from any Jujutsu repository:

```sh
cd /path/to/another/repo
rgd
rgd @-
rgd some-bookmark
```

`rgd` uses the caller's working directory, not the source checkout. Re-run the
install command from this checkout after code changes to update the executable.
If `~/.local/bin` isn't on your `PATH`, add it or choose a different absolute
`GOBIN` directory that is. Git-only repositories are not supported yet.

### Build and test

```sh
go build -o rgd ./cmd/rgd
go test -race ./...
go vet ./...
```

Running `rgd` in a terminal opens an interactive, full-screen diff browser with
a walkthrough sidebar and colored additions, deletions, and hunk headers. It runs in
the current pane rather than creating Rex tabs. To use it without installing:

```sh
go run ./cmd/rgd
go run ./cmd/rgd --plain
go run ./cmd/rgd --json @-
go run ./cmd/rgd list --json
go run ./cmd/rgd inspect --json REVIEW_ID
```

Flags precede positional arguments. Use `rgd -- list` to review a revision named
`list` rather than invoke the subcommand. `list` and `inspect` work outside a
repository; review IDs are full 64-character hashes.

### Interactive controls

| Key | Action |
| --- | --- |
| `j` / `k`, up / down | Scroll the diff; navigate steps/files when the outline is focused |
| Tab | Switch focus between diff and outline |
| Click a step / file | Jump to the step's first hunk / the beginning of that file |
| Mouse wheel | Scroll the pane under the pointer |
| `e` | Read the complete explanation; press again to return to the diff |
| Enter | Return focus to the diff |
| `[` / `]` | Previous / next file |
| `n` / `N` | Next / previous hunk |
| `{` / `}` | Previous / next walkthrough step, jumping to its first hunk |
| `u` | Cycle through changes not covered by the guide |
| Page Up / Page Down, Ctrl-B / Ctrl-F | Scroll a page |
| Ctrl-U / Ctrl-D | Scroll half a page |
| `g` / `G`, Home / End | Jump to the beginning / end |
| `h` / `l`, left / right | Pan long lines horizontally |
| `q`, Ctrl-C | Exit and restore the terminal |

The sidebar hides in narrow terminals, but file navigation still works. The
browser keeps the whole diff accessible. When a guide is present, its current
step and explanation appear above the diff; guides reload about once per second.
No agent is invoked automatically.

The outline lists each step with its referenced files nested underneath, plus
uncovered files under “Other changes.” Without a guide it falls back to a file
list. The reading pane uses gutters, section spacing, and word-wrapped prose
limited to 76 columns. Long explanations can be read with `e` without losing the
diff position.

In Rex, Ghostty, and Kitty, the guide uses larger rasterized titles and
proportional explanation text through the Kitty graphics protocol (the same
typography approach used by `rex-md`). The diff stays native-size terminal text.
Press `e` for the full explanation as selectable text; long image explanations
show an explicit continuation prompt so some diff remains visible. Small
viewports and other terminals use text instead. `RGD_GRAPHICS=0` disables these
images; `RGD_GRAPHICS=1` explicitly enables them. Images are placed in cell units
using a nominal 1:2 cell aspect, replaced on layout changes, and deleted on exit.
The dark palette uses bright primary/secondary text and explicit unchanged-code
colors rather than inheriting a potentially faint terminal foreground.
Use `--plain` for text output or `--json` for agent consumption. Redirecting or
piping output also falls back to plain text. `inspect` continues to print a saved
review rather than opening the browser.

## Review a diff

```sh
rgd                          # Current change: @- → @
rgd @-                       # Previous change: @-- → @-
rgd some-bookmark             # Bookmark relative to its merge-base with main
rgd stack-pr-2 stack-pr-1     # stack-pr-1 → stack-pr-2
```

The positional interface is **`rgd [TARGET [BASE]]`**, deliberately not
`BASE TARGET`. The first argument always identifies what to review; the second
overrides its base. The viewer prominently displays the resolved `BASE → TARGET`
comparison.

Jujutsu is the initial version-control backend. With no arguments, review the
current change. A single revision expression reviews that change against its
parent. A single bookmark reviews its branch changes since the common ancestor
with `main`. An explicit base compares the two endpoints directly.

Only exact local bookmark names trigger the bookmark default, and `main` must
be a local bookmark. Revision expressions remain supported. Targets, parents,
and merge bases must each resolve to exactly one commit. Ambiguous targets,
merge changes with multiple parents, and multiple merge bases produce an error;
provide an explicit target or base rather than silently selecting one.

Capture snapshots working-copy edits, then pins a Jujutsu operation for all
resolution and diff queries so the comparison remains consistent.

## Review experience

- Open the complete diff immediately, even when no guide exists.
- Offer an ordered walkthrough grouping related hunks across files.
- Explain what changed, why, and what deserves attention for each step.
- Allow navigation by walkthrough step or ordinary file order.
- Keep every hunk accessible. Uncovered hunks appear as “Other changes.”
- Save local review progress.
- Keep code as selectable terminal text. Explore Kitty graphics for annotations
  and diagrams, with a text fallback rather than rasterizing the entire diff.
- Use Rex for the review workspace. Verify graphics placement, resizing, and
  cleanup in a rendering spike before committing to a rendering architecture.

## Review sessions

Start with shared on-disk sessions, not a persistent server. Each review contains:

| Component | Purpose |
| --- | --- |
| Comparison | Original request, comparison mode, and resolved revisions |
| Snapshot | Immutable diff and stable references to its files and hunks |
| Guide | Ordered explanations referencing snapshot hunks |
| Progress | Locally reviewed steps or hunks |

A guide belongs to one specific snapshot, not to a moving revision name or
bookmark. Guide submissions must identify their snapshot. Reject stale submissions
rather than attaching explanations to a newer diff.

Guide updates should be atomic; viewers observe them without restarting. Refresh
creates a new snapshot and marks the previous guide as stale. Carry progress
forward only where the underlying changes can be reliably matched.

The initial implementation stores one immutable JSON file per snapshot. Repeating
the same capture reuses its content-derived review ID and original creation time.
Writes publish complete files atomically without overwriting an existing snapshot.
The full patch is retained, including binary-change notices and metadata-only
changes. Agent-addressable hunks and guides are implemented; refresh and progress
are not implemented yet.

Session files live in `$XDG_STATE_HOME/rgd/sessions`, falling back to
`~/.local/state/rgd/sessions`. `RGD_STATE_DIR` overrides the complete directory.
They contain source-code diffs: files are created with owner-only permissions,
but snapshots currently persist until manually removed. Keep the state directory
outside the repository. `list` currently includes all saved snapshots, not just
open viewers.

## Agent interface

### Install the agent skill

```sh
rgd skill setup
```

Setup always opens a checkbox picker. It detects OpenCode, Claude Code, and Codex
from their executable or configuration directory and preselects likely matches;
all supported agents remain available. Use arrows or `j`/`k` to move, Space to
toggle, `a` for all/none, Enter to install, or Escape to cancel. Nothing is
installed before confirmation. Setup requires a real interactive terminal;
there is no unattended confirmation bypass.

Selected agents receive a small `SKILL.md` registration:

| Agent | User-level location |
| --- | --- |
| OpenCode | `$XDG_CONFIG_HOME/opencode/skills/rgd/SKILL.md` (default `~/.config/opencode/skills/rgd/SKILL.md`) |
| Claude Code | `~/.claude/skills/rgd/SKILL.md` |
| Codex | `~/.agents/skills/rgd/SKILL.md` |

The registration loads the bundled workflow with `rgd skill show`, keeping
instructions aligned with the installed binary. The source lives in
[`skills/rgd/SKILL.md`](skills/rgd/SKILL.md). `rgd` must be on the agent's `PATH`.

```sh
rgd skill show
rgd skill status                         # OpenCode by default
rgd skill status --agent claude
rgd skill remove --agent codex
```

Setup is repeatable and updates unchanged registrations generated by rgd. It
refuses to overwrite unfamiliar or locally edited files unless you explicitly
run `rgd skill setup --force` and confirm the selection. Skill-directory or
`SKILL.md` symlinks are refused even with `--force`. Removal only deletes an
unchanged managed registration; it leaves other files and directories alone.
No agent settings, hooks, or permissions are changed.

The skill teaches agents to select the intended saved snapshot, write a semantic
walkthrough grounded in its patch, publish validated hunk references, and verify
coverage. It does not launch viewers or Rex tabs unless requested. In particular,
`rgd list` contains saved reviews—not a list of currently open viewers.

### Author a guide

The viewer owns diff resolution, snapshots, and review state. Agents author guides
through a separate interface, so an existing coding agent can contribute without
the viewer starting another one.

Agent discovery, inspection, and guide publication are implemented:

```sh
rgd list
rgd inspect REVIEW_ID
rgd guide set REVIEW_ID --file guide.json
rgd open REVIEW_ID
```

Use `rgd inspect --json REVIEW_ID` to get the immutable patch and its `changes`
index (hunk IDs, file labels, and zero-based patch-line offsets). Metadata-only
or binary changes get a whole-file reference. Author a guide using those IDs:

```json
{
  "schema_version": 1,
  "snapshot_id": "SNAPSHOT_ID_FROM_INSPECT",
  "steps": [
    {
      "title": "Start with the behavior change",
      "explanation": "Explain why this change exists and what to check.",
      "hunks": ["HUNK_ID_FROM_CHANGES"]
    }
  ]
}
```

Publication validates the snapshot and every referenced hunk before atomically
replacing the guide. Concurrent valid submissions are last-writer-wins. Invalid
submissions leave the previous guide untouched. Partial guides are allowed;
uncovered changes are counted as “other changes” and remain reachable with `u`.
Guide text is displayed as plain terminal text, not interpreted as control codes.

`rgd open REVIEW_ID` reopens the exact saved snapshot and watches its guide.
This matters when working-copy edits have moved on: running `rgd` again captures
a new snapshot and does not silently reuse explanations for older code.

Automatic agent invocation can later use this same interface. It is not part of
the first version, and neither credentials nor network access should be required
to view a diff or load an existing guide.

## Implementation milestones

1. Resolve comparisons and create immutable diff sessions; expose list/inspect.
2. Validate and atomically publish agent-authored guides.
3. Spike selectable text and Kitty graphics inside Rex.
4. Build complete-diff and walkthrough navigation with saved progress.

Milestones 1 and 2 and an initial Bubble Tea/Lip Gloss diff browser are implemented.
Kitty heading integration and clickable outline navigation are implemented.
Saved progress and richer graphical annotations are still to come.

No daemon, automatic agent invocation, or general-purpose VCS abstraction is
required for the initial version.

## Stack

Go, Bubble Tea, and Lip Gloss, with isolated Kitty graphics support and
file-backed sessions. See [ADR 0001](docs/adr/0001-go-terminal-stack.md).
