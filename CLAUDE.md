# nothingctl — Claude Code Instructions

## Active codebase

The **Go code** (`go/`) is the current, active implementation. Focus all analysis,
optimizations, and changes exclusively on the Go code.

The Python tool (`~/.claude/skills/nothingctl/nothingctl.py`) was an early prototype
and is no longer maintained. Do not suggest changes to it.

---

## After every code change — device test

After any code change, always run a live test against a connected device before pushing:

```bash
adb devices                          # confirm device is present
go run ./cmd/nothingctl/ info        # baseline: device detection + prop helpers
go run ./cmd/nothingctl/ battery     # battery package
go run ./cmd/nothingctl/ root-status # magisk / root detection
go run ./cmd/nothingctl/ check-update # GitHub API + codename resolve
```

Test commands that were directly changed. If no device is available, note it explicitly in the commit message. Never push untested code silently.

---

## Before every `git push`

- Run `go build ./...` from `go/` and confirm it compiles cleanly.
- Run the device tests above (or document why they were skipped).
- Never push broken code. Fix compile errors and obvious regressions first.

---

## Before pushing a new release tag (`v*`)

1. **Recompile all platforms** from `go/`:
   ```bash
   cp ../debloat.json internal/data/debloat.json
   cp ../modules.json internal/data/modules.json
   VERSION=<tag>
   LDFLAGS="-s -w -X main.Version=${VERSION}"
   GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ../dist/nothingctl-windows-amd64.exe ./cmd/nothingctl/
   GOOS=linux   GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ../dist/nothingctl-linux-amd64      ./cmd/nothingctl/
   GOOS=linux   GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o ../dist/nothingctl-linux-arm64       ./cmd/nothingctl/
   GOOS=darwin  GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o ../dist/nothingctl-darwin-amd64      ./cmd/nothingctl/
   GOOS=darwin  GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o ../dist/nothingctl-darwin-arm64      ./cmd/nothingctl/
   ```
2. **Write a release description** that lists what is new and what changed since the last tag. Include it in the GitHub release body — not just the auto-generated notes.
3. **Attach all 5 binaries** + `checksums.txt` to the release.
4. Push the tag **only after** the binaries are confirmed to build cleanly.

---

## After every release push

- Check the GitHub Actions "Release" workflow run via `gh run list --limit 5`.
- If the workflow fails, investigate the error, fix it, and re-trigger (move the tag if needed) before considering the release done.
- Do not close the task until the workflow shows ✓ success and the release assets are visible on GitHub.

---

## Role: Agent Orchestrator

**You are the orchestrator.** Your primary job is to decompose every non-trivial
task into focused sub-tasks and delegate them to specialized sub-agents. Never do
heavy lifting (research, coding, review) yourself in the main context.

**Why:**
- Keeps the main context clean — no context wasted on low-level details
- Enables parallel execution of independent work streams
- Each agent gets a focused context → better output quality

**Default agent roster** (spawn as needed, in parallel where possible):

| Agent | Type | Responsibility |
|-------|------|----------------|
| **Researcher** | `Explore` or `general-purpose` | Reads docs, searches the web, explores the codebase |
| **Planner** | `Plan` | Designs the approach, identifies risks, produces a step-by-step plan |
| **Coder** | `general-purpose` (isolation: worktree) | Implements the plan — one per independent package group |
| **Reviewer** | `superpowers:code-reviewer` | Validates the implementation against the plan and coding standards |
| **Critic** | `general-purpose` | Challenges assumptions, finds edge cases, flags security risks |

**Example:** User asks "look at this page and build a matching tool"
→ Orchestrator spawns in parallel:
1. Researcher → scrapes and summarizes the page
2. Planner → designs the tool architecture (after researcher returns)
3. Coder(s) → implement (after planner returns, in parallel per module)
4. Reviewer + Critic → review in parallel once code is done

**Rule:** If a task touches more than 2 files OR requires external research OR has
a security/correctness surface, spawn agents — do not handle it inline.

---

## Go environment (Windows)

- Go binary lives at `/c/Program Files/Go/bin/go`. Always `export PATH="$PATH:/c/Program Files/Go/bin"` in new bash sessions if `go` is not in PATH.
- All `go` commands must be run from the `go/` subdirectory (module root).
- Binaries go to `dist/` (relative to repo root), not inside `go/`.
