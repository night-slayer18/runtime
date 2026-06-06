// Package model defines the core data model for pulse: streaming log tailing,
// pattern filtering, and similar-error grouping.
package model

import (
	"bufio"
	"io"
	"os"
	"strings"
)

// DefaultCapacity is the number of most-recent log lines a LogReader keeps in
// memory when no explicit capacity is supplied. It bounds memory use so that a
// log source far larger than available memory can still be tailed: only the
// trailing window of lines is retained while navigation stays responsive.
const DefaultCapacity = 10000

// LogEntry is a single parsed log line.
type LogEntry struct {
	// Line is the 1-based line number within the source.
	Line int
	// Text is the line content with the trailing newline (and any preceding
	// carriage return) stripped.
	Text string
	// Offset is the byte offset of the start of the line within the source.
	Offset int64
}

// LogReader tails a log source incrementally. It reads only complete lines
// (those terminated by a newline) and retains at most capacity of the most
// recent lines in a ring buffer, so the full file is never buffered in memory.
// Poll may be called repeatedly to pick up bytes appended since the last call,
// which is what enables live tailing without external dependencies.
//
// A LogReader is not safe for concurrent use; callers should poll it from a
// single goroutine (for example, a Bubble Tea command loop).
type LogReader struct {
	path    string
	src     io.ReadCloser
	reader  *bufio.Reader
	partial []byte // bytes of an incomplete trailing line awaiting a newline

	lineStart int64 // byte offset of the start of the next line
	lineNum   int   // number of complete lines read so far

	// Ring buffer of the most recent entries. When capacity is 0 the buffer
	// grows without bound; otherwise it holds at most capacity entries.
	buf      []LogEntry
	capacity int
	start    int // index of the oldest entry in buf (ring origin)
	count    int // number of entries currently buffered
	total    int // total complete lines ever read (including evicted ones)
}

// NewLogReader returns a LogReader that tails the file at path with the default
// in-memory capacity. The file is opened immediately; call Poll to read the
// lines available so far and again to pick up appended lines.
func NewLogReader(path string) (*LogReader, error) {
	return NewLogReaderCapacity(path, DefaultCapacity)
}

// NewLogReaderCapacity is like NewLogReader but lets the caller bound the number
// of most-recent lines retained in memory. A capacity of 0 retains every line.
func NewLogReaderCapacity(path string, capacity int) (*LogReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r := newReaderFromSource(f, capacity)
	r.path = path
	return r, nil
}

// NewLogReaderFromReader builds a LogReader over an arbitrary io.ReadCloser. It
// is primarily useful for tests and for tailing non-file sources (for example,
// a pipe). The same incremental, bounded-memory semantics apply.
func NewLogReaderFromReader(src io.ReadCloser, capacity int) *LogReader {
	return newReaderFromSource(src, capacity)
}

func newReaderFromSource(src io.ReadCloser, capacity int) *LogReader {
	if capacity < 0 {
		capacity = 0
	}
	r := &LogReader{
		src:      src,
		reader:   bufio.NewReader(src),
		capacity: capacity,
	}
	if capacity > 0 {
		r.buf = make([]LogEntry, capacity)
	}
	return r
}

// Poll reads every complete line currently available from the source and
// returns the newly read entries in order. A trailing line without a newline is
// retained internally and only emitted once its terminating newline arrives, so
// repeated calls correctly tail an actively-written log. Poll returns a nil
// error at end of input; only genuine read failures are surfaced.
func (r *LogReader) Poll() ([]LogEntry, error) {
	var out []LogEntry
	for {
		chunk, err := r.reader.ReadString('\n')
		if len(chunk) > 0 {
			r.partial = append(r.partial, chunk...)
			if strings.HasSuffix(chunk, "\n") {
				full := r.partial
				text := strings.TrimRight(string(full), "\r\n")
				r.lineNum++
				entry := LogEntry{Line: r.lineNum, Text: text, Offset: r.lineStart}
				r.lineStart += int64(len(full))
				r.partial = nil
				r.push(entry)
				out = append(out, entry)
			}
		}
		if err != nil {
			if err == io.EOF {
				return out, nil
			}
			return out, err
		}
	}
}

// push appends an entry to the ring buffer, evicting the oldest entry when the
// buffer is at capacity. This is what keeps memory bounded while tailing.
func (r *LogReader) push(e LogEntry) {
	r.total++
	if r.capacity == 0 {
		r.buf = append(r.buf, e)
		r.count++
		return
	}
	if r.count < r.capacity {
		r.buf[(r.start+r.count)%r.capacity] = e
		r.count++
		return
	}
	// Buffer full: overwrite the oldest entry and advance the ring origin.
	r.buf[r.start] = e
	r.start = (r.start + 1) % r.capacity
}

// Entries returns the buffered entries in line order. The result is a copy, so
// callers may retain or mutate it freely. With a bounded capacity this is the
// trailing window of the source rather than the entire file.
func (r *LogReader) Entries() []LogEntry {
	out := make([]LogEntry, r.count)
	if r.capacity == 0 {
		copy(out, r.buf[:r.count])
		return out
	}
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(r.start+i)%r.capacity]
	}
	return out
}

// Buffered reports how many entries are currently held in memory. It never
// exceeds the configured capacity (when one is set).
func (r *LogReader) Buffered() int { return r.count }

// Total reports the total number of complete lines read from the source,
// including any that have been evicted from the in-memory window.
func (r *LogReader) Total() int { return r.total }

// Capacity returns the configured in-memory line budget (0 means unbounded).
func (r *LogReader) Capacity() int { return r.capacity }

// Close releases the underlying source.
func (r *LogReader) Close() error {
	if r.src == nil {
		return nil
	}
	return r.src.Close()
}
