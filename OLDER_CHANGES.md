# Older Changes

This file contains historical changelog entries for sanshain-go.
For recent changes, see [CHANGELOG.md](CHANGELOG.md).

## [2.5.0] - 2026-11-11

### Added
- `autoGa: true` in `sanshain.yaml` makes a provide GA whenever the build declares trunk or a tag; `--ga=false` or `SANSHAIN_GA=false` keeps a build a snapshot.
- `annotations` in `sanshain.yaml` travel with provide, require, bundle and catalog push, their `{{NAME}}` placeholders filled from `--set NAME=value` or `SANSHAIN_SET_NAME` (needs Sanshain Service 2.13.0 or later).

### Changed
- `--ga=false` now beats `SANSHAIN_GA=true` and publishes a snapshot (before, the variable made it GA).
- A missing or broken `sanshain.yaml` (unreadable, invalid YAML, 1.x fields, failed validation) under `--best-effort` or `SANSHAIN_BEST_EFFORT` prints `Warning: <error>` and exits 0 without a call, now without the `Warning loading config:` prefix and the skip notice that followed it. Under best effort a file that does not validate no longer fails over unknown fields because of its own `strict: true`; its own `bestEffort` still does not count. Without best effort a missing file fails, naming it, as before.

### Fixed
- A require with `apiType: grpc` goes to `/require/grpc` and is saved as `.proto`, and its bundle sends `api_type: proto`; it went to `/require` and was saved as `.yaml`.
- Best effort works the same in provide, require and catalog: missing configuration under strict now fails also with best effort, and a failed provide or require no longer stops the next entry.
- A `403` on provide or retire names the releaser role only when the service refused for want of it; any other `403`, such as a read-only token's, shows the service's message alone.
- The require ETag cache in `.sanshain-cache.json` is kept per output directory: the same endpoint required into two directories is written to both instead of the second getting a `304` and staying empty, and a deleted output file is fetched again instead of being skipped.
- An unknown `requires[].apiType` fails as a configuration error when `sanshain.yaml` is loaded, as an unknown provide `apiType` does; it was sent to `/require` as OpenAPI.
- The `go generate` recipe in `docs/integration.md` works as written: it points `--config` from the package directory at `sanshain.yaml` and the generator at the file the client saves, and the GitLab example uses `golang:1.23`, the floor `go.mod` names (it used `golang:1.21`).
- The build, release and security workflows pin every action to a full commit SHA; they referenced tags and a branch.

## [2.4.0] - 2026-10-25

### Changed
- The release binaries are built with the current Go and need macOS 12 or later (was macOS 11); `go install` still works from Go 1.23.3.
- YAML is read with `go.yaml.in/yaml/v3`, the maintained successor of `gopkg.in/yaml.v3`; `sanshain.yaml` is read as before.

### Security
- The release binaries no longer carry the 36 known vulnerabilities of the Go 1.23 standard library, among them several in `net/http`, `crypto/tls` and `crypto/x509`.

## [2.3.0] - 2026-10-21

### Added
- `provide` sends the origin of the build: `--build-url` and `--change-url` default to the runner's own variables on GitHub Actions, GitLab CI and Jenkins (overridden by `SANSHAIN_BUILD_URL` / `SANSHAIN_CHANGE_URL`), plus `--tickets` / `SANSHAIN_TICKETS` and `--note` / `SANSHAIN_NOTE`. Sanshain Service 2.9.0 and later stores it with the version and shows it as "why".

## [2.2.0] - 2026-10-16

### Added
- `sanshain-go catalog` pushes the Backstage `catalog-info.yaml` named by the new `catalogInfo` field in `sanshain.yaml`, so Sanshain records the system's Peripheral Resources; a trunk build updates the main graph as well as the dev graph. A file Sanshain cannot read records nothing and is printed as a warning, exiting 0 (failing under strict mode).
- `catalog` requires Sanshain Service 2.4.0; all other commands work with any 2.x service.
- The `SANSHAIN_URL` environment variable overrides `sanshainUrl`, which may then be left out of `sanshain.yaml`.
- Unknown fields in `sanshain.yaml` are named on stderr and ignored; with `strict: true` they fail the command.
- `format: openapi` on a `requires` entry with `apiType: asyncapi` downloads the endpoints as an OpenAPI document (needs Sanshain Service 2.4.0).

### Changed
- Relative paths in `sanshain.yaml` (`file`, `openApiFile`, `asyncApiFile`, `protoFile`, `outputDirectory`) resolve against the directory containing `sanshain.yaml` instead of the working directory.
- Declaring both trunk and a tag (`--trunk`/`SANSHAIN_TRUNK` and `--tag`/`SANSHAIN_TAG`) no longer fails: it prints `trunk and tag are mutually exclusive - this build is recorded in neither` and sends neither; strict mode fails instead.
- Every warning Sanshain sends with a response (the `Sanshain-Warning` header, e.g. for an unknown sanshain-branch) is printed as `⚠ <text>`, one line per warning; under strict mode it fails the command with `strict: Sanshain warned: <text>`, even with best-effort.
- `SANSHAIN_GA`, `SANSHAIN_TRUNK` and `SANSHAIN_BEST_EFFORT` are on for `true` or `1`, case-insensitive and trimmed; anything else is off.

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
- `--version` prints the client version.
- `--best-effort` and `SANSHAIN_BEST_EFFORT` turn failures, including a config that cannot be loaded, into warnings.
- A `409` on provide shows the server's message and the version to publish as instead.
- A `404` on require (unknown producer or version) and a `410` (endpoint absent in that version) get distinct error messages.
- Against a pre-2.0 server, a failed call reports the server version and asks for an upgrade.

### Changed
- Speaks the Sanshain Service 2.x API; the provided version is read from the spec file (`info.version`, or a `// sanshain-version:` comment for proto).
- The module moved to the `/v2` major-version path.
- Every `requires` entry needs an exact `version` pin.
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
- A missing `serviceName` or a missing provide configuration now warns and skips instead of failing.

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
