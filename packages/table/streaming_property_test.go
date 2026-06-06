package table

// Feature: runtime-ecosystem, Property 5: Streaming data loading
//
// Property 5: For any row count that exceeds a simulated memory budget, an
// unfiltered, unsorted table backed by a RowProvider pulls rows incrementally
// through the provider for the visible window only. It never materializes the
// full data set: across rendering and navigation the number of distinct rows
// resident at any instant stays within the memory budget, and the cumulative
// set of rows ever touched stays far below the total row count.
//
// Validates: Requirements 3.2

import (
	"math/rand"
	"testing"

	"github.com/runtime-sh/runtime/packages/theme"
)

// streamingIterations is the minimum number of randomized scenarios exercised
// by the property, per the design's testing strategy (>= 100).
const streamingIterations = 200

// budgetProvider is a RowProvider that simulates an out-of-core data source
// with a hard memory budget. It generates rows lazily and tracks, per render
// frame, how many *distinct* rows are pulled. If a single frame ever pulls more
// distinct rows than the budget allows, the data set has effectively been
// materialized beyond what memory can hold, which fails the streaming property.
//
// It also records the union of every row index ever fetched so the test can
// assert the table never touches the full data set.
type budgetProvider struct {
	t      *testing.T
	n      int
	budget int

	frame      map[int]struct{} // distinct rows pulled in the current frame
	peakFrame  int              // largest distinct-per-frame seen
	everPulled map[int]struct{} // union of all rows ever pulled
	violated   bool             // set if a frame exceeded the budget
}

func newBudgetProvider(t *testing.T, n, budget int) *budgetProvider {
	return &budgetProvider{
		t:          t,
		n:          n,
		budget:     budget,
		frame:      make(map[int]struct{}),
		everPulled: make(map[int]struct{}),
	}
}

// beginFrame resets the per-frame residency tracker. A "frame" models one unit
// of work the table performs in response to a user action (a render, or a
// navigation followed by a render): only the rows resident during that frame
// count against the in-memory budget, since the table caches nothing.
func (p *budgetProvider) beginFrame() {
	p.frame = make(map[int]struct{})
}

func (p *budgetProvider) Len() int { return p.n }

func (p *budgetProvider) RowAt(i int) (Row, bool) {
	if i < 0 || i >= p.n {
		return Row{}, false
	}
	p.frame[i] = struct{}{}
	p.everPulled[i] = struct{}{}
	if len(p.frame) > p.peakFrame {
		p.peakFrame = len(p.frame)
	}
	// Enforce the simulated memory budget: holding more distinct rows than the
	// budget at once means the table failed to stream incrementally.
	if len(p.frame) > p.budget {
		p.violated = true
		p.t.Errorf("memory budget exceeded: %d distinct rows resident in one frame, budget %d (n=%d)",
			len(p.frame), p.budget, p.n)
	}
	return NewRow(itoa(i), "row"+itoa(i), "note"), true
}

// navKeysStreaming are the unfiltered, unsorted navigation actions the property
// drives the table through. None of them should ever force a full scan. (The
// package-level navKeys from the Property 4 test covers the same bindings; this
// test keeps its own list to stay self-documenting and independent.)
var navKeysStreaming = []string{"down", "up", "pgdn", "pgup", "g", "G", "down", "pgdn"}

// TestProperty5_StreamingDataLoading verifies that for any row count exceeding
// a simulated memory budget, navigating and rendering an unfiltered, unsorted
// provider-backed table pulls rows incrementally and never materializes the
// full data set.
//
// Feature: runtime-ecosystem, Property 5: Streaming data loading
// Validates: Requirements 3.2
func TestProperty5_StreamingDataLoading(t *testing.T) {
	rng := rand.New(rand.NewSource(0x57DA7A)) // fixed seed: reproducible failures

	for iter := 0; iter < streamingIterations; iter++ {
		// Random viewport: 1..40 data rows visible (height includes the header).
		visibleRows := 1 + rng.Intn(40)
		height := visibleRows + 1
		width := 40 + rng.Intn(200)

		// The memory budget is a small multiple of the viewport: enough to hold
		// a few windows worth of rows but nothing close to the full data set.
		// A correct streaming table never needs more than a couple of windows
		// resident at once (the visible window plus the selected row).
		budget := visibleRows*3 + 8

		// Row count strictly exceeds the budget — this is the "larger than
		// available memory" precondition of the property. Make it dramatically
		// larger so "never materializes the full set" is a meaningful claim.
		n := budget + 1 + rng.Intn(2_000_000)

		p := newBudgetProvider(t, n, budget)
		tbl := New(theme.DefaultStyles)
		tbl.SetSize(width, height)
		tbl.SetProvider(p, sampleColumns())

		// The table must be unfiltered and unsorted (the streaming fast path).
		if tbl.RowCount() != n {
			t.Fatalf("iter %d: expected RowCount %d, got %d", iter, n, tbl.RowCount())
		}

		// Initial render: pulls only the visible window.
		p.beginFrame()
		_ = tbl.View()

		// Drive a random sequence of navigation actions, rendering after each.
		// Every frame is independently bounded by the memory budget.
		steps := 5 + rng.Intn(20)
		for s := 0; s < steps; s++ {
			key := navKeysStreaming[rng.Intn(len(navKeysStreaming))]
			p.beginFrame()
			tbl.Navigate(key)
			_ = tbl.View()
			if p.violated {
				return // error already reported by RowAt
			}
		}

		// The table must never have materialized the full data set: the union
		// of all rows ever touched stays strictly (and far) below n.
		if len(p.everPulled) >= n {
			t.Fatalf("iter %d: table materialized the full data set: pulled %d of %d rows",
				iter, len(p.everPulled), n)
		}

		// Stronger incremental guarantee: peak per-frame residency is bounded
		// by the budget and is independent of the dataset size.
		if p.peakFrame > budget {
			t.Fatalf("iter %d: peak per-frame residency %d exceeded budget %d (n=%d)",
				iter, p.peakFrame, budget, n)
		}
	}
}
