# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).


## [2.5.0]

### Added
- `autoGa: true` in `sanshain.yaml` makes a provide GA whenever the build declares trunk or a tag; `--ga=false` or `SANSHAIN_GA=false` keeps a build a snapshot.

### Changed
- `--ga=false` now beats `SANSHAIN_GA=true` and publishes a snapshot (before, the variable made it GA).

---

Historical changes have been moved to [OLDER_CHANGES.md](OLDER_CHANGES.md).
