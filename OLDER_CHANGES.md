# Older Changes

This file contains historical changelog entries for sanshain-go.
For recent changes, see [CHANGELOG.md](CHANGELOG.md).

## [2.1.0] - 2026-08-14

### Added
- `--trunk` / `SANSHAIN_TRUNK` and `--tag <branch>` / `SANSHAIN_TAG` declare which dependency graph a build's provides and requires belong to; declaring both fails before any request is sent.
- A `provides` entry marked `retired: true` retires that API family on the next `provide`.
- Subscriptions harvested from an AsyncAPI provide are printed, with drift or a missing publisher shown as warnings.
- A GA provide refused with `403` names the `releaser` role and the snapshot fallback.

### Changed
- The module path is now `github.com/paxel/sanshain-go/v2`, so the install command is `go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest`.

### Fixed
- Spec files with CRLF line endings are provided with LF line endings, so a Windows checkout no longer conflicts with the same spec published from Linux.

## [2.0.0] - 2026-08-03

### Added
- `--ga` / `SANSHAIN_GA=true` publishes an immutable GA version; every other provide is a snapshot.
- Every `requires` entry needs an exact `version` pin.
- `--version` prints the client version.
- `--best-effort` and `SANSHAIN_BEST_EFFORT` turn failures, including a config that cannot be loaded, into warnings.
- A `409` on provide shows the server's message and the version to publish as instead.
- A `404` on require (unknown producer or version) and a `410` (endpoint absent in that version) get distinct error messages.
- Against a pre-2.0 server, a failed call reports the server version and asks for an upgrade.

### Changed
- Speaks the Sanshain Service 2.x API; the provided version is read from the spec file (`info.version`, or a `// sanshain-version:` comment for proto).
- The module moved to the `/v2` major-version path.
- Require ETag cache entries are keyed by producer and pinned version.

### Removed
- Branches and git branch detection; the `branch`, `timeout`, `baseVersion` and `releaseBranches` fields are rejected by name with a migration hint.
- Skipping an unchanged provide by content hash, and the automatic `baseVersion` tracking.

## [1.3.0] - 2026-04-26

### Changed
- Downloaded specs and `.sanshain-cache.json` are written readable by the owner only.
- A provide whose response cannot be read now fails instead of passing silently.
- The license is now Apache-2.0 (previously AGPL-3.0).

## [1.2.0] - 2026-04-25

### Added
- `baseVersion` on a provide entry detects concurrent modifications; the last provided version is tracked automatically.
- A provide prints a summary of new, updated and deleted endpoints.
- A provide is skipped when the spec has not changed since the last one.
- Requires send the cached `ETag` and skip writing on `304 Not Modified`.
- `.sanshain-cache.json` in the working directory holds this state.
- `strict: true` turns a missing `serviceName`, `provides` or `requires` into an error.

### Changed
- A missing `serviceName`, no provide configuration or no requires now warn and skip instead of failing.

## [1.1.0] - 2026-04-23

### Changed
- Branch detection reads more CI systems (GitHub pull requests, GitLab, Jenkins, Bitbucket, Azure DevOps, Travis CI, CircleCI) and resolves the branch of a detached HEAD checkout.

### Fixed
- Provides and requires send their API type.
- Gzip-compressed responses are decompressed.
- Server error bodies in error messages are truncated and stripped of unprintable characters.

## [1.0.0] - 2026-04-22

### Added
- AsyncAPI and proto/gRPC specs can be provided (`provides` list with `file` and `apiType`, or `asyncApiFile` / `protoFile`) and required (`apiType` on a requires entry).
- `bestEffort: true` in `sanshain.yaml` turns failures into warnings.

### Changed
- The project is named by a top-level `serviceName` (`clientName` stays as an alias) instead of `clientName` plus `provide.serviceName`.

## [0.2.0] - 2026-04-21

### Added
- The `sanshain-go` command with `provide` and `require`, plus `--config` and `--insecure`.

## [0.1.0] - 2026-04-21

### Added
- Client for the Sanshain Service: provide an OpenAPI spec and require single endpoints or bundles of other services, configured by `sanshain.yaml`, with branch detection from the environment or git, optional gzip compression and bearer token authentication.
