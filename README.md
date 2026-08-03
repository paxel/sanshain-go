# sanshain-go

A Go-based CLI tool and client for the [Sanshain Service](https://github.com/paxel/sanshain). It allows Go microservices to provide their API specifications and download version-pinned API dependencies during build time, following the same patterns as the Sanshain Maven and JavaScript clients.

**Compatibility: client 2.x speaks Sanshain Service 2.x.** The 2.0 release is a clean break from the 1.x branch model — there is no dual-mode operation. Against a pre-2.0 server the client fails with a clear "upgrade the server" message.

## The 2.0 model in one paragraph

Branches are gone. Every **provide** publishes a spec under the version declared *inside the spec file* — `info.version` for OpenAPI/AsyncAPI, a mandatory `// sanshain-version: MAJOR.MINOR.PATCH` comment for proto. Each provide is a `snapshot` (overwritable work-in-progress) unless the **ga switch** is set (`SANSHAIN_GA=true` or `--ga`), which publishes an immutable `ga` version. Every **require** pins an exact version — no ranges, no `latest`, no fallback, no waiting.

## Features

- **Provide**: Upload your OpenAPI/AsyncAPI/Proto spec; the version travels inside the spec file.
- **Require**: Download specific endpoints of other services at an exact pinned version.
- **Platform Agnostic**: Works in any CI/CD environment (GitHub Actions, GitLab CI, Jenkins, etc.).
- **ETag Caching**: `304 Not Modified` handling avoids redundant downloads and code generation.
- **Standard Configuration**: Uses the standard `sanshain.yaml` file.

## Installation

The 2.0 module lives under the `/v2` major-version path (Go semantic import versioning):

```bash
go install github.com/paxel/sanshain/sanshain-go/v2/cmd/sanshain-go@latest
```

Importers of the library packages must update their import paths from
`github.com/paxel/sanshain/sanshain-go/...` to `github.com/paxel/sanshain/sanshain-go/v2/...`.

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
GA is always an explicit act.

#### Require Dependencies
Downloads the pinned API specs to the specified directories:
```bash
export SANSHAIN_TOKEN=your_token
sanshain-go require
```

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
