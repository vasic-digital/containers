# Migration guide: helixcode-v1.1.0 -> containers-1.2.1

Baseline: tag `helixcode-v1.1.0` (the last release before the project-prefixed
`containers-` scheme; `containers-1.2.0` is the first `containers-*` tag, so no
earlier `containers-*` tag exists to diff against). 1.2.1 has zero code delta
versus 1.2.0 (verified: `origin/main` == `containers-1.2.0`, cb5c727).
Note: `containers-1.2.2` (docs-only) and `containers-1.2.3` (test hygiene, no
production-code change) add no API change; this guide applies to them unchanged.

Method: `golang.org/x/exp/cmd/apidiff -incompatible` per package, old tree =
`helixcode-v1.1.0`, new tree = this release; exported symbols only
(`internal/*` is not importable and excluded).

## Breaking signature changes (source-level)

| Package | Change | Fix in consumer |
|---|---|---|
| `pkg/serviceregistry` | `(*ServiceRegistry).Unregister(name string)` -> `... error` | handle/ignore returned `error` |
| `pkg/serviceregistry` | `(*ServiceRegistry).UpdateHealth(name string, healthy bool)` -> `... error` | handle returned `error` |
| `pkg/emulator` | `Cleanup(ctx)` -> `Cleanup(ctx, avdName string)` | pass AVD name you own |
| `pkg/cuttlefish` | `Cleanup(ctx)` -> `Cleanup(ctx, ownerToken string)` | pass owner token |
| `pkg/runtime` | interface `ContainerRuntime` gained method `Run` (as reported by apidiff; all in-tree runtimes implement it) | custom/fake runtimes must implement `Run` |
| `pkg/boot` | interface `Distributor` gained `Undistribute(ctx) error` | custom/mock distributors must implement it |
| `pkg/emulator` | interface `Emulator` gained `RunADBCommand(ctx, port, args...) ([]byte, error)` | custom/mock emulators must implement it |

## Breaking type-comparability changes

`==`/map-key use no longer compiles for: `applesim.Recording`,
`distribution.Options` (gained `VolumeMountsFor func(...)`),
`emulator.AndroidEmulator`, `AndroidMatrixRunner`, `Containerized`,
`ContainerizedConfig` (gained slice fields such as `GradleProperties`,
`ExtraAPKPaths`). Compare field-wise or via `reflect.DeepEqual`.

## Behaviour changes (no signature change)

- `cuttlefish`: default `NetworkHost=true`; image refs passed after `--`.
- `health.CheckHTTP`: 4xx now UNHEALTHY; only 2xx-3xx healthy.
- `envconfig`: SET-but-invalid env values now return errors, no silent default.
- `cmd/emulator-cleanup` updated for the new `Cleanup` signatures.

## New packages (additive)

`pkg/egress`, `pkg/remoteexec`, `internal/netaddr` (internal, not importable).

## Known consumers (scan of ~/Projects/catalogizer and ~/Projects/constitution/submodules, `*.go` importing `digital.vasic.containers`, excluding containers itself)

| Consumer | go.mod wiring | Importing .go files |
|---|---|---|
| catalogizer `catalog-api` | `replace => ../submodules/containers` | 4 (`cmd/boot/main.go`, `internal/infra/provisioner.go`, `provisioner_test.go`, `main.go`) |
| catalogizer `submodules/helix_qa` | `replace => ../containers` | 9 (`cmd/ocu-dispatch-test`, `cmd/ocu-probe`, `pkg/infra/qa_infra.go`, `pkg/nexus/native/probe/*`, `pkg/nexus/native/remote/*`) |
| catalogizer `constitution/submodules/llmctl` | `replace => ./submodules/containers` | 11 (`internal/registry/*`, `internal/vantage/*`, `internal/gateway/registry_resolver_test.go`) |
| `~/Projects/constitution/submodules/llmctl` | same as above (hub copy) | 11 (same files) |

Totals: 4 + 9 + 11 distinct in the catalogizer tree (24 files; the constitution
hub llmctl copy adds 11 duplicates). `constitution/submodules/containers` and
`catalogizer/submodules/containers` are the library itself. Per-file usage of
the changed symbols (grep): llmctl `internal/registry/registry.go` calls
`serviceregistry` `Unregister`/`UpdateHealth` and `internal/vantage` uses
`runtime.ContainerRuntime`. **Status (verified 2026-10-27): llmctl needs no
further change.** llmctl `v3.3.0` (7cc9e83) already consumes the new API: each
`sr.Unregister(...)` call either checks the returned `error` (lines 366, 552,
565, 594) or returns it to the caller (line 382), all in
`internal/registry/registry.go`; `UpdateHealth` is
returned to the caller (line 326), and `internal/vantage` compiles against
`runtime.ContainerRuntime`. Evidence: `go build ./...` rc=0 and
`go test -count=1 ./internal/registry ./internal/vantage ./internal/gateway`
all `ok` for BOTH llmctl `v3.3.0` and llmctl local `main` (81645ab), each built
against this repository at `containers-1.2.1` (06a0b52), wired through
`replace digital.vasic.containers => ./submodules/containers`. llmctl no longer
vendors `containers` as a gitlink (since `v3.3.0` it is resolved from the hub via
`lib/resolve_deps.sh`), so a consumer links the checkout they want. Earlier
revisions of this guide said llmctl "must be updated"; that was stale and is
withdrawn. `helix_qa` and `catalog-api` were NOT compile-checked against 1.2.x
(UNCONFIRMED); the catalogizer submodule pin `submodules/containers` @ 4a8f04e
is an ancestor of 1.2.0 (7 files differ).
