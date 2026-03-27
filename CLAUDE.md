# nothingctl — Claude Code Instructions

## Before every `git push`

- Run `go build ./...` from `go/` and confirm it compiles cleanly.
- If new commands were added or existing ones changed, test them against a connected device (or confirm no device is available and note it).
- Never push broken code. Fix compile errors and obvious regressions first.

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

## After every release push

- Check the GitHub Actions "Release" workflow run via `gh run list --limit 5`.
- If the workflow fails, investigate the error, fix it, and re-trigger (move the tag if needed) before considering the release done.
- Do not close the task until the workflow shows ✓ success and the release assets are visible on GitHub.

## Go environment (Windows)

- Go binary lives at `/c/Program Files/Go/bin/go`. Always `export PATH="$PATH:/c/Program Files/Go/bin"` in new bash sessions if `go` is not in PATH.
- All `go` commands must be run from the `go/` subdirectory (module root).
- Binaries go to `dist/` (relative to repo root), not inside `go/`.
