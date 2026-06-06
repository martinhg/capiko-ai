# Releasing

capiko ships via goreleaser on a `v*` tag push (`.github/workflows/release.yml`).

## One-time setup (already done for this repo)

- Repos `martinhg/homebrew-tap` and `martinhg/scoop-bucket` exist.
- A `HOMEBREW_TAP_TOKEN` repo secret (a PAT with `repo`/Contents:write on those two
  repos) is set on `martinhg/capiko-ai`.

## Cutting a release

1. Bump `baseVersion` in `internal/tui/version.go` to the new version (this is the
   version shown on local/`go install` builds; goreleaser injects the tag for real
   releases via ldflags).
2. Make sure `main` is green: `gofmt -l .`, `go vet ./...`, `go test -race ./...`.
3. Tag and push:
   ```bash
   git tag -a vX.Y.Z -m "capiko-ai vX.Y.Z" && git push origin vX.Y.Z
   ```
4. Watch the release workflow. goreleaser builds all platforms, publishes the
   GitHub release (binaries + `checksums.txt`), and pushes the Homebrew formula and
   Scoop manifest to the tap repos.
5. Verify: `gh release view vX.Y.Z`, and that the tap's `Formula/capiko-ai.rb`
   points at the new version.

## Rollback

A bad release: `gh release delete vX.Y.Z` and `git push --delete origin vX.Y.Z`,
then revert the tap/scoop commits if needed. Prefer cutting a fixed `vX.Y.Z+1`.

## Note

`brews` in `.goreleaser.yaml` is deprecated by newer goreleaser (still valid;
gentle-ai uses the same). Migrating to `homebrew_casks` is a future follow-up.
