# Go Integration Guide

This guide explains how to integrate `sanshain-go` into your Go project's build lifecycle.

## Using `go generate`

You can use `go generate` to trigger the download of API dependencies and subsequent code generation.

1.  **Install `sanshain-go` and a generator** (e.g., `oapi-codegen`):
    ```bash
    go install github.com/paxel/sanshain/sanshain-go/cmd/sanshain-go@latest
    go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
    ```

2.  **Add generate comments** in your Go files (e.g., `internal/api/doc.go`):
    ```go
    //go:generate sanshain-go require
    //go:generate oapi-codegen -package auth -o auth/client.gen.go internal/api/auth/auth-service_bundle.yaml
    package api
    ```

3.  **Run generate**:
    ```bash
    export SANSHAIN_TOKEN=xxx
    go generate ./...
    ```

## CI/CD Pipeline Integration

### GitLab CI
```yaml
generate-api:
  image: golang:1.21
  script:
    - go install github.com/paxel/sanshain/sanshain-go/cmd/sanshain-go@latest
    - sanshain-go require
    # ... run code generation and tests
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
```

### GitHub Actions
```yaml
- name: Sanshain Require
  run: |
    go install github.com/paxel/sanshain/sanshain-go/cmd/sanshain-go@latest
    sanshain-go require
  env:
    SANSHAIN_TOKEN: ${{ secrets.SANSHAIN_TOKEN }}
```

## Tips

- **Insecure Connections**: If your Sanshain instance uses a self-signed certificate, use the `--insecure` flag.
- **Custom Config**: Use `--config path/to/sanshain.yaml` if your config is not in the root directory.
