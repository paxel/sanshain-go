# sanshain-go

A Go-based CLI tool and client for the [Sanshain Service](https://github.com/paxel/sanshain). It allows Go microservices to provide their OpenAPI specifications and download API dependencies during build time, following the same patterns as the Sanshain Maven and JavaScript clients.

## Features

- **Provide**: Upload your local OpenAPI spec to the Sanshain registry.
- **Require**: Download specific API endpoints from other services.
- **Platform Agnostic**: Works in any CI/CD environment (GitHub Actions, GitLab CI, Jenkins, etc.).
- **Automatic Branch Detection**: Detects the current git branch from environment variables or the `git` command.
- **Standard Configuration**: Uses the standard `sanshain.yaml` file.

## Installation

```bash
go install github.com/paxel/sanshain/sanshain-go/cmd/sanshain-go@latest
```

## Usage

### Configuration (`sanshain.yaml`)

Create a `sanshain.yaml` file in your project root:

```yaml
sanshainUrl: https://sanshain.example.com
serviceName: my-go-service
provides:
  - file: api/openapi.yaml
requires:
  - serviceName: auth-service
    outputDirectory: internal/api/auth
    endpoints:
      - method: GET
        path: /users/{id}
```

### Commands

#### Provide Spec
Uploads the local OpenAPI spec defined in `sanshain.yaml`.
```bash
export SANSHAIN_TOKEN=your_token
sanshain-go provide
```

#### Require Dependencies
Downloads the required API specs to the specified directories.
```bash
export SANSHAIN_TOKEN=your_token
sanshain-go require
```

## v0.13.0 Features

### Optimistic Concurrency Control (`baseVersion`)

Add `baseVersion` to your provide configuration to detect concurrent modifications:

```yaml
provides:
  - file: api/openapi.yaml
    baseVersion: 5
```

If the server version has advanced beyond your `baseVersion`, the provide call fails with:

> Concurrent modification detected. Server version has advanced beyond your base_version. Re-run to fetch the latest state.

The CLI automatically tracks the last known version in a local cache, so after the first successful provide, subsequent runs send the correct `base_version` automatically.

### Provide Response Summary

After each successful provide, the CLI logs a human-readable summary:

```
✓ Provided to Sanshain v5: 2 new, 1 updated, 0 deleted endpoints
```

### Client-Side Content Caching (Skip-if-unchanged)

Before uploading, the CLI computes the SHA-256 hash of the spec file and compares it with the cached hash from the last provide. If unchanged:

```
⏭ Spec unchanged (hash match), skipping provide.
```

### Require-Side ETag Caching

The CLI stores the `ETag` from require responses and sends `If-None-Match` on subsequent runs. On `304 Not Modified`:

```
⏭ user-service spec unchanged (304), skipping code generation.
```

### State File

The local cache is stored at `.sanshain-cache.json` in the current working directory. The format is:

```json
{
  "provides": {
    "openapi.yaml": {
      "content_hash": "sha256:abc123...",
      "version": 5,
      "last_provided": "2026-04-25T12:00:00Z"
    }
  },
  "requires": {
    "user-service|main|GET|/api/v1/users": {
      "etag": "\"sha256:def456...\"",
      "last_fetched": "2026-04-25T12:00:00Z"
    }
  }
}
```

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

## Integration with Go Build Process

We recommend using `go generate` to automate the process. See [docs/integration.md](docs/integration.md) for details.

## License

This project is licensed under the GNU Affero General Public License (AGPL-3.0). See the [LICENSE](LICENSE) file for details.
