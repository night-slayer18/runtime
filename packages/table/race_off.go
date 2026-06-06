//go:build !race

package table

// raceEnabled reports whether the binary was built with the race detector
// (`go test -race`). When the race detector is off this is false, so timing
// budgets stay at their strict design targets.
const raceEnabled = false
