# pkg/nasemu/seed (PROVISIONAL)

Deterministic seed-corpus generator, golden manifest and tree verifier for the
NAS emulator work (BG-NASEMU NE-3). Library plus `cmd/nasemu-seed`. See
`doc.go` for the API and limits.

## CLI

```
nasemu-seed generate -spec spec.json -out DIR -manifest-out manifest.json [-evidence-dir D -label L]
nasemu-seed verify   -manifest manifest.json -manifest-sha256 HEX -root DIR [-mode] [-mtime] [-skip-symlinks] [-evidence-dir D -label L]
```

`generate` prints `manifest_sha256=<hex>`; keep that value, `verify` refuses to
run without it. Exit codes: 0 PASS, 1 FAIL, 2 ERROR (could not determine),
64 usage.

## Spec example

```json
{"version":1,"seed":20261113,"mtime_unix":1700000000,"entries":[
 {"path":"media/clip one.wav","kind":"file","size":4096,"pattern":"wav"},
 {"path":"docs/ünïcödé 日本語.txt","kind":"file","size":500,"pattern":"text"},
 {"path":"docs/empty.dat","kind":"file"},
 {"path":"bulk/f{n}.bin","kind":"file","count":3,"size":128,"pattern":"prng"},
 {"path":"link","kind":"symlink","target":"media/clip one.wav"}]}
```

## Tests and mutants

```
GOMAXPROCS=2 go test -p 1 -count=1 -race ./pkg/nasemu/seed ./cmd/nasemu-seed
scripts/nasemu_seed_mutants.sh     # 21 paired mutants, cmp-verified restore
```

Mocks are not used: generation and verification run against real directories;
the verifier golden-good/golden-bad fixtures use `testing/fstest` as an
fs.FS stand-in for a served-protocol view (these prove the verifier's own
logic, not any server). Migration source for the Catalogizer corpus approach:
its `docs/qa/test-servers` (manifest.sha256, 12 files); mapping that corpus to a
spec is NE-4 work, not done here.

## Not covered / UNCONFIRMED

- Windows/macOS behaviour; Go versions other than 1.26.8 on linux/amd64.
- Verification over a real mounted SMB/NFS/FTP view (needs the NE-4 profiles).
- Case-insensitive collision rule uses `strings.ToLower`, not full Unicode folding.
