---
schema_version: 1
constitution_rule: CONST-035
last_audit: 2026-05-01
---

# Behavior Anchor Manifest — Containers

Every row is a user-facing capability and the single anchor test that
proves it works end-to-end. See CONST-035 in `CONSTITUTION.md`.

## Status legend

- `active` — anchor exists and is callable; capability is verified.
- `pending-anchor` — capability declared, anchor test does not yet
  exist. Listed in `challenges/baselines/bluff-baseline.txt` Section 3.
  Reducing this state is the work of campaign sub-project 4.
- `retired` — capability removed; row kept for history.

## Path format

For Go tests: `<path>.go::<TestFuncName>`. Verifier greps for
`func <TestFuncName>\b` in the file.

## Capabilities

| id | layer | capability | anchor_test_path | verifies | status |
|----|-------|------------|------------------|----------|--------|
| CAP-001 | submodule:Containers | Boot all registered services via BootManager | pkg/boot/manager_test.go::TestBootManager_BootAll_BasicSuccess | BootManager.BootAll() brings up registered endpoints and reports success | active |
| CAP-002 | submodule:Containers | Parse `docker compose ps` status output into structured records | pkg/compose/orchestrator_test.go::TestParseStatusOutput_ValidLines | Compose status parser handles valid status table lines correctly | active |
| CAP-003 | submodule:Containers | Discover service via TCP probe | pkg/discovery/tcp_test.go::TestTCPDiscoverer_Discover_Success | TCPDiscoverer.Discover() reports the service when port is reachable | active |
| CAP-004 | submodule:Containers | Distribute container locally via DefaultDistributor | pkg/distribution/distributor_test.go::TestDefaultDistributor_Distribute_Local | Distributor places container on local host when no remotes are eligible | active |
| CAP-005 | submodule:Containers | Build a service Endpoint with sane defaults | pkg/endpoint/builder_test.go::TestBuilder_Defaults | Endpoint builder produces a configured endpoint with default port/proto | active |
| CAP-006 | submodule:Containers | Load remote host registry from environment variables | pkg/envconfig/parser_test.go::TestLoadFromEnv | LoadFromEnv parses CONTAINERS_REMOTE_HOST_N_* and registers hosts | active |
| CAP-007 | submodule:Containers | Publish/subscribe lifecycle events on the event bus | pkg/event/bus_test.go::TestDefaultEventBus_PublishSubscribe | EventBus.Subscribe() receives events published with Publish() | active |
| CAP-008 | submodule:Containers | Construct DefaultChecker that registers built-in TCP/HTTP/gRPC health checks | pkg/health/checker_test.go::TestNewDefaultChecker_RegistersBuiltins | DefaultChecker exposes tcp/http/grpc check types out of the box | active |
| CAP-009 | submodule:Containers | Register a service with the lifecycle manager | pkg/lifecycle/manager_test.go::TestDefaultManager_Register | LifecycleManager.Register() returns no error for fresh service ID | active |
| CAP-010 | submodule:Containers | Collect system resource snapshot (CPU/memory/disk) | pkg/monitor/system_test.go::TestDefaultSystemCollector_Collect | SystemCollector returns a populated snapshot with non-zero values | active |
| CAP-011 | submodule:Containers | Allocate an unused port | pkg/network/port_allocator_test.go::TestPortAllocator_Allocate | PortAllocator returns a free port and tracks it as in-use | active |
| CAP-012 | submodule:Containers | SSH executor surfaces invalid-host error cleanly | pkg/remote/ssh_executor_test.go::TestSSHExecutor_Execute_InvalidHost | SSHExecutor.Execute against unreachable host returns descriptive error, not panic | active |
| CAP-013 | submodule:Containers | Auto-detect available container runtime, preferring Docker | pkg/runtime/detect_test.go::TestAutoDetectWith_DockerFirst | runtime.AutoDetect() prefers docker over podman/containerd when all are present | active |
| CAP-014 | submodule:Containers | Schedule containers with resource-aware strategy | pkg/scheduler/scheduler_test.go::TestDefaultScheduler_Schedule_ResourceAware | Scheduler places containers on hosts with sufficient CPU/memory headroom | active |

(More capabilities — distributor failover, ctop monitor display, plugin
event hooks, integration/remote/security flows — populated in subsequent
iterations of sub-project 3.)
