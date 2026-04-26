# AI Development Rules & Best Practices

To ensure the long-term maintainability, security, and quality of the `sanshain-go` project, all LLM-based development must adhere to the following rules.

## 1. Validation Pipeline

After any code modification, the following checks MUST be executed locally. A task is not considered complete until all checks are "green" (pass without errors).

- **Tests**: Run the full test suite to ensure no regressions.
  ```bash
  go test ./...
  ```
- **Linting**: Use `golangci-lint` to ensure code quality and adherence to Go standards.
  ```bash
  golangci-lint run ./...
  ```
- **Security**: Use `gosec` to scan for potential security vulnerabilities.
  ```bash
  gosec ./...
  ```

## 2. Code Philosophy & Maintainability

- **KISS (Keep It Simple, Stupid)**: Avoid over-engineering. Prefer straightforward, readable code over complex abstractions.
- **Single Responsibility**: Each package and function should have one clear purpose.
- **Go Idioms**: Follow modern Go idioms (Go 1.23+). Use `any` instead of `interface{}`, `errors.Is`/`errors.As` for error handling, and standard library features where possible.
- **Small Functions**: Break down large functions into smaller, testable units.
- **Documentation**: Maintain clear and concise documentation in code and Markdown files.

## 3. Tooling Reference

- **Linter**: [golangci-lint](https://github.com/golangci/golangci-lint)
- **Security**: [gosec](https://github.com/securego/gosec)
- **Format**: `go fmt`
