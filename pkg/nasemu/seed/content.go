package seed

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
)

// contentReader yields exactly size bytes derived only from (seed, path,
// pattern, data, size). It uses no clock and no external randomness.
type contentReader struct {
	seed      uint64
	path      string
	pat       Pattern
	data      string
	remaining int64
	block     uint64
	buf       []byte
}

func newContentReader(seed uint64, r Resolved) *contentReader {
	c := &contentReader{seed: seed, path: r.Path, pat: r.Pattern, data: r.Data, remaining: r.Size}
	c.buf = header(r.Pattern, r.Size)
	return c
}

func header(p Pattern, size int64) []byte {
	switch p {
	case PatternWAV:
		h := make([]byte, 44)
		copy(h[0:], "RIFF")
		binary.LittleEndian.PutUint32(h[4:], uint32(max64(size-8, 0)))
		copy(h[8:], "WAVEfmt ")
		binary.LittleEndian.PutUint32(h[16:], 16)
		binary.LittleEndian.PutUint16(h[20:], 1)
		binary.LittleEndian.PutUint16(h[22:], 1)
		binary.LittleEndian.PutUint32(h[24:], 8000)
		binary.LittleEndian.PutUint32(h[28:], 16000)
		binary.LittleEndian.PutUint16(h[32:], 2)
		binary.LittleEndian.PutUint16(h[34:], 16)
		copy(h[36:], "data")
		binary.LittleEndian.PutUint32(h[40:], uint32(max64(size-44, 0)))
		return h
	case PatternMP3:
		return []byte("ID3\x03\x00\x00\x00\x00\x00\x00")
	case PatternMP4:
		return []byte("\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomavc1")
	}
	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (c *contentReader) prng(i uint64) [32]byte {
	h := sha256.New()
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], c.seed)
	h.Write(b[:])
	h.Write([]byte(c.path))
	h.Write([]byte{0})
	binary.BigEndian.PutUint64(b[:], i)
	h.Write(b[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (c *contentReader) next() []byte {
	i := c.block
	c.block++
	switch c.pat {
	case PatternZero:
		return make([]byte, 32)
	case PatternRepeat:
		return []byte(c.data)
	case PatternText:
		p := c.prng(i)
		return []byte(fmt.Sprintf("line %06d %s\n", i, hex.EncodeToString(p[:16])))
	default:
		p := c.prng(i)
		return p[:]
	}
}

func (c *contentReader) Read(p []byte) (int, error) {
	if c.remaining <= 0 {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && c.remaining > 0 {
		if len(c.buf) == 0 {
			c.buf = c.next()
		}
		k := copy(p[n:], c.buf)
		if int64(k) > c.remaining {
			k = int(c.remaining)
		}
		n += k
		c.buf = c.buf[k:]
		c.remaining -= int64(k)
	}
	return n, nil
}
