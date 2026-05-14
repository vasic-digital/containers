# Containers Module Constitution

This Constitution governs the `digital.vasic.containers` module. It
inherits the universal mandatory constraints cascaded from the
HelixAgent root `CLAUDE.md` and applies them to this module's scope.
Module-specific addenda are welcome but cannot weaken or override the
universal rules.

## Scope

`digital.vasic.containers` is a generic, project-agnostic Go module for
container orchestration, health checking, lifecycle management, remote
distribution, and service discovery across Docker, Podman, and
Kubernetes runtimes. It is the single integration point through which
the HelixAgent binary (and any other consumer) brings up its full
container topology — local and remote — driven entirely by the
consumer's `.env` file (`Containers/.env` for HelixAgent).

This module is foundational: it has no upstream sibling modules and is
consumed by `Challenges`, `HelixLLM`, `HelixQA`, and HelixAgent itself.

## Module-Specific Invariants

1. **Project-agnostic.** No hardcoded project-specific package names,
   endpoints, device serials, or fixtures. All consumer-specific data is
   registered via the public API. Default values are empty or generic.
2. **Sole orchestrator role.** The module's runtime is the only sanctioned
   path for container lifecycle operations. Direct `docker`/`podman`
   commands and `docker-compose up|down` are prohibited as workflows in
   any consumer.
3. **Dynamic remote-host enrolment (CONST-031).** Remote hosts are loaded
   from `CONTAINERS_REMOTE_HOST_N_*` env vars (N=1..100). The loader
   (`pkg/envconfig/parser.go`) stops at the first absent `_NAME`. No
   hostname is hardcoded anywhere else in the repo.
4. **Rootless / no sudo.** No `sudo` or `su` in source, scripts, tests,
   or docs. Use rootless container runtimes only.
5. **Health-check parity.** Every service started by this module must
   expose a TCP or HTTP health endpoint and pass `HealthChecker` checks
   with retries before being considered up.
6. **Rebuild-on-change.** Any code change affecting a containerised
   component requires rebuilding and redeploying the container locally,
   and re-running with `CONTAINERS_REMOTE_ENABLED=true` for remote
   distribution.

## Universal Mandatory Constraints

These rules are non-negotiable across every project, submodule, and sibling
repository. They are derived from the HelixAgent root `CLAUDE.md`. Each
project MUST surface them in its own `CLAUDE.md`, `AGENTS.md`, and
`CONSTITUTION.md`. Project-specific addenda are welcome but cannot weaken
or override these.

### Hard Stops (permanent, non-negotiable)

1. **NO CI/CD pipelines.** No `.github/workflows/`, `.gitlab-ci.yml`,
   `Jenkinsfile`, `.travis.yml`, `.circleci/`, or any automated pipeline.
   No Git hooks either. All builds and tests run manually or via Makefile/
   script targets.
2. **NO HTTPS for Git.** SSH URLs only (`git@github.com:…`,
   `git@gitlab.com:…`, etc.) for clones, fetches, pushes, and submodule
   updates. Including for public repos. SSH keys are configured on every
   service.
3. **NO manual container commands.** Container orchestration is owned by
   the project's binary/orchestrator (e.g. `make build` → `./bin/<app>`).
   Direct `docker`/`podman start|stop|rm` and `docker-compose up|down`
   are prohibited as workflows. The orchestrator reads its configured
   `.env` and brings up everything.

### Mandatory Development Standards

1. **100% Test Coverage.** Every component MUST have unit, integration,
   E2E, automation, security/penetration, and benchmark tests. No false
   positives. Mocks/stubs ONLY in unit tests; all other test types use
   real data and live services.
2. **Challenge Coverage.** Every component MUST have Challenge scripts
   (`./challenges/scripts/`) validating real-life use cases. No false
   success — validate actual behavior, not return codes.
3. **Real Data.** Beyond unit tests, all components MUST use actual API
   calls, real databases, live services. No simulated success. Fallback
   chains tested with actual failures.
4. **Health & Observability.** Every service MUST expose health
   endpoints. Circuit breakers for all external dependencies. Prometheus
   / OpenTelemetry integration where applicable.
5. **Documentation & Quality.** Update `CLAUDE.md`, `AGENTS.md`, and
   relevant docs alongside code changes. Pass language-appropriate
   format/lint/security gates. Conventional Commits:
   `<type>(<scope>): <description>`.
6. **Validation Before Release.** Pass the project's full validation
   suite (`make ci-validate-all`-equivalent) plus all challenges
   (`./challenges/scripts/run_all_challenges.sh`).
7. **No Mocks or Stubs in Production.** Mocks, stubs, fakes, placeholder
   classes, TODO implementations are STRICTLY FORBIDDEN in production
   code. All production code is fully functional with real integrations.
   Only unit tests may use mocks/stubs.
8. **Comprehensive Verification.** Every fix MUST be verified from all
   angles: runtime testing (actual HTTP requests / real CLI invocations),
   compile verification, code structure checks, dependency existence
   checks, backward compatibility, and no false positives in tests or
   challenges. Grep-only validation is NEVER sufficient.
9. **Resource Limits for Tests & Challenges (CRITICAL).** ALL test and
   challenge execution MUST be strictly limited to 30-40% of host system
   resources. Use `GOMAXPROCS=2`, `nice -n 19`, `ionice -c 3`, `-p 1`
   for `go test`. Container limits required. The host runs
   mission-critical processes — exceeding limits causes system crashes.
10. **Bugfix Documentation.** All bug fixes MUST be documented in
    `docs/issues/fixed/BUGFIXES.md` (or the project's equivalent) with
    root cause analysis, affected files, fix description, and a link to
    the verification test/challenge.
11. **Real Infrastructure for All Non-Unit Tests.** Mocks/fakes/stubs/
    placeholders MAY be used ONLY in unit tests (files ending `_test.go`
    run under `go test -short`, equivalent for other languages). ALL
    other test types — integration, E2E, functional, security, stress,
    chaos, challenge, benchmark, runtime verification — MUST execute
    against the REAL running system with REAL containers, REAL
    databases, REAL services, and REAL HTTP calls. Non-unit tests that
    cannot connect to real services MUST skip (not fail).
12. **Reproduction-Before-Fix (CONST-032 — MANDATORY).** Every reported
    error, defect, or unexpected behavior MUST be reproduced by a
    Challenge script BEFORE any fix is attempted. Sequence:
    (1) Write the Challenge first. (2) Run it; confirm fail (it
    reproduces the bug). (3) Then write the fix. (4) Re-run; confirm
    pass. (5) Commit Challenge + fix together. The Challenge becomes
    the regression guard for that bug forever.
13. **Concurrent-Safe Containers (Go-specific, where applicable).** Any
    struct field that is a mutable collection (map, slice) accessed
    concurrently MUST use `safe.Store[K,V]` / `safe.Slice[T]` from
    `digital.vasic.concurrency/pkg/safe` (or the project's equivalent
    primitives). Bare `sync.Mutex + map/slice` combinations are
    prohibited for new code.

### Definition of Done (universal)

A change is NOT done because code compiles and tests pass. "Done"
requires pasted terminal output from a real run, produced in the same
session as the change.

- **No self-certification.** Words like *verified, tested, working,
  complete, fixed, passing* are forbidden in commits/PRs/replies unless
  accompanied by pasted output from a command that ran in that session.
- **Demo before code.** Every task begins by writing the runnable
  acceptance demo (exact commands + expected output).
- **Real system, every time.** Demos run against real artifacts.
- **Skips are loud.** `t.Skip` / `@Ignore` / `xit` / `describe.skip`
  without a trailing `SKIP-OK: #<ticket>` comment break validation.
- **Evidence in the PR.** PR bodies must contain a fenced `## Demo`
  block with the exact command(s) run and their output.

<!-- BEGIN host-power-management addendum (CONST-033) -->

### CONST-033 — Host Power Management is Forbidden

**Status:** Mandatory. Non-negotiable. Applies to every project,
submodule, container entry point, build script, test, challenge, and
systemd unit shipped from this repository.

**Rule:** No code in this repository may invoke a host-level power-
state transition (suspend, hibernate, hybrid-sleep, suspend-then-
hibernate, poweroff, halt, reboot, kexec) on the host machine. This
includes — but is not limited to:

- `systemctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot,kexec}`
- `loginctl {suspend,hibernate,hybrid-sleep,suspend-then-hibernate,poweroff,halt,reboot}`
- `pm-{suspend,hibernate,suspend-hybrid}`
- `shutdown {-h,-r,-P,-H,now,--halt,--poweroff,--reboot}`
- DBus calls to `org.freedesktop.login1.Manager.{Suspend,Hibernate,HybridSleep,SuspendThenHibernate,PowerOff,Reboot}`
- DBus calls to `org.freedesktop.UPower.{Suspend,Hibernate,HybridSleep}`
- `gsettings set ... sleep-inactive-{ac,battery}-type` to any value other than `'nothing'` or `'blank'`

**Why:** The host runs mission-critical parallel CLI-agent and
container workloads. On 2026-04-26 18:23:43 the host was auto-
suspended by the GDM greeter's idle policy mid-session, killing
HelixAgent and 41 dependent services. Recurring memory-pressure
SIGKILLs of `user@1000.service` (perceived as "logged out") have the
same outcome. Auto-suspend, hibernate, and any power-state transition
are unsafe for this host.

**Defence in depth (mandatory artifacts in every project):**
1. `scripts/host-power-management/install-host-suspend-guard.sh` —
   privileged installer, manual prereq, run once per host with sudo.
   Masks `sleep.target`, `suspend.target`, `hibernate.target`,
   `hybrid-sleep.target`; writes `AllowSuspend=no` drop-in; sets
   logind `IdleAction=ignore` and `HandleLidSwitch=ignore`.
2. `scripts/host-power-management/user_session_no_suspend_bootstrap.sh` —
   per-user, no-sudo defensive layer. Idempotent. Safe to source from
   `start.sh` / `setup.sh` / `bootstrap.sh`.
3. `scripts/host-power-management/check-no-suspend-calls.sh` —
   static scanner. Exits non-zero on any forbidden invocation.
4. `challenges/scripts/host_no_auto_suspend_challenge.sh` — asserts
   the running host's state matches layer-1 masking.
5. `challenges/scripts/no_suspend_calls_challenge.sh` — wraps the
   scanner as a challenge that runs in CI / `run_all_challenges.sh`.

**Enforcement:** Every project's CI / `run_all_challenges.sh`
equivalent MUST run both challenges (host state + source tree). A
violation in either channel blocks merge. Adding files to the
scanner's `EXCLUDE_PATHS` requires an explicit justification comment
identifying the non-host context.

**See also:** `docs/HOST_POWER_MANAGEMENT.md` for full background and
runbook.

<!-- END host-power-management addendum (CONST-033) -->


## MANDATORY HOST-SESSION SAFETY (Constitution §12)

**Forensic incident, 2026-04-27 22:22:14 (MSK):** the developer's
`user@1000.service` was SIGKILLed under an OOM cascade triggered by
`pip3 install --user openai-whisper` running on top of chronic
podman-pod memory pressure. The cascade SIGKILLed gnome-shell, every
ssh session, claude-code, tmux, btop, npm, node, java, pip3 — full
session loss. Evidence: `journalctl --since "2026-04-27 22:00"
--until "2026-04-27 22:23"`.

This invariant applies to **every script, test, helper, and AI agent**
in this submodule. Non-compliance is a release blocker.

### Forbidden — directly OR indirectly

1. **Suspending the host**: `systemctl suspend`, `pm-suspend`,
   `loginctl suspend`, DBus `org.freedesktop.login1.Suspend`,
   GNOME idle-suspend, lid-close handler.
2. **Hibernating / hybrid-sleeping**: any `Hibernate` / `HybridSleep`
   / `SuspendThenHibernate` method.
3. **Logging out the user**: `loginctl terminate-session`,
   `pkill -u <user>`, `systemctl --user --kill`, anything that
   signals `user@<uid>.service`.
4. **Unbounded-memory operations** inside `user@<uid>.service`
   cgroup. Any single command expected to exceed 4 GB RSS MUST be
   wrapped in `bounded_run` (defined in
   `scripts/lib/host_session_safety.sh`, parent repo).
5. **Programmatic rfkill toggles, lid-switch handlers, or
   power-button handlers** — these cascade into idle-actions.
6. **Disabling systemd-logind, GDM, or session managers** "to make
   things faster" — even temporary stops leave the system unable to
   recover the user session.

### Required safeguards

Every script in this submodule that performs heavy work (build,
transcription, model inference, large compression, multi-GB git op)
MUST:

1. Source `scripts/lib/host_session_safety.sh` from the parent repo.
2. Call `host_check_safety` at the top and **abort if it fails**.
3. Wrap any subprocess expected to exceed ~4 GB RSS in
   `bounded_run "<name>" <max-mem> <max-time> -- <cmd...>` so the
   kernel OOM killer is contained to that scope and cannot escalate
   to user.slice.
4. Cap parallelism (`-j`) to fit available RAM (each AOSP job ≈ 5 GB
   peak RSS).

### Container hygiene

Containers (Docker / Podman) we own or rely on MUST:

1. Declare an explicit memory limit (`mem_limit` / `--memory` /
   `MemoryMax`).
2. Set `OOMPolicy=stop` in their systemd unit to avoid retry loops.
3. Use exponential-backoff restart policies, never immediate retry.
4. Be clean-slate destroyed (`podman pod stop && rm`, `podman
   volume prune`) and rebuilt after any host crash or session loss
   so stale lock files don't keep producing failures.

### When in doubt

Don't run heavy work blind. Check `journalctl -k --since "1 hour ago"
| grep -c oom-kill`. If it's non-zero, **fix the offending workload
first**. Do not stack new work on a host already in distress.

**Cross-reference:** parent `docs/guides/ATMOSPHERE_CONSTITUTION.md`
§12 (full forensic, library API, operator directives) +
parent `scripts/lib/host_session_safety.sh`.

## MANDATORY ANTI-BLUFF VALIDATION (Constitution §8.1 + §11)

**This submodule inherits the parent ATMOSphere project's anti-bluff covenant.
A test that PASSes while the feature it claims to validate is unusable to an
end user is the single most damaging failure mode in this codebase. It has
shipped working-on-paper / broken-on-device builds before, and that MUST NOT
happen again.**

The canonical authority is `docs/guides/ATMOSPHERE_CONSTITUTION.md` §8.1
("NO BLUFF — positive-evidence-only validation") and §11 ("Bleeding-edge
ultra-perfection") in the parent repo. Every contribution to THIS submodule
is bound by it. Summarised non-negotiables:

1. **Tests MUST validate user-visible behaviour, not just metadata.** A gate
   that greps for a string in a config XML, an XML attribute, a manifest
   entry, or a build-time symbol is METADATA — not evidence the feature
   works for the end user. Such a gate is allowed ONLY when paired with a
   runtime / on-device test that exercises the user-visible path and reads
   POSITIVE EVIDENCE that the behaviour actually occurred (kernel `/proc/*`
   runtime state, captured audio/video, dumpsys output produced *during*
   playback, real input-event delivery, real surface composition, etc).
2. **PASS / FAIL / SKIP must be mechanically distinguishable.** SKIP is for
   environment limitations (no HDMI sink, no USB mic, geo-restricted endpoint
   unreachable) and MUST always carry an explicit reason. PASS is reserved
   for cases where positive evidence was observed. A test that completes
   without observing evidence MUST NOT report PASS.
3. **Every gate MUST have a paired mutation test in
   `scripts/testing/meta_test_false_positive_proof.sh` (parent repo).** The
   mutation deliberately breaks the feature and the gate MUST then FAIL.
   A gate without a paired mutation is a BLUFF gate and is a Constitution
   violation regardless of how many checks it appears to make.
4. **Challenges (HelixQA) and tests are in the same boat.** A Challenge that
   reports "completed" by checking the test runner exited 0, without
   observing the system behaviour the Challenge is supposed to verify, is a
   bluff. Challenge runners MUST cross-reference real device telemetry
   (logcat, captured frames, network probes, kernel state) to confirm the
   user-visible promise was kept.
5. **The bar for shipping is not "tests pass" but "users can use the feature."**
   If the on-device experience does not match what the test claims, the test
   is the bug. Fix the test (positive-evidence harder), do not silence it.
6. **No false-success results are tolerable.** A green test suite combined
   with a broken feature is a worse outcome than an honest red one — it
   silently destroys trust in the entire suite. Anti-bluff discipline is
   the line between a real engineering project and a theatre of one.

When in doubt: capture runtime evidence, attach it to the test result, and
let a hostile reviewer (i.e. yourself, in six months) try to disprove that
the feature really worked. If they can, the test is bluff and must be hardened.

**Cross-references:** parent CLAUDE.md "MANDATORY DEVELOPMENT PRINCIPLES",
parent AGENTS.md "NO BLUFF" section, parent `scripts/testing/meta_test_false_positive_proof.sh`.

## Seventh Law inheritance (Anti-Bluff Enforcement, 2026-04-30)

In addition to the Sixth Law above, this submodule inherits Lava's **Seventh Law — Tests MUST Confirm User-Reachable Functionality (Anti-Bluff Enforcement)** when consumed by the Lava project (`vasic-digital/Lava`). The Seventh Law was added to Lava's `CLAUDE.md` on 2026-04-30 to mechanically enforce the Sixth Law: every test commit MUST carry a Bluff-Audit stamp (mutation/observed-failure/reverted protocol); every feature MUST pass a real-stack verification gate; release tags MUST be preceded by a real-device attestation; forbidden test patterns (mocking the SUT, verification-only assertions, ignored tests without follow-up, build-success-as-only-assertion) are pre-push-rejected; a recurring bluff hunt and a bluff discovery protocol apply.

The authoritative verbatim text lives in the parent Lava `CLAUDE.md` under "Seventh Law — Tests MUST Confirm User-Reachable Functionality (Anti-Bluff Enforcement)". This submodule MAY add stricter clauses but MUST NOT relax any of the seven Seventh-Law clauses. Both the submodule's own anti-bluff rules and Lava's Sixth + Seventh Laws are binding when consumed by Lava; the stricter of the two applies.

## Clause 6.L — Anti-Bluff Functional Reality Mandate (Operator's Standing Order)

Inherited verbatim from parent Lava `/CLAUDE.md` §6.L. The operator has invoked this mandate **TWENTY-THREE TIMES** across two working days. The 10th invocation (2026-05-05, after Phase 7 readiness was reported, when the operator commissioned the full rebuild-and-test-everything cycle for tag Lava-Android-1.2.3): "Rebuild Go API and client app(s), put new builds into releases dir (with properly updated version codes) and execute all existing tests and Challenges! Any issue that pops up MUST BE properly addressed by addressing the root causes (fixing them) and covering everything with validation and verification tests and Challenges!"

Every test, every Challenge Test, every CI gate added to or maintained in this submodule MUST do exactly one job: confirm the feature it claims to cover actually works for an end user, end-to-end, on the gating matrix. CI green is necessary, NEVER sufficient. Tests must guarantee the product works — anything else is theatre.

Inheritance is recursive. Sub-submodules MAY paste this clause verbatim; they MUST NOT abbreviate or relax it.

## Clause 6.O (added 2026-05-05, inherited per 6.F)

- **Clause 6.O — Crashlytics-Resolved Issue Coverage Mandate** — see root `/CLAUDE.md` §6.O. Every Crashlytics-recorded issue (fatal OR non-fatal) closed/resolved by any commit MUST gain (a) a validation test in the language of the crashing surface that reproduces the conditions, (b) a Challenge Test under `app/src/androidTest/kotlin/lava/app/challenges/` (client) or `tests/e2e/` (server) that drives the same user-facing path, and (c) a closure log at `.lava-ci-evidence/crashlytics-resolved/<date>-<slug>.md` recording the issue ID, root-cause analysis, fix commit SHA, and links to the tests. `scripts/tag.sh` MUST refuse release tags whose CHANGELOG mentions Crashlytics fixes without matching closure logs. Marking a Crashlytics issue "closed" in the Console requires the test coverage to land first — never close-mark before the regression-immunity tests exist. Forensic anchor: 2026-05-05, 2 Crashlytics-recorded crashes within minutes of the first Firebase-instrumented APK distribution (Lava-Android-1.2.3-1023, commit `e9de508`); post-mortem at `.lava-ci-evidence/crashlytics-resolved/2026-05-05-firebase-init-hardening.md`. The operator's ELEVENTH §6.L invocation made this clause load-bearing.

## Clause 6.P (added 2026-05-05, inherited per 6.F)

- **Clause 6.P — Distribution Versioning + Changelog Mandate** — see root `/CLAUDE.md` §6.P. Every distribute action (Firebase App Distribution, container registry pushes, releases/ snapshots, scripts/tag.sh) MUST: (1) carry a strictly increasing versionCode (no re-distribution of already-published codes); (2) include a CHANGELOG entry — canonical file `CHANGELOG.md` at repo root + per-version snapshot at `.lava-ci-evidence/distribute-changelog/<channel>/<version>-<code>.md`; (3) inject the changelog into the App Distribution release-notes via `--release-notes`. `scripts/firebase-distribute.sh` REFUSES to operate when current versionCode ≤ last-distributed versionCode for the channel, OR when CHANGELOG.md lacks an entry for the current version, OR when the per-version snapshot file is missing. `scripts/tag.sh` enforces the same gates pre-tag. Re-distributing the same versionCode is forbidden across distribute sessions; idempotent retry within a single session is permitted. Forensic anchor: 2026-05-05 23:11 operator's TWELFTH §6.L invocation: "when distributing new build it must have version code bigger by at least one then the last version code available for download (already distribited). Every distributed build MUST CONTAIN changelog with the details what it includes compared to previous one we have published!"

## Clause 6.Q (added 2026-05-05, inherited per 6.F)

- **Clause 6.Q — Compose Layout Antipattern Guard** — see root `/CLAUDE.md` §6.Q. Forbids nesting vertically-scrolling lazy layouts (LazyColumn, LazyVerticalGrid, LazyVerticalStaggeredGrid) inside parents giving unbounded vertical space (verticalScroll, unbounded wrapContentHeight, LinearLayout-with-weight wrapper). Equivalent rule horizontally for LazyRow / LazyHorizontalGrid / LazyHorizontalStaggeredGrid. Per-feature structural tests + Compose UI Challenge Tests on the §6.I matrix are the load-bearing acceptance gates. Forensic anchor: 2026-05-05 23:51 operator-reported "Opening Trackers from Settings crashes the app" — TrackerSelectorList used LazyColumn nested in TrackerSettingsScreen's Column(verticalScroll). Closure log: `.lava-ci-evidence/crashlytics-resolved/2026-05-05-tracker-settings-nested-scroll.md`. Pattern guard: `feature/tracker_settings/src/test/.../TrackerSelectorListLazyColumnRegressionTest.kt`. The operator THIRTEENTH §6.L invocation triggered this clause.

## Article XI §11.9 — Anti-Bluff Forensic Anchor (CONST-035) — cascaded from HelixCode root

> Verbatim user mandate: *"We had been in position that all tests do execute with success and all Challenges as well, but in reality the most of the features does not work and can't be used! This MUST NOT be the case and execution of tests and Challenges MUST guarantee the quality, the completion and full usability by end users of the product!"*

**Operative rule:** The bar for shipping is not "tests pass" but "users can use the feature." Every PASS in this codebase MUST carry positive runtime evidence captured during execution. Metadata-only / configuration-only / absence-of-error / grep-based PASS without runtime evidence are critical defects regardless of how green the summary line looks. No false-success results are tolerable.

**Repository scope:** This anchor applies to all tests, all Challenges, and all CI/CD validation in this repository. It is cascaded from HelixCode root CONSTITUTION.md / CLAUDE.md / AGENTS.md and is identical across the HelixDevelopment + vasic-digital organizations.


## CONST-036 — Continuation Document Maintenance Mandate

`docs/CONTINUATION.md` MUST be the single-file source-of-truth handoff document
for resuming work across any CLI session. Every commit that changes phase
status, lands a new spec/plan, bumps a submodule pin, ships a release
artifact, discovers/resolves a known issue, or implements an operator scope
directive MUST update `docs/CONTINUATION.md` in the SAME COMMIT. The "Last
updated" line MUST track HEAD. See root `CLAUDE.md` §6.S for the inherited
clause text.

## CONST-042 — No-Secret-Leak (cascaded)
No credential may be committed. All secrets in .env (gitignored). Any leak is a release blocker.

## CONST-043 — No-Force-Push (cascaded)
No force push, history rewrite, branch deletion of main without explicit per-operation user approval.

## §6.R — No-Hardcoding Mandate (inherited 2026-05-06, per §6.F)

See root `/CLAUDE.md` §6.R. No connection address, port, header field name, credential, key, salt, secret, schedule, algorithm parameter, or domain literal shall appear as a string/int constant in tracked source code. Every such value MUST come from `.env` (gitignored), generated config class, runtime env var, or mounted file. This submodule MAY add stricter rules but MUST NOT relax.

## §6.T — Universal Quality Constraints (inherited 2026-05-06, per §6.F)

See root `/CLAUDE.md` §6.T. All four sub-points (Reproduction-Before-Fix, Resource Limits for Tests & Challenges, No-Force-Push, Bugfix Documentation) apply verbatim. This submodule MAY add stricter rules but MUST NOT relax any of §6.T.1–§6.T.4.

## §6.U — No sudo/su Mandate (inherited 2026-05-08, per §6.F)

See root `/CLAUDE.md` §6.U. Every use of `sudo` or `su` is strictly forbidden. Operations requiring elevated privileges MUST use container-based solutions from the `vasic-digital/Containers` submodule or be provided by local project/Submodule dependencies that build automatically. The pre-push hook rejects files containing `sudo ` or `su ` patterns. This submodule MAY add stricter rules but MUST NOT relax.

## §6.V — Container Emulators Mandate (inherited 2026-05-08, per §6.F)

See root `/CLAUDE.md` §6.V. Every Android emulator instance for Challenge Tests / UI verification MUST run inside a container managed by the `vasic-digital/Containers` submodule. Rootless Podman/Docker only. All tests execute inside containers. The §6.I matrix (API 28/30/34/latest, phone/tablet/TV) runs inside container-bound emulators. This submodule MAY add stricter rules but MUST NOT relax.

## §6.W — GitHub + GitLab Only Remotes (inherited 2026-05-08, per §6.F)

See root `/CLAUDE.md` §6.W. Only GitHub (`vasic-digital/*`, `HelixDevelopment/*`) and GitLab (`vasic-digital/*`, `HelixDevelopment/*`) are permitted as Git remotes. GitFlic, GitVerse, and all other providers are forbidden. The 4-mirror model is replaced by 2-mirror (GitHub + GitLab). This submodule MAY add stricter rules but MUST NOT relax.

## §6.X — Container-Submodule Emulator Wiring Mandate (inherited 2026-05-13, per §6.F)

See root `/CLAUDE.md` §6.X. Every Android emulator instance the project depends on for testing MUST execute its emulator process INSIDE a podman/docker container managed by `Submodules/Containers/`, NOT be host-direct-launched by Containers-submodule code that runs on the host. The Containers submodule's `pkg/runtime/` (rootless podman/docker auto-detection) brings the container up; `pkg/emulator/` orchestrates the AVD lifecycle inside it. Lava-side `scripts/run-emulator-tests.sh` is thin glue forwarding to the Containers CLI. The container-bound path is the gate — host-direct emulators are permitted for workstation iteration only. §6.X-debt tracks the wiring implementation owed to `Submodules/Containers/`. This submodule MAY add stricter rules but MUST NOT relax.

## §11.4.7 — Operator-Path Test Coverage Rule (inherited from vasic-digital/tmux, 2026-05-13)

**Forensic anchor.** Caught in `vasic-digital/tmux` 2026-05-13: tests
that reported GREEN while the operator-facing feature was broken,
because the tests bypassed the operator's wrapper and hand-crafted
the underlying primitives (systemd-run scopes) directly. Two stacked
failure modes:

- **Test 11** always passed `-S "$SOCKET"` → exercised the explicit-
  socket path only; the operator's `tmx new -s X` (default socket)
  was uncovered. The captured-evidence claim was partial. Operator
  reported the bug visually: status bar showed default green instead
  of the hostname-derived colour. README marketing was a §1 bluff.

- **Test 14** hand-spawned three `systemd-run --user --scope` units
  with explicit `--unit` names to simulate isolation. The actual
  `tmx new -s A -d; tmx new -s B -d` placed every session in ONE
  shared cgroup scope. Test 14 PASSed; operator-facing isolation
  did not exist. README's "if one session OOMs, others survive"
  was a §1 bluff.

**The mandate.** Every gate test for a feature MUST exercise the SAME
entry point an end-user would invoke in production. Tests that bypass
the operator's wrapper, helper, or install path — and instead
reproduce its effects with hand-crafted equivalents — DO NOT satisfy
captured-evidence requirements. When the operator's path and the
test's path diverge:

1. The test header MUST EXPLICITLY name what divergence exists.
2. A SEPARATE end-to-end test MUST close that divergence with
   captured evidence on the operator-facing entry point.

**Operative test.** For every test under this submodule's
`scripts/anti-bluff/` / `tests/` / `cmd/distributed-test/` paths,
ask: "would a consumer of `digital.vasic.containers` hit this code
path in their normal workflow?" If no, that test is supplementary;
the operator-path test must exist alongside it.

**No grep-on-script-content alone.** A `grep` for a literal flag or
property name inside the wrapper/orchestrator/script is allowed AS A
STATIC CHECK in addition to a runtime readback — never as the only
assertion.

**Layer-4 mutations MUST target operator-path code, not synthetic-
test code.** When the submodule provides BOTH a thin host-side
bridge (e.g. `scripts/tmx` on Darwin) AND a thick body of behaviour
(e.g. `scripts/tmx-vm` inside the VM), paired mutations MUST target
the body — that's the file consumers actually exercise. Mutating
the bridge while the consumer-path lives in the body is a §1 bluff
inside the gate itself.

**Inheritance and propagation.** This submodule's tests inherit
§11.4.7 from the parent `vasic-digital/tmux` Constitution. Submodule
rules MAY add stricter clauses but MUST NOT relax. Both the parent
project's rule set and this submodule's own apply; the stricter
applies.

Non-compliance is a release blocker.
<!-- BEGIN submodule-decoupling-and-reusability (parent-mirror) -->

### Submodule Decoupling & Reusability — Mandatory

**Status:** Mandatory. Non-negotiable.

**Rule:** This repository is a **shared submodule** consumed by
multiple independent consumer projects. Its value depends on staying
**fully decoupled and reusable**. No change in this repository may
introduce coupling that breaks its standalone reusability for any
consumer.

**Prohibited inside this repository:**

1. Hardcoding any specific consumer project's name, paths, platform
   list, version strings, release-naming conventions, branding, or
   feature names.
2. `import` / dependency on any consumer-project namespace, package,
   or build coordinate.
3. Embedding consumer-project-specific governance, rule numbering, or
   release cadence into this repository's `CONSTITUTION.md` /
   `CLAUDE.md` / `AGENTS.md`.
4. Assuming this repository is consumed by a particular CLI, build
   system, language toolchain version, or target architecture beyond
   what its public interface documents.

**Required inside this repository:**

1. All public surfaces (APIs, CLIs, configuration files, environment
   variables, scripts) MUST be expressed in terms of THIS repository's
   own domain — not any consumer's.
2. Governance MUST describe responsibilities and contract from THIS
   repository's perspective. Consumer projects appear as illustrative
   examples at most, never as load-bearing requirements.
3. Cross-project rules adopted from a consumer (such as a
   cross-platform impact mandate) MUST be phrased generically —
   "every consuming project's full platform matrix" — and never
   hardcode any single consumer's matrix.

**Why:** Repositories like this one have shipped changes in the past
where one consumer's platform list, feature names, or rule numbering
leaked into shared-repo governance — and then collided at merge time
with another consumer's parallel work, leaving the repository
unmergeable until manual conflict resolution stripped the
consumer-specific text back out. Decoupling is the only mechanism
that preserves this repository's value as shared infrastructure.

**Recursive scope:** any submodule this repository consumes inherits
the same decoupling+reusability rule. Third-party upstream submodules
that this repository merely vendors (e.g. open-source tools under a
`tools/opensource/` tree, if present) are explicitly out of scope —
we are not their owners.

<!-- END submodule-decoupling-and-reusability (parent-mirror) -->

---

## CONST-047 — Recursive Submodule Application Mandate (cascaded from root CONSTITUTION.md)

> Verbatim user mandate (2026-05-14): *"Make sure all work we do is applied ALWAYS to all Submodules we control under our organizations (vasic-digital and HelixDevelopment) fully recursively everywhere with full bluff-proofing and comprehensive documentation, user manuals and guides and full tests and Challenges coverage!"*

Every engineering deliverable produced for the main project MUST be applied — fully and recursively — to every owned submodule under the `vasic-digital` and `HelixDevelopment` GitHub organizations. Each owned submodule (including this one) MUST receive in lockstep: (1) anti-bluff posture (CONST-035 / Article XI §11.9), (2) comprehensive documentation matching actual capabilities, (3) full tests + Challenges coverage with captured runtime evidence, (4) recursive propagation through nested submodules under the same orgs, (5) synchronized commits when meta-repo state advances this surface.

See the root `CONSTITUTION.md` §CONST-047 for the full mandate. This anchor MUST remain in this submodule's CONSTITUTION.md, CLAUDE.md, and AGENTS.md.
<!-- BEGIN cross-platform-impact (mirrors Yole CONST-037) -->

### Cross-Platform Impact — Mandatory Consideration (mirrors Yole CONST-037)

**Status:** Mandatory. Non-negotiable. Mirrors CONST-037 in the parent
Yole project's `CONSTITUTION.md`.

**Rule:** This submodule is consumed by the Yole multi-platform project
(Android / Desktop / iOS / Web). Every change MUST be reasoned about
across all four target platforms BEFORE coding. A fix that works on one
target but silently breaks another is a regression.

**Pre-edit checklist:** Before any code change, answer:

1. Does this compile on every Yole target (Android, Desktop, iOS, Web)?
2. Does it behave identically — or by-design differently — on each?
3. Is the change covered by a test on every affected target?
4. Are platform manifests (AndroidManifest.xml, Info.plist, web
   manifest, container packaging) updated coherently?

**Commit body requirement:** every change MUST include a
"Cross-platform impact" block listing each Yole platform's disposition
(changed / unchanged / N/A with reason).

```
Cross-platform impact:
- Android: <disposition>
- Desktop: <disposition>
- iOS:     <disposition>
- Web:     <disposition>
```

**Why:** End users experience the integrated Yole product, not this
submodule in isolation. Cross-platform regressions caused by
submodule-local changes have shipped to users in the past; mandatory
up-front consideration is the only mitigation.

**Enforcement:** the parent Yole repo runs
`yole-challenges/scripts/cross_platform_parity_challenge.sh` in
`make qa-all`. Submodule changes that cause that challenge to fail
MUST be reverted or fixed.

**See also:** CONST-037 in the parent Yole repo's `CONSTITUTION.md`
for the full rule and forensic anchor.

<!-- END cross-platform-impact (mirrors Yole CONST-037) -->
## §6.Z — Anti-Bluff Distribute Guard (inherited 2026-05-14, per §6.F)

See root `/CLAUDE.md` §6.Z. No artifact may be distributed (Firebase App Distribution, Google Play Store release, container image push, this submodule's binary release, any future channel) UNLESS the corresponding end-to-end tests have been **EXECUTED — not source-compiled, EXECUTED** — against the EXACT artifact about to be distributed, AND have **passed**. Pre-distribute test-evidence file required at `.lava-ci-evidence/distribute-changelog/<channel>/<version>-<code>-test-evidence.{md,json}` with matching commit SHA, timestamp within 24h, `BUILD SUCCESSFUL` (or per-language pass marker) verbatim in captured output. Cold-start verification is the load-bearing canary. Distributing a faulty version is a constitutional violation by construction. §6.Z-debt is open: mechanical enforcement via `scripts/firebase-distribute.sh` Phase 1 Gate 6 + pre-push hook check is documented but not yet enforced. Forensic anchor: 2026-05-14 Galaxy S23 Ultra cold-launch crash on Lava-Android-1.2.19-1039 (Crashlytics `40a62f97a5c65abb56142b4ca2c37eeb` — `painterResource()` rejection of `<layer-list>` drawable); agent had skipped Compose UI test execution citing the wrong §6.X caveat. Operator's 26th §6.L invocation: "Anti-bluff policy MUST BE ENFORCED ALWAYS!!!" This submodule MAY add stricter rules but MUST NOT relax this clause.
