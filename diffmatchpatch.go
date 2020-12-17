// Package diffmatchpatch offers robust algorithms to perform the operations
// required for synchronizing plain text.
package diffmatchpatch

import (
	"time"
)

// Config is the configuration for diff-match-patch operations.
type Config struct {
	// DiffTimeout is the number of seconds to map a diff before giving up (0
	// for infinity).
	DiffTimeout time.Duration
	// Cost of an empty edit operation in terms of edit characters.
	DiffEditCost int

	// How far to search for a match (0 = exact location, 1000+ = broad match).
	// A match this many characters away from the expected location will add
	// 1.0 to the score (0.0 is a perfect match).
	MatchDistance int
	// The number of bits in an int.
	MatchMaxBits int
	// At what point is no match declared (0.0 = perfection, 1.0 = very loose).
	MatchThreshold float64

	// When deleting a large block of text (over ~64 characters), how close do
	// the contents have to be to match the expected contents. (0.0 =
	// perfection, 1.0 = very loose).  Note that MatchThreshold controls how
	// closely the end points of a delete need to match.
	PatchDeleteThreshold float64
	// Chunk size for context length.
	PatchMargin int
}

// NewDefaultConfig creates a new configuration with default parameters.
func NewDefaultConfig() *Config {
	return &Config{
		DiffTimeout:          time.Second,
		DiffEditCost:         4,
		MatchThreshold:       0.5,
		MatchDistance:        1000,
		MatchMaxBits:         32,
		PatchDeleteThreshold: 0.5,
		PatchMargin:          4,
	}
}
