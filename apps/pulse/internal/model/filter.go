package model

import (
	"regexp"

	"github.com/runtime-sh/runtime/packages/search"
)

// Filter restricts a stream of log entries to those whose text matches a query
// pattern. The query is interpreted as a regular expression (via the standard
// regexp package) when it compiles successfully, giving Pulse true pattern
// filtering; queries that are not valid regular expressions fall back to
// case-aware substring matching delegated to the shared search package, so the
// filter never rejects input and behaves consistently with the rest of the
// ecosystem. Matching is case-insensitive by default. An empty query matches
// every entry.
type Filter struct {
	searcher *search.Searcher
	query    string

	// re is the compiled regular expression for the active query, or nil when
	// the query is empty or not a valid regular expression (substring mode).
	re        *regexp.Regexp
	caseSense bool
}

// NewFilter returns a Filter backed by a fresh case-insensitive searcher.
func NewFilter() *Filter {
	return &Filter{searcher: search.New()}
}

// SetCaseSensitive controls whether pattern matching is case-sensitive. It
// recompiles the active query under the new mode and returns the receiver to
// allow chaining.
func (f *Filter) SetCaseSensitive(on bool) *Filter {
	f.caseSense = on
	f.searcher.SetCaseSensitive(on)
	f.compile()
	return f
}

// SetQuery sets the active filter pattern, compiling it as a regular expression
// when possible.
func (f *Filter) SetQuery(query string) {
	f.query = query
	f.compile()
}

// compile (re)compiles the active query into a regular expression. The case
// sensitivity flag is honored by prepending the (?i) inline flag for
// case-insensitive matching. When the query is empty or fails to compile, re is
// cleared and matching falls back to substring search.
func (f *Filter) compile() {
	f.re = nil
	if f.query == "" {
		return
	}
	pattern := f.query
	if !f.caseSense {
		pattern = "(?i)" + pattern
	}
	if re, err := regexp.Compile(pattern); err == nil {
		f.re = re
	}
}

// Query returns the active filter pattern.
func (f *Filter) Query() string { return f.query }

// Matches reports whether a single entry passes the active filter. An empty
// query matches everything. A valid regular expression query matches via
// regexp; otherwise it falls back to substring matching.
func (f *Filter) Matches(e LogEntry) bool {
	if f.query == "" {
		return true
	}
	if f.re != nil {
		return f.re.MatchString(e.Text)
	}
	return len(f.searcher.Search(e.Text, f.query)) > 0
}

// Apply returns the subset of entries whose text matches the active query,
// preserving their original order. An empty query returns a copy of every
// entry. The input slice is never mutated.
func (f *Filter) Apply(entries []LogEntry) []LogEntry {
	if f.query == "" {
		out := make([]LogEntry, len(entries))
		copy(out, entries)
		return out
	}
	out := make([]LogEntry, 0, len(entries))
	for _, e := range entries {
		if f.Matches(e) {
			out = append(out, e)
		}
	}
	return out
}
