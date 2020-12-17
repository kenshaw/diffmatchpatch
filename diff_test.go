package diffmatchpatch

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func diffRebuildTexts(diffs []Diff) []string {
	texts := []string{"", ""}
	for _, d := range diffs {
		if d.Op != OpInsert {
			texts[0] += d.Text
		}
		if d.Op != OpDelete {
			texts[1] += d.Text
		}
	}
	return texts
}

func TestDiffCommonPrefix(t *testing.T) {
	tests := []struct {
		Name     string
		Text1    string
		Text2    string
		Expected int
	}{
		{"Null", "abc", "xyz", 0},
		{"Non-null", "1234abcdef", "1234xyz", 4},
		{"Whole", "1234", "1234xyz", 4},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCommonPrefix(test.Text1, test.Text2)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestCommonPrefixLength(t *testing.T) {
	tests := []struct {
		Text1    string
		Text2    string
		Expected int
	}{
		{"abc", "xyz", 0},
		{"1234abcdef", "1234xyz", 4},
		{"1234", "1234xyz", 4},
	}
	for i, test := range tests {
		actual := commonPrefixLength([]rune(test.Text1), []rune(test.Text2))
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffCommonSuffix(t *testing.T) {
	tests := []struct {
		Name     string
		Text1    string
		Text2    string
		Expected int
	}{
		{"Null", "abc", "xyz", 0},
		{"Non-null", "abcdef1234", "xyz1234", 4},
		{"Whole", "1234", "xyz1234", 4},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCommonSuffix(test.Text1, test.Text2)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

var SinkInt int // exported sink var to avoid compiler optimizations in benchmarks

func TestCommonSuffixLength(t *testing.T) {
	tests := []struct {
		Text1    string
		Text2    string
		Expected int
	}{
		{"abc", "xyz", 0},
		{"abcdef1234", "xyz1234", 4},
		{"1234", "xyz1234", 4},
		{"123", "a3", 1},
	}
	for i, test := range tests {
		actual := commonSuffixLength([]rune(test.Text1), []rune(test.Text2))
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffCommonOverlap(t *testing.T) {
	tests := []struct {
		Name     string
		Text1    string
		Text2    string
		Expected int
	}{
		{"Null", "", "abcd", 0},
		{"Whole", "abc", "abcd", 3},
		{"Null", "123456", "abcd", 0},
		{"Null", "123456xxx", "xxxabcd", 3},
		// Some overly clever languages (C#) may treat ligatures as equal to
		// their component letters, e.g. U+FB01 == 'fi'
		{"Unicode", "fi", "\ufb01i", 0},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCommonOverlap(test.Text1, test.Text2)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffHalfMatch(t *testing.T) {
	tests := []struct {
		Text1    string
		Text2    string
		Timeout  time.Duration
		Expected []string
	}{
		// No match
		{
			"1234567890",
			"abcdef",
			1,
			nil,
		},
		{
			"12345",
			"23",
			1,
			nil,
		},
		// Single Match
		{
			"1234567890",
			"a345678z",
			1,
			[]string{"12", "90", "a", "z", "345678"},
		},
		{
			"a345678z",
			"1234567890",
			1,
			[]string{"a", "z", "12", "90", "345678"},
		},
		{
			"abc56789z",
			"1234567890",
			1,
			[]string{"abc", "z", "1234", "0", "56789"},
		},
		{
			"a23456xyz",
			"1234567890",
			1,
			[]string{"a", "xyz", "1", "7890", "23456"},
		},
		// Multiple Matches
		{
			"121231234123451234123121",
			"a1234123451234z",
			1,
			[]string{"12123", "123121", "a", "z", "1234123451234"},
		},
		{
			"x-=-=-=-=-=-=-=-=-=-=-=-=",
			"xx-=-=-=-=-=-=-=",
			1,
			[]string{"", "-=-=-=-=-=", "x", "", "x-=-=-=-=-=-=-="},
		},
		{
			"-=-=-=-=-=-=-=-=-=-=-=-=y",
			"-=-=-=-=-=-=-=yy",
			1,
			[]string{"-=-=-=-=-=", "", "", "y", "-=-=-=-=-=-=-=y"},
		},
		// Non-optimal halfmatch, ptimal diff would be -q+x=H-i+e=lloHe+Hu=llo-Hew+y not -qHillo+x=HelloHe-w+Hulloy
		{
			"qHilloHelloHew",
			"xHelloHeHulloy",
			1,
			[]string{"qHillo", "w", "x", "Hulloy", "HelloHe"},
		},
		// Optimal no halfmatch
		{
			"qHilloHelloHew",
			"xHelloHeHulloy",
			0,
			nil,
		},
	}
	for i, test := range tests {
		config := NewDefaultConfig()
		config.DiffTimeout = test.Timeout
		actual := config.DiffHalfMatch(test.Text1, test.Text2)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffBisectSplit(t *testing.T) {
	tests := []struct {
		Text1 string
		Text2 string
	}{
		{"STUV\x05WX\x05YZ\x05[", "WĺĻļ\x05YZ\x05ĽľĿŀZ"},
	}
	config := NewDefaultConfig()
	for _, test := range tests {
		diffs := config.diffBisectSplit([]rune(test.Text1),
			[]rune(test.Text2), 7, 6, time.Now().Add(time.Hour))
		for _, d := range diffs {
			assert.True(t, utf8.ValidString(d.Text))
		}
		// TODO define the expected outcome
	}
}

func TestDiffLinesToChars(t *testing.T) {
	tests := []struct {
		Text1          string
		Text2          string
		ExpectedChars1 string
		ExpectedChars2 string
		ExpectedLines  []string
	}{
		{
			"",
			"alpha\r\nbeta\r\n\r\n\r\n",
			"",
			"1,2,3,3",
			[]string{"", "alpha\r\n", "beta\r\n", "\r\n"},
		},
		{
			"a",
			"b",
			"1",
			"2",
			[]string{"", "a", "b"},
		},
		// Omit final newline.
		{
			"alpha\nbeta\nalpha",
			"",
			"1,2,3",
			"",
			[]string{"", "alpha\n", "beta\n", "alpha"},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actualChars1, actualChars2, actualLines := config.DiffLinesToChars(test.Text1, test.Text2)
		assert.Equal(t, test.ExpectedChars1, actualChars1, fmt.Sprintf("Test case #%d, %#v", i, test))
		assert.Equal(t, test.ExpectedChars2, actualChars2, fmt.Sprintf("Test case #%d, %#v", i, test))
		assert.Equal(t, test.ExpectedLines, actualLines, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
	// More than 256 to reveal any 8-bit limitations.
	n := 300
	lineList := []string{
		"", // Account for the initial empty element of the lines array.
	}
	var charList []string
	for x := 1; x < n+1; x++ {
		lineList = append(lineList, strconv.Itoa(x)+"\n")
		charList = append(charList, strconv.Itoa(x))
	}
	lines := strings.Join(lineList, "")
	chars := strings.Join(charList[:], ",")
	assert.Equal(t, n, len(strings.Split(chars, ",")))
	actualChars1, actualChars2, actualLines := config.DiffLinesToChars(lines, "")
	assert.Equal(t, chars, actualChars1)
	assert.Equal(t, "", actualChars2)
	assert.Equal(t, lineList, actualLines)
}

func TestDiffCharsToLines(t *testing.T) {
	tests := []struct {
		Diffs    []Diff
		Lines    []string
		Expected []Diff
	}{
		{
			Diffs: []Diff{
				{OpEqual, "1,2,1"},
				{OpInsert, "2,1,2"},
			},
			Lines: []string{"", "alpha\n", "beta\n"},
			Expected: []Diff{
				{OpEqual, "alpha\nbeta\nalpha\n"},
				{OpInsert, "beta\nalpha\nbeta\n"},
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCharsToLines(test.Diffs, test.Lines)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
	// More than 256 to reveal any 8-bit limitations.
	n := 300
	lineList := []string{
		"", // Account for the initial empty element of the lines array.
	}
	charList := []string{}
	for x := 1; x <= n; x++ {
		lineList = append(lineList, strconv.Itoa(x)+"\n")
		charList = append(charList, strconv.Itoa(x))
	}
	assert.Equal(t, n, len(charList))
	chars := strings.Join(charList[:], ",")
	actual := config.DiffCharsToLines([]Diff{Diff{OpDelete, chars}}, lineList)
	assert.Equal(t, []Diff{Diff{OpDelete, strings.Join(lineList, "")}}, actual)
}

func TestDiffCleanupMerge(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		Expected []Diff
	}{
		{
			"Null case",
			[]Diff{},
			[]Diff{},
		},
		{
			"No Diff case",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "b"},
				Diff{OpInsert, "c"},
			},
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "b"},
				Diff{OpInsert, "c"},
			},
		},
		{
			"Merge equalities",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpEqual, "b"},
				Diff{OpEqual, "c"},
			},
			[]Diff{
				Diff{OpEqual, "abc"},
			},
		},
		{
			"Merge deletions",
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpDelete, "b"},
				Diff{OpDelete, "c"},
			},
			[]Diff{
				Diff{OpDelete, "abc"},
			},
		},
		{
			"Merge insertions",
			[]Diff{
				Diff{OpInsert, "a"},
				Diff{OpInsert, "b"},
				Diff{OpInsert, "c"},
			},
			[]Diff{
				Diff{OpInsert, "abc"},
			},
		},
		{
			"Merge interweave",
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpInsert, "b"},
				Diff{OpDelete, "c"},
				Diff{OpInsert, "d"},
				Diff{OpEqual, "e"},
				Diff{OpEqual, "f"},
			},
			[]Diff{
				Diff{OpDelete, "ac"},
				Diff{OpInsert, "bd"},
				Diff{OpEqual, "ef"},
			},
		},
		{
			"Prefix and suffix detection",
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpInsert, "abc"},
				Diff{OpDelete, "dc"},
			},
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "d"},
				Diff{OpInsert, "b"},
				Diff{OpEqual, "c"},
			},
		},
		{
			"Prefix and suffix detection with equalities",
			[]Diff{
				Diff{OpEqual, "x"},
				Diff{OpDelete, "a"},
				Diff{OpInsert, "abc"},
				Diff{OpDelete, "dc"},
				Diff{OpEqual, "y"},
			},
			[]Diff{
				Diff{OpEqual, "xa"},
				Diff{OpDelete, "d"},
				Diff{OpInsert, "b"},
				Diff{OpEqual, "cy"},
			},
		},
		{
			"Same test as above but with unicode (\u0101 will appear in diffs with at least 257 unique lines)",
			[]Diff{
				Diff{OpEqual, "x"},
				Diff{OpDelete, "\u0101"},
				Diff{OpInsert, "\u0101bc"},
				Diff{OpDelete, "dc"},
				Diff{OpEqual, "y"},
			},
			[]Diff{
				Diff{OpEqual, "x\u0101"},
				Diff{OpDelete, "d"},
				Diff{OpInsert, "b"},
				Diff{OpEqual, "cy"},
			},
		},
		{
			"Slide edit left",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpInsert, "ba"},
				Diff{OpEqual, "c"},
			},
			[]Diff{
				Diff{OpInsert, "ab"},
				Diff{OpEqual, "ac"},
			},
		},
		{
			"Slide edit right",
			[]Diff{
				Diff{OpEqual, "c"},
				Diff{OpInsert, "ab"},
				Diff{OpEqual, "a"},
			},
			[]Diff{
				Diff{OpEqual, "ca"},
				Diff{OpInsert, "ba"},
			},
		},
		{
			"Slide edit left recursive",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "b"},
				Diff{OpEqual, "c"},
				Diff{OpDelete, "ac"},
				Diff{OpEqual, "x"},
			},
			[]Diff{
				Diff{OpDelete, "abc"},
				Diff{OpEqual, "acx"},
			},
		},
		{
			"Slide edit right recursive",
			[]Diff{
				Diff{OpEqual, "x"},
				Diff{OpDelete, "ca"},
				Diff{OpEqual, "c"},
				Diff{OpDelete, "b"},
				Diff{OpEqual, "a"},
			},
			[]Diff{
				Diff{OpEqual, "xca"},
				Diff{OpDelete, "cba"},
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCleanupMerge(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffCleanupSemanticLossless(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		Expected []Diff
	}{
		{
			"Null case",
			[]Diff{},
			[]Diff{},
		},
		{
			"Blank lines",
			[]Diff{
				Diff{OpEqual, "AAA\r\n\r\nBBB"},
				Diff{OpInsert, "\r\nDDD\r\n\r\nBBB"},
				Diff{OpEqual, "\r\nEEE"},
			},
			[]Diff{
				Diff{OpEqual, "AAA\r\n\r\n"},
				Diff{OpInsert, "BBB\r\nDDD\r\n\r\n"},
				Diff{OpEqual, "BBB\r\nEEE"},
			},
		},
		{
			"Line boundaries",
			[]Diff{
				Diff{OpEqual, "AAA\r\nBBB"},
				Diff{OpInsert, " DDD\r\nBBB"},
				Diff{OpEqual, " EEE"},
			},
			[]Diff{
				Diff{OpEqual, "AAA\r\n"},
				Diff{OpInsert, "BBB DDD\r\n"},
				Diff{OpEqual, "BBB EEE"},
			},
		},
		{
			"Word boundaries",
			[]Diff{
				Diff{OpEqual, "The c"},
				Diff{OpInsert, "ow and the c"},
				Diff{OpEqual, "at."},
			},
			[]Diff{
				Diff{OpEqual, "The "},
				Diff{OpInsert, "cow and the "},
				Diff{OpEqual, "cat."},
			},
		},
		{
			"Alphanumeric boundaries",
			[]Diff{
				Diff{OpEqual, "The-c"},
				Diff{OpInsert, "ow-and-the-c"},
				Diff{OpEqual, "at."},
			},
			[]Diff{
				Diff{OpEqual, "The-"},
				Diff{OpInsert, "cow-and-the-"},
				Diff{OpEqual, "cat."},
			},
		},
		{
			"Hitting the start",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "a"},
				Diff{OpEqual, "ax"},
			},
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpEqual, "aax"},
			},
		},
		{
			"Hitting the end",
			[]Diff{
				Diff{OpEqual, "xa"},
				Diff{OpDelete, "a"},
				Diff{OpEqual, "a"},
			},
			[]Diff{
				Diff{OpEqual, "xaa"},
				Diff{OpDelete, "a"},
			},
		},
		{
			"Sentence boundaries",
			[]Diff{
				Diff{OpEqual, "The xxx. The "},
				Diff{OpInsert, "zzz. The "},
				Diff{OpEqual, "yyy."},
			},
			[]Diff{
				Diff{OpEqual, "The xxx."},
				Diff{OpInsert, " The zzz."},
				Diff{OpEqual, " The yyy."},
			},
		},
		{
			"UTF-8 strings",
			[]Diff{
				Diff{OpEqual, "The ♕. The "},
				Diff{OpInsert, "♔. The "},
				Diff{OpEqual, "♖."},
			},
			[]Diff{
				Diff{OpEqual, "The ♕."},
				Diff{OpInsert, " The ♔."},
				Diff{OpEqual, " The ♖."},
			},
		},
		{
			"Rune boundaries",
			[]Diff{
				Diff{OpEqual, "♕♕"},
				Diff{OpInsert, "♔♔"},
				Diff{OpEqual, "♖♖"},
			},
			[]Diff{
				Diff{OpEqual, "♕♕"},
				Diff{OpInsert, "♔♔"},
				Diff{OpEqual, "♖♖"},
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCleanupSemanticLossless(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffCleanupSemantic(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		Expected []Diff
	}{
		{
			"Null case",
			[]Diff{},
			[]Diff{},
		},
		{
			"No elimination #1",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "cd"},
				Diff{OpEqual, "12"},
				Diff{OpDelete, "e"},
			},
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "cd"},
				Diff{OpEqual, "12"},
				Diff{OpDelete, "e"},
			},
		},
		{
			"No elimination #2",
			[]Diff{
				Diff{OpDelete, "abc"},
				Diff{OpInsert, "ABC"},
				Diff{OpEqual, "1234"},
				Diff{OpDelete, "wxyz"},
			},
			[]Diff{
				Diff{OpDelete, "abc"},
				Diff{OpInsert, "ABC"},
				Diff{OpEqual, "1234"},
				Diff{OpDelete, "wxyz"},
			},
		},
		{
			"No elimination #3",
			[]Diff{
				Diff{OpEqual, "2016-09-01T03:07:1"},
				Diff{OpInsert, "5.15"},
				Diff{OpEqual, "4"},
				Diff{OpDelete, "."},
				Diff{OpEqual, "80"},
				Diff{OpInsert, "0"},
				Diff{OpEqual, "78"},
				Diff{OpDelete, "3074"},
				Diff{OpEqual, "1Z"},
			},
			[]Diff{
				Diff{OpEqual, "2016-09-01T03:07:1"},
				Diff{OpInsert, "5.15"},
				Diff{OpEqual, "4"},
				Diff{OpDelete, "."},
				Diff{OpEqual, "80"},
				Diff{OpInsert, "0"},
				Diff{OpEqual, "78"},
				Diff{OpDelete, "3074"},
				Diff{OpEqual, "1Z"},
			},
		},
		{
			"Simple elimination",
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpEqual, "b"},
				Diff{OpDelete, "c"},
			},
			[]Diff{
				Diff{OpDelete, "abc"},
				Diff{OpInsert, "b"},
			},
		},
		{
			"Backpass elimination",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpEqual, "cd"},
				Diff{OpDelete, "e"},
				Diff{OpEqual, "f"},
				Diff{OpInsert, "g"},
			},
			[]Diff{
				Diff{OpDelete, "abcdef"},
				Diff{OpInsert, "cdfg"},
			},
		},
		{
			"Multiple eliminations",
			[]Diff{
				Diff{OpInsert, "1"},
				Diff{OpEqual, "A"},
				Diff{OpDelete, "B"},
				Diff{OpInsert, "2"},
				Diff{OpEqual, "_"},
				Diff{OpInsert, "1"},
				Diff{OpEqual, "A"},
				Diff{OpDelete, "B"},
				Diff{OpInsert, "2"},
			},
			[]Diff{
				Diff{OpDelete, "AB_AB"},
				Diff{OpInsert, "1A2_1A2"},
			},
		},
		{
			"Word boundaries",
			[]Diff{
				Diff{OpEqual, "The c"},
				Diff{OpDelete, "ow and the c"},
				Diff{OpEqual, "at."},
			},
			[]Diff{
				Diff{OpEqual, "The "},
				Diff{OpDelete, "cow and the "},
				Diff{OpEqual, "cat."},
			},
		},
		{
			"No overlap elimination",
			[]Diff{
				Diff{OpDelete, "abcxx"},
				Diff{OpInsert, "xxdef"},
			},
			[]Diff{
				{OpDelete, "abcxx"},
				{OpInsert, "xxdef"},
			},
		},
		{
			"Overlap elimination",
			[]Diff{
				{OpDelete, "abcxxx"},
				{OpInsert, "xxxdef"},
			},
			[]Diff{
				{OpDelete, "abc"},
				{OpEqual, "xxx"},
				{OpInsert, "def"},
			},
		},
		{
			"Reverse overlap elimination",
			[]Diff{
				Diff{OpDelete, "xxxabc"},
				Diff{OpInsert, "defxxx"},
			},
			[]Diff{
				Diff{OpInsert, "def"},
				Diff{OpEqual, "xxx"},
				Diff{OpDelete, "abc"},
			},
		},
		{
			"Two overlap eliminations",
			[]Diff{
				Diff{OpDelete, "abcd1212"},
				Diff{OpInsert, "1212efghi"},
				Diff{OpEqual, "----"},
				Diff{OpDelete, "A3"},
				Diff{OpInsert, "3BC"},
			},
			[]Diff{
				Diff{OpDelete, "abcd"},
				Diff{OpEqual, "1212"},
				Diff{OpInsert, "efghi"},
				Diff{OpEqual, "----"},
				Diff{OpDelete, "A"},
				Diff{OpEqual, "3"},
				Diff{OpInsert, "BC"},
			},
		},
		{
			"Test case for adapting DiffCleanupSemantic to be equal to the Python version #19",
			[]Diff{
				Diff{OpEqual, "James McCarthy "},
				Diff{OpDelete, "close to "},
				Diff{OpEqual, "sign"},
				Diff{OpDelete, "ing"},
				Diff{OpInsert, "s"},
				Diff{OpEqual, " new "},
				Diff{OpDelete, "E"},
				Diff{OpInsert, "fi"},
				Diff{OpEqual, "ve"},
				Diff{OpInsert, "-yea"},
				Diff{OpEqual, "r"},
				Diff{OpDelete, "ton"},
				Diff{OpEqual, " deal"},
				Diff{OpInsert, " at Everton"},
			},
			[]Diff{
				Diff{OpEqual, "James McCarthy "},
				Diff{OpDelete, "close to "},
				Diff{OpEqual, "sign"},
				Diff{OpDelete, "ing"},
				Diff{OpInsert, "s"},
				Diff{OpEqual, " new "},
				Diff{OpInsert, "five-year deal at "},
				Diff{OpEqual, "Everton"},
				Diff{OpDelete, " deal"},
			},
		},
		{
			"Taken from python / CPP library",
			[]Diff{
				Diff{OpInsert, "星球大戰：新的希望 "},
				Diff{OpEqual, "star wars: "},
				Diff{OpDelete, "episodio iv - un"},
				Diff{OpEqual, "a n"},
				Diff{OpDelete, "u"},
				Diff{OpEqual, "e"},
				Diff{OpDelete, "va"},
				Diff{OpInsert, "w"},
				Diff{OpEqual, " "},
				Diff{OpDelete, "es"},
				Diff{OpInsert, "ho"},
				Diff{OpEqual, "pe"},
				Diff{OpDelete, "ranza"},
			},
			[]Diff{
				Diff{OpInsert, "星球大戰：新的希望 "},
				Diff{OpEqual, "star wars: "},
				Diff{OpDelete, "episodio iv - una nueva esperanza"},
				Diff{OpInsert, "a new hope"},
			},
		},
		{
			"panic",
			[]Diff{
				Diff{OpInsert, "킬러 인 "},
				Diff{OpEqual, "리커버리"},
				Diff{OpDelete, " 보이즈"},
			},
			[]Diff{
				Diff{OpInsert, "킬러 인 "},
				Diff{OpEqual, "리커버리"},
				Diff{OpDelete, " 보이즈"},
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffCleanupSemantic(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffCleanupEfficiency(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		EditCost int
		Expected []Diff
	}{
		{
			"Null case",
			[]Diff{},
			4,
			[]Diff{},
		},
		{
			"No elimination",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "12"},
				Diff{OpEqual, "wxyz"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "34"},
			},
			4,
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "12"},
				Diff{OpEqual, "wxyz"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "34"},
			},
		},
		{
			"Four-edit elimination",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "12"},
				Diff{OpEqual, "xyz"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "34"},
			},
			4,
			[]Diff{
				Diff{OpDelete, "abxyzcd"},
				Diff{OpInsert, "12xyz34"},
			},
		},
		{
			"Three-edit elimination",
			[]Diff{
				Diff{OpInsert, "12"},
				Diff{OpEqual, "x"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "34"},
			},
			4,
			[]Diff{
				Diff{OpDelete, "xcd"},
				Diff{OpInsert, "12x34"},
			},
		},
		{
			"Backpass elimination",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "12"},
				Diff{OpEqual, "xy"},
				Diff{OpInsert, "34"},
				Diff{OpEqual, "z"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "56"},
			},
			4,
			[]Diff{
				Diff{OpDelete, "abxyzcd"},
				Diff{OpInsert, "12xy34z56"},
			},
		},
		{
			"High cost elimination",
			[]Diff{
				Diff{OpDelete, "ab"},
				Diff{OpInsert, "12"},
				Diff{OpEqual, "wxyz"},
				Diff{OpDelete, "cd"},
				Diff{OpInsert, "34"},
			},
			5,
			[]Diff{
				Diff{OpDelete, "abwxyzcd"},
				Diff{OpInsert, "12wxyz34"},
			},
		},
	}
	for i, test := range tests {
		config := NewDefaultConfig()
		config.DiffEditCost = test.EditCost
		actual := config.DiffCleanupEfficiency(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffPrettyHtml(t *testing.T) {
	tests := []struct {
		Diffs    []Diff
		Expected string
	}{
		{
			Diffs: []Diff{
				Diff{OpEqual, "a\n"},
				Diff{OpDelete, "<B>b</B>"},
				Diff{OpInsert, "c&d"},
			},
			Expected: "<span>a&para;<br></span><del style=\"background:#ffe6e6;\">&lt;B&gt;b&lt;/B&gt;</del><ins style=\"background:#e6ffe6;\">c&amp;d</ins>",
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffPrettyHtml(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffPrettyText(t *testing.T) {
	tests := []struct {
		Diffs    []Diff
		Expected string
	}{
		{
			Diffs: []Diff{
				{OpEqual, "a\n"},
				{OpDelete, "<B>b</B>"},
				{OpInsert, "c&d"},
			},
			Expected: "a\n\x1b[31m<B>b</B>\x1b[0m\x1b[32mc&d\x1b[0m",
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffPrettyText(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffText(t *testing.T) {
	tests := []struct {
		Diffs         []Diff
		ExpectedText1 string
		ExpectedText2 string
	}{
		{
			Diffs: []Diff{
				Diff{OpEqual, "jump"},
				Diff{OpDelete, "s"},
				Diff{OpInsert, "ed"},
				Diff{OpEqual, " over "},
				Diff{OpDelete, "the"},
				Diff{OpInsert, "a"},
				Diff{OpEqual, " lazy"},
			},
			ExpectedText1: "jumps over the lazy",
			ExpectedText2: "jumped over a lazy",
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actualText1 := config.DiffText1(test.Diffs)
		assert.Equal(t, test.ExpectedText1, actualText1, fmt.Sprintf("Test case #%d, %#v", i, test))
		actualText2 := config.DiffText2(test.Diffs)
		assert.Equal(t, test.ExpectedText2, actualText2, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestDiffDelta(t *testing.T) {
	tests := []struct {
		Name               string
		Text               string
		Delta              string
		ErrorMessagePrefix string
	}{
		{"Delta shorter than text", "jumps over the lazyx", "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog", "Delta length (19) is different from source text length (20)"},
		{"Delta longer than text", "umps over the lazy", "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog", "Delta length (19) is different from source text length (18)"},
		{"Invalid URL escaping", "", "+%c3%xy", "invalid URL escape \"%xy\""},
		{"Invalid UTF-8 sequence", "", "+%c3xy", "invalid UTF-8 token: \"\\xc3xy\""},
		{"Invalid diff operation", "", "a", "Invalid diff operation in DiffFromDelta: a"},
		{"Invalid diff syntax", "", "-", "strconv.ParseInt: parsing \"\": invalid syntax"},
		{"Negative number in delta", "", "--1", "Negative number in DiffFromDelta: -1"},
		{"Empty case", "", "", ""},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		diffs, err := config.DiffFromDelta(test.Text, test.Delta)
		msg := fmt.Sprintf("Test case #%d, %s", i, test.Name)
		if test.ErrorMessagePrefix == "" {
			assert.Nil(t, err, msg)
			assert.Nil(t, diffs, msg)
		} else {
			e := err.Error()
			if strings.HasPrefix(e, test.ErrorMessagePrefix) {
				e = test.ErrorMessagePrefix
			}
			assert.Nil(t, diffs, msg)
			assert.Equal(t, test.ErrorMessagePrefix, e, msg)
		}
	}
	// Convert a diff into delta string.
	diffs := []Diff{
		Diff{OpEqual, "jump"},
		Diff{OpDelete, "s"},
		Diff{OpInsert, "ed"},
		Diff{OpEqual, " over "},
		Diff{OpDelete, "the"},
		Diff{OpInsert, "a"},
		Diff{OpEqual, " lazy"},
		Diff{OpInsert, "old dog"},
	}
	text1 := config.DiffText1(diffs)
	assert.Equal(t, "jumps over the lazy", text1)
	delta := config.DiffToDelta(diffs)
	assert.Equal(t, "=4\t-1\t+ed\t=6\t-3\t+a\t=5\t+old dog", delta)
	// Convert delta string into a diff.
	deltaDiffs, err := config.DiffFromDelta(text1, delta)
	assert.NoError(t, err, "expected no error")
	assert.Equal(t, diffs, deltaDiffs)
	// Test deltas with special characters.
	diffs = []Diff{
		Diff{OpEqual, "\u0680 \x00 \t %"},
		Diff{OpDelete, "\u0681 \x01 \n ^"},
		Diff{OpInsert, "\u0682 \x02 \\ |"},
	}
	text1 = config.DiffText1(diffs)
	assert.Equal(t, "\u0680 \x00 \t %\u0681 \x01 \n ^", text1)
	// Lowercase, due to UrlEncode uses lower.
	delta = config.DiffToDelta(diffs)
	assert.Equal(t, "=7\t-7\t+%DA%82 %02 %5C %7C", delta)
	deltaDiffs, err = config.DiffFromDelta(text1, delta)
	assert.Equal(t, diffs, deltaDiffs)
	assert.Nil(t, err)
	// Verify pool of unchanged characters.
	diffs = []Diff{
		Diff{OpInsert, "A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # "},
	}
	delta = config.DiffToDelta(diffs)
	assert.Equal(t, "+A-Z a-z 0-9 - _ . ! ~ * ' ( ) ; / ? : @ & = + $ , # ", delta, "Unchanged characters.")
	// Convert delta string into a diff.
	deltaDiffs, err = config.DiffFromDelta("", delta)
	assert.Equal(t, diffs, deltaDiffs)
	assert.Nil(t, err)
}

func TestDiffXIndex(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		Location int
		Expected int
	}{
		{
			"Translation on equality",
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpInsert, "1234"},
				Diff{OpEqual, "xyz"},
			},
			2,
			5,
		},
		{
			"Translation on deletion",
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "1234"},
				Diff{OpEqual, "xyz"},
			},
			3,
			1,
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffXIndex(test.Diffs, test.Location)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffLevenshtein(t *testing.T) {
	tests := []struct {
		Name     string
		Diffs    []Diff
		Expected int
	}{
		{
			"Levenshtein with trailing equality",
			[]Diff{
				Diff{OpDelete, "абв"},
				Diff{OpInsert, "1234"},
				Diff{OpEqual, "эюя"},
			},
			4,
		},
		{
			"Levenshtein with leading equality",
			[]Diff{
				Diff{OpEqual, "эюя"},
				Diff{OpDelete, "абв"},
				Diff{OpInsert, "1234"},
			},
			4,
		},
		{
			"Levenshtein with middle equality",
			[]Diff{
				Diff{OpDelete, "абв"},
				Diff{OpEqual, "эюя"},
				Diff{OpInsert, "1234"},
			},
			7,
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffLevenshtein(test.Diffs)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
}

func TestDiffBisect(t *testing.T) {
	tests := []struct {
		Name     string
		Time     time.Time
		Expected []Diff
	}{
		{
			Name: "normal",
			Time: time.Date(9999, time.December, 31, 23, 59, 59, 59, time.UTC),
			Expected: []Diff{
				Diff{OpDelete, "c"},
				Diff{OpInsert, "m"},
				Diff{OpEqual, "a"},
				Diff{OpDelete, "t"},
				Diff{OpInsert, "p"},
			},
		},
		{
			Name: "Negative deadlines count as having infinite time",
			Time: time.Date(0o001, time.January, 0o1, 0o0, 0o0, 0o0, 0o0, time.UTC),
			Expected: []Diff{
				Diff{OpDelete, "c"},
				Diff{OpInsert, "m"},
				Diff{OpEqual, "a"},
				Diff{OpDelete, "t"},
				Diff{OpInsert, "p"},
			},
		},
		{
			Name: "Timeout",
			Time: time.Now().Add(time.Nanosecond),
			Expected: []Diff{
				Diff{OpDelete, "cat"},
				Diff{OpInsert, "map"},
			},
		},
	}
	config := NewDefaultConfig()
	for i, test := range tests {
		actual := config.DiffBisect("cat", "map", test.Time)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %s", i, test.Name))
	}
	// Test for invalid UTF-8 sequences
	assert.Equal(t, []Diff{
		Diff{OpEqual, "��"},
	}, config.DiffBisect("\xe0\xe5", "\xe0\xe5", time.Now().Add(time.Minute)))
}

func TestDiff(t *testing.T) {
	tests := []struct {
		Text1    string
		Text2    string
		Timeout  time.Duration
		Expected []Diff
	}{
		{
			"",
			"",
			time.Second,
			nil,
		},
		{
			"abc",
			"abc",
			time.Second,
			[]Diff{
				Diff{OpEqual, "abc"},
			},
		},
		{
			"abc",
			"ab123c",
			time.Second,
			[]Diff{
				Diff{OpEqual, "ab"},
				Diff{OpInsert, "123"},
				Diff{OpEqual, "c"},
			},
		},
		{
			"a123bc",
			"abc",
			time.Second,
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "123"},
				Diff{OpEqual, "bc"},
			},
		},
		{
			"abc",
			"a123b456c",
			time.Second,
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpInsert, "123"},
				Diff{OpEqual, "b"},
				Diff{OpInsert, "456"},
				Diff{OpEqual, "c"},
			},
		},
		{
			"a123b456c",
			"abc",
			time.Second,
			[]Diff{
				Diff{OpEqual, "a"},
				Diff{OpDelete, "123"},
				Diff{OpEqual, "b"},
				Diff{OpDelete, "456"},
				Diff{OpEqual, "c"},
			},
		},
		// Perform a real diff and switch off the timeout.
		{
			"a",
			"b",
			0,
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpInsert, "b"},
			},
		},
		{
			"Apples are a fruit.",
			"Bananas are also fruit.",
			0,
			[]Diff{
				Diff{OpDelete, "Apple"},
				Diff{OpInsert, "Banana"},
				Diff{OpEqual, "s are a"},
				Diff{OpInsert, "lso"},
				Diff{OpEqual, " fruit."},
			},
		},
		{
			"ax\t",
			"\u0680x\u0000",
			0,
			[]Diff{
				Diff{OpDelete, "a"},
				Diff{OpInsert, "\u0680"},
				Diff{OpEqual, "x"},
				Diff{OpDelete, "\t"},
				Diff{OpInsert, "\u0000"},
			},
		},
		{
			"1ayb2",
			"abxab",
			0,
			[]Diff{
				Diff{OpDelete, "1"},
				Diff{OpEqual, "a"},
				Diff{OpDelete, "y"},
				Diff{OpEqual, "b"},
				Diff{OpDelete, "2"},
				Diff{OpInsert, "xab"},
			},
		},
		{
			"abcy",
			"xaxcxabc",
			0,
			[]Diff{
				Diff{OpInsert, "xaxcx"},
				Diff{OpEqual, "abc"},
				Diff{OpDelete, "y"},
			},
		},
		{
			"ABCDa=bcd=efghijklmnopqrsEFGHIJKLMNOefg",
			"a-bcd-efghijklmnopqrs",
			0,
			[]Diff{
				Diff{OpDelete, "ABCD"},
				Diff{OpEqual, "a"},
				Diff{OpDelete, "="},
				Diff{OpInsert, "-"},
				Diff{OpEqual, "bcd"},
				Diff{OpDelete, "="},
				Diff{OpInsert, "-"},
				Diff{OpEqual, "efghijklmnopqrs"},
				Diff{OpDelete, "EFGHIJKLMNOefg"},
			},
		},
		{
			"a [[Pennsylvania]] and [[New",
			" and [[Pennsylvania]]",
			0,
			[]Diff{
				Diff{OpInsert, " "},
				Diff{OpEqual, "a"},
				Diff{OpInsert, "nd"},
				Diff{OpEqual, " [[Pennsylvania]]"},
				Diff{OpDelete, " and [[New"},
			},
		},
	}
	// Perform a trivial diff.
	for i, test := range tests {
		config := NewDefaultConfig()
		config.DiffTimeout = test.Timeout
		actual := config.Diff(test.Text1, test.Text2, false)
		assert.Equal(t, test.Expected, actual, fmt.Sprintf("Test case #%d, %#v", i, test))
	}
	// Test for invalid UTF-8 sequences
	config := NewDefaultConfig()
	config.DiffTimeout = 0
	assert.Equal(t, []Diff{Diff{OpDelete, "��"}}, config.Diff("\xe0\xe5", "", false))
}

func TestDiffWithTimeout(t *testing.T) {
	config := NewDefaultConfig()
	config.DiffTimeout = 200 * time.Millisecond
	a := "`Twas brillig, and the slithy toves\nDid gyre and gimble in the wabe:\nAll mimsy were the borogoves,\nAnd the mome raths outgrabe.\n"
	b := "I am the very model of a modern major general,\nI've information vegetable, animal, and mineral,\nI know the kings of England, and I quote the fights historical,\nFrom Marathon to Waterloo, in order categorical.\n"
	// Increase the text lengths by 1024 times to ensure a timeout.
	for x := 0; x < 13; x++ {
		a = a + a
		b = b + b
	}
	startTime := time.Now()
	config.Diff(a, b, true)
	endTime := time.Now()
	delta := endTime.Sub(startTime)
	// Test that we took at least the timeout period.
	assert.True(t, delta >= config.DiffTimeout, fmt.Sprintf("%v !>= %v", delta, config.DiffTimeout))
	// Test that we didn't take forever (be very forgiving). Theoretically this test could fail very occasionally if the OS task swaps or locks up for a second at the wrong moment.
	assert.True(t, delta < (config.DiffTimeout*100), fmt.Sprintf("%v !< %v", delta, config.DiffTimeout*100))
}

func TestDiffWithCheckLines(t *testing.T) {
	tests := []struct {
		Text1 string
		Text2 string
	}{
		{
			"1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n",
			"abcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\nabcdefghij\n",
		},
		{
			"1234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890",
			"abcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghijabcdefghij",
		},
		{
			"1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n1234567890\n",
			"abcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n1234567890\n1234567890\n1234567890\nabcdefghij\n",
		},
	}
	config := NewDefaultConfig()
	config.DiffTimeout = 0
	// Test cases must be at least 100 chars long to pass the cutoff.
	for i, test := range tests {
		resultWithoutCheckLines := config.Diff(test.Text1, test.Text2, false)
		resultWithCheckLines := config.Diff(test.Text1, test.Text2, true)
		// TODO this fails for the third test case, why?
		if i != 2 {
			assert.Equal(t, resultWithoutCheckLines, resultWithCheckLines, fmt.Sprintf("Test case #%d, %#v", i, test))
		}
		assert.Equal(t, diffRebuildTexts(resultWithoutCheckLines), diffRebuildTexts(resultWithCheckLines), fmt.Sprintf("Test case #%d, %#v", i, test))
	}
}

func TestMassiveRuneDiffConversion(t *testing.T) {
	sNew, err := ioutil.ReadFile(filepath.Join("testdata", "fixture.go"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	config := NewDefaultConfig()
	t1, t2, tt := config.DiffLinesToChars("", string(sNew))
	diffs := config.Diff(t1, t2, false)
	diffs = config.DiffCharsToLines(diffs, tt)
	assert.NotEmpty(t, diffs)
}

func BenchmarkDiff(bench *testing.B) {
	s1 := "`Twas brillig, and the slithy toves\nDid gyre and gimble in the wabe:\nAll mimsy were the borogoves,\nAnd the mome raths outgrabe.\n"
	s2 := "I am the very model of a modern major general,\nI've information vegetable, animal, and mineral,\nI know the kings of England, and I quote the fights historical,\nFrom Marathon to Waterloo, in order categorical.\n"
	// Increase the text lengths by 1024 times to ensure a timeout.
	for x := 0; x < 10; x++ {
		s1 = s1 + s1
		s2 = s2 + s2
	}
	config := NewDefaultConfig()
	config.DiffTimeout = time.Second
	bench.ResetTimer()
	for i := 0; i < bench.N; i++ {
		config.Diff(s1, s2, true)
	}
}

func BenchmarkDiffLarge(b *testing.B) {
	s1, s2 := speedtestTexts()
	config := NewDefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Diff(s1, s2, true)
	}
}

func BenchmarkDiffRunesLargeLines(b *testing.B) {
	s1, s2 := speedtestTexts()
	config := NewDefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text1, text2, linearray := config.DiffLinesToRunes(s1, s2)
		diffs := config.DiffRunes(text1, text2, false)
		_ = config.DiffCharsToLines(diffs, linearray)
	}
}

func BenchmarkDiffRunesLargeDiffLines(b *testing.B) {
	fp, _ := os.Open(filepath.Join("testdata", "diff10klinestest.txt"))
	defer fp.Close()
	data, _ := ioutil.ReadAll(fp)
	config := NewDefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		text1, text2, linearray := config.DiffLinesToRunes(string(data), "")
		diffs := config.DiffRunes(text1, text2, false)
		_ = config.DiffCharsToLines(diffs, linearray)
	}
}

func BenchmarkDiffCommonPrefix(b *testing.B) {
	s := "ABCDEFGHIJKLMNOPQRSTUVWXYZÅÄÖ"
	config := NewDefaultConfig()
	for i := 0; i < b.N; i++ {
		config.DiffCommonPrefix(s, s)
	}
}

func BenchmarkDiffCommonSuffix(b *testing.B) {
	s := "ABCDEFGHIJKLMNOPQRSTUVWXYZÅÄÖ"
	config := NewDefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SinkInt = config.DiffCommonSuffix(s, s)
	}
}

func BenchmarkCommonLength(b *testing.B) {
	tests := []struct {
		Name string
		X    []rune
		Y    []rune
	}{
		{
			Name: "empty",
			X:    nil,
			Y:    []rune{},
		},
		{
			Name: "short",
			X:    []rune("AABCC"),
			Y:    []rune("AA-CC"),
		},
		{
			Name: "long",
			X:    []rune(strings.Repeat("A", 1000) + "B" + strings.Repeat("C", 1000)),
			Y:    []rune(strings.Repeat("A", 1000) + "-" + strings.Repeat("C", 1000)),
		},
	}
	b.Run("prefix", func(b *testing.B) {
		for _, test := range tests {
			b.Run(test.Name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					SinkInt = commonPrefixLength(test.X, test.Y)
				}
			})
		}
	})
	b.Run("suffix", func(b *testing.B) {
		for _, test := range tests {
			b.Run(test.Name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					SinkInt = commonSuffixLength(test.X, test.Y)
				}
			})
		}
	})
}

func BenchmarkDiffHalfMatch(b *testing.B) {
	s1, s2 := speedtestTexts()
	config := NewDefaultConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.DiffHalfMatch(s1, s2)
	}
}

func BenchmarkDiffCleanupSemantic(b *testing.B) {
	s1, s2 := speedtestTexts()
	config := NewDefaultConfig()
	diffs := config.Diff(s1, s2, false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.DiffCleanupSemantic(diffs)
	}
}
