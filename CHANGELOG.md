# Changelog

## [Unreleased]

(Nothing yet.)

## [1.2.1] - 2026-10-27 (prepared, NOT tagged)

Tag `containers-1.2.1` (to be cut by the operator). Code delta versus
`containers-1.2.0` (cb5c727): NONE; `origin/main` == `containers-1.2.0` at
preparation time. This release adds `MIGRATION.md` (every breaking signature
change since `helixcode-v1.1.0`, verified with `apidiff` + consumer scan) and
corrects the release metadata.

**SemVer deviation (operator decision):** the breaking Go API changes listed in
`MIGRATION.md` were first shipped in `containers-1.2.0` (a minor bump). Strict
SemVer requires a major bump (2.0.0). The operator chose a patch release
`1.2.1` to attach the migration guide without renumbering the already-published
1.2.0 line; consumers MUST NOT assume 1.2.x patch releases are source-compatible
with 1.1.x. See `MIGRATION.md` and `RELEASE_NOTES_1.2.1.md`.

## [1.2.0] - 2026-10-27

Tag `containers-1.2.0` (project-prefixed per section 11.4.151). Range:
`helixcode-v1.1.0..919ad1e` + release-docs commit = 159 commits (21 first-parent),
399 files changed, +45683 / -2371 lines (`git diff --stat`). Commit-type counts by
conventional-commit prefix: fix 82, hardening 54, feat 8, docs 4, govern 2,
chore 2, test 1, style 1, remainder merges/WIP.

Verification run for this release (host-run, NOT containerised; see Known
limitations): `go vet ./...` rc=0; `go test -count=1 -race -p 1 ./...` over 56
packages = 43 ok, 12 with no test files, 1 FAIL (`pkg/remote`, root-caused to
the test environment, see Fixed) and, after the test-only fix, `pkg/remote`
passes (`ok ... 15.4s`) under the default umask 0002.

**Versioning decision (pending owner confirmation):** strict SemVer would call
this release 2.0.0 because exported signatures changed (see BREAKING). The owner
picked 1.2.0 by default; that decision is recorded as PENDING, not final.

### BREAKING (Go API, source-level) - verified against `git diff` of non-test code

Earlier drafts stated "no breaking changes"; that is NOT correct. The following
exported signatures changed since `helixcode-v1.1.0`:

- `serviceregistry.(*ServiceRegistry).Unregister(name)` and `UpdateHealth(name, healthy)`
  now return `error` (a551ffd, persistence errors are surfaced).
- `emulator.Cleanup(ctx)` is now `Cleanup(ctx, avdName string)` and
  `cuttlefish.Cleanup(ctx)` is now `Cleanup(ctx, ownerToken string)` (552be2c,
  process kill is scoped to OUR device; the host-wide reap fallbacks were removed).
  `cmd/emulator-cleanup` was updated accordingly.
- Behaviour change: `cuttlefish` default `NetworkHost=true` and image refs are
  passed after `--` (fbff142). `health.CheckHTTP` treats 4xx as UNHEALTHY and
  accepts only 2xx-3xx (d21f0d8). `envconfig` now surfaces SET-but-invalid env
  values as errors instead of silently defaulting (90cb2f9).

Callers of the changed signatures must be updated; this is why the proposed
version is a minor bump on a 1.x line only if the maintainers accept the
signature changes (decision pending).

### Added
- `emulator-matrix`: `--build-type` and `--gradle-property` flags to close the
  release-variant device-testing gap (4a8f04e).
- `emulator`: install extra APKs alongside the primary matrix APK (0788dd7);
  containerised-runner support in `RunCanary` / `emulator-canary` (7fa49f7);
  LVA-014 durable device-gate fixes: baked-AVD resolution, `WaitForBoot`
  container liveness, per-API image template (746efe3).
- `network`: race-free `ListenEphemeral` port helper (0053986).
- `orchestrator`: dependency-ordered startup in topological waves (c8178cd).
- `egress` + `remoteexec`: SOCKS egress verification, durable remote execution
  and durable-run shell (`scripts/lib/durable-run.sh`) (cde354f).
- QEMU end-to-end integration test: QMP capability negotiation, `query-version`,
  clean exit (feeba3f).
- Docs: `docs/PRE_INTEGRATION_MATERIALS.md`.

### Changed
- `lifecycle`: idle-shutdown, lease and lazy-boot reworked for race/leak safety
  (see Fixed); `lazyservice` detects dependency cycles instead of overflowing
  the stack (406e7b9).
- `scheduler`: gpu-affinity fitting-zero-score false negative fixed and
  deterministic tie-breaks across all 5 strategies (f0349a8).
- `runtime`: `AutoDetect` now honours `SetRuntimePriority` (9e92e51); docker /
  nerdctl `List` populate `Created` and `Ports` (c7cd749).
- `health` / `endpoint`: IPv6 literals are bracketed correctly (11280cc, f810e6d,
  6158d9b).
- Governance: universal covenant anchors carried (9da662f), four-carrier
  lockstep restored (4dab992), GEMINI.md mirror backfilled, upstream scripts
  lowercased per section 11.4.29, shebangs made portable (d940b51).

### Fixed / hardened (82 fix + 54 hardening commits; selected, by area)
- Security (argument / shell injection class): shell-quoting of caller- and
  registry-controlled values spliced into remote commands (7f7649a, 4b89cda,
  3c1dc01, 70a9084, e97636e, 2862ec0, cf52b5f); leading-dash ssh/scp/simctl/
  device-name refusal and `--` end-of-options before image refs (14e73b5,
  5627348, 3a52825, 26a1009, c76d4f5, ddc4b45, a028b4c); VM guest-shell/argv
  injection (b745208); crossbuild artifact path-traversal, symlink exfiltration
  and FIFO copy-hang (cc4281a, 69df446, 4884191); dotenv inline-comment
  credential corruption (d293baa).
- Concurrency / lifecycle: ReleaseFunc double-release, idle-vs-lease reclaim,
  TOCTOU double-boot, Stop-during-Start resurrection, event bus Close hang and
  self-unsubscribe deadlock, panicking subscriber/`onIdle` containment
  (9470007, a9c660d, c6d401c, 031d305, da4c3b8, 2f3c0b1, 6800059).
- Durability / honesty: atomic registry persistence, fsync-before-rename cache
  durability and corrupt-blob quarantine, remoteexec honest transport and
  sentinel reporting, boot rollback tears down distributed endpoints
  (06e9872, 686db32, ca83bd7, 9b8d6f5, fc25fc8).
- Resource leaks / teardown: tunnel port leaks and dead-tunnel reaping,
  `Undistribute` of `StateMigrating` containers, deadline-bounded waits across
  emulator / cuttlefish / genymotion / brokertest / applesim / vm
  (bfd8fd3, 7c228f8, 8e16441, 1ec51e7, 5e38856, 793d402, a900f3a, 822a6d1).
- Challenges / scanners: a challenge no longer certifies a panicking binary as
  PASSED (7f59225); the CONST-033 host-power scanner gains a paired
  `--prove-failure` proof and stops reporting false greens (919ad1e, VSC-368);
  scripts no longer report success for work never done (b3758dd).
- Per-package fix/hardening commit counts (measured from subject prefixes; a
  commit naming several packages counts for each): remote 8, lifecycle 8,
  vm 7, serviceregistry 7, network 7, emulator 7, distribution 7, volume 6,
  scheduler 5, runtime 5, health 5, event 5, and further packages (`git log`).

- Tests: `pkg/remote` now pins umask 0022 in `TestMain`
  (`main_umask_unix_test.go`). Under umask 0002 `t.TempDir()` is 0775 and the
  pool's hardening (refuse a pre-existing group/other-writable control-socket
  dir) rejected it; `TestNewConnectionPool` then dereferenced a nil pool and the
  panic aborted the whole package. Test-only change, no product code touched.

### Known limitations / imperfections
- `internal/buildpkg/executor.go` `LaunchLocalBuild` is a stub that returns an
  error ("local builds are handled by the shell pipeline, not the Go executor");
  the design question is open in `docs/LOCAL_HOST_BUILD_TARGET_PROPOSAL.md`.
- Test run was on the local host, not inside a build container (the repository
  ships no containerised test target); section 11.4.173 compliance for this
  release is therefore NOT claimed.
- The test-only umask fix has not had an independent review (section 11.4.142 /
  11.4.209); it is owed.
- Tag naming in this repository is inconsistent: `helixcode-v1.1.0`,
  `helix_translate-2.3.x`, `1.0.2-dev-0.0.2`, `1.1.0-dev`, `1.1.1-dev-0.0.x`,
  `yole-1.0.0-0.0.0.0.6-dev`. `containers-1.2.0` follows section 11.4.151
  (`<prefix>-<version>`); no `v1.2.0` alias was created because no existing tag
  uses a bare `vX.Y.Z` form.
- HTML/PDF twins of README/CHANGELOG/docs are stale (no exporter in the repo).

### Notes
- The former `[Unreleased]` entries (Apple `container` crossbuild backend,
  GPU-aware scheduling, `CONTAINERS_REMOTE_HOST_N_GPU_AUTOPROBE`) were already
  contained in tag `helixcode-v1.1.0` (verified: commits 61e01dc and a3ee5ce are
  ancestors of that tag); they are kept below under that tag, not re-announced.
- HTML/PDF twins of this file are NOT regenerated (no documented exporter in
  the repository); they are stale until regenerated.

## [helixcode-v1.1.0] - 2026-06-23 (retroactive heading; content previously under [Unreleased])

### Added
- Apple `container` crossbuild backend (`pkg/crossbuild/apple_container.go`
  + `apple_container_run.go`): boots a real Linux micro-VM via Apple's
  `container` CLI on macOS / Apple-Silicon hosts and bind-mounts host
  directories into it, providing the same Linux-target capability the
  podman/docker `LinuxContainerBackend` provides on Linux. Selected on
  darwin/arm64 ahead of the podman/docker fallback. Anti-bluff evidence
  (host darwin/arm64, container CLI v1.0.0): `go test -tags=integration
  -run AppleContainer` PASS — `container uname -s -m` => `Linux aarch64`
  while the host is Darwin (unforgeable real-Linux-kernel-on-macOS proof);
  host-dir mount round-trips both ways; `challenges/scripts/apple_container_linux_challenge.sh`
  PASS, `--mutate` => exit 99 (stripping `--mount` breaks the round-trip).
  Project-decoupled (§11.4.28): no consumer-project references.
- GPU-aware scheduling: `HostResources.GPU`, `GPURequirement`,
  `StrategyGPUAffinity`, `GPUHealthCheck`, `ProbeGPU`. See
  `docs/gpu-scheduling.md`.
- `CONTAINERS_REMOTE_HOST_N_GPU_AUTOPROBE` env var.
