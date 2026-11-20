# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).


## [2.6.1]

### Added
- The release on GitHub carries the binaries with a CycloneDX SBOM of the source, a build provenance attestation and its Sigstore bundle; `gh attestation verify <file> -R paxel/sanshain-go` checks a download ([sango-1005](https://github.com/paxel/sanshain-go/issues/18)). The command behaves as in 2.6.0.

### Changed
- `go.mod` names the toolchain `go1.27.4`, and the released binaries are built with it. `go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest` is not affected: it reads the `go` line, which still asks for Go 1.26.0 or newer. A build inside a clone of the repository with an older Go downloads Go 1.27.4 first, unless `GOTOOLCHAIN=local` is set ([sango-1000](https://github.com/paxel/sanshain-go/issues/13)).

---

Historical changes have been moved to [OLDER_CHANGES.md](OLDER_CHANGES.md).
