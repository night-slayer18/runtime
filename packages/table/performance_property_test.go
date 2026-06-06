package table

import (
	"math/rand"
	"testing"
	"time"

	"github.com/runtime-sh/runtime/packages/theme"
)

// Feature: runtime-ecosystem, Property 4: Performance with large datasets
//
// For any dataset with N rows where N >= 1,000,000, a Navigate step renders
// within 100ms and touches only the visible window: the work done per keystroke
// is bounded by the viewport size, not the dataset size. This is what backs the
// design's claim of 60 FPS / <=100ms response on million-row data sets.
//
// Validates: Requirements 3.1, 3.3

// perfIterations is the number of randomized Navigate keystrokes exercised by
// the property, comfortably above the design's 100-iteration minimum.
const perfIterations = 200

// navKeys is the set of navigation keystrokes the property samples over. It
// covers every binding Navigate recognizes so the bounded-work guarantee is
// exercised across single-row moves, paging, jumps, and horizontal scrolling.
var navKeys = []string{
	"up", "k",
	"down", "j",
	"pgup", "ctrl+u",
	"pgdn", "ctrl+d",
	"home", "g",
	"end", "G",
	"left", "h",
	"right", "l",
}

// reset clears the fetch record so the next Navigate+View cycle can be measured
// in isolation. It lets the property assert distinct rows fetched *per render*
// rather than cumulatively.
func (p *countingProvider) reset() {
	p.fetched = make(map[int]int)
}

// TestProperty4_PerformanceWithLargeDatasets drives a table backed by a lazily
// generated, 1,000,000+ row provider through a long sequence of randomized
// navigation keystrokes. For every keystroke it asserts two things:
//
//  1. Navigate followed by View completes within 100ms (response time bound).
//  2. The render touches only a viewport-sized, bounded set of distinct rows —
//     independent of the dataset's size — proving navigation never scans or
//     materializes the full data set.
//
// Feature: runtime-ecosystem, Property 4: Performance with large datasets
//
// Validates: Requirements 3.1, 3.3
func TestProperty4_PerformanceWithLargeDatasets(t *testing.T) {
	rng := rand.New(rand.NewSource(0xF00DCAFE))

	// The design budget is 100ms per Navigate+View. Under the race detector the
	// elapsed time measures instrumented execution, not real performance, so we
	// scale the budget by 10x (race overhead is well within that). The default
	// `make test` run keeps the strict 100ms target. The deterministic
	// bounded-work invariant below is what actually proves O(viewport) behavior
	// and stays strict regardless of the race detector.
	timeBudget := 100 * time.Millisecond
	if raceEnabled {
		timeBudget *= 10
	}

	for iter := 0; iter < perfIterations; iter++ {
		// N is always at least 1,000,000; vary it so the property holds across
		// a range of very large datasets, not a single magic size. Generation
		// is lazy, so even tens of millions of rows cost nothing until fetched.
		n := 1_000_000 + rng.Intn(50_000_000)

		tbl := New(theme.DefaultStyles)
		// A realistic terminal viewport: header + visible data rows.
		tbl.SetSize(80, 25)
		provider := newCountingProvider(n)
		tbl.SetProvider(provider, sampleColumns())

		// The number of data rows in the viewport bounds the work a render may
		// do. Allow a small constant margin for selection/edge fetches while
		// keeping the bound strictly independent of N.
		visible := tbl.visibleRows()
		bound := visible + 4

		// Sub-property A: SetProvider must not have scanned the dataset. With no
		// filter or sort active the table reports the provider length without
		// pulling rows, so nothing should have been fetched yet.
		if got := provider.distinctFetched(); got != 0 {
			t.Fatalf("iter %d: SetProvider fetched %d rows for an unfiltered/unsorted provider; expected 0 (no scan of N=%d)", iter, got, n)
		}
		if tbl.RowCount() != n {
			t.Fatalf("iter %d: RowCount = %d, want %d", iter, tbl.RowCount(), n)
		}

		// Drive a long random walk; each step must stay bounded and fast.
		for step := 0; step < perfIterations; step++ {
			key := navKeys[rng.Intn(len(navKeys))]

			// Measure only this keystroke's work: reset the fetch record, then
			// time Navigate + View together (a render is what a keystroke
			// triggers in the Bubble Tea update loop).
			provider.reset()
			start := time.Now()
			tbl.Navigate(key)
			_ = tbl.View()
			elapsed := time.Since(start)

			// Bound 1: response time. The design budget is 100ms per operation
			// (scaled under the race detector; see timeBudget above).
			if elapsed > timeBudget {
				t.Fatalf("iter %d step %d: Navigate(%q)+View took %v on N=%d, exceeds %v budget (raceEnabled=%v)",
					iter, step, key, elapsed, n, timeBudget, raceEnabled)
			}

			// Bound 2: bounded work. A single render must touch only the
			// visible window, never a count that scales with N.
			if got := provider.distinctFetched(); got > bound {
				t.Fatalf("iter %d step %d: Navigate(%q)+View fetched %d distinct rows on N=%d, exceeds viewport-sized bound %d",
					iter, step, key, got, n, bound)
			}

			// The cursor must always stay a valid selection within the view.
			if c := tbl.Cursor(); c < 0 || c >= n {
				t.Fatalf("iter %d step %d: cursor %d out of range [0,%d) after Navigate(%q)",
					iter, step, c, n, key)
			}
		}
	}
}
