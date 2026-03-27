# nothingctl — Claude Code Instructions

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

## Multi-agent workflow for non-trivial tasks

For optimizations, refactoring, new features, or any task that touches multiple packages, always use multiple agents:

| Role | Minimum | Responsibility |
|------|---------|----------------|
| **Planner** | 1 | Explore the codebase, design the approach, identify risks, produce a step-by-step plan before any code is written. Uses the `Plan` subagent. |
| **Coder** | 2 | Implement the plan in parallel where possible (e.g. one per package group). Uses the `general-purpose` subagent with `isolation: worktree`. |

**When to apply:**
- Adding a new command or feature
- Refactoring across multiple files or packages
- Performance or code-quality improvements
- Any change that affects more than 3 files

**Process:**
1. Launch a `Plan` agent first — get the full plan and file list before opening any editors.
2. Launch 2+ `general-purpose` coder agents (in parallel if the work is independent).
3. Review each agent's diff, then compile + device-test before committing.

---

## Go environment (Windows)

- Go binary lives at `/c/Program Files/Go/bin/go`. Always `export PATH="$PATH:/c/Program Files/Go/bin"` in new bash sessions if `go` is not in PATH.
- All `go` commands must be run from the `go/` subdirectory (module root).
- Binaries go to `dist/` (relative to repo root), not inside `go/`.
