# Changelog

## [Unreleased]

CAT-DEVEMU, gaps G1, G2 and G11 of the device-emulation plan. **PROVISIONAL**:
unit-tested with paired mutants; one rootless-podman cgroup read-back test
(alpine, no pull). No emulator was booted and no live Android/Tauri run exists.

### Added
- `pkg/runtime` (G1): typed run options `WithRunMemory`, `WithRunMemorySwap`
  (swap defaults to memory), `WithRunCPUs`, `WithRunPidsLimit`, `WithRunDevice`
  (clean path below `/dev`, caller allowlist, `/dev/kvm` opened read-write),
  `WithRunUserns("keep-id")` (podman only; docker/nerdctl get
  `ErrRunUnsupportedOption`), `WithRunPublish` (literal host IP required,
  wildcard needs `WithRunPublishAllInterfaces`), `WithRunLabel`,
  `WithRunImageDigest` (refuses a tag). New sentinels `ErrRunForbiddenOption`,
  `ErrRunUnsupportedOption`, `ErrRunInvalidOption`; `RunSpec.Err`.
- `pkg/hostbudget` (G2): `Admit(Need, Policy, Probe)` returning
  `fits | does_not_fit | undetermined` from `/proc` (60% memory ceiling, free
  reserve, memory/CPU PSI, load per CPU, RLIMIT_NPROC) and an open-for-write
  `KVMUsable` probe; JSON evidence.
- `pkg/emulator` (G2): `ContainerizedConfig.Memory/CPUs/PidsLimit/Admit/EvidenceDir`;
  `Boot` refuses with `ErrHostBudget` before anything is started when the verdict
  is not `fits`; the emulator container and the AVD-listing probe container now
  always carry `--memory`, `--memory-swap` (= memory), `--cpus`, `--pids-limit`.
- `pkg/emulator` (G11): `ReachableAPI` resolves the host-service URL a target can
  reach (Android emulator `10.0.2.2`, container gateway name per runtime, bridge
  IP, host) with optional in-target probes (`ADBProbe`, `ContainerProbe`) and a
  `Reachable` evidence record.

### Changed
- `pkg/emulator`: `kvmAvailable` opens `/dev/kvm` read-write instead of `stat`
  (it now agrees with `pkg/vm`, which delegates to the same
  `hostbudget.KVMUsable`).
- `pkg/remote` `RemoteRuntime.Run` returns `RunSpec.Err` instead of executing a
  refused option set.

### Round 2 (after an independent review returned NO-GO; PROVISIONAL)
- `pkg/runtime`: `--cap-add` is split on commas, `CAP_` stripped, compared
  case-insensitively, and refuses ALL, SYS_ADMIN, SYS_MODULE, SYS_RAWIO,
  SYS_PTRACE in any element; `ns:<path>` and `container:<id>` values of
  `--network/--pid/--ipc/--uts/--cgroupns` (and `allow_host_loopback=true`) are
  refused by policy; `--device` / `--device-cgroup-rule` in ExtraArgs are
  refused (devices only through `WithRunDevice`); bind mounts (`-v`, `-vSRC`,
  `--volume`, `--mount type=bind`, `WithRunVolumes`) whose source is `/`, inside
  `/proc /sys /dev /run /var/run`, `/etc /boot /root /home` themselves, or any
  `*.sock`, are refused (symlinks resolved); `--security-opt label:disable`
  (colon form) is refused. Ordinary subpaths of `/etc /home ...` stay allowed.
- `pkg/hostbudget`: pids axis counts tasks (threads) of the current UID from
  `/proc/<pid>/status`, not every process; an unparsable table is
  `undetermined`; negative `Need` is `undetermined`; `WriteJSON` uses `O_EXCL`
  (no symlink follow, no overwrite); RLIMIT_NPROC now comes from
  `golang.org/x/sys/unix` (fixes the wrong darwin constants; linux, darwin,
  freebsd and windows cross-build).
- `pkg/emulator`: `ContainerizedConfig.RequireKVM` (refuse with `ErrKVMRequired`
  instead of silent TCG fallback); an unwritable `EvidenceDir` is an error
  (`ErrEvidenceWrite`) and nothing starts. `ADBProbe` now runs
  `nc -z ...; echo RC=$?` and requires the marker (absent = could not run);
  `ContainerProbe` separates exit 125-127 / non-exit failures (could not run)
  from other non-zero exits (ran, no connection).
- Notes: `docker`/`nerdctl` containers need
  `--add-host=host.docker.internal:host-gateway` for the G11 URL to resolve
  (podman provides `host.containers.internal`). The development host measured
  about 30 GiB RAM and 16 CPUs, not 251 GiB; thresholds are scaled from `/proc`.

### Round 3 (STRUCTURAL: allowlist replaces the denylist scan; PROVISIONAL)
Review rounds 1 and 2 kept finding new members of one class (host file-system
exposure via mounts, weakened confinement, capability gaps) because free-form
`ExtraArgs` were scanned against a denylist. Replaced, not layered.

- `pkg/runtime` BREAKING for callers of `WithRunExtraArgs`/`WithRunVolumes`:
  `ExtraArgs` may carry only allowlisted flags (README table); `--mount`, `-v`,
  `--volume`, `--cap-add`, `--cap-drop`, `--security-opt`, `--network`, `--userns`,
  `--device`, `--publish`, `--name`, `--user`, `--entrypoint`, `--memory` ... are
  no longer accepted there. CORRECTION of an earlier claim of this entry: it said no consumer passed
  ExtraArgs outside tests. That was wrong: `constitution/submodules/llmctl/internal/
  vantage/vantage.go` calls `WithRunVolumes(probeDir+":/vantage:ro")` and
  `WithRunExtraArgs("-d","--pull=never","--memory",M,"--memory-swap",M,
  "--pids-limit","64","--cap-drop","all","--read-only","--label",...)`. Round 4
  makes that shape expressible with typed options (not edited in llmctl); the
  verbatim old call is refused. Migration: `WithRunDetach()`,
  `WithRunMemory(n)`/`WithRunMemorySwap(n)` (`ParseRunMemory("128m")` converts the
  engine spelling; memory >= 4 MiB), `WithRunPidsLimit(64)` (1..1048576),
  `WithRunCapDropAll()`, `WithRunLabel(k, v)` per label,
  `WithRunMountAllowedRoots(probeDir)` + `WithRunVolumes(probeDir+":/vantage:ro")`
  (the roots call is NEW and required, and probeDir must not be in a denied
  location), and `WithRunExtraArgs("--pull=never","--read-only")` for the rest.
- New typed options: `WithRunMount`/`RunMount`, `WithRunMountAllowedRoots`,
  `WithRunCapAdd`, `WithRunCapAddReason`, `WithRunCapDropAll`,
  `WithRunNoNewPrivileges`; `RunSpec.Reasons`. `WithRunVolumes` is parsed into typed
  mounts (only `ro`/`rw` options) and now needs allowed roots for bind sources.
  Mounts are emitted as `--mount`. `WithRunNetwork` is validated (no `host`,
  `container`, `ns:`, no `pasta:`/`slirp4netns:` option forms).
- `WithRunName`, `WithRunWorkDir`, `WithRunEnv` names are validated.
- `pkg/hostbudget`: `Need.CPUs` NaN/Inf is `undetermined` (evidence stays
  serialisable); one unreadable or unparsable `/proc/<pid>/status` (anything but
  ENOENT) makes the pids axis `undetermined` instead of under-counting;
  `WriteJSON` fsyncs and removes the partial file on failure.
- `pkg/emulator`: `ADBProbe` treats RC 126/127 (nc not executable/found) as
  could-not-run.
- Tests: `run_allowlist_test.go` (a row for every R2-1..R2-4/R2-7/R2-8 form),
  a live rootless-podman test (typed read-only bind mount works; glob/volume-opt/
  rootfs attempts never reach the engine, counted by a wrapping executor) and a
  live capability-bit read-back; `scripts/mutation_g1_round3.sh` (mutant table in
  `scripts/mutants_g1_round3.py`).

Known gaps (round 3): the policy is still a gate, not a sandbox. Not covered:
a named volume that was created elsewhere with `device=`/`o=bind` options (the
volume name is accepted; its backing path is not inspected); a custom podman/docker
graph root outside the listed directories; paths changed between the check and the
engine start (the resolved path narrows, does not close, the race); `WithRunDevice`
and the user-home deny list inspect the LOCAL host, which is wrong through
`RemoteRuntime` (home deny is skipped there, device probing is not); `--add-host`
and `--dns` can still point a container at any address; `--ulimit nproc` interacts
with host pid accounting. Whether the engines accept every flag spelling in the
allowlist on every version is UNCONFIRMED beyond podman on this host.

### Round 4
- Typed `WithRunDetach`, `ParseRunMemory`; bounds on memory (>= 4 MiB), pids
  (<= 1048576), cpus (<= 4096, finite); `--stop-signal` numbers 1..64.
- Policy: a symlinked allowed root is re-checked after resolution; comma/quote
  in mount targets and tmpfs destinations covered by tests; extra denies
  (`~/.bashrc .bash_profile .profile .zshrc .netrc .config/systemd
  .config/autostart .local/bin`, `/mnt /media /snap`); a capability reason is now
  tied to the capabilities it was given for (`RunSpec.Reasons` entries read
  `CAP,CAP: reason`).
- `pkg/hostbudget`: ESRCH is skipped like ENOENT. EACCES (for example a `hidepid`
  mount hiding other users' status files) is NOT skipped: it makes the pids axis
  `undetermined`; on such hosts the axis cannot be `fits`.

### Known gaps (rounds 1-2, superseded in part by round 3)
- Bind-mount policy remainder: other sensitive sources (`/var/lib/containers`,
  `--volumes-from`, `--mount type=volume` aliasing a host path, paths created
  after the check) and capabilities beyond the five listed are not blocked; the
  builder is a policy gate, not a sandbox. `container:` namespace joins are
  refused by policy, not by danger. `WithRunDevice` probes the LOCAL host, which
  is wrong through `RemoteRuntime`. On hosts where `/var/run` is not a symlink to
  `/run` the lexical `/var/run` entry is what protects it (untested here).
- `pkg/cuttlefish` still builds a `--privileged` / `--network host` run argv by
  design (excluded from the rootless emulation path); it is not changed here.
- Default emulator memory (5 GiB), pids (4096) and the `nc -z` in-device probe
  are unmeasured starting points.

### Added (PROVISIONAL)
- `pkg/nasemu/seed` and `cmd/nasemu-seed` (BG-NASEMU NE-3): deterministic
  seed-corpus generator driven by spec DATA, golden manifest writer/parser
  (manifest sha256 mandatory), `io/fs`-based verifier with a closed result and
  diff-code vocabulary (symlinks never followed, parent-symlink escapes
  detected), and path-safe machine-written evidence JSON without wall-clock
  fields. Tests cover 3-run byte identity, tamper cases, verifier
  golden-good/golden-bad fixtures and 21 paired mutants
  (`scripts/nasemu_seed_mutants.sh`, cmp-verified restore). Linux/amd64 only;
  other platforms UNCONFIRMED.

## [2.0.0] - 2026-10-28 (RT-RACE-1 API fix; test and tooling hygiene)

Tag `containers-2.0.0`. Module path unchanged (`digital.vasic.containers`); see
`MIGRATION.md` for why no `/v2` path or bare `v2.0.0` tag exists.

### Breaking
- `pkg/runtime`: exported variable `RuntimePriority` removed (RT-RACE-1). Use
  `GetRuntimePriority()` (copy) and `SetRuntimePriority()`. This fixes the data
  race that 1.2.3 could only flag. Replaced the child-process race-evidence test
  with `priority_api_test.go` (AST check that no exported mutable
  `RuntimePriority` exists, concurrent-accessor race test, copy-semantics test;
  each verified to fail under a mutation).

### Fixed
- `go vet -tags integration ./...` failed: `wave18b_real_podman_stats_test.go`
  used `mockExecutor` (defined only under `!integration`) without the matching
  build tag. Added `//go:build !integration`. `make vet` now vets the default,
  `integration`, and `integration e2e security stress` tag sets.
- `make test-integration` swallowed failures (`2>/dev/null || echo`) and ran
  without `-tags integration`. Real, non-swallowing `test-integration`,
  `test-e2e`, `test-security`, `test-stress` targets added.
- `tests/integration/remote_deployment_test.go` lacked its `integration` tag;
  `TestDistributor_WithMockHosts` renamed `TestDistributor_NoSchedulerErrors`
  (it never used a mock).
- `cmd/distributed-build`: local assignments were printed and silently never
  built. They now yield FAILED results (`ErrLocalBuildUnsupported`), non-zero
  exit. `LaunchLocalBuild` returns the sentinel error.
- Stale comments in `pkg/vm` claiming the SSH/QMP clients are stubs corrected.

### Known limitations
- Local-host build target remains unimplemented (open operator decisions in
  `docs/LOCAL_HOST_BUILD_TARGET_PROPOSAL.md`).
- No LICENSE file in the tree (README says MIT; copyright holder not recorded).
- Remote-deployment integration tests SKIP honestly without a configured host.
- HTML/PDF twins not regenerated unless stated in the release notes.

### CONT-DENY-STRUCT (structural rework of the host-path deny; PROVISIONAL)

Replaces the per-anchor patches of rounds 4-5 with ONE class-wide rule
(`pkg/runtime/run_policy.go`): a single set of protected anchors (filesystem
root, `/home`, the tmp anchor, the user home, every credential child of the
home, every system subtree) is expanded to {lexical,
symlink-resolved (longest existing prefix, tolerant of missing tails)} forms.
A candidate path (also in all forms) is denied iff for any anchor form A:
P == A, P is under A (tree anchors only: credential children, system
subtrees, tmp state prefixes), or A is under P (ancestor mount, all anchors).
Closes N1 (symlinked credential child), N2 (ancestor of a credential child,
e.g. `~/.config`, `~/.local`) and N3 (unset HOME in local mode now falls back
to the passwd home, else refuses: fail closed). No ancestor of the home may be
mounted; a work dir under the home that is neither an ancestor nor a
descendant of a protected anchor is accepted. Remote mode stays purely lexical
(the local filesystem is not consulted). Anchors are injectable
(`denySubtrees`, `tmpAnchor`, `homeDir`, `fallbackHome`) so resolved system
and tmp anchors are tested through the real path. Behaviour change: mounting
`~/.config`, `~/.local`, `~/.local/share` (ancestors of credential children)
is now refused. Limits: a symlink loop or unreadable component stops
resolution at that prefix; check-to-use races are not closed.

### Round 5 (after reviewer round 4; PROVISIONAL)

- Fix (F1, important): a symlinked HOME no longer bypasses the home/credential
  denies for the HOME anchor. NOTE (corrected in CONT-DENY-STRUCT): round 5 did
  NOT resolve the credential children, ancestors of them, or an unset HOME, and
  its tests did not exercise resolved system or `/tmp` anchors; the earlier
  wording claimed every anchor was resolved, which overstated it.
- Denies added (F4): `/tmp` `tmux-*`, `ssh-*`, `.X11-unix`, `systemd-private-*`;
  home `.config/gh`, `.git-credentials`, `.local/share/keyrings`, `.pgpass`.
- Container targets containing CR, LF or TAB are refused (F5).
- Tests: symlinked-HOME probe mirror, every `/tmp` prefix, the new home children,
  CR/TAB targets; the real-socket leg now `t.Skip`s with a reason instead of
  silently passing (F3). Mutant table extended to 81 entries.
- Note (F6): commit a8b9fc0 ("README round 4 additions") carried a transient
  mutant in `pkg/runtime/run_policy.go` by mistake; it was repaired by a743b8f.
  History was not rewritten (no force-push).

## [1.2.3] - 2026-10-27 (test hygiene; no production-code change)

Tag `containers-1.2.3`. Production (non-`_test.go`) code delta versus
`containers-1.2.2` (4f507ca): NONE. Public API unchanged (`MIGRATION.md` gains only a note).

- test: `make anti-bluff-scan` reported 17 pre-existing `BLUFF-G-001`
  (`t.Skip` without exempt marker) hits. Each was classified by reading the
  surrounding code. 16 were genuine conditional skips with a concrete
  environmental reason (no POSIX shell / Windows, no `/dev/kvm`, no
  `systemd --user`, running as root, `-short`, QEMU absent, or an inert
  subprocess worker) and now carry the exact `// SKIP-OK: <reason>` marker the
  scanner accepts. They still run on every capable host.
- test: the 17th was a genuine violation:
  `TestRTRace1_DirectAccessRacesWithSetRuntimePriority` (`pkg/runtime`) was an
  unconditional `t.Skip` since its introduction in b325a83, so the RT-RACE-1
  evidence never executed. It is now a real test: under `go test -race` it
  re-executes the test binary as a worker that performs the racy direct read
  of the exported `RuntimePriority` against `SetRuntimePriority` and asserts
  the child really reported `WARNING: DATA RACE` with a
  `runtime.SetRuntimePriority(` frame in `detect.go`. Without `-race` it skips
  for that concrete reason.
  Mutation check: making the worker use `GetRuntimePriority()` (race-free)
  makes the test fail. The underlying race is still flagged, not fixed (the
  clean fix removes an exported symbol and needs an operator API decision).
  New helper files: `pkg/runtime/race_on_test.go`, `race_off_test.go`.
- `make anti-bluff-scan`: 17 hits -> 0.

## [1.2.2] - 2026-10-27 (docs-only)

Tag `containers-1.2.2`. Code delta versus `containers-1.2.1` (06a0b52): NONE.

- docs: `MIGRATION.md` no longer says llmctl must be updated. Verified
  2026-10-27: llmctl `v3.3.0` (7cc9e83) and llmctl local `main` (81645ab)
  both build (`go build ./...` rc=0) and pass their `internal/registry`,
  `internal/vantage`, `internal/gateway` tests against `containers-1.2.1`
  (06a0b52); llmctl already handles the `error` returns of `Unregister` /
  `UpdateHealth`. No code change in this repository.

## [1.2.1] - 2026-10-27

Tag `containers-1.2.1`. Code delta versus
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
