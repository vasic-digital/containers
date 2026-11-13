#!/usr/bin/env bash
# nasemu_seed_mutants.sh - paired mutation run for pkg/nasemu/seed (PROVISIONAL).
#
# For each mutant: back up the target file, apply ONE exact-string edit
# (aborts if the old text is not found exactly once), confirm the file really
# changed (cmp), confirm the mutated code still builds (go build), run the package tests,
# require them to FAIL, restore the file and confirm the restore is
# byte-identical (cmp). Finishes with an unmutated baseline run that must PASS.
#
# Usage: scripts/nasemu_seed_mutants.sh            (run from anywhere)
# Env:   SAFE_RM_LIB  path to constitution scripts/lib/safe_rm.sh (optional;
#                     without it only the script's own mktemp root is removed
#                     with a ${VAR:?}-guarded rm -rf).
# Exit:  0 all mutants killed and baseline green; 1 survivor / invalid mutant /
#        restore mismatch / baseline red.
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKG=pkg/nasemu/seed
TMP="$(mktemp -d "${TMPDIR:-/tmp}/nasemu-mut.XXXXXX")" || exit 1
: "${TMP:?}"
CUR_FILE=""; CUR_BAK=""
cleanup() {
  if [ -n "$CUR_FILE" ] && [ -f "$CUR_BAK" ]; then cp -f "$CUR_BAK" "$ROOT/$CUR_FILE"; fi
  if [ -n "${SAFE_RM_LIB:-}" ] && [ -r "$SAFE_RM_LIB" ]; then
    # shellcheck disable=SC1090
    . "$SAFE_RM_LIB"; safe_rm_rf "$TMP"
  else
    rm -rf "${TMP:?}"
  fi
}
trap cleanup EXIT

# id@@@file@@@old@@@new  (delimiter @@@ because the code contains '|')
MUTANTS=(
'M01-seed-ignored@@@pkg/nasemu/seed/content.go@@@binary.LittleEndian.PutUint64(b[:], c.seed)@@@binary.LittleEndian.PutUint64(b[:], 0)'
'M02-block-index-ignored@@@pkg/nasemu/seed/content.go@@@binary.BigEndian.PutUint64(b[:], i)@@@binary.BigEndian.PutUint64(b[:], 0)'
'M03-path-ignored@@@pkg/nasemu/seed/content.go@@@h.Write([]byte(c.path))@@@_ = c.path'
'M04-dir-mtime-off@@@pkg/nasemu/seed/generate.go@@@rt.Chtimes(dirs[i].Path, mt, mt)@@@rt.Chtimes(dirs[i].Path, mt, mt.Add(time.Second))'
'M05-file-mode-wrong@@@pkg/nasemu/seed/generate.go@@@rt.Chmod(r.Path, r.Mode)@@@rt.Chmod(r.Path, 0o644)'
'M06-symlink-target-unchecked@@@pkg/nasemu/seed/verify.go@@@case tgt != e.Target:@@@case false:'
'M07-hash-unchecked@@@pkg/nasemu/seed/verify.go@@@case hex.EncodeToString(h.Sum(nil)) != e.SHA256:@@@case hex.EncodeToString(h.Sum(nil)) != e.SHA256 \&\& false:'
'M08-extras-ignored@@@pkg/nasemu/seed/verify.go@@@if p == "." || exp[p] {@@@if p == "." || exp[p] || true {'
'M09-kind-unchecked@@@pkg/nasemu/seed/verify.go@@@got != e.Kind {@@@got != e.Kind \&\& false {'
'M10-mtime-unchecked@@@pkg/nasemu/seed/verify.go@@@opt.CheckMtime \&\& info@@@false \&\& info'
'M11-bad-parent-ignored@@@pkg/nasemu/seed/verify.go@@@if bad[a] {@@@if false {'
'M12-symlink-escape-undetected@@@pkg/nasemu/seed/verify.go@@@return r == resEscape || r == resLoop@@@return r == resLoop'
'M13-manifest-hash-unchecked@@@pkg/nasemu/seed/manifest.go@@@if ManifestSHA256(b) != strings.ToLower(wantSHA256) {@@@if false {'
'M14-dotdot-accepted@@@pkg/nasemu/seed/spec.go@@@case c == "." || c == "..":@@@case c == ".":'
'M15-case-collision-allowed@@@pkg/nasemu/seed/spec.go@@@ok \&\& prev != p {@@@ok \&\& false {'
'M16-nonempty-root-allowed@@@pkg/nasemu/seed/generate.go@@@if len(ents) != 0 {@@@if len(ents) != 0 \&\& false {'
'M17-label-slash-allowed@@@pkg/nasemu/seed/evidence.go@@@[a-z0-9_.-]{0,63}@@@[a-z0-9_./-]{0,63}'
'M18-unsorted-manifest-allowed@@@pkg/nasemu/seed/manifest.go@@@if i > 0 \&\& e.Path <= prev {@@@if i > 0 \&\& e.Path <= prev \&\& false {'
'M19-read-error-never-fatal@@@pkg/nasemu/seed/verify.go@@@if d.Code != CodeReadError {@@@if d.Code != CodeReadError \&\& false {'
'M20-dangling-symlink-allowed@@@pkg/nasemu/seed/spec.go@@@case resMissing:@@@case resMissing + 100:'
'M21-cli-fail-exit-zero@@@cmd/nasemu-seed/main.go@@@return exitFail@@@return exitPass'
'N02-implicit-parent-casefold-skipped@@@pkg/nasemu/seed/spec.go@@@if err := claim(a); err != nil {@@@if false {'
'N05-total-bytes-cap-removed@@@pkg/nasemu/seed/spec.go@@@if total > MaxTotalBytes {@@@if false {'
'N11-cli-verify-evidence-pairing-removed@@@cmd/nasemu-seed/main.go@@@*mpath == "" || *root == "" || (*evDir != "") != (*label != "")@@@*mpath == "" || *root == ""'
'N13-dir-extra-fields-allowed@@@pkg/nasemu/seed/spec.go@@@e.Size != 0 || e.Pattern != "" || e.Data != "" || e.Target != "" {@@@e.Size != 0 {'
'N14-negative-count-allowed@@@pkg/nasemu/seed/spec.go@@@case e.Count < 0 || e.Count > MaxEntries:@@@case e.Count > MaxEntries:'
'N16-absolute-target-check-removed@@@pkg/nasemu/seed/verify.go@@@if strings.HasPrefix(target, "/") {@@@if strings.HasPrefix(target, "/") \&\& false {'
'F1a-symlink-hop-not-expanded-spec@@@pkg/nasemu/seed/spec.go@@@hops++@@@hops++; continue'
'F1b-manifest-kinds-ignored-in-verifier@@@pkg/nasemu/seed/verify.go@@@e, ok := kinds[p]@@@e, ok := ManifestEntry{}, false'
'F2a-template-length-unchecked@@@pkg/nasemu/seed/spec.go@@@len(e.Path)+n*(digits-3) > maxPathLen@@@len(e.Path)+n*(digits-3) > maxPathLen*1000'
'F2b-spec-input-uncapped@@@pkg/nasemu/seed/spec.go@@@if len(data) > MaxSpecBytes {@@@if false {'
'F3-readlinkfs-not-required@@@pkg/nasemu/seed/verify.go@@@if _, ok := fsys.(fs.ReadLinkFS); !ok {@@@if false {'
'F4-skip-symlinks-skips-kind-check@@@pkg/nasemu/seed/verify.go@@@rep.Counts.SkippedSymlinks++@@@rep.Counts.SkippedSymlinks++; continue'
'F5a-c1-controls-allowed@@@pkg/nasemu/seed/spec.go@@@case r >= 0x80 \&\& r <= 0x9f:@@@case false:'
'F5b-bidi-isolates-allowed@@@pkg/nasemu/seed/spec.go@@@r >= 0x2066 \&\& r <= 0x2069@@@false'
'F5c-zero-width-space-allowed@@@pkg/nasemu/seed/spec.go@@@r == 0x200b, @@@'
'F5d-combining-marks-allowed@@@pkg/nasemu/seed/spec.go@@@case r >= 0x0300 \&\& r <= 0x036f:@@@case false:'
'F5e-fold-ascii-only@@@pkg/nasemu/seed/spec.go@@@return strings.ToLower(strings.ToUpper(p))@@@return p'
'F6a-cli-verify-hash-optional@@@cmd/nasemu-seed/main.go@@@fs.NArg() != 0 || *want == "" || @@@fs.NArg() != 0 || '
'F6b-cli-generate-extra-args-allowed@@@cmd/nasemu-seed/main.go@@@fs.NArg() != 0 || *specPath == ""@@@*specPath == ""'
'F6c-cli-manifest-precheck-removed@@@cmd/nasemu-seed/main.go@@@if _, err := os.Lstat(*mpath); err == nil {@@@if false {'
'F6d-cli-paths-unescaped@@@cmd/nasemu-seed/main.go@@@"%s\t%q\t%q\n"@@@"%s\t%s\t%s\n"'
)

apply() { # file old new  (old/new use \& for a literal &)
  python3 -I - "$ROOT/$1" "$2" "$3" <<'PY'
import sys
p, old, new = sys.argv[1], sys.argv[2].replace("\\&", "&"), sys.argv[3].replace("\\&", "&")
s = open(p, encoding="utf-8").read()
if s.count(old) != 1:
    sys.exit("old text found %d times" % s.count(old))
open(p, "w", encoding="utf-8").write(s.replace(old, new))
PY
}

cd "$ROOT" || exit 1
export GOMAXPROCS=2
rc=0; killed=0
for m in "${MUTANTS[@]}"; do
  id=${m%%@@@*}; r=${m#*@@@}; file=${r%%@@@*}; r=${r#*@@@}; old=${r%%@@@*}; new=${r#*@@@}
  bak="$TMP/$id.orig"; cp -f "$ROOT/$file" "$bak"; CUR_FILE="$file"; CUR_BAK="$bak"
  if ! apply "$file" "$old" "$new"; then echo "INVALID $id: edit not applied"; rc=1; cp -f "$bak" "$ROOT/$file"; CUR_FILE=""; continue; fi
  if cmp -s "$bak" "$ROOT/$file"; then echo "INVALID $id: file unchanged"; rc=1; CUR_FILE=""; continue; fi
  if ! nice -n 10 go build ./$PKG ./cmd/nasemu-seed >"$TMP/vet.out" 2>&1; then
    echo "INVALID $id: mutant does not build"; rc=1
  else
    nice -n 10 go test -vet=off -p 1 -count=1 ./$PKG ./cmd/nasemu-seed >"$TMP/$id.out" 2>&1; t=$?
    if [ $t -ne 0 ]; then echo "KILLED   $id"; killed=$((killed+1)); else echo "SURVIVED $id"; rc=1; fi
  fi
  cp -f "$bak" "$ROOT/$file"
  if cmp -s "$bak" "$ROOT/$file"; then CUR_FILE=""; else echo "RESTORE MISMATCH $id"; rc=1; fi
done
echo "mutants: ${#MUTANTS[@]} total, $killed killed"
if nice -n 10 go test -vet=off -p 1 -count=1 ./$PKG ./cmd/nasemu-seed >"$TMP/baseline.out" 2>&1; then
  echo "BASELINE PASS"
else
  echo "BASELINE FAIL"; tail -20 "$TMP/baseline.out"; rc=1
fi
exit $rc
