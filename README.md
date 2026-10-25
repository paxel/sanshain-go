# sanshain-go

[![Build and Test](https://github.com/paxel/sanshain-go/actions/workflows/build.yml/badge.svg?branch=summer)](https://github.com/paxel/sanshain-go/actions/workflows/build.yml)
[![Security](https://github.com/paxel/sanshain-go/actions/workflows/security.yml/badge.svg?branch=summer)](https://github.com/paxel/sanshain-go/actions/workflows/security.yml)
[![Release](https://img.shields.io/github/v/release/paxel/sanshain-go)](https://github.com/paxel/sanshain-go/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxel/sanshain-go/v2.svg)](https://pkg.go.dev/github.com/paxel/sanshain-go/v2)
[![Go version](https://img.shields.io/github/go-mod/go-version/paxel/sanshain-go)](https://github.com/paxel/sanshain-go/blob/summer/go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue)](https://github.com/paxel/sanshain-go/blob/summer/LICENSE)

A Go-based CLI tool and client for the [Sanshain Service](https://github.com/paxel/sanshain-service). It allows Go microservices to provide their API specifications and download version-pinned API dependencies during build time, following the same patterns as the Sanshain Maven and JavaScript clients.

**Compatibility: client 2.x speaks Sanshain Service 2.x.** The 2.0 release is a clean break from the 1.x branch model — there is no dual-mode operation. Against a pre-2.0 server the client fails with a clear "upgrade the server" message.

## The 2.0 model in one paragraph

Branches are gone. Every **provide** publishes a spec under the version declared *inside the spec file* — `info.version` for OpenAPI/AsyncAPI, a mandatory `// sanshain-version:` comment for proto. Accepted spellings are `MAJOR[.MINOR[.PATCH]]` with an optional leading `v`; omitted parts are zero (`v2` becomes `2.0.0`) and the stored form is always the full three-part version. Each provide is a `snapshot` (overwritable work-in-progress) unless the **ga switch** is set (`SANSHAIN_GA=true` or `--ga`), which publishes an immutable `ga` version. Every **require** pins an exact version — no ranges, no `latest`, no fallback, no waiting.

## Features

- **Provide**: Upload your OpenAPI/AsyncAPI/Proto spec; the version travels inside the spec file.
- **Require**: Download specific endpoints of other services at an exact pinned version.
- **Platform Agnostic**: Works in any CI/CD environment (GitHub Actions, GitLab CI, Jenkins, etc.).
- **ETag Caching**: `304 Not Modified` handling avoids redundant downloads and code generation.
- **Standard Configuration**: Uses the standard `sanshain.yaml` file.

## Installation

The 2.0 module lives under the `/v2` major-version path (Go semantic import versioning):

```bash
go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest
```

Importers of the library packages must update their import paths from
`github.com/paxel/sanshain-go/...` to `github.com/paxel/sanshain-go/v2/...`.

## Usage

### Configuration (`sanshain.yaml`)

Create a `sanshain.yaml` file in your project root:

```yaml
sanshainUrl: https://sanshain.example.com
serviceName: my-go-service
provides:
  - file: api/openapi.yaml        # version read from info.version
requires:
  - serviceName: auth-service
    version: 1.2.0                # exact pin, MAJOR.MINOR.PATCH
    outputDirectory: internal/api/auth
    endpoints:
      - method: GET
        path: /users/{id}
```

The environment variable `SANSHAIN_URL` overrides `sanshainUrl`; with it set, `sanshainUrl` can
be left out of `sanshain.yaml`.

Every boolean switch read from the environment (`SANSHAIN_GA`, `SANSHAIN_TRUNK`,
`SANSHAIN_BEST_EFFORT`) is on for `true` or `1` (trimmed, case-insensitive) and off for anything else.

Every relative path in `sanshain.yaml` (`file`, `openApiFile`, `asyncApiFile`, `protoFile`,
`outputDirectory`, `catalogInfo`) is resolved against the directory that contains `sanshain.yaml`, not the
current working directory. Absolute paths stay as they are. A build that points the client at
another config file (`--config path/to/sanshain.yaml`) therefore finds the same files as a build
started next to it.

Every `requires` entry **must** carry a `version` pin. The 1.x branch-era fields
(`branch`, `timeout`, `baseVersion`, `releaseBranches`) are rejected at config parse
time with a migration hint naming the offending field.

### Commands

#### Provide Spec
Uploads the spec(s) defined in `sanshain.yaml` as a `snapshot`:
```bash
export SANSHAIN_TOKEN=your_token
sanshain-go provide
```

To publish an immutable **GA** version, set the ga switch — typically only on a
protected-branch CI pipeline:
```bash
sanshain-go --ga provide
# or
SANSHAIN_GA=true sanshain-go provide
```

There is no git detection and no branch matching: snapshot is always the default,
GA is always an explicit act. Publishing GA requires the `releaser` role — without
it the server answers `403` and the error names the role and the snapshot fallback.

#### Streams: trunk and release branches

Alongside stability, the pipeline declares which dependency graph its calls
belong to. Like the ga switch this is a property of the invocation and never
appears in `sanshain.yaml`:

```bash
sanshain-go --trunk provide      # trunk CI: maintains the main graph (also SANSHAIN_TRUNK=true)
sanshain-go --tag R1 provide     # release/hotfix pipeline: updates that sanshain-branch (also SANSHAIN_TAG)
```

The same flags apply to `require` — trunk pins feed the main graph. Declaring
both records the build in neither graph: it prints
`⚠ trunk and tag are mutually exclusive - this build is recorded in neither` and
sends neither (strict mode fails before any request). An unknown tag is recorded
in no release graph — the call succeeds and Sanshain warns (see [Warnings](#warnings));
a releaser must create the branch first.

#### Origin: why this build provides

A provide may carry what the pipeline knows about *why* it runs (Sanshain Service 2.9.0 and
later). Sanshain stores it with the version and shows it on the version row, in the diff and the
blame:

```bash
sanshain-go --tickets ORD-7,ORD-9 --note "Adds the users endpoint" provide
```

- `--build-url` / `SANSHAIN_BUILD_URL` and `--change-url` / `SANSHAIN_CHANGE_URL` (the pull or
  merge request) default to the runner's own variables on GitHub Actions (`GITHUB_SERVER_URL`,
  `GITHUB_REPOSITORY`, `GITHUB_RUN_ID`, a pull request's `GITHUB_REF`), GitLab CI (`CI_JOB_URL`,
  `CI_MERGE_REQUEST_PROJECT_URL`, `CI_MERGE_REQUEST_IID`) and Jenkins (`BUILD_URL`, `CHANGE_URL`).
- `--tickets` / `SANSHAIN_TICKETS` (comma-separated ids) and `--note` / `SANSHAIN_NOTE` come only
  from the pipeline.

The latest provide that carries any of it wins; a byte-identical re-provide only fills an empty
origin. Nothing is read from git. A service before 2.9.0 refuses the origin fields; the client then
warns and repeats the provide without them, so upgrading the client first never fails a build.

A flag wins over its `SANSHAIN_*` variable, which wins over the runner's variable. A blank value
counts as unset, so an empty flag or variable does not switch the runner's URL off; only unsetting
the runner variable does. Other CI systems (Azure Pipelines, Bitbucket Pipelines, CircleCI,
Buildkite, TeamCity, …) fill nothing: set `SANSHAIN_BUILD_URL` and `SANSHAIN_CHANGE_URL` from their
variables. The service accepts http(s) URLs, at most 20 ticket ids made of letters, digits, `-`, `_`
and `.`, and a note of at most 200 characters.

This client sends no `dry_run`. A pull-request pipeline that wants Sanshain to check the version
rules without storing anything calls the route with `curl` and `"dry_run": true` (see
[CI integration](https://paxel.github.io/sanshain-service/developer/ci-integration.html#dry-runs-for-pull-requests)).

#### Retiring a protocol

Removing a `provides` entry tells Sanshain nothing — it cannot distinguish a
dropped protocol from a pipeline that stopped running. Keep the entry and mark
it:

```yaml
provides:
  - apiType: asyncapi
    retired: true
```

The next `provide` retires that family: the capability tag is cleared, it
leaves the current dependency graph, and its AsyncAPI channel contracts are
released. Version history and existing Consumer pins are untouched. Retiring
needs the `releaser` role — which a release pipeline already holds — or a
maintainer grant on the Producer.

> ⚠️ **AsyncAPI 2.x perspective.** Sanshain reads 2.x `publish`/`subscribe`
> from the **application's** perspective: `publish` means *this service
> publishes to the channel*, `subscribe` means *this service consumes it*. The
> AsyncAPI 2.x specification defines those keywords from the **client's**
> perspective — exactly inverted. Sanshain deliberately uses the
> application-perspective reading because it matches the unambiguous 3.x
> `send`/`receive` mapping. A document authored with the spec-literal reading
> registers its contracts, and has its subscriptions harvested, exactly
> backwards. Harvested subscriptions are printed after every AsyncAPI provide;
> those with drift or no publisher yet are printed as warnings and never fail
> the build.

#### Require Dependencies
Downloads the pinned API specs to the specified directories:
```bash
export SANSHAIN_TOKEN=your_token
sanshain-go require
```

#### Catalog (Peripheral Resources)

Pushes the project's Backstage `catalog-info.yaml` so the Sanshain network graph shows the
**Peripheral Resources** (databases, buckets, queues, ...) the system depends on. Point to the
file with `catalogInfo` in `sanshain.yaml`:

```yaml
serviceName: orders-service
catalogInfo: catalog-info.yaml   # relative to sanshain.yaml
```

```bash
export SANSHAIN_TOKEN=your_token
sanshain-go catalog
```

**`catalog` requires Sanshain Service 2.4.0 or later**; all other commands work with any 2.x
service. Against an older service the command fails with
`sanshain catalog needs Sanshain Service 2.4.0 or later (server is <version>)`.

- The call is `PUT /catalog/<serviceName>` with the file's content. A trunk build
  (`--trunk` / `SANSHAIN_TRUNK`) updates the main graph as well as the dev graph; any other build
  only the dev graph.
- On success the recorded resources are printed:
  ```
  Catalog recorded for orders-service (main+dev): 2 resources
    resource:default/orders-db
    resource:default/orders-bucket
  ```
- Sanshain reads only the `Component` whose `metadata.name` equals `serviceName`. If it cannot
  (usually a name mismatch), it records nothing and keeps the previous periphery: the command
  prints Sanshain's reason as a warning and exits 0 (fails under strict mode).
- Without `catalogInfo` the command warns and exits 0; with `strict: true` it fails. Strict here
  checks only `serviceName` and `catalogInfo`.
- With best-effort (`--best-effort` / `SANSHAIN_BEST_EFFORT`), a missing file or a failed call
  is a warning and the command exits 0.

Run it once per build as its own CI step, e.g. next to `provide`/`require` (see
[docs/integration.md](docs/integration.md)).

#### Requiring AsyncAPI as OpenAPI (`format`)

A `requires` entry with `apiType: asyncapi` may add `format: openapi` to receive the endpoints as
an OpenAPI document instead of AsyncAPI. That is the only allowed combination; any other `format`
value, or `format` with another `apiType`, is a configuration error before any call is made. The
file name and location are the same as without `format`. To get both formats, list the entry
twice with different `outputDirectory` values:

```yaml
requires:
  - serviceName: order-events
    apiType: asyncapi
    version: 1.4.0
    outputDirectory: internal/events/asyncapi
    endpoints:
      - method: SEND
        path: /orders
  - serviceName: order-events
    apiType: asyncapi
    format: openapi              # needs Sanshain Service 2.4.0 or later
    version: 1.4.0
    outputDirectory: internal/events/openapi
    endpoints:
      - method: SEND
        path: /orders
```

`format: openapi` needs Sanshain Service 2.4.0 or later. An older service ignores it and answers
AsyncAPI; the client detects that and fails with
`format: openapi needs Sanshain Service 2.4.0 or later (server is <version>)`, writing nothing
(best-effort turns it into a warning).
An older service rejects the single-endpoint request or returns AsyncAPI; the client reports
that it needs 2.4.0.

#### Version
```bash
sanshain-go --version
```

## Failure Modes

- **`409` on provide** — rejected by the version rules (e.g. re-publishing an existing GA
  version with different content, or a semver-dishonest GA). The client surfaces the server
  message and the server's `proposed_version`:

  ```
  Error: version 1.2.0 is GA and immutable
    Publish as 1.3.0 — update info.version in api/openapi.yaml and re-run
  ```

  The client never modifies your spec files — the fix is always a version bump in your own file.

- **`404` on require (Unknown)** — the producer or the pinned version does not exist. This is a
  configuration error and fails immediately; nothing waits for a version to appear. The error
  names the pin and points at `GET /producers/<name>/versions` to list what is available.

- **`410` on require (Absent)** — the pinned version exists but deliberately does not include the
  requested endpoint(s). Distinct from 404: the version is fine, the endpoint selection is not.

- **Pre-2.0 server** — on a failed provide/require the client performs one lazy `GET /version`;
  if the instance is older than 2.0.0 the confusing wire error is replaced with:

  ```
  Sanshain server at https://sanshain.example.com is 1.7.3; this client requires Sanshain 2.x — upgrade the server
  ```

## Require-Side ETag Caching

The CLI stores the `ETag` from require responses in `.sanshain-cache.json` (in the current
working directory) and sends `If-None-Match` on subsequent runs. On `304 Not Modified`:

```
⏭ user-service spec unchanged (304), skipping code generation.
```

A pin on a GA version can never change content; a pin on a snapshot can — which is exactly
what the ETag detects. Cache entries are keyed by producer and pinned version, so 1.x
branch-keyed entries are simply ignored.

## Strict Mode

By default, the CLI warns and skips when configuration is incomplete (no `serviceName`, no `provides`, no `requires`). This makes it safe to run both commands even if only one applies.

To fail on missing configuration, enable strict mode in `sanshain.yaml`:

```yaml
strict: true
```

When strict mode is enabled:
- Missing `serviceName` → exit with error
- No `provides` configured → exit with error
- No `requires` configured → exit with error
- No `catalogInfo` configured (for `catalog`, which checks only `serviceName` and `catalogInfo`) → exit with error
- Any warning Sanshain sends with a response → exit with `strict: Sanshain warned: <text>`, also under best-effort
- Both trunk and a tag declared → exit with error before any request

## Warnings

The dependency graph is informational and never fails a build. Where Sanshain does not record a
call in a graph — an unknown `--tag`, a `catalog-info.yaml` it cannot read — the call succeeds and
Sanshain sends a `Sanshain-Warning` header. Each one is printed as `⚠ <text>`, one line per
warning, and the build carries on:

```
⚠ no sanshain-branch 'R9' - this build was not recorded in a release graph
```

Under strict mode a warning fails the command right after the call, with
`strict: Sanshain warned: <text>` (the first warning); best-effort does not soften that.

## Unknown Fields

A key in `sanshain.yaml` that no Sanshain client knows (usually a typo) is reported once per run
on stderr before any other validation error, and the run continues:

```
   /\_/\  ?
  ( O_o )   sanshain.yaml: unknown field 'catalogInf' - ignored
 ---U-U---
```

Nested keys are named by their path, e.g. `provides[1].verison` or
`requires[0].endpoints[2].methd`. Fields another Sanshain client supports (`force`, `insecure`,
`combine`) are accepted silently. With `strict: true` the same notice ends in
`rejected (strict)` and the command fails, also under best-effort. The 1.x fields listed above
keep failing with their migration hint.

## Migrating from 1.x

1. Put a strict `MAJOR.MINOR.PATCH` version in each provided spec (`info.version`, or a
   `// sanshain-version:` comment for proto).
2. Remove `branch`, `timeout`, `baseVersion` (and `releaseBranches`, if present) from
   `sanshain.yaml` — the config loader rejects them by name.
3. Add an exact `version:` pin to every `requires` entry
   (`GET /producers/<name>/versions` lists what a producer offers).
4. On protected-branch CI pipelines set `SANSHAIN_GA=true` (or pass `--ga`) so releases go
   out as GA; every other build publishes snapshots.
5. Update import paths / install commands to the `/v2` module path.

## Integration with Go Build Process

We recommend using `go generate` to automate the process. See [docs/integration.md](docs/integration.md) for details.

## License

This project is licensed under the GNU Affero General Public License (AGPL-3.0). See the [LICENSE](LICENSE) file for details.
