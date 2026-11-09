# ADR 0001: Go terminal application with file-backed review sessions

- Status: Accepted
- Date: 2026-11-09

## Context

`rgd` is a local guided diff viewer for Rex. It must show the complete diff,
accept walkthroughs from an existing coding agent, and work without a daemon,
LLM credentials, or network access. Jujutsu is the first VCS backend.

The CLI is `rgd [TARGET [BASE]]`. The first argument identifies the change to
review; the optional second argument overrides its base. No arguments reviews
the current change. A single bookmark defaults to its common ancestor with
`main`; other single revisions default to their parent.

Rex's Ghostty foundation makes Kitty graphics a reasonable platform baseline.
The remaining rendering risk is coordinating image placement and cleanup with
a text TUI's redraw, scrolling, and resize behavior, not basic protocol support.

## Decision

- Implement `rgd` in Go, distributed as one executable.
- Use Bubble Tea for the interactive UI and Lip Gloss for terminal styling.
- Start with standard-library CLI parsing; add a CLI framework only if needed.
- Invoke `jj` as a subprocess rather than embedding a VCS library.
- Keep code as terminal text; use Kitty graphics for supplementary visuals.
  Isolate protocol handling in a small package with a text fallback.
- Store versioned review sessions as local files, publishing updates atomically.
  Do not require a database or persistent server.
- Bind agent-authored guides to immutable diff snapshots. Validate snapshot and
  hunk references before publishing a guide; reject stale submissions.
- Expose machine-readable CLI operations for discovering and inspecting reviews
  and publishing guides. Defer automatic agent invocation.

Before building the full UI, spike Bubble Tea and Kitty graphics together inside
Rex. Check text selection, image placement, redraws, scrolling, resizing, and
cleanup on exit. Adjust the rendering layer if needed without coupling it to
comparison resolution, storage, or guide validation.

## Consequences

Go provides straightforward subprocess control, JSON encoding, cancellation,
and portable builds. A file-backed session model lets the viewer and an existing
agent cooperate without a service lifecycle or agent-specific SDK.

We must explicitly design atomic updates and concurrency handling. Graphics must
cooperate with the TUI renderer, and terminal text alone does not guarantee a
good selection experience in an alternate-screen application.

Rust/Ratatui is a viable alternative but offers no compelling advantage for this
initial scope. TypeScript would fit a web UI or agent-SDK-heavy application, but
neither is the first milestone. A daemon or database can be introduced later if
real coordination requirements justify it.
