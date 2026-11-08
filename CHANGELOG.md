# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).


## [2.5.0]

### Added
- `autoGa: true` in `sanshain.yaml` makes a provide GA whenever the build declares trunk or a tag; `--ga=false` or `SANSHAIN_GA=false` keeps a build a snapshot.
- `annotations` in `sanshain.yaml` travel with provide, require, bundle and catalog push, their `{{NAME}}` placeholders filled from `--set NAME=value` or `SANSHAIN_SET_NAME` (needs Sanshain Service 2.13.0 or later).

### Changed
- `--ga=false` now beats `SANSHAIN_GA=true` and publishes a snapshot (before, the variable made it GA).

### Fixed
- A require with `apiType: grpc` goes to `/require/grpc` and is saved as `.proto`, and its bundle sends `api_type: proto`; it went to `/require` and was saved as `.yaml`.
- Best effort softens failed calls only, the same in provide, require and catalog: an unreadable or invalid `sanshain.yaml` (1.x fields and an unsupported provide `apiType` included) and missing configuration under strict now fail also with best effort, and a failed provide or require no longer stops the next entry.
- A `403` on provide or retire names the releaser role only when the service refused for want of it; any other `403`, such as a read-only token's, shows the service's message alone.
- The require ETag cache in `.sanshain-cache.json` is kept per output directory: the same endpoint required into two directories is written to both instead of the second getting a `304` and staying empty, and a deleted output file is fetched again instead of being skipped.
- An unknown `requires[].apiType` fails as a configuration error when `sanshain.yaml` is loaded, as an unknown provide `apiType` does; it was sent to `/require` as OpenAPI.
- The `go generate` recipe in `docs/integration.md` works as written: it points `--config` from the package directory at `sanshain.yaml` and the generator at the file the client saves, and the GitLab example uses `golang:1.23`, the floor `go.mod` names (it used `golang:1.21`).

---

Historical changes have been moved to [OLDER_CHANGES.md](OLDER_CHANGES.md).
