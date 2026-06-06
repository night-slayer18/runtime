package model

import "testing"

func entries(lines ...string) []LogEntry {
	out := make([]LogEntry, len(lines))
	for i, l := range lines {
		out[i] = LogEntry{Line: i + 1, Text: l}
	}
	return out
}

func texts(es []LogEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.Text
	}
	return out
}

func TestFilterEmptyQueryMatchesAll(t *testing.T) {
	f := NewFilter()
	in := entries("alpha", "beta", "gamma")
	got := f.Apply(in)
	if len(got) != len(in) {
		t.Fatalf("empty query should match all: got %d want %d", len(got), len(in))
	}
}

func TestFilterCaseInsensitiveByDefault(t *testing.T) {
	f := NewFilter()
	f.SetQuery("ERROR")
	in := entries("info: ok", "error: boom", "Error: again", "debug")
	got := texts(f.Apply(in))
	want := []string{"error: boom", "Error: again"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestFilterCaseSensitive(t *testing.T) {
	f := NewFilter().SetCaseSensitive(true)
	f.SetQuery("Error")
	in := entries("error: lower", "Error: upper")
	got := texts(f.Apply(in))
	if len(got) != 1 || got[0] != "Error: upper" {
		t.Fatalf("case-sensitive filter wrong: %v", got)
	}
}

func TestFilterNoMatches(t *testing.T) {
	f := NewFilter()
	f.SetQuery("zzz")
	got := f.Apply(entries("a", "b", "c"))
	if len(got) != 0 {
		t.Fatalf("expected no matches, got %v", texts(got))
	}
}

func TestFilterPreservesOrderAndDoesNotMutate(t *testing.T) {
	f := NewFilter()
	f.SetQuery("x")
	in := entries("x1", "y", "x2", "x3", "z")
	got := texts(f.Apply(in))
	want := []string{"x1", "x2", "x3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order not preserved: got %v want %v", got, want)
		}
	}
	if in[0].Text != "x1" || in[1].Text != "y" {
		t.Fatalf("input slice was mutated")
	}
}

func TestFilterRegexPattern(t *testing.T) {
	f := NewFilter()
	f.SetQuery(`error \d+`)
	in := entries("error 42 occurred", "error code", "warn 7", "error 100 again")
	got := texts(f.Apply(in))
	want := []string{"error 42 occurred", "error 100 again"}
	if len(got) != len(want) {
		t.Fatalf("regex filter wrong: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("regex filter wrong: got %v want %v", got, want)
		}
	}
}

func TestFilterRegexAnchorsAndAlternation(t *testing.T) {
	f := NewFilter()
	f.SetQuery(`^(WARN|ERROR):`)
	in := entries("ERROR: boom", "INFO: ERROR: nested", "WARN: careful", "DEBUG: ok")
	got := texts(f.Apply(in))
	want := []string{"ERROR: boom", "WARN: careful"}
	if len(got) != len(want) {
		t.Fatalf("anchored regex wrong: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("anchored regex wrong: got %v want %v", got, want)
		}
	}
}

func TestFilterRegexCaseInsensitiveByDefault(t *testing.T) {
	f := NewFilter()
	f.SetQuery(`err.r`)
	in := entries("ERROR here", "no match", "Err0r too")
	got := texts(f.Apply(in))
	want := []string{"ERROR here", "Err0r too"}
	if len(got) != len(want) {
		t.Fatalf("case-insensitive regex wrong: got %v want %v", got, want)
	}
}

func TestFilterRegexCaseSensitive(t *testing.T) {
	f := NewFilter().SetCaseSensitive(true)
	f.SetQuery(`ERROR`)
	in := entries("ERROR upper", "error lower")
	got := texts(f.Apply(in))
	if len(got) != 1 || got[0] != "ERROR upper" {
		t.Fatalf("case-sensitive regex wrong: %v", got)
	}
}

func TestFilterInvalidRegexFallsBackToSubstring(t *testing.T) {
	f := NewFilter()
	// An unclosed group is not a valid regular expression; the filter must
	// treat it as a literal substring rather than failing.
	f.SetQuery("err(or")
	in := entries("an err(or here", "error without paren", "clean")
	got := texts(f.Apply(in))
	if len(got) != 1 || got[0] != "an err(or here" {
		t.Fatalf("invalid regex should match literally: got %v", got)
	}
}
