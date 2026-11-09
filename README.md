# rgd — local diff guides

A local diff viewer with an agent-authored walkthrough, built for Rex and the
Kitty graphics protocol. Inspired by Linear's guided reviews.

**Status:** design; not implemented yet.

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

Bookmark detection, ambiguous revisions, merge commits, missing `main`, and
multiple merge bases need explicit resolution rules before implementation.
Do not silently pick an arbitrary parent, revision, or base.

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

Storage location, identifiers, schema, concurrency handling, and retention policy
are still to be specified.

## Agent interface

The viewer owns diff resolution, snapshots, and review state. Agents author guides
through a separate interface, so an existing coding agent can contribute without
the viewer starting another one.

Proposed commands:

```sh
rgd list
rgd inspect REVIEW_ID
rgd guide set REVIEW_ID --file guide.json
```

The agent interface needs machine-readable output and enough snapshot content to
author a guide. Guide validation must check the snapshot ID and all referenced
hunks. An incomplete guide is allowed; it must not hide uncovered changes.

Automatic agent invocation can later use this same interface. It is not part of
the first version, and neither credentials nor network access should be required
to view a diff or load an existing guide.

## Implementation milestones

1. Resolve comparisons and create immutable diff sessions; expose list/inspect.
2. Validate and atomically publish agent-authored guides.
3. Spike selectable text and Kitty graphics inside Rex.
4. Build complete-diff and walkthrough navigation with saved progress.

No daemon, automatic agent invocation, or general-purpose VCS abstraction is
required for the initial version.

## Stack

Go, Bubble Tea, and Lip Gloss, with isolated Kitty graphics support and
file-backed sessions. See [ADR 0001](docs/adr/0001-go-terminal-stack.md).
