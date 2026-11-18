# sanshain-go

The Go client of the [Sanshain Service](https://github.com/paxel/sanshain-service): one Go
module, `github.com/paxel/sanshain-go/v2`, that builds the command `sanshain-go`.

| Path              | What it is                                                              |
|-------------------|-------------------------------------------------------------------------|
| `cmd/sanshain-go` | The command: flags, the commands and what they print.                   |
| `internal/config` | Reads and validates `sanshain.yaml`.                                    |
| `internal/api`    | The calls to the service, the wire payloads and the annotation filling. |
| `internal/cache`  | The require state file with the ETag of every require.                  |
| `internal/utils`  | Helpers the other packages share.                                       |

Everything outside `cmd/` is `internal`: the module exports no Go API. What users rely on is
the command, its flags and variables, what it prints, its exit codes, `sanshain.yaml` and the
state file. The user documentation lives on the Sanshain site, in the repository of the
service.

## Agent skills

### Issue tracker

Issues live as GitHub issues in `paxel/sanshain-go`, managed with the `gh` CLI. See
`docs/agents/issue-tracker.md`.

### Triage labels

Default canonical vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`,
`ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `CONTEXT.md` plus `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Ticket IDs

Prefix `sango`, as in `sango-1003`. See `docs/agents/ticket-ids.md`.

## Quality gates

The standard sanshain-go is held to is [paxel/gates](https://github.com/paxel/gates);
`docs/adr/0001-move-to-gates.md` says where sanshain-go adds to it and why. The workflows
call gates' shared workflows, pinned by commit; Dependabot moves the pin.

### Before a commit

A change is done only when the format check, the linter and the tests are green, on the Go
release the `toolchain` line of `go.mod` pins. These are the commands of the shared Go gate;
golangci-lint is v2.15.0 there:

```sh
gofmt -l .                 # prints nothing
golangci-lint run ./...
go test -race -coverprofile=coverage/go.out ./...
```

`.golangci.yml` is the standard's config with gosec, errcheck, revive, bodyclose and
errorlint; the tests are linted too. The `go` line of `go.mod` is the oldest Go that builds
the client, 1.26.0. Do not use what that release does not have.

### How the code is written

- KISS: the straightforward, readable solution before an abstraction. A package and a
  function have one purpose each.
- Small functions that can be tested on their own, formatted with `go fmt`.
- The Go of the `go` line: `any`, not `interface{}`; `errors.Is` and `errors.As`; `slices`
  and `maps` from the standard library; `for i := range n` for a counting loop.
- What repeats is shared: one helper posts to the service (JSON, compression, headers), and
  logic that is the same for OpenAPI, AsyncAPI and proto is written once.

### Coverage ratchet

Line coverage may only go up. The Go gate in `ci.yml` fails under the floor in
`coverage-floors.toml`. Every feature ships with tests that hold or raise coverage; a
feature that lowers it is not done. When coverage rises, raise the floor and the badge in
the same pull request, with gates checked out beside the repository:

```sh
mkdir -p coverage && go test -race -coverprofile=coverage/go.out ./...
python3 ../gates/scripts/to_lcov.py go coverage/go.out > coverage/go.lcov
python3 ../gates/scripts/coverage_ratchet.py --floors coverage-floors.toml
```

Never lower the floor.

### Fuzzing

The fuzz functions stand beside the code, in `fuzz_test.go` of their package: `FuzzConfig`
in `internal/config`, `FuzzAnnotations` and `FuzzResponse` in `internal/api`, `FuzzState` in
`internal/cache`; `fuzz.yml` runs them weekly. Their seeds run with every `go test`. One
function by hand: `cd internal/config && go test -run='^$' -fuzz='^FuzzConfig$' -fuzztime=30s .`

### Before a release

gofmt, golangci-lint, the tests under the race detector, osv-scanner with gates'
`scripts/osv_gate.py`, the coverage ratchet, and `go build ./...` with `go vet ./...` for
`darwin/amd64`, `darwin/arm64` and `windows/amd64`. The version stands in the constant
`version` of `cmd/sanshain-go/main.go` and in the test of `--version` in
`cmd/sanshain-go/main_test.go`. The release workflow runs the shared release gate before it
builds the binaries.

### Security rules

The decisions behind these are in `docs/adr/0001-move-to-gates.md`.

- A finding of the linter is fixed in the code. One that must stay is silenced at its line
  with `//nolint:<linter> // <reason>`, never in `.golangci.yml` and never without the reason.
- Every action in `.github/workflows/` is pinned to a full commit SHA with its version in
  a trailing comment. Never reference an action by tag or branch alone.
- Go is pinned by the `toolchain` line of `go.mod`; the shared workflows and the release
  read it from there. A toolchain bump is a commit of its own.
- Every workflow starts with `permissions: contents: read`; a job that writes widens it
  itself.
- A dependency advisory is fixed by an update. When no fixed release exists, it is accepted
  in `osv-scanner.toml` with the reason sanshain-go is not exposed and an `ignoreUntil` date.
  A licence outside the allowlist is accepted per package with `effectiveUntil`. Never
  ignore without reason and date.
- A code-scanning alert or a SonarQube Cloud finding that stays open is accepted in
  `code-scanning-ignores.toml` with its tool, rule, file, reason and an `ignoreUntil` date.
  Nothing is dismissed in GitHub or marked in SonarQube Cloud.
- The release fails on a known vulnerability, on every open code-scanning alert, secret
  alert and SonarQube Cloud finding that is not accepted, and on a missing `CHANGELOG.md`
  section for the version. The weekly Security run fails on the same, and on unmaintained
  dependencies and passed dates.
- `summer` takes pull requests only.
