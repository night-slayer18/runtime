package model

import (
	"io"
	"strings"
	"testing"
)

// stringCloser adapts a strings.Reader to io.ReadCloser for tests.
type stringCloser struct{ *strings.Reader }

func (stringCloser) Close() error { return nil }

func newReader(s string, capacity int) *LogReader {
	return NewLogReaderFromReader(stringCloser{strings.NewReader(s)}, capacity)
}

func TestLogReaderReadsCompleteLines(t *testing.T) {
	r := newReader("a\nb\nc\n", 0)
	got, err := r.Poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0].Text != "a" || got[2].Text != "c" {
		t.Fatalf("unexpected entries: %+v", got)
	}
	if got[0].Line != 1 || got[2].Line != 3 {
		t.Fatalf("unexpected line numbers: %+v", got)
	}
}

func TestLogReaderWithholdsIncompleteTrailingLine(t *testing.T) {
	r := newReader("done\npartial", 0)
	got, _ := r.Poll()
	if len(got) != 1 || got[0].Text != "done" {
		t.Fatalf("incomplete trailing line should be withheld: %+v", got)
	}
	if r.Buffered() != 1 {
		t.Fatalf("expected 1 buffered, got %d", r.Buffered())
	}
}

// pipeReader lets the test feed bytes incrementally to simulate live tailing.
// After delivering a chunk it returns io.EOF on the next Read so a single Poll
// drains only the bytes available "so far"; a subsequent Poll picks up the next
// chunk, modeling a log that is appended to over time.
type pipeReader struct {
	chunks     [][]byte
	eofPending bool
}

func (p *pipeReader) Read(b []byte) (int, error) {
	if p.eofPending {
		p.eofPending = false
		return 0, io.EOF
	}
	if len(p.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.chunks[0])
	p.chunks[0] = p.chunks[0][n:]
	if len(p.chunks[0]) == 0 {
		p.chunks = p.chunks[1:]
	}
	p.eofPending = true
	return n, nil
}

func (p *pipeReader) Close() error { return nil }

func TestLogReaderLiveTailAcrossPolls(t *testing.T) {
	pr := &pipeReader{chunks: [][]byte{[]byte("first\nsec"), []byte("ond\nthird\n")}}
	r := NewLogReaderFromReader(pr, 0)

	got1, _ := r.Poll()
	if len(got1) != 1 || got1[0].Text != "first" {
		t.Fatalf("first poll wrong: %+v", got1)
	}

	got2, _ := r.Poll()
	if len(got2) != 2 || got2[0].Text != "second" || got2[1].Text != "third" {
		t.Fatalf("second poll wrong: %+v", got2)
	}
	if r.Total() != 3 {
		t.Fatalf("expected total 3, got %d", r.Total())
	}
}

func TestLogReaderBoundedCapacityRetainsMostRecent(t *testing.T) {
	r := newReader("l1\nl2\nl3\nl4\nl5\n", 2)
	if _, err := r.Poll(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if r.Buffered() != 2 {
		t.Fatalf("expected 2 buffered, got %d", r.Buffered())
	}
	if r.Total() != 5 {
		t.Fatalf("expected total 5, got %d", r.Total())
	}
	entries := r.Entries()
	if entries[0].Text != "l4" || entries[1].Text != "l5" {
		t.Fatalf("expected most-recent window [l4 l5], got %+v", entries)
	}
}

func TestLogReaderStripsCarriageReturn(t *testing.T) {
	r := newReader("windows\r\nline\r\n", 0)
	got, _ := r.Poll()
	if got[0].Text != "windows" || got[1].Text != "line" {
		t.Fatalf("CRLF not stripped: %+v", got)
	}
}
