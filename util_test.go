package diffmatchpatch

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunesIndexOf(t *testing.T) {
	tests := []struct {
		Pattern  string
		Start    int
		Expected int
	}{
		{"abc", 0, 0},
		{"cde", 0, 2},
		{"e", 0, 4},
		{"cdef", 0, -1},
		{"abcdef", 0, -1},
		{"abc", 2, -1},
		{"cde", 2, 2},
		{"e", 2, 4},
		{"cdef", 2, -1},
		{"abcdef", 2, -1},
		{"e", 6, -1},
	}
	for i, test := range tests {
		actual := runesIndexOf([]rune("abcde"), []rune(test.Pattern), test.Start)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		String   string
		Pattern  string
		Position int
		Expected int
	}{
		{"hi world", "world", -1, 3},
		{"hi world", "world", 0, 3},
		{"hi world", "world", 1, 3},
		{"hi world", "world", 2, 3},
		{"hi world", "world", 3, 3},
		{"hi world", "world", 4, -1},
		{"abbc", "b", -1, 1},
		{"abbc", "b", 0, 1},
		{"abbc", "b", 1, 1},
		{"abbc", "b", 2, 2},
		{"abbc", "b", 3, -1},
		{"abbc", "b", 4, -1},
		// The greek letter beta is the two-byte sequence of "\u03b2".
		{"a\u03b2\u03b2c", "\u03b2", -1, 1},
		{"a\u03b2\u03b2c", "\u03b2", 0, 1},
		{"a\u03b2\u03b2c", "\u03b2", 1, 1},
		{"a\u03b2\u03b2c", "\u03b2", 3, 3},
		{"a\u03b2\u03b2c", "\u03b2", 5, -1},
		{"a\u03b2\u03b2c", "\u03b2", 6, -1},
	}
	for i, test := range tests {
		actual := indexOf(test.String, test.Pattern, test.Position)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestLastIndexOf(t *testing.T) {
	tests := []struct {
		String   string
		Pattern  string
		Position int
		Expected int
	}{
		{"hi world", "world", -1, -1},
		{"hi world", "world", 0, -1},
		{"hi world", "world", 1, -1},
		{"hi world", "world", 2, -1},
		{"hi world", "world", 3, -1},
		{"hi world", "world", 4, -1},
		{"hi world", "world", 5, -1},
		{"hi world", "world", 6, -1},
		{"hi world", "world", 7, 3},
		{"hi world", "world", 8, 3},
		{"abbc", "b", -1, -1},
		{"abbc", "b", 0, -1},
		{"abbc", "b", 1, 1},
		{"abbc", "b", 2, 2},
		{"abbc", "b", 3, 2},
		{"abbc", "b", 4, 2},
		// The greek letter beta is the two-byte sequence of "\u03b2".
		{"a\u03b2\u03b2c", "\u03b2", -1, -1},
		{"a\u03b2\u03b2c", "\u03b2", 0, -1},
		{"a\u03b2\u03b2c", "\u03b2", 1, 1},
		{"a\u03b2\u03b2c", "\u03b2", 3, 3},
		{"a\u03b2\u03b2c", "\u03b2", 5, 3},
		{"a\u03b2\u03b2c", "\u03b2", 6, 3},
	}
	for i, test := range tests {
		actual := lastIndexOf(test.String, test.Pattern, test.Position)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func speedtestTexts() (s1 string, s2 string) {
	d1, err := ioutil.ReadFile(filepath.Join("testdata", "speedtest1.txt"))
	if err != nil {
		panic(err)
	}
	d2, err := ioutil.ReadFile(filepath.Join("testdata", "speedtest2.txt"))
	if err != nil {
		panic(err)
	}
	return string(d1), string(d2)
}
