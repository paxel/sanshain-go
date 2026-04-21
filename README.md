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
clientName: my-go-service
provide:
  serviceName: my-go-service
  openApiFile: api/openapi.yaml
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

## Integration with Go Build Process

We recommend using `go generate` to automate the process. See [docs/integration.md](docs/integration.md) for details.

## License

This project is licensed under the GNU Affero General Public License (AGPL-3.0). See the [LICENSE](LICENSE) file for details.
