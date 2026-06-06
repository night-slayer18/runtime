// Package search provides shared search and filtering primitives for all
// Runtime applications.
//
// The package centers on a stateful Searcher that finds every occurrence of a
// query within a body of text, exposes those occurrences as Match values, and
// can render highlighted output for terminal display. Matching is
// case-insensitive by default so that interactive search behaves the way users
// expect; callers that need exact matching can opt in via SetCaseSensitive.
//
// Offsets reported in a Match are byte offsets into the searched text, which
// makes them directly usable for slicing the original string (for example, in
// Highlight). Case-insensitive matching is implemented by lowercasing both the
// text and the query; this is exact for ASCII and the overwhelming majority of
// real-world text. Inputs containing runes whose lowercase form has a different
// byte length may report shifted offsets.
package search

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Match describes a single occurrence of a query within a piece of text.
type Match struct {
	// Index is the byte offset of the match within the searched text.
	Index int
	// Length is the byte length of the matched substring.
	Length int
	// Context is the full line containing the match, useful for displaying a
	// match in results lists where the surrounding text gives it meaning.
	Context string
}

// Searcher performs case-insensitive (by default) search and filtering and
// retains the most recent query and results so that Highlight can operate
// without re-specifying the query.
type Searcher struct {
	query     string
	caseSense bool
	results   []Match
	style     lipgloss.Style
	open      string
	close     string
	useMarker bool
}

// New returns a Searcher configured for case-insensitive matching with a
// default highlight style (reverse video + bold) suitable for terminal output.
func New() *Searcher {
	return &Searcher{
		style: lipgloss.NewStyle().Reverse(true).Bold(true),
	}
}

// SetCaseSensitive controls whether matching is case-sensitive. The default is
// case-insensitive matching. It returns the receiver to allow chaining.
func (s *Searcher) SetCaseSensitive(on bool) *Searcher {
	s.caseSense = on
	return s
}

// CaseSensitive reports whether the searcher matches case-sensitively.
func (s *Searcher) CaseSensitive() bool { return s.caseSense }

// SetHighlightStyle sets the lipgloss style used by Highlight to render matched
// spans and clears any previously configured plain-text markers. It returns the
// receiver to allow chaining.
func (s *Searcher) SetHighlightStyle(style lipgloss.Style) *Searcher {
	s.style = style
	s.useMarker = false
	return s
}

// SetMarkers configures Highlight to wrap matched spans with the given open and
// close strings instead of applying a lipgloss style. This is useful for
// producing deterministic, style-free output (for example "<b>" / "</b>"). It
// returns the receiver to allow chaining.
func (s *Searcher) SetMarkers(open, close string) *Searcher {
	s.open = open
	s.close = close
	s.useMarker = true
	return s
}

// Query returns the query from the most recent Search call.
func (s *Searcher) Query() string { return s.query }

// Results returns the matches from the most recent Search call.
func (s *Searcher) Results() []Match { return s.results }

// Search finds every non-overlapping occurrence of query within text and
// returns them in order. The query and results are stored on the Searcher so a
// subsequent call to Highlight can reuse them. An empty query matches nothing
// and yields a nil result.
func (s *Searcher) Search(text string, query string) []Match {
	s.query = query
	s.results = nil

	if query == "" {
		return nil
	}

	haystack := text
	needle := query
	if !s.caseSense {
		haystack = strings.ToLower(text)
		needle = strings.ToLower(query)
	}

	var matches []Match
	offset := 0
	for {
		idx := strings.Index(haystack[offset:], needle)
		if idx < 0 {
			break
		}
		start := offset + idx
		length := len(needle)
		matches = append(matches, Match{
			Index:   start,
			Length:  length,
			Context: lineAround(text, start),
		})
		// Advance past this match to find non-overlapping occurrences.
		offset = start + length
		if offset > len(haystack) {
			break
		}
	}

	s.results = matches
	return matches
}

// Filter returns the subset of items that contain query, preserving their
// original order. An empty query matches every item.
func (s *Searcher) Filter(items []string, query string) []string {
	if query == "" {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}

	needle := query
	if !s.caseSense {
		needle = strings.ToLower(query)
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		hay := item
		if !s.caseSense {
			hay = strings.ToLower(item)
		}
		if strings.Contains(hay, needle) {
			out = append(out, item)
		}
	}
	return out
}

// Highlight returns text with every occurrence of the searcher's current query
// wrapped for emphasis. By default each match is rendered with the configured
// lipgloss highlight style; if plain-text markers have been set via SetMarkers
// they are used instead. When the query is empty, text is returned unchanged.
func (s *Searcher) Highlight(text string) string {
	matches := s.Search(text, s.query)
	if len(matches) == 0 {
		return text
	}

	var b strings.Builder
	prev := 0
	for _, m := range matches {
		b.WriteString(text[prev:m.Index])
		segment := text[m.Index : m.Index+m.Length]
		if s.useMarker {
			b.WriteString(s.open)
			b.WriteString(segment)
			b.WriteString(s.close)
		} else {
			b.WriteString(s.style.Render(segment))
		}
		prev = m.Index + m.Length
	}
	b.WriteString(text[prev:])
	return b.String()
}

// lineAround returns the line of text that contains the byte offset idx, with
// surrounding newlines excluded. It is used to build a Match's Context.
func lineAround(text string, idx int) string {
	if idx < 0 || idx > len(text) {
		return ""
	}
	start := strings.LastIndexByte(text[:idx], '\n') + 1
	end := strings.IndexByte(text[idx:], '\n')
	if end < 0 {
		end = len(text)
	} else {
		end += idx
	}
	return text[start:end]
}
