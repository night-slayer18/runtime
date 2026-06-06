//go:build race

package table

// raceEnabled reports whether the binary was built with the race detector
// (`go test -race`). Under the race detector, wall-clock measurements reflect
// heavily instrumented execution rather than real performance, so timing
// budgets are scaled up accordingly. The deterministic bounded-work invariants
// are unaffected and stay strict.
const raceEnabled = true
