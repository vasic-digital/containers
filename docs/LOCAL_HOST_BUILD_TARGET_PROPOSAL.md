# Proposal: first-class local-host build target (OPEN, NOT IMPLEMENTED)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-26 |
| Last modified | 2026-10-26 |
| Status | OPEN PROPOSAL - pending operator authorisation under Constitution 11.4.74 (extend the containers submodule upstream; no ad-hoc host build) |
| Implemented | NO. No code in this repository implements it. |

## Why this exists

Today the distributed build path in `internal/buildpkg` and `cmd/distributed-build`
targets remote hosts. The Go executor's local entry point is a stub:
`BuildExecutor.LaunchLocalBuild` (`internal/buildpkg/executor.go`) returns the
error "local builds are handled by the shell pipeline, not the Go executor", and
`cmd/distributed-build` prints "local (local builds handled by shell pipeline)"
for local assignments. (As of containers-2.0.0 the stub returns the sentinel
`ErrLocalBuildUnsupported` and the CLI reports local assignments as FAILED
instead of silently skipping them; the build capability itself is still absent.) A consumer that wants to run the build container on the
machine it is already on (for example when no remote build host is reachable)
has no first-class, configuration-driven way to do so through this module.

## Proposed shape (for discussion only)

1. Treat "local host" as a regular `remote.RemoteHost`-like build target selected
   by configuration (a `CONTAINERS_*` variable or a host entry flagged local),
   never by hardcoded names, keeping the module project-decoupled (11.4.28).
2. Implement `LaunchLocalBuild` on top of the existing runtime abstraction
   (`pkg/runtime`, rootless Podman preferred where available, Docker otherwise)
   so the build still runs INSIDE a build container, not on the bare host.
3. Reuse the existing artifact-copy hardening (`pkg/crossbuild`: path-traversal,
   symlink-escape and stale-artifact refusals) for the local path.
4. Keep remote behaviour unchanged; add paired tests and a challenge following
   the repository's anti-bluff and paired-mutation rules.

## Open questions

- Whether the consumer constitution's distributed-build mandate allows a local
  build target at all, or only as an explicit operator-approved fallback.
- Host safety limits for local builds (memory / thread headroom) and how they
  are expressed in configuration.
- Rootless Podman versus Docker as the local default.

## Status and non-claims

Nothing in this document is implemented, tested or released. It must not be
cited as a feature. Implementation requires explicit operator authorisation.
