# Containers — Test Coverage Ledger

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-05-19 |
| Last modified | 2026-05-19 |
| Status | active |
| Round | 299 (template mirror of round-220/295/298) |
| Anti-bluff anchor | §11.4 + CONST-035 + CONST-050(B) |

> Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17): *"all existing
> tests and Challenges do work in anti-bluff manner - they MUST confirm that
> all tested codebase really works as expected! We had been in position that
> all tests do execute with success and all Challenges as well, but in
> reality the most of the features does not work and can't be used! This
> MUST NOT be the case and execution of tests and Challenges MUST guarantee
> the quality, the completition and full usability by end users of the
> product!"*

## Table of contents

- [Purpose](#purpose)
- [Symbol → test ledger](#symbol--test-ledger)
- [Challenge → invariant ledger](#challenge--invariant-ledger)
- [CONST-045 .env-driven remote-host configuration coverage](#const-045-env-driven-remote-host-configuration-coverage)
- [CONST-050(B) compliance summary](#const-050b-compliance-summary)
- [Anti-bluff guarantees](#anti-bluff-guarantees)

## Purpose

This ledger satisfies CONST-050(B) — every production symbol of meaningful
public surface MUST be covered by every test type the domain warrants. The
ledger is the single point of truth answering "is `pkg/<X>.<Func>` covered
by unit + integration + e2e + Challenge + paired mutation?"

A row whose symbol is uncovered is a §11.4 PASS-bluff at the coverage layer:
the suite reports green while the symbol is unproven. The round-299 sweep
populated the ledger from the live tree; subsequent rounds extend it.

## Symbol → test ledger

| Package | Public symbol | Unit | Integration | E2E | Challenge | Paired-mutation |
|---|---|---|---|---|---|---|
| `pkg/runtime` | `AutoDetect`, `DockerRuntime`, `PodmanRuntime`, `KubernetesRuntime` | yes (`pkg/runtime/*_test.go`) | yes (`tests/integration/distribution_integration_test.go`) | yes (`tests/e2e/remote_e2e_test.go`) | `containers_describe_challenge.sh` (round-299) | yes (mutate_describe → exit 99) |
| `pkg/compose` | `ComposeOrchestrator.Up`, `Down`, `Logs` | yes | yes (`tests/integration/distribution_integration_test.go`) | yes (real Docker Compose in `tests/e2e/`) | `ux_end_to_end_flow_challenge.sh` | yes (`mutation_ratchet_challenge.sh`) |
| `pkg/health` | `TCPChecker`, `HTTPChecker`, `GRPCChecker`, `RetryPolicy` | yes | yes (real TCP / HTTP servers) | yes (real services up under `tests/e2e/`) | `ddos_health_flood_challenge.sh` | yes (negation in `mutation_ratchet`) |
| `pkg/envconfig` | `LoadFromEnv`, `LoadFromFile`, `ToRemoteHosts` | yes (`pkg/envconfig/*_test.go`) | yes (round-trips `tests/configs/.env.*`) | yes (boot flow consumes loaded hosts) | `containers_describe_challenge.sh` (round-299; CONST-045 enforcement) | yes (mutate `.env` → describe drift) |
| `pkg/distribution` | `Distributor.Distribute`, `Undistribute`, `Rebalance` | yes | yes (`tests/integration/distribution_integration_test.go`) | yes (`tests/e2e/remote_e2e_test.go`) | `chaos_failure_injection_challenge.sh`, `scaling_horizontal_challenge.sh` | yes (`mutation_ratchet_challenge.sh`) |
| `pkg/scheduler` | 5 strategies (`resource_aware`, `round_robin`, `affinity`, `spread`, `bin_pack`) + `ScheduleBatch` | yes (`pkg/scheduler/*_test.go`) | yes (`tests/integration/`) | yes (live multi-host placement) | `scaling_horizontal_challenge.sh` | yes (mutate strategy → mismatch) |
| `pkg/remote` | `SSHExecutor.Run`, `HostManager.AddHost`, `ProbeAll`, `IsReachable` | yes (mocks ONLY in `_test.go`) | yes (real SSH against `tests/configs/.env.remote-single` when `CONTAINERS_REMOTE_ENABLED=true`) | yes (`tests/e2e/remote_e2e_test.go`) | `containers_describe_challenge.sh` (round-299; SKIP-OK when `CONTAINERS_REMOTE_ENABLED=false`) | yes |
| `pkg/orchestrator` | `Service.Add`, `DiscoverServices`, `StartAll`, `StartService` | yes | yes (`tests/integration/`) | yes (multi-compose boot) | `ux_end_to_end_flow_challenge.sh` | yes |
| `pkg/lifecycle` | `LazyBooter`, `IdleShutdown`, `ConcurrencySemaphore` | yes | yes (real timers) | yes | `stress_sustained_load_challenge.sh` | yes |
| `pkg/network` | `TunnelManager.CreateTunnel`, `Close` | yes | yes (real SSH port-forward) | yes | `chaos_failure_injection_challenge.sh` | yes |
| `pkg/volume` | `VolumeManager.Mount`, `Unmount` (SSHFS/NFS/rsync) | yes | yes (real mounts when supported) | yes | `chaos_failure_injection_challenge.sh` | yes |
| `pkg/event` | `EventBus.Publish`, `Subscribe`, 20 event types | yes | yes | yes | `ux_end_to_end_flow_challenge.sh` | yes |
| `pkg/metrics` | `MetricsCollector.Register`, Prometheus exposition | yes | yes (`/metrics` scrape) | yes | `stress_sustained_load_challenge.sh` | yes |
| `pkg/discovery` | `TCPDiscoverer.Probe`, `DNSDiscoverer.Resolve` | yes | yes (real TCP / DNS) | yes | `ux_end_to_end_flow_challenge.sh` | yes |
| `pkg/i18n` | `Translator.T`, bundle load (`active.en.yaml` + 5 locales added round-299) | yes (`pkg/i18n/*_test.go`) | yes (locale-switch round-trip) | yes (rendered strings reach UI) | `containers_describe_challenge.sh` (round-299; asserts 6 locales present) | yes (drop a bundle → loader FAILs) |
| `pkg/ctop` | `Collector.Collect`, `Display.Run`, `RenderSnapshot`, `RenderJSON` | yes | yes (real container snapshots) | yes | `ui_terminal_interaction_challenge.sh` | yes |
| `pkg/boot` | `BootManager.BootAll`, `WithRuntime`, `WithLogger` | yes | yes (real compose Up + health) | yes | `ux_end_to_end_flow_challenge.sh` | yes |

## Challenge → invariant ledger

| Challenge | Invariant exercised | Normal exit | Mutate-mode exit |
|---|---|---|---|
| `anchor_manifest_challenge.sh` | Governance anchors literal-present in CONSTITUTION/CLAUDE/AGENTS | 0 | 99 (strip an anchor) |
| `bluff_scanner_challenge.sh` | No `simulated` / `for now` / `TODO implement` in production code | 0 | 99 (plant a literal) |
| `chaos_failure_injection_challenge.sh` | `Distributor.Rebalance` recovers when a host is force-killed mid-deploy | 0 | 99 (skip the recovery step) |
| `ddos_health_flood_challenge.sh` | Health endpoints survive flood without panic; retry policy honored | 0 | 99 (remove retry wrap) |
| `host_no_auto_suspend_challenge.sh` | CONST-033 host-power-management ban — host does not auto-suspend | 0 | 99 (re-enable auto-suspend) |
| `mutation_ratchet_challenge.sh` | Every covenant gate fails when its production code is mutated | 0 | 99 (gate that survives mutation is a bluff gate) |
| `no_suspend_calls_challenge.sh` | Source tree contains zero suspend / hibernate / poweroff calls | 0 | 99 (plant a call) |
| `scaling_horizontal_challenge.sh` | 5 scheduler strategies converge to consistent placements at N=10 hosts | 0 | 99 (corrupt a strategy's score function) |
| `stress_sustained_load_challenge.sh` | Sustained load (boot/teardown) does not leak SSH sockets or fds | 0 | 99 (skip pool close) |
| `ui_terminal_interaction_challenge.sh` | `ctop` TUI keystrokes drive snapshot/sort/filter end-to-end | 0 | 99 (break the keybind) |
| `ux_end_to_end_flow_challenge.sh` | Full multi-service boot + health + monitor + shutdown UX flow | 0 | 99 (skip the health gate) |
| `containers_describe_challenge.sh` (round-299) | Loaded `.env` host list, 6 i18n bundles, governance anchors, runtime auto-detect, paired-mutation discipline | 0 | 99 (any mutation → fail) |

## CONST-045 .env-driven remote-host configuration coverage

| `CONTAINERS_REMOTE_HOST_N_*` field | Loader | Test asserting round-trip | Challenge |
|---|---|---|---|
| `_NAME` | `pkg/envconfig/parser.go` | `pkg/envconfig/parser_test.go` | `containers_describe_challenge.sh` |
| `_ADDRESS` | same | same | same |
| `_PORT` | same | same | same |
| `_USER` | same | same | same |
| `_KEY` | same | same | same |
| `_RUNTIME` | same | same | same |
| `_LABELS` | same | same | same |

**SKIP-OK discipline (CONST-045):** when `CONTAINERS_REMOTE_ENABLED=false`,
remote-touching tests + challenges emit `SKIP-OK: CONST-045 remote disabled
in this environment` and exit 0 (not 1) — skip is not failure, but skip is
loud (per the parent §SKIP-vs-FAIL decision tree).

## CONST-050(B) compliance summary

- **Unit** — every package has `*_test.go` files; mocks ONLY in `_test.go`.
- **Integration** — `tests/integration/` exercises real Docker / Podman.
- **E2E** — `tests/e2e/remote_e2e_test.go` against real SSH targets.
- **Security** — `tests/security/ssh_security_test.go`.
- **Stress** — `tests/stress/distribution_stress_test.go`.
- **Benchmark** — `tests/benchmark/scheduler_bench_test.go`.
- **Challenges** — 12 (11 pre-existing + `containers_describe_challenge.sh`
  added round-299) under `challenges/scripts/`.
- **Mutation** — every gate has a paired mutation in
  `mutation_ratchet_challenge.sh`; round-299 challenge ships its own paired
  mutation via the `--mutate` flag.

## Anti-bluff guarantees

1. Every row's "Challenge" column points to a script that runs the real
   exerciser. The round-299 `containers_describe_challenge.sh` invokes
   `go run ./cmd/containers-describe` (when present) OR falls back to a
   pure-shell `.env`-driven describe path. Both exit 0 only on real
   success; mutate-mode (`--mutate`) deliberately corrupts a precondition
   and asserts exit 99.
2. CONST-045 SKIP behavior is asserted — a passing run with
   `CONTAINERS_REMOTE_ENABLED=false` MUST emit the SKIP-OK marker; absence
   of the marker on a skipped run is a §11.4.1 FAIL-bluff (silent skip).
3. The 6 locale bundles MUST all load without error; missing-file or
   parse-error on any bundle is a §11.4 PASS-bluff if the suite still
   reports green.
4. No metadata-only PASS — every assertion compares loaded bytes /
   process exit / runtime state, never just file presence.
