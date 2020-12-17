package diffmatchpatch

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchAlphabet(t *testing.T) {
	tests := []struct {
		Pattern  string
		Expected map[byte]int
	}{
		{
			Pattern: "abc",
			Expected: map[byte]int{
				'a': 4,
				'b': 2,
				'c': 1,
			},
		},
		{
			Pattern: "abcaba",
			Expected: map[byte]int{
				'a': 37,
				'b': 18,
				'c': 8,
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.MatchAlphabet(test.Pattern)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestMatchBitap(t *testing.T) {
	tests := []struct {
		Name      string
		Text      string
		Pattern   string
		Location  int
		Distance  int
		Threshold float64
		Expected  int
	}{
		{"Exact match #1", "abcdefghijk", "fgh", 5, 100, 0.5, 5},
		{"Exact match #2", "abcdefghijk", "fgh", 0, 100, 0.5, 5},
		{"Fuzzy match #1", "abcdefghijk", "efxhi", 0, 100, 0.5, 4},
		{"Fuzzy match #2", "abcdefghijk", "cdefxyhijk", 5, 100, 0.5, 2},
		{"Fuzzy match #3", "abcdefghijk", "bxy", 1, 100, 0.5, -1},
		{"Overflow", "123456789xx0", "3456789x0", 2, 100, 0.5, 2},
		{"Before start match", "abcdef", "xxabc", 4, 100, 0.5, 0},
		{"Beyond end match", "abcdef", "defyy", 4, 100, 0.5, 3},
		{"Oversized pattern", "abcdef", "xabcdefy", 0, 100, 0.5, 0},
		{"Threshold #1", "abcdefghijk", "efxyhi", 1, 100, 0.4, 4},
		{"Threshold #2", "abcdefghijk", "efxyhi", 1, 100, 0.3, -1},
		{"Threshold #3", "abcdefghijk", "bcdef", 1, 100, 0.0, 1},
		{"Multiple select #1", "abcdexyzabcde", "abccde", 3, 100, 0.5, 0},
		{"Multiple select #2", "abcdexyzabcde", "abccde", 5, 100, 0.5, 8},
		// Strict location.
		{"Distance test #1", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, 10, 0.5, -1},
		{"Distance test #2", "abcdefghijklmnopqrstuvwxyz", "abcdxxefg", 1, 10, 0.5, 0},
		// Loose location.
		{"Distance test #3", "abcdefghijklmnopqrstuvwxyz", "abcdefg", 24, 1000, 0.5, 0},
	}
	for i, test := range tests {
		config := NewDefaultConfig()
		config.MatchDistance = test.Distance
		config.MatchThreshold = test.Threshold
		actual := config.MatchBitap(test.Text, test.Pattern, test.Location)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestMatch(t *testing.T) {
	tests := []struct {
		Name      string
		Text1     string
		Text2     string
		Location  int
		Threshold float64
		Expected  int
	}{
		{"Equality", "abcdef", "abcdef", 1000, 0.5, 0},
		{"Null text", "", "abcdef", 1, 0.5, -1},
		{"Null pattern", "abcdef", "", 3, 0.5, 3},
		{"Exact match", "abcdef", "de", 3, 0.5, 3},
		{"Beyond end match", "abcdef", "defy", 4, 0.5, 3},
		{"Oversized pattern", "abcdef", "abcdefy", 0, 0.5, 0},

		{"Complex match", "I am the very model of a modern major general.", " that berry ", 5, 0.7, 4},
	}
	for i, test := range tests {
		config := NewDefaultConfig()
		config.MatchThreshold = test.Threshold
		actual := config.Match(test.Text1, test.Text2, test.Location)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}
