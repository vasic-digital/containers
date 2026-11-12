// Package seed generates a deterministic seed corpus from DATA, writes a
// golden manifest of it, and verifies live trees against that manifest.
//
// STATUS: PROVISIONAL (BG-NASEMU NE-3). Verified on linux/amd64 only; other
// platforms and Go versions are UNCONFIRMED.
//
// # Generate
//
// A Spec (JSON) names a seed, a fixed mtime and a list of entries (dirs, files
// with a size and a content pattern, relative in-tree symlinks). Content is a
// pure function of (seed, path, pattern, size): no wall clock, no external
// randomness. Patterns: zero, prng, text, wav, mp3, mp4 (valid-looking headers
// followed by prng bytes) and repeat. A path containing "{n}" with count N
// expands to N entries. Names may be unicode, contain spaces or be long
// (components up to 255 bytes); depth is limited only by the 4096-byte path
// cap. Specs are validated hard: no traversal, absolute paths, control
// characters, case-insensitive collisions, escaping or dangling symlinks, and
// total file bytes are capped at MaxTotalBytes.
//
//	spec, _ := seed.ParseSpec(raw)
//	m, err := seed.Generate(spec, "/path/to/empty-or-absent-dir")
//	mb, _ := seed.MarshalManifest(m)
//	hash := seed.ManifestSHA256(mb) // record this next to the manifest
//
// # Verify
//
// ParseManifest refuses a manifest whose sha256 is not supplied or does not
// match. Verify works on any io/fs.FS, so it can check what a client sees
// over a mounted or served protocol; VerifyDir checks a local directory
// through os.Root. Entries are examined with Lstat semantics and symlinks are
// never followed. Results are PASS, FAIL or ERROR (could not determine); each
// difference carries a code from the closed set missing, extra,
// kind_mismatch, size_mismatch, hash_mismatch, mode_mismatch, mtime_mismatch,
// target_mismatch, symlink_escape, parent_invalid, read_error. Mode and mtime
// are compared only when requested (Options), because many protocols do not
// preserve them; SkipSymlinks exists for the same reason and is counted, not
// hidden.
//
// # Evidence
//
// NewGenerateEvidence / NewVerifyEvidence build a JSON record (inputs hash,
// manifest hash, result, counts, diffs) with no wall-clock field, so identical
// inputs give byte-identical evidence. WriteEvidence only accepts labels that
// match ^[a-z0-9][a-z0-9_.-]{0,63}$ and writes <label>.<operation>.json inside
// an existing directory, atomically. The evidence class of a generate record
// is "generator-self-report": it proves nothing about any served tree.
//
// # Honest limits
//
// The verifier proves a tree matches the manifest, not that the manifest is
// right; the manifest hash must come from a trusted channel. Verification of a
// local directory has a check-then-read race window against a concurrent
// writer. Symlink verification needs an fs.FS that implements fs.ReadLinkFS.
// This package does not start any server; it supplies the corpus and the
// oracle for the NAS emulator profiles.
package seed
