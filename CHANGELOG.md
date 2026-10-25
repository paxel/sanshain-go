# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).


## [2.4.0]

### Changed
- The release binaries are built with the current Go and need macOS 12 or later (was macOS 11); `go install` still works from Go 1.23.3.
- YAML is read with `go.yaml.in/yaml/v3`, the maintained successor of `gopkg.in/yaml.v3`; `sanshain.yaml` is read as before.

### Security
- The release binaries no longer carry the 36 known vulnerabilities of the Go 1.23 standard library, among them several in `net/http`, `crypto/tls` and `crypto/x509`.

---

Historical changes have been moved to [OLDER_CHANGES.md](OLDER_CHANGES.md).
