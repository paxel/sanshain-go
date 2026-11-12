# Go Integration Guide

This guide explains how to integrate `sanshain-go` into your Go project's build lifecycle.

## Using `go generate`

You can use `go generate` to trigger the download of API dependencies and subsequent code generation.

1.  **Install `sanshain-go` and a generator** (e.g., `oapi-codegen`):
    ```bash
    go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    ```

2.  **Add generate comments** in your Go files (e.g., `internal/api/doc.go`). `go generate` runs
    each command in the directory of the package holding the directive, not in the module root,
    so point `--config` from there at `sanshain.yaml`; the `outputDirectory` inside it resolves
    against the file's directory. With this `sanshain.yaml` in the module root:
    ```yaml
    serviceName: my-go-service
    requires:
      - serviceName: auth-service
        version: 1.2.0
        outputDirectory: internal/api/auth
        endpoints:
          - method: GET
            path: /users/{id}
    ```
    the directives in `internal/api/doc.go` are:
    ```go
    //go:generate sanshain-go --config ../../sanshain.yaml require
    //go:generate oapi-codegen -package auth -o auth/client.gen.go auth/auth-service.yaml
    package api
    ```
    One endpoint is saved as `auth-service.yaml`; two or more as `auth-service_bundle.yaml`. The
    ETag state file `.sanshain-cache.json` lands in the package directory, so add it to
    `.gitignore`.

3.  **Run generate**:
    ```bash
    export SANSHAIN_URL=https://sanshain.example.com SANSHAIN_TOKEN=xxx
    go generate ./...
    ```

## CI/CD Pipeline Integration

### GitLab CI
```yaml
generate-api:
  image: golang:1.26
  script:
    - go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest
    - sanshain-go require
    # ... run code generation and tests
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

### GitHub Actions
```yaml
- name: Sanshain Require
  run: |
    go install github.com/paxel/sanshain-go/v2/cmd/sanshain-go@latest
    sanshain-go require
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
```

### Providing GA Releases from CI

Every provide is a `snapshot` by default. On the protected-branch (release) pipeline, set the ga
switch so the spec is published as an immutable GA version:

```yaml
- name: Sanshain Provide (GA)
  run: sanshain-go provide
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
    SANSHAIN_GA: "true"
```

The published version is read from the spec file itself (`info.version`, or the
`// sanshain-version:` comment for proto) — there is no branch detection and no version flag.

With `autoGa: true` in `sanshain.yaml` the ga switch can go: a build that declares trunk
(`SANSHAIN_TRUNK: "true"`) or a tag (`SANSHAIN_TAG`) provides GA by itself, and
`SANSHAIN_GA: "false"` keeps one such build a snapshot. The CI token's user needs the
`releaser` role, and a trunk build that changes the spec without a version bump fails with
`409 immutable_ga` and the proposed version.

```yaml
- name: Sanshain Provide (trunk, GA through autoGa)
  run: sanshain-go provide
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
    SANSHAIN_TRUNK: "true"
```

### Pushing the Backstage Catalog from CI

If the project has a Backstage `catalog-info.yaml`, set `catalogInfo` in `sanshain.yaml` and add a
step of its own (needs Sanshain Service 2.4.0 or later):

```yaml
- name: Sanshain Catalog
  run: sanshain-go catalog
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
    SANSHAIN_TRUNK: "true"   # only on the trunk pipeline
```

### Annotations from CI

The `{{NAME}}` placeholders of the `annotations` in `sanshain.yaml` are filled from
`SANSHAIN_SET_NAME` variables (or `--set NAME=value`); annotations need Sanshain Service
2.13.0 or later:

```yaml
- name: Sanshain Provide
  run: sanshain-go provide
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
    SANSHAIN_SET_AREA: "7"
    SANSHAIN_SET_ID: ${{ vars.REQUIREMENTS_ID }}
```

## Tips

- **Insecure Connections**: If your Sanshain instance uses a self-signed certificate, use the `--insecure` flag.
- **Custom Config**: Use `--config path/to/sanshain.yaml` if your config is not in the root directory. Relative paths inside it resolve against that file's directory.
