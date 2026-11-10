# sanshain-go

[![Build and Test](https://github.com/paxel/sanshain-go/actions/workflows/build.yml/badge.svg?branch=summer)](https://github.com/paxel/sanshain-go/actions/workflows/build.yml)
[![Security](https://github.com/paxel/sanshain-go/actions/workflows/security.yml/badge.svg?branch=summer)](https://github.com/paxel/sanshain-go/actions/workflows/security.yml)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=paxel_sanshain-go&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=paxel_sanshain-go)
[![Security Rating](https://sonarcloud.io/api/project_badges/measure?project=paxel_sanshain-go&metric=security_rating)](https://sonarcloud.io/summary/new_code?id=paxel_sanshain-go)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=paxel_sanshain-go&metric=coverage)](https://sonarcloud.io/summary/new_code?id=paxel_sanshain-go)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/paxel/sanshain-go/badge)](https://scorecard.dev/viewer/?uri=github.com/paxel/sanshain-go)
[![Release](https://img.shields.io/github/v/release/paxel/sanshain-go)](https://github.com/paxel/sanshain-go/releases/latest)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxel/sanshain-go/v2.svg)](https://pkg.go.dev/github.com/paxel/sanshain-go/v2)
[![Go version](https://img.shields.io/github/go-mod/go-version/paxel/sanshain-go)](https://github.com/paxel/sanshain-go/blob/summer/go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue)](https://github.com/paxel/sanshain-go/blob/summer/LICENSE)

A command-line client for the [Sanshain Service](https://github.com/paxel/sanshain-service): a Go project provides its API specifications (OpenAPI, AsyncAPI, gRPC/Proto) under the version declared in the spec file, and downloads the endpoint snippets it depends on, pinned to an exact version, during the build.

**The documentation lives on the Sanshain site: [Go client](https://paxel.github.io/sanshain-service/developer/clients/go.html).** It lists every flag, variable and `sanshain.yaml` field with the version it arrived in, the precedence, the ETag state file, strict mode and best effort, the failure modes, `go generate` wiring and CI recipes. The file itself is described in [`sanshain.yaml`](https://paxel.github.io/sanshain-service/developer/sanshain-yaml.html).

**Compatibility: client 2.x speaks Sanshain Service 2.x.**

## Install

```bash
go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest
```

Prebuilt binaries for Linux, macOS and Windows are attached to each [release](https://github.com/paxel/sanshain-go/releases).

## A minimal example

`sanshain.yaml` in the project root:

```yaml
serviceName: my-go-service
provides:
  - file: api/openapi.yaml          # the version is read from info.version
requires:
  - serviceName: auth-service
    version: 1.2.0                  # an exact pin
    outputDirectory: internal/api/auth
    endpoints:
      - method: GET
        path: /users/{id}
```

With `SANSHAIN_URL` and `SANSHAIN_TOKEN` in the environment (flags go before the command):

```bash
sanshain-go provide          # snapshot (the default)
sanshain-go --ga provide     # immutable GA, from the release pipeline
sanshain-go require          # writes internal/api/auth/auth-service.yaml
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
