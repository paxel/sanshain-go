# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).


## [2.2.0] - Unreleased

### Added
- `sanshain-go catalog` pushes the Backstage `catalog-info.yaml` named by the new `catalogInfo` field in `sanshain.yaml`, so Sanshain records the system's Peripheral Resources; a trunk build updates the main graph as well as the dev graph.
- `catalog` requires Sanshain Service 2.4.0; all other commands work with any 2.x service.
- The `SANSHAIN_URL` environment variable overrides `sanshainUrl`, which may then be left out of `sanshain.yaml`.
- Unknown fields in `sanshain.yaml` are named on stderr and ignored; with `strict: true` they fail the command.

### Changed
- Relative paths in `sanshain.yaml` (`file`, `openApiFile`, `asyncApiFile`, `protoFile`, `outputDirectory`) resolve against the directory containing `sanshain.yaml` instead of the working directory.
- `SANSHAIN_GA`, `SANSHAIN_TRUNK` and `SANSHAIN_BEST_EFFORT` are on for `true` or `1`, case-insensitive and trimmed; anything else is off.

---

Historical changes have been moved to [OLDER_CHANGES.md](OLDER_CHANGES.md).
