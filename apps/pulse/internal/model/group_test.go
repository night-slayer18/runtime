package model

import "testing"

func TestNormalizeReplacesNumbers(t *testing.T) {
	got := Normalize("request 42 took 1.5 ms")
	want := "request <NUM> took <NUM> ms"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeReplacesUUID(t *testing.T) {
	got := Normalize("user 550e8400-e29b-41d4-a716-446655440000 not found")
	want := "user <UUID> not found"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeReplacesHex(t *testing.T) {
	got := Normalize("segfault at 0xdeadbeef")
	want := "segfault at <HEX>"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeCollapsesWhitespace(t *testing.T) {
	got := Normalize("  too    much   space  ")
	want := "too much space"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestGroupSimilarCollapsesByTemplate(t *testing.T) {
	in := entries(
		"connection failed for user 1",
		"connection failed for user 2",
		"connection failed for user 3",
		"disk full on /dev/sda1",
		"timeout after 30 seconds",
		"timeout after 45 seconds",
	)
	groups := GroupSimilar(in)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d: %+v", len(groups), groups)
	}
	// Most frequent group first.
	if groups[0].Count != 3 {
		t.Fatalf("expected top group count 3, got %d", groups[0].Count)
	}
	if groups[0].Template != "connection failed for user <NUM>" {
		t.Fatalf("unexpected top template: %q", groups[0].Template)
	}
}

func TestGroupSimilarEmpty(t *testing.T) {
	if got := GroupSimilar(nil); len(got) != 0 {
		t.Fatalf("expected no groups, got %d", len(got))
	}
}

func TestGroupSimilarCountSumsToInput(t *testing.T) {
	in := entries("a 1", "a 2", "b", "c 3", "c 4", "c 5")
	groups := GroupSimilar(in)
	total := 0
	for _, g := range groups {
		total += g.Count
		if g.Count != len(g.Entries) {
			t.Fatalf("group count %d != entries %d", g.Count, len(g.Entries))
		}
	}
	if total != len(in) {
		t.Fatalf("group counts sum to %d, want %d", total, len(in))
	}
}
