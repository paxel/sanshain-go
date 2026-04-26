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
- **Go Idioms**: Follow modern Go idioms (Go 1.23+).
    - Use `any` instead of `interface{}`.
    - Use `errors.Is`/`errors.As` for error handling.
    - Use `slices.Contains`, `maps.Keys`, etc., from the standard library.
    - Use `for i := range n` for simple loops.
- **DRY (Don't Repeat Yourself)**: Avoid code repetition. 
    - Use helper methods (e.g., generic `post` method for HTTP requests) to centralize common logic like JSON serialization, compression, and header management.
    - Unify repetitive logic for different API types or similar command-line operations.
- **Efficient Patterns**:
    - Use slices and loops to check multiple environment variables or configurations instead of long `if-else` chains.
    - Group similar data/logic into structs or maps to reduce boilerplate.
- **Small Functions**: Break down large functions into smaller, testable units.
- **Documentation**: Maintain clear and concise documentation in code and Markdown files.

## 3. Tooling Reference

- **Linter**: [golangci-lint](https://github.com/golangci/golangci-lint)
- **Security**: [gosec](https://github.com/securego/gosec)
- **Format**: `go fmt`
