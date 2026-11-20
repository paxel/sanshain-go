# sanshain-go moves to the gates standard

Decided on 2026-11-18.

sanshain-go had its own gates: a build workflow with golangci-lint at its defaults, gosec
and the tests, govulncheck, a SonarQube Cloud workflow, CodeQL, Scorecard, and a release
that built the binaries and attached them. Every paxel repository had done so, each
differently. [paxel/gates](https://github.com/paxel/gates) is the one standard that replaces
those, as shared workflows a repository calls, and its audit found 19 checks of it missing
here ([issue 11](https://github.com/paxel/sanshain-go/issues/11)).

sanshain-go meets the standard of gates v1.2.0
([sango-1000](https://github.com/paxel/sanshain-go/issues/13) to
[sango-1007](https://github.com/paxel/sanshain-go/issues/20)). Where the standard decides, it
is followed without a note here: the shared OSV gate, Go gate, SonarQube Cloud job, CodeQL,
Scorecard, fuzz, release gate and provenance workflows, one coverage ratchet, the lint
config, the files and the badge row. What follows is where sanshain-go adds to it, or had a
choice.

**The Go release.** `go.mod` keeps `go 1.26.0`, the oldest Go that builds the client, and
gains `toolchain go1.27.4`, the latest stable Go, which CI, the fuzz run and the release
build with. `go install …@version` reads the `go` line alone and is not affected; a build
inside a clone with an older Go downloads the pinned toolchain.

**The oldest supported Go.** The `go` line promises Go 1.26, and the old build workflow
tested on it. The shared Go gate runs on the pinned toolchain, 1.27.4. So CI keeps a job of
the repository's own that runs the tests on Go 1.26 with `GOTOOLCHAIN=local`, because the
toolchain line would otherwise choose there too.

**The other systems.** The tests run on Linux. The release also ships binaries for macOS
and Windows, so CI asks the shared gate to build and vet the code for `darwin/amd64`,
`darwin/arm64` and `windows/amd64`.

**Lints.** The standard's `.golangci.yml` is taken as it is; the staticcheck settings the
repository already had stay beside it, among them ST1005 switched off, because error
messages start with the product name, word for word the same in every Sanshain client.
revive wanted comments on what is exported and `API` in capitals in names. Every package is
`internal`, so the names were changed and nothing that a user sees moved. gosec stays
silenced at eight lines, each with its reason: four where a file named by the user or by
`sanshain.yaml` is read or written, one where `--insecure` switches the certificate check
off, and three in tests. The separate gosec job and govulncheck with its `tool` line are gone;
golangci-lint runs gosec, and osv-scanner reads the Go advisories.

**Coverage.** One part, `go`, over the whole module, measured by the gate with
`go test -race -coverprofile`. The first floor is 87.10, the local measurement on Go 1.27.4
minus 0.1. SonarQube Cloud reads the file the gate measured.

**Fuzz targets.** Four, with the names every Sanshain client uses, as Go fuzz functions
beside the code: `FuzzConfig` reads `sanshain.yaml`, `FuzzAnnotations` fills the
placeholders of annotation values, `FuzzResponse` reads the body, the version and the
warning headers of an answer of the service, `FuzzState` reads the require state file. So
that a fuzzer reaches them without a file or a server, four readers became unexported
functions of their own: `parseConfig`, `decodeProvideResponse`, `warningLines` and
`decodeState`. `FuzzResponse` also covers the look into a required document that tells
whether it is OpenAPI. The answers to a retire, a catalog push and the version call are read
by `encoding/json` alone, inside the calls, and have no target.

**Accepted findings.** SonarQube Cloud reports the line where `--insecure` switches the
verification of the certificate off under two rules, S4830 and S5527.
`sonar-project.properties` used to silence them; they are now accepted in
`code-scanning-ignores.toml` until 2027-02-18, beside the three Scorecard findings of a
single maintainer. No advisory and no licence is accepted: the one dependency,
`go.yaml.in/yaml/v3`, is under MIT and Apache-2.0.

**Release.** The shared release gate checks alerts, findings, vulnerabilities, licences and
the changelog section, and runs no tests. The release has no test job of its own: a Go
module version is published by the tag itself, through the module proxy, so a job in the
release could not hold it back. The binaries are built with the pinned toolchain, where they
used to be built with whatever Go was the newest that day.

**What the release attaches.** The five binaries, for Linux (amd64, arm64), macOS (amd64,
arm64) and Windows (amd64), under the names they always had, with the SBOM of the source, a
build provenance attestation and its Sigstore bundle.

## Consequences

- A branch gets its CI run through its pull request; pushes run CI only on `summer`.
- SonarQube Cloud analyses pull requests as well as `summer`, and its quality gate now
  fails CI.
- The tests run under the race detector.
- govulncheck is gone; osv-scanner reads the advisories for the modules and the licences
  as well.
- Fuzzing, CodeQL, Scorecard and the weekly Security run come from gates; the weekly run
  also fails on SonarQube Cloud findings and secret alerts.
- The release and the weekly Security run need the secret `GATES_ALERTS_TOKEN`, a
  fine-grained token that can read the repository's alerts
  ([sango-1008](https://github.com/paxel/sanshain-go/issues/21)).
- The release's SBOM and Sigstore bundle are named `sanshain-go-<tag>-source.cdx.json` and
  `sanshain-go-<tag>-provenance.sigstore.json`.
- The instruction file of earlier agents, `ai/rules.md`, is folded into `AGENTS.md` and
  deleted.
