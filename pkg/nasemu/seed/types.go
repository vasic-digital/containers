package seed

import (
	"errors"
	"io/fs"
)

// Schema identifiers written into manifests and evidence.
const (
	SpecVersion      = 1
	ManifestSchema   = "nasemu.seed.manifest/1"
	EvidenceSchema   = "nasemu.seed.evidence/1"
	MaxEntries       = 100000
	MaxTotalBytes    = int64(1) << 30
	MaxSpecBytes     = 16 << 20
	maxComponentLen  = 255
	maxPathLen       = 4096
	defaultFileMode  = fs.FileMode(0o644)
	defaultDirMode   = fs.FileMode(0o755)
	labelMaxLen      = 64
	maxEvidenceDiffs = 100
)

// Kind is the closed vocabulary of tree entry kinds.
type Kind string

// Entry kinds.
const (
	KindDir     Kind = "dir"
	KindFile    Kind = "file"
	KindSymlink Kind = "symlink"
)

// Pattern is the closed vocabulary of file content patterns.
type Pattern string

// Content patterns.
const (
	PatternZero   Pattern = "zero"
	PatternPRNG   Pattern = "prng"
	PatternText   Pattern = "text"
	PatternWAV    Pattern = "wav"
	PatternMP3    Pattern = "mp3"
	PatternMP4    Pattern = "mp4"
	PatternRepeat Pattern = "repeat"
)

// Entry is one element of a tree spec. Path may contain "{n}" together with
// Count > 0, which expands to Count entries numbered from 0.
type Entry struct {
	Path    string  `json:"path"`
	Kind    Kind    `json:"kind"`
	Size    int64   `json:"size,omitempty"`
	Pattern Pattern `json:"pattern,omitempty"`
	Data    string  `json:"data,omitempty"`
	Target  string  `json:"target,omitempty"`
	Mode    string  `json:"mode,omitempty"`
	Count   int     `json:"count,omitempty"`
}

// Spec is the DATA that fully determines a seed corpus.
type Spec struct {
	Version   int     `json:"version"`
	Seed      uint64  `json:"seed"`
	MtimeUnix int64   `json:"mtime_unix"`
	Entries   []Entry `json:"entries"`
}

// Resolved is a validated, expanded spec entry.
type Resolved struct {
	Path    string
	Kind    Kind
	Size    int64
	Pattern Pattern
	Data    string
	Target  string
	Mode    fs.FileMode
}

// ManifestEntry is one golden-manifest row.
type ManifestEntry struct {
	Path      string `json:"path"`
	Kind      Kind   `json:"kind"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256,omitempty"`
	Mode      string `json:"mode"`
	MtimeUnix int64  `json:"mtime_unix,omitempty"`
	Target    string `json:"target,omitempty"`
}

// Manifest is the golden manifest of a generated tree.
type Manifest struct {
	Schema     string          `json:"schema"`
	SpecSHA256 string          `json:"spec_sha256"`
	Entries    []ManifestEntry `json:"entries"`
}

// Result is the closed vocabulary of verification outcomes.
type Result string

// Verification results.
const (
	ResultPass  Result = "PASS"
	ResultFail  Result = "FAIL"
	ResultError Result = "ERROR"
)

// Code is the closed vocabulary of diff codes.
type Code string

// Diff codes.
const (
	CodeMissing        Code = "missing"
	CodeExtra          Code = "extra"
	CodeKindMismatch   Code = "kind_mismatch"
	CodeSizeMismatch   Code = "size_mismatch"
	CodeHashMismatch   Code = "hash_mismatch"
	CodeModeMismatch   Code = "mode_mismatch"
	CodeMtimeMismatch  Code = "mtime_mismatch"
	CodeTargetMismatch Code = "target_mismatch"
	CodeSymlinkEscape  Code = "symlink_escape"
	CodeParentInvalid  Code = "parent_invalid"
	CodeReadError      Code = "read_error"
)

// Diff is one exact difference between a manifest and a live tree.
type Diff struct {
	Path   string `json:"path"`
	Code   Code   `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Counts summarises a manifest or a verification.
type Counts struct {
	Dirs            int          `json:"dirs"`
	Files           int          `json:"files"`
	Symlinks        int          `json:"symlinks"`
	Bytes           int64        `json:"bytes"`
	SkippedSymlinks int          `json:"skipped_symlinks,omitempty"`
	ByCode          map[Code]int `json:"by_code,omitempty"`
}

// Report is the outcome of a verification.
type Report struct {
	Result Result `json:"result"`
	Diffs  []Diff `json:"diffs"`
	Counts Counts `json:"counts"`
}

// Options tune verification. Mode and mtime are opt-in because many served
// protocols do not preserve them.
type Options struct {
	CheckMode    bool
	CheckMtime   bool
	SkipSymlinks bool
}

// Errors.
var (
	ErrNotImplemented   = errors.New("seed: not implemented")
	ErrManifestHash     = errors.New("seed: manifest sha256 mismatch")
	ErrManifestHashNeed = errors.New("seed: expected manifest sha256 is required")
	ErrInvalidSpec      = errors.New("seed: invalid spec")
	ErrInvalidManifest  = errors.New("seed: invalid manifest")
	ErrInvalidLabel     = errors.New("seed: invalid label")
	ErrRootNotEmpty     = errors.New("seed: generation root must be absent or an empty directory")
)

// Evidence is the machine-written evidence record. It carries no wall-clock
// value so that identical inputs yield byte-identical evidence.
type Evidence struct {
	Schema         string `json:"schema"`
	Label          string `json:"label"`
	Operation      string `json:"operation"`
	InputsSHA256   string `json:"inputs_sha256,omitempty"`
	ManifestSHA256 string `json:"manifest_sha256"`
	Result         Result `json:"result"`
	Counts         Counts `json:"counts"`
	Diffs          []Diff `json:"diffs,omitempty"`
	DiffsTruncated bool   `json:"diffs_truncated,omitempty"`
	Class          string `json:"evidence_class"`
}
