//go:build !race

package runtime

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = false
