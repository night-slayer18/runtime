package search

import (
	"reflect"
	"testing"
)

// indices is a small helper that extracts the byte offsets of a slice of
// matches so tests can assert on positions concisely.
func indices(matches []Match) []int {
	out := make([]int, len(matches))
	for i, m := range matches {
		out[i] = m.Index
	}
	return out
}

// TestSearchCaseInsensitiveByDefault verifies that, with the default
// configuration, a query matches occurrences regardless of letter case.
//
// Validates: Requirements 7.2, 7.3
func TestSearchCaseInsensitiveByDefault(t *testing.T) {
	s := New()
	matches := s.Search("Hello HELLO hello", "hello")
	if len(matches) != 3 {
		t.Fatalf("expected 3 case-insensitive matches, got %d: %+v", len(matches), matches)
	}
	if got, want := indices(matches), []int{0, 6, 12}; !reflect.DeepEqual(got, want) {
		t.Errorf("match indices = %v, want %v", got, want)
	}
	for _, m := range matches {
		if m.Length != len("hello") {
			t.Errorf("match length = %d, want %d", m.Length, len("hello"))
		}
	}
}

// TestSearchCaseSensitiveOptIn verifies that enabling case-sensitive matching
// only finds occurrences with the exact casing of the query.
//
// Validates: Requirements 7.2
func TestSearchCaseSensitiveOptIn(t *testing.T) {
	s := New().SetCaseSensitive(true)
	matches := s.Search("Hello HELLO hello", "hello")
	if len(matches) != 1 {
		t.Fatalf("expected 1 case-sensitive match, got %d: %+v", len(matches), matches)
	}
	if matches[0].Index != 12 {
		t.Errorf("match index = %d, want 12", matches[0].Index)
	}
}

// TestSearchMultipleMatches verifies that all non-overlapping occurrences are
// reported in order.
//
// Validates: Requirements 7.2, 7.3
func TestSearchMultipleMatches(t *testing.T) {
	s := New()
	matches := s.Search("ababab", "ab")
	if got, want := indices(matches), []int{0, 2, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("match indices = %v, want %v", got, want)
	}
}

// TestSearchNonOverlapping verifies that matches do not overlap: after a match
// the scan advances past it, so "aaa"/"aa" yields a single match, not two.
//
// Validates: Requirements 7.2
func TestSearchNonOverlapping(t *testing.T) {
	s := New()
	matches := s.Search("aaaa", "aa")
	if got, want := indices(matches), []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("match indices = %v, want %v", got, want)
	}
}

// TestSearchNoMatch verifies that a query absent from the text yields no
// matches and stores an empty result.
//
// Validates: Requirements 7.2
func TestSearchNoMatch(t *testing.T) {
	s := New()
	matches := s.Search("the quick brown fox", "zebra")
	if len(matches) != 0 {
		t.Fatalf("expected no matches, got %d: %+v", len(matches), matches)
	}
	if s.Results() != nil {
		t.Errorf("expected nil stored results, got %+v", s.Results())
	}
}

// TestSearchEmptyQuery verifies that an empty query matches nothing.
//
// Validates: Requirements 7.2
func TestSearchEmptyQuery(t *testing.T) {
	s := New()
	matches := s.Search("any text at all", "")
	if matches != nil {
		t.Errorf("expected nil matches for empty query, got %+v", matches)
	}
	if s.Query() != "" {
		t.Errorf("expected stored query to be empty, got %q", s.Query())
	}
}

// TestSearchContext verifies that a match's Context is the full line that
// contains it.
//
// Validates: Requirements 7.2
func TestSearchContext(t *testing.T) {
	s := New()
	matches := s.Search("first line\nsecond needle line\nthird line", "needle")
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if got, want := matches[0].Context, "second needle line"; got != want {
		t.Errorf("context = %q, want %q", got, want)
	}
}

// TestHighlightSpanBoundaries verifies that Highlight wraps exactly the matched
// spans with the configured markers, leaving surrounding text untouched. Using
// markers keeps the output deterministic and free of terminal styling codes.
//
// Validates: Requirements 7.3
func TestHighlightSpanBoundaries(t *testing.T) {
	s := New().SetMarkers("<", ">")
	s.Search("foo bar foo", "foo")
	got := s.Highlight("foo bar foo")
	want := "<foo> bar <foo>"
	if got != want {
		t.Errorf("highlight = %q, want %q", got, want)
	}
}

// TestHighlightAtBoundaries verifies highlighting matches that sit at the very
// start and very end of the text, ensuring no characters are dropped or
// duplicated at the edges.
//
// Validates: Requirements 7.3
func TestHighlightAtBoundaries(t *testing.T) {
	s := New().SetMarkers("[", "]")
	s.Search("abXYZab", "ab")
	got := s.Highlight("abXYZab")
	want := "[ab]XYZ[ab]"
	if got != want {
		t.Errorf("highlight = %q, want %q", got, want)
	}
}

// TestHighlightAdjacentMatches verifies that back-to-back matches are each
// wrapped without overlap or gaps.
//
// Validates: Requirements 7.3
func TestHighlightAdjacentMatches(t *testing.T) {
	s := New().SetMarkers("[", "]")
	s.Search("abab", "ab")
	got := s.Highlight("abab")
	want := "[ab][ab]"
	if got != want {
		t.Errorf("highlight = %q, want %q", got, want)
	}
}

// TestHighlightNoMatchUnchanged verifies that text with no matching query is
// returned verbatim.
//
// Validates: Requirements 7.3
func TestHighlightNoMatchUnchanged(t *testing.T) {
	s := New().SetMarkers("[", "]")
	s.Search("hello world", "xyz")
	got := s.Highlight("hello world")
	if got != "hello world" {
		t.Errorf("highlight = %q, want unchanged input", got)
	}
}

// TestHighlightEmptyQueryUnchanged verifies that an empty current query leaves
// text unchanged.
//
// Validates: Requirements 7.3
func TestHighlightEmptyQueryUnchanged(t *testing.T) {
	s := New().SetMarkers("[", "]")
	// No prior Search call, so the query is empty.
	got := s.Highlight("hello world")
	if got != "hello world" {
		t.Errorf("highlight = %q, want unchanged input", got)
	}
}

// TestHighlightCaseInsensitive verifies that highlighting preserves the
// original casing of matched spans even though matching is case-insensitive.
//
// Validates: Requirements 7.2, 7.3
func TestHighlightCaseInsensitive(t *testing.T) {
	s := New().SetMarkers("[", "]")
	s.Search("Foo foo FOO", "foo")
	got := s.Highlight("Foo foo FOO")
	want := "[Foo] [foo] [FOO]"
	if got != want {
		t.Errorf("highlight = %q, want %q", got, want)
	}
}

// TestFilterEmptyQueryReturnsAll verifies that an empty query keeps every item.
//
// Validates: Requirements 7.2
func TestFilterEmptyQueryReturnsAll(t *testing.T) {
	s := New()
	items := []string{"alpha", "beta", "gamma"}
	got := s.Filter(items, "")
	if !reflect.DeepEqual(got, items) {
		t.Errorf("filter = %v, want %v", got, items)
	}
}

// TestFilterCaseInsensitive verifies that filtering selects matching items
// regardless of case while preserving original order.
//
// Validates: Requirements 7.2
func TestFilterCaseInsensitive(t *testing.T) {
	s := New()
	items := []string{"Apple", "banana", "Apricot", "cherry"}
	got := s.Filter(items, "ap")
	want := []string{"Apple", "Apricot"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filter = %v, want %v", got, want)
	}
}
