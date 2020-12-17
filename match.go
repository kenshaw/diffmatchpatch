package diffmatchpatch

import (
	"math"
)

// Match locates the best instance of 'pattern' in 'text' near 'loc'. Returns
// -1 if no match found.
func (config *Config) Match(text, pattern string, loc int) int {
	// Check for null inputs not needed since null can't be passed in C#.
	loc = max(0, min(loc, len(text)))
	if text == pattern {
		// Shortcut (potentially not guaranteed by the algorithm)
		return 0
	} else if len(text) == 0 {
		// Nothing to match.
		return -1
	} else if loc+len(pattern) <= len(text) && text[loc:loc+len(pattern)] == pattern {
		// Perfect match at the perfect spot!  (Includes case of null pattern)
		return loc
	}
	// Do a fuzzy compare.
	return config.MatchBitap(text, pattern, loc)
}

// MatchBitap locates the best instance of 'pattern' in 'text' near 'loc' using
// the Bitap algorithm.  Returns -1 if no match was found.
func (config *Config) MatchBitap(text, pattern string, loc int) int {
	// Initialise the alphabet.
	s := config.MatchAlphabet(pattern)
	// Highest score beyond which we give up.
	scoreThreshold := config.MatchThreshold
	// Is there a nearby exact match? (speedup)
	bestLoc := indexOf(text, pattern, loc)
	if bestLoc != -1 {
		scoreThreshold = math.Min(config.matchBitapScore(0, bestLoc, loc, pattern), scoreThreshold)
		// What about in the other direction? (speedup)
		bestLoc = lastIndexOf(text, pattern, loc+len(pattern))
		if bestLoc != -1 {
			scoreThreshold = math.Min(config.matchBitapScore(0, bestLoc, loc, pattern), scoreThreshold)
		}
	}
	// Initialise the bit arrays.
	matchmask := 1 << uint((len(pattern) - 1))
	bestLoc = -1
	var binMin, binMid int
	binMax := len(pattern) + len(text)
	lastRd := []int{}
	for d := 0; d < len(pattern); d++ {
		// Scan for the best match; each iteration allows for one more error.
		// Run a binary search to determine how far from 'loc' we can stray at
		// this error level.
		binMin = 0
		binMid = binMax
		for binMin < binMid {
			if config.matchBitapScore(d, loc+binMid, loc, pattern) <= scoreThreshold {
				binMin = binMid
			} else {
				binMax = binMid
			}
			binMid = (binMax-binMin)/2 + binMin
		}
		// Use the result from this iteration as the maximum for the next.
		binMax = binMid
		start := max(1, loc-binMid+1)
		finish := min(loc+binMid, len(text)) + len(pattern)
		rd := make([]int, finish+2)
		rd[finish+1] = (1 << uint(d)) - 1
		for j := finish; j >= start; j-- {
			var charMatch int
			if len(text) <= j-1 {
				// Out of range.
				charMatch = 0
			} else if _, ok := s[text[j-1]]; !ok {
				charMatch = 0
			} else {
				charMatch = s[text[j-1]]
			}
			if d == 0 {
				// First pass: exact match.
				rd[j] = ((rd[j+1] << 1) | 1) & charMatch
			} else {
				// Subsequent passes: fuzzy match.
				rd[j] = ((rd[j+1]<<1)|1)&charMatch | (((lastRd[j+1] | lastRd[j]) << 1) | 1) | lastRd[j+1]
			}
			if (rd[j] & matchmask) != 0 {
				score := config.matchBitapScore(d, j-1, loc, pattern)
				// This match will almost certainly be better than any existing
				// match.  But check anyway.
				if score <= scoreThreshold {
					// Told you so.
					scoreThreshold = score
					bestLoc = j - 1
					if bestLoc > loc {
						// When passing loc, don't exceed our current distance from loc.
						start = max(1, 2*loc-bestLoc)
					} else {
						// Already passed loc, downhill from here on in.
						break
					}
				}
			}
		}
		if config.matchBitapScore(d+1, loc, loc, pattern) > scoreThreshold {
			// No hope for a (better) match at greater error levels.
			break
		}
		lastRd = rd
	}
	return bestLoc
}

// matchBitapScore computes and returns the score for a match with e errors and x location.
func (config *Config) matchBitapScore(e, x, loc int, pattern string) float64 {
	accuracy := float64(e) / float64(len(pattern))
	proximity := math.Abs(float64(loc - x))
	if config.MatchDistance == 0 {
		// Dodge divide by zero error.
		if proximity == 0 {
			return accuracy
		}
		return 1.0
	}
	return accuracy + (proximity / float64(config.MatchDistance))
}

// MatchAlphabet initialises the alphabet for the Bitap algorithm.
func (config *Config) MatchAlphabet(pattern string) map[byte]int {
	s := map[byte]int{}
	charPattern := []byte(pattern)
	for _, c := range charPattern {
		_, ok := s[c]
		if !ok {
			s[c] = 0
		}
	}
	i := 0
	for _, c := range charPattern {
		value := s[c] | int(uint(1)<<uint((len(pattern)-i-1)))
		s[c] = value
		i++
	}
	return s
}
