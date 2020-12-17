package diffmatchpatch

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// unescaper unescapes selected chars for compatibility with JavaScript's
// encodeURI.
//
// In speed critical applications this could be dropped since the receiving
// application will certainly decode these fine. Note that this function is
// case-sensitive.  Thus "%3F" would not be unescaped.  But this is ok because
// it is only called with the output of HttpUtility.UrlEncode which returns
// lowercase hex. Example: "%3f" -> "?", "%24" -> "$", etc.
var unescaper = strings.NewReplacer(
	"%21", "!", "%7E", "~", "%27", "'",
	"%28", "(", "%29", ")", "%3B", ";",
	"%2F", "/", "%3F", "?", "%3A", ":",
	"%40", "@", "%26", "&", "%3D", "=",
	"%2B", "+", "%24", "$", "%2C", ",",
	"%23", "#", "%2A", "*",
)

// indexOf returns the first index of pattern in s, starting at s[i].
func indexOf(s string, pattern string, i int) int {
	if i > len(s)-1 {
		return -1
	}
	if i <= 0 {
		return strings.Index(s, pattern)
	}
	ind := strings.Index(s[i:], pattern)
	if ind == -1 {
		return -1
	}
	return ind + i
}

// lastIndexOf returns the last index of pattern in s, starting at s[i].
func lastIndexOf(s string, pattern string, i int) int {
	if i < 0 {
		return -1
	}
	if i >= len(s) {
		return strings.LastIndex(s, pattern)
	}
	_, size := utf8.DecodeRuneInString(s[i:])
	return strings.LastIndex(s[:i+size], pattern)
}

// runesIndexOf returns the index of pattern in target, starting at target[i].
func runesIndexOf(target, pattern []rune, i int) int {
	if i > len(target)-1 {
		return -1
	}
	if i <= 0 {
		return runesIndex(target, pattern)
	}
	ind := runesIndex(target[i:], pattern)
	if ind == -1 {
		return -1
	}
	return ind + i
}

func runesEqual(r1, r2 []rune) bool {
	if len(r1) != len(r2) {
		return false
	}
	for i, c := range r1 {
		if c != r2[i] {
			return false
		}
	}
	return true
}

// runesIndex is the equivalent of strings.Index for rune slices.
func runesIndex(r1, r2 []rune) int {
	last := len(r1) - len(r2)
	for i := 0; i <= last; i++ {
		if runesEqual(r1[i:i+len(r2)], r2) {
			return i
		}
	}
	return -1
}

func intArrayToString(ns []uint32) string {
	if len(ns) == 0 {
		return ""
	}
	// Appr. 3 chars per num plus the comma.
	b := []byte{}
	for _, n := range ns {
		b = strconv.AppendInt(b, int64(n), 10)
		b = append(b, ',')
	}
	b = b[:len(b)-1]
	return string(b)
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// splice removes amount elements from slice at index index, replacing them
// with elements.
func splice(slice []Diff, index int, amount int, elements ...Diff) []Diff {
	if len(elements) == amount {
		// Easy case: overwrite the relevant items.
		copy(slice[index:], elements)
		return slice
	}
	if len(elements) < amount {
		// Fewer new items than old.
		// Copy in the new items.
		copy(slice[index:], elements)
		// Shift the remaining items left.
		copy(slice[index+len(elements):], slice[index+amount:])
		// Calculate the new end of the slice.
		end := len(slice) - amount + len(elements)
		// Zero stranded elements at end so that they can be garbage collected.
		tail := slice[end:]
		for i := range tail {
			tail[i] = Diff{}
		}
		return slice[:end]
	}
	// More new items than old.
	// Make room in slice for new elements.
	// There's probably an even more efficient way to do this,
	// but this is simple and clear.
	need := len(slice) - amount + len(elements)
	for len(slice) < need {
		slice = append(slice, Diff{})
	}
	// Shift slice elements right to make room for new elements.
	copy(slice[index+len(elements):], slice[index+amount:])
	// Copy in new elements.
	copy(slice[index:], elements)
	return slice
}

// commonPrefixLength returns the length of the common prefix of two rune
// slices.
func commonPrefixLength(text1, text2 []rune) int {
	// Linear search. See comment in commonSuffixLength.
	n := 0
	for ; n < len(text1) && n < len(text2); n++ {
		if text1[n] != text2[n] {
			return n
		}
	}
	return n
}

// commonSuffixLength returns the length of the common suffix of two rune slices.
func commonSuffixLength(text1, text2 []rune) int {
	// Use linear search rather than the binary search discussed at
	// https://neil.fraser.name/news/2007/10/09/.  See discussion at
	// https://github.com/sergi/go-diff/issues/54.
	i1, i2 := len(text1), len(text2)
	for n := 0; ; n++ {
		i1--
		i2--
		if i1 < 0 || i2 < 0 || text1[i1] != text2[i2] {
			return n
		}
	}
}
