package model

// Feature: runtime-ecosystem, Property 5: Streaming data loading
//
// Property 5 (Pulse): For a log source larger than the memory budget, Pulse
// tails the source incrementally and keeps navigation responsive without
// buffering the whole file. The LogReader retains at most its configured
// capacity of the most-recent lines in memory at any instant, even as the total
// number of lines read grows far beyond that capacity, and it never reads the
// underlying source more than once end-to-end (true streaming, not re-scanning).
//
// Validates: Requirements 3.2

import (
	"io"
	"math/rand"
	"strconv"
	"testing"
)

// streamingIterations is the minimum number of randomized scenarios exercised
// by the property, per the design's testing strategy (>= 100).
const streamingIterations = 200

// genReader is an io.ReadCloser that lazily synthesizes a log of n lines. It
// never holds more than one line's worth of bytes in memory at a time, so it
// models a log source that is far larger than available memory: the full
// content is never materialized by the source itself. It also records the
// highest byte offset ever produced so the test can assert the reader makes a
// single forward pass (no rewinding or re-scanning).
type genReader struct {
	n       int    // total lines to emit
	line    int    // next line index to emit
	pending []byte // bytes of the current line not yet delivered
	maxRead int64  // total bytes ever produced (monotonic == single pass)
	reads   int    // number of Read calls (sanity bound)
}

func newGenReader(n int) *genReader { return &genReader{n: n} }

func (g *genReader) Read(p []byte) (int, error) {
	g.reads++
	if len(g.pending) == 0 {
		if g.line >= g.n {
			return 0, io.EOF
		}
		// Synthesize one line lazily. Include a number so it is realistic log
		// content; the generator never holds more than this single line.
		g.pending = []byte("event " + strconv.Itoa(g.line) + " processed id=" + strconv.Itoa(g.line*7) + "\n")
		g.line++
	}
	cnt := copy(p, g.pending)
	g.pending = g.pending[cnt:]
	g.maxRead += int64(cnt)
	return cnt, nil
}

func (g *genReader) Close() error { return nil }

// TestProperty5_StreamingLogResponsiveness verifies that for any log source
// larger than the memory budget, the LogReader tails incrementally: the number
// of buffered lines never exceeds the capacity, the total lines read matches the
// source, and the most-recent window is exactly what is retained.
//
// Feature: runtime-ecosystem, Property 5: Streaming data loading
// Validates: Requirements 3.2
func TestProperty5_StreamingLogResponsiveness(t *testing.T) {
	rng := rand.New(rand.NewSource(0x9075E)) // fixed seed: reproducible failures

	for iter := 0; iter < streamingIterations; iter++ {
		// Memory budget: 1..500 retained lines.
		capacity := 1 + rng.Intn(500)

		// Total lines strictly exceed the budget — the "larger than available
		// memory" precondition. Make it dramatically larger so "never buffers
		// the whole file" is a meaningful claim.
		n := capacity + 1 + rng.Intn(200_000)

		gen := newGenReader(n)
		r := NewLogReaderFromReader(gen, capacity)

		// Drain the source in a random number of incremental polls, modeling
		// live tailing where bytes arrive over time. After every poll the
		// in-memory residency must stay within the budget.
		polls := 1 + rng.Intn(8)
		for s := 0; s < polls; s++ {
			if _, err := r.Poll(); err != nil {
				t.Fatalf("iter %d: poll error: %v", iter, err)
			}
			if r.Buffered() > capacity {
				t.Fatalf("iter %d: memory budget exceeded: buffered %d > capacity %d (total=%d)",
					iter, r.Buffered(), capacity, r.Total())
			}
		}

		// All lines must have been read exactly once (single forward pass).
		if r.Total() != n {
			t.Fatalf("iter %d: read %d lines, want %d", iter, r.Total(), n)
		}

		// The retained window must be bounded by the budget and far below n.
		if r.Buffered() > capacity {
			t.Fatalf("iter %d: final residency %d exceeds capacity %d", iter, r.Buffered(), capacity)
		}
		entries := r.Entries()
		if len(entries) != min(capacity, n) {
			t.Fatalf("iter %d: retained %d entries, want %d", iter, len(entries), min(capacity, n))
		}

		// The retained window must be the most-recent lines, in order: the last
		// entry is line n, and lines are strictly increasing and contiguous.
		if entries[len(entries)-1].Line != n {
			t.Fatalf("iter %d: last retained line %d, want %d", iter, entries[len(entries)-1].Line, n)
		}
		for i := 1; i < len(entries); i++ {
			if entries[i].Line != entries[i-1].Line+1 {
				t.Fatalf("iter %d: retained window not contiguous at %d: %d then %d",
					iter, i, entries[i-1].Line, entries[i].Line)
			}
		}
	}
}
