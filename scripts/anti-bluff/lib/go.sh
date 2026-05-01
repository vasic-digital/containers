#!/usr/bin/env bash
# Go-flavored bluff patterns. Sourced by bluff-scanner.sh.
# Each pattern emits "<relative path>:<line>:BLUFF-G-NNN:<context>"
#
# Skip-exempt markers recognized (either form on the line above the offending
# line, or on the same line as a trailing comment):
#   // SKIP-OK: #<ticket>             (project-wide convention)
#   // ANTI-BLUFF-EXEMPT: <ticket>    (synonym)

scan_go() {
  local relpath="$1" fpath="$2"

  # BLUFF-G-001: t.Skip() / t.Skipf() without a SKIP-OK or ANTI-BLUFF-EXEMPT
  # marker on the previous line OR as a trailing comment on the same line.
  awk -v rel="${relpath}" '
    /SKIP-OK:|ANTI-BLUFF-EXEMPT/ { exempt[NR+1] = 1 }
    /[[:space:]]t\.Skip[f]?\(/ {
      same_line_exempt = ($0 ~ /SKIP-OK:|ANTI-BLUFF-EXEMPT/)
      if (!(NR in exempt) && !same_line_exempt) {
        print rel ":" NR ":BLUFF-G-001:t.Skip without exempt comment"
      }
    }
  ' "$fpath"

  # BLUFF-G-005: t.Run("", func(t *testing.T) { }) — empty named subtest.
  awk -v rel="${relpath}" '
    /t\.Run\("",[[:space:]]*func\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}\)/ {
      print rel ":" NR ":BLUFF-G-005:empty t.Run subtest"
    }
  ' "$fpath"

  # BLUFF-G-006: empty TestXxx body on a single line.
  awk -v rel="${relpath}" '
    /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{[[:space:]]*\}[[:space:]]*$/ {
      print rel ":" NR ":BLUFF-G-006:empty test body"
    }
  ' "$fpath"

  # BLUFF-G-007: assert.True(t, true) / assert.NotNil(t, x) as a literal line.
  awk -v rel="${relpath}" '
    /^[[:space:]]*assert\.True\(t,[[:space:]]*true\)/ {
      print rel ":" NR ":BLUFF-G-007:assert.True(t, true) trivial"
    }
  ' "$fpath"

  # BLUFF-G-003: test function whose body has only t.Log calls (no t.Fatal/Error/Errorf, no assert.).
  # Conservative single-pass: extract each TestXxx body, count assertion-like calls.
  awk -v rel="${relpath}" '
    function flush(start_line) {
      if (start_line > 0 && asserts == 0 && logs > 0) {
        print rel ":" start_line ":BLUFF-G-003:no-assert test (only t.Log)"
      }
    }
    /^func Test[A-Z][A-Za-z0-9_]*\(t \*testing\.T\)[[:space:]]*\{/ {
      flush(start_line)
      start_line = NR; brace = 1; asserts = 0; logs = 0
      next
    }
    start_line > 0 {
      n = gsub(/\{/, "&"); brace += n
      n = gsub(/\}/, "&"); brace -= n
      if ($0 ~ /t\.(Fatal|Fatalf|Error|Errorf)\(|assert\./)  asserts++
      if ($0 ~ /t\.Log[f]?\(/) logs++
      if (brace == 0) { flush(start_line); start_line = 0 }
    }
    END { flush(start_line) }
  ' "$fpath"
}
