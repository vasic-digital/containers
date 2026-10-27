# containers 1.2.1 (tagged `containers-1.2.1`)

- Tag to cut: `containers-1.2.1` on top of this branch (`release/containers-1.2.1`).
- Code delta vs `containers-1.2.0` (cb5c727): none. Adds `MIGRATION.md`, CHANGELOG
  1.2.1 section, README version line.
- **SemVer deviation:** 1.2.0 (and therefore 1.2.1) contain breaking Go API
  changes relative to `helixcode-v1.1.0` (changed signatures, widened
  interfaces, non-comparable structs; see `MIGRATION.md`). SemVer would require
  2.0.0. Operator decision: ship as a patch release 1.2.1 so the migration guide
  accompanies the already-published 1.2.0 line without renumbering. Pin exact
  versions; do not treat 1.2.x as source-compatible with 1.1.x.
- Verification (host-run, NOT containerised; a known imperfection vs 11.4.173):
  `go build ./...` ok, `go vet ./...` ok, `go test -count=1 -p 2 ./...` rc=0,
  44 packages ok; `pkg/remote` umask test fix (`main_umask_unix_test.go`) re-run
  under umask 0002, 0022 and 0077: ok.
