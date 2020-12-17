package diffmatchpatch

import (
	"bytes"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Patch holds information about a patch.
type Patch struct {
	Diffs   []Diff
	Start1  int
	Start2  int
	Length1 int
	Length2 int
}

// String satisfies the fmt.Stringer interface.
//
// Generates a string that emulates GNU diff's format like the following:
//
// 	Header: @@ -382,8 +481,9 @@
//
// Indices are printed as 1-based, not 0-based.
func (p *Patch) String() string {
	var coords1, coords2 string
	if p.Length1 == 0 {
		coords1 = strconv.Itoa(p.Start1) + ",0"
	} else if p.Length1 == 1 {
		coords1 = strconv.Itoa(p.Start1 + 1)
	} else {
		coords1 = strconv.Itoa(p.Start1+1) + "," + strconv.Itoa(p.Length1)
	}
	if p.Length2 == 0 {
		coords2 = strconv.Itoa(p.Start2) + ",0"
	} else if p.Length2 == 1 {
		coords2 = strconv.Itoa(p.Start2 + 1)
	} else {
		coords2 = strconv.Itoa(p.Start2+1) + "," + strconv.Itoa(p.Length2)
	}
	var buf bytes.Buffer
	_, _ = buf.WriteString("@@ -" + coords1 + " +" + coords2 + " @@\n")
	// Escape the body of the patch with %xx notation.
	for _, d := range p.Diffs {
		switch d.Op {
		case OpInsert:
			_, _ = buf.WriteString("+")
		case OpDelete:
			_, _ = buf.WriteString("-")
		case OpEqual:
			_, _ = buf.WriteString(" ")
		}
		_, _ = buf.WriteString(strings.Replace(url.QueryEscape(d.Text), "+", " ", -1))
		_, _ = buf.WriteString("\n")
	}
	return unescaper.Replace(buf.String())
}

// PatchAddContext increases the context until it is unique, but doesn't let
// the pattern expand beyond MatchMaxBits.
func (config *Config) PatchAddContext(patch Patch, text string) Patch {
	if len(text) == 0 {
		return patch
	}
	pattern := text[patch.Start2 : patch.Start2+patch.Length1]
	padding := 0
	// Look for the first and last matches of pattern in text.  If two
	// different matches are found, increase the pattern length.
	for strings.Index(text, pattern) != strings.LastIndex(text, pattern) &&
		len(pattern) < config.MatchMaxBits-2*config.PatchMargin {
		padding += config.PatchMargin
		maxStart := max(0, patch.Start2-padding)
		minEnd := min(len(text), patch.Start2+patch.Length1+padding)
		pattern = text[maxStart:minEnd]
	}
	// Add one chunk for good luck.
	padding += config.PatchMargin
	// Add the prefix.
	prefix := text[max(0, patch.Start2-padding):patch.Start2]
	if len(prefix) != 0 {
		patch.Diffs = append([]Diff{Diff{OpEqual, prefix}}, patch.Diffs...)
	}
	// Add the suffix.
	suffix := text[patch.Start2+patch.Length1 : min(len(text), patch.Start2+patch.Length1+padding)]
	if len(suffix) != 0 {
		patch.Diffs = append(patch.Diffs, Diff{OpEqual, suffix})
	}
	// Roll back the start points.
	patch.Start1 -= len(prefix)
	patch.Start2 -= len(prefix)
	// Extend the lengths.
	patch.Length1 += len(prefix) + len(suffix)
	patch.Length2 += len(prefix) + len(suffix)
	return patch
}

// PatchMake computes a list of patches.
func (config *Config) PatchMake(opt ...interface{}) []Patch {
	if len(opt) == 1 {
		diffs, _ := opt[0].([]Diff)
		text1 := config.DiffText1(diffs)
		return config.PatchMake(text1, diffs)
	} else if len(opt) == 2 {
		text1 := opt[0].(string)
		switch t := opt[1].(type) {
		case string:
			diffs := config.Diff(text1, t, true)
			if len(diffs) > 2 {
				diffs = config.DiffCleanupSemantic(diffs)
				diffs = config.DiffCleanupEfficiency(diffs)
			}
			return config.PatchMake(text1, diffs)
		case []Diff:
			return config.patchMake2(text1, t)
		}
	} else if len(opt) == 3 {
		return config.PatchMake(opt[0], opt[2])
	}
	return []Patch{}
}

// patchMake2 computes a list of patches to turn text1 into text2.  text2 is
// not provided, diffs are the delta between text1 and text2.
func (config *Config) patchMake2(text1 string, diffs []Diff) []Patch {
	// Check for null inputs not needed since null can't be passed in C#.
	patches := []Patch{}
	if len(diffs) == 0 {
		return patches // Get rid of the null case.
	}
	patch := Patch{}
	charCount1 := 0 // Number of characters into the text1 string.
	charCount2 := 0 // Number of characters into the text2 string.
	// Start with text1 (prepatchText) and apply the diffs until we arrive at
	// text2 (postpatchText). We recreate the patches one by one to determine
	// context info.
	prepatchText := text1
	postpatchText := text1
	for i, d := range diffs {
		if len(patch.Diffs) == 0 && d.Op != OpEqual {
			// A new patch starts here.
			patch.Start1 = charCount1
			patch.Start2 = charCount2
		}
		switch d.Op {
		case OpInsert:
			patch.Diffs = append(patch.Diffs, d)
			patch.Length2 += len(d.Text)
			postpatchText = postpatchText[:charCount2] +
				d.Text + postpatchText[charCount2:]
		case OpDelete:
			patch.Length1 += len(d.Text)
			patch.Diffs = append(patch.Diffs, d)
			postpatchText = postpatchText[:charCount2] + postpatchText[charCount2+len(d.Text):]
		case OpEqual:
			if len(d.Text) <= 2*config.PatchMargin &&
				len(patch.Diffs) != 0 && i != len(diffs)-1 {
				// Small equality inside a patch.
				patch.Diffs = append(patch.Diffs, d)
				patch.Length1 += len(d.Text)
				patch.Length2 += len(d.Text)
			}
			if len(d.Text) >= 2*config.PatchMargin {
				// Time for a new patch.
				if len(patch.Diffs) != 0 {
					patch = config.PatchAddContext(patch, prepatchText)
					patches = append(patches, patch)
					patch = Patch{}
					// Unlike Unidiff, our patch lists have a rolling context.
					// http://code.google.com/p/google-diff-match-patch/wiki/Unidiff
					// Update prepatch text & pos to reflect the application of
					// the just completed patch.
					prepatchText = postpatchText
					charCount1 = charCount2
				}
			}
		}
		// Update the current character count.
		if d.Op != OpInsert {
			charCount1 += len(d.Text)
		}
		if d.Op != OpDelete {
			charCount2 += len(d.Text)
		}
	}
	// Pick up the leftover patch if not empty.
	if len(patch.Diffs) != 0 {
		patch = config.PatchAddContext(patch, prepatchText)
		patches = append(patches, patch)
	}
	return patches
}

// PatchDeepCopy returns an array that is identical to a given array of
// patches.
func (config *Config) PatchDeepCopy(patches []Patch) []Patch {
	patchesCopy := []Patch{}
	for _, p := range patches {
		patchCopy := Patch{}
		for _, d := range p.Diffs {
			patchCopy.Diffs = append(patchCopy.Diffs, Diff{d.Op, d.Text})
		}
		patchCopy.Start1 = p.Start1
		patchCopy.Start2 = p.Start2
		patchCopy.Length1 = p.Length1
		patchCopy.Length2 = p.Length2
		patchesCopy = append(patchesCopy, patchCopy)
	}
	return patchesCopy
}

// PatchApply merges a set of patches onto the text.  Returns a patched text,
// as well as an array of true/false values indicating which patches were
// applied.
func (config *Config) PatchApply(patches []Patch, text string) (string, []bool) {
	if len(patches) == 0 {
		return text, []bool{}
	}
	// Deep copy the patches so that no changes are made to originals.
	patches = config.PatchDeepCopy(patches)
	nullPadding := config.PatchAddPadding(patches)
	text = nullPadding + text + nullPadding
	patches = config.PatchSplitMax(patches)
	x := 0
	// delta keeps track of the offset between the expected and actual location
	// of the previous patch.  If there are patches expected at positions 10
	// and 20, but the first patch was found at 12, delta is 2 and the second
	// patch has an effective expected position of 22.
	delta := 0
	results := make([]bool, len(patches))
	for _, p := range patches {
		expectedLoc := p.Start2 + delta
		text1 := config.DiffText1(p.Diffs)
		var startLoc int
		endLoc := -1
		if len(text1) > config.MatchMaxBits {
			// PatchSplitMax will only provide an oversized pattern in the case
			// of a monster delete.
			startLoc = config.Match(text, text1[:config.MatchMaxBits], expectedLoc)
			if startLoc != -1 {
				endLoc = config.Match(text,
					text1[len(text1)-config.MatchMaxBits:], expectedLoc+len(text1)-config.MatchMaxBits)
				if endLoc == -1 || startLoc >= endLoc {
					// Can't find valid trailing context.  Drop this patch.
					startLoc = -1
				}
			}
		} else {
			startLoc = config.Match(text, text1, expectedLoc)
		}
		if startLoc == -1 {
			// No match found.  :(
			results[x] = false
			// Subtract the delta for this failed patch from subsequent patches.
			delta -= p.Length2 - p.Length1
		} else {
			// Found a match.  :)
			results[x] = true
			delta = startLoc - expectedLoc
			var text2 string
			if endLoc == -1 {
				text2 = text[startLoc:min(startLoc+len(text1), len(text))]
			} else {
				text2 = text[startLoc:min(endLoc+config.MatchMaxBits, len(text))]
			}
			if text1 == text2 {
				// Perfect match, just shove the Replacement text in.
				text = text[:startLoc] + config.DiffText2(p.Diffs) + text[startLoc+len(text1):]
			} else {
				// Imperfect match.  Run a diff to get a framework of
				// equivalent indices.
				diffs := config.Diff(text1, text2, false)
				if len(text1) > config.MatchMaxBits && float64(config.DiffLevenshtein(diffs))/float64(len(text1)) > config.PatchDeleteThreshold {
					// The end points match, but the content is unacceptably bad.
					results[x] = false
				} else {
					diffs = config.DiffCleanupSemanticLossless(diffs)
					index1 := 0
					for _, d := range p.Diffs {
						if d.Op != OpEqual {
							index2 := config.DiffXIndex(diffs, index1)
							if d.Op == OpInsert {
								// Insertion
								text = text[:startLoc+index2] + d.Text + text[startLoc+index2:]
							} else if d.Op == OpDelete {
								// Deletion
								startIndex := startLoc + index2
								text = text[:startIndex] +
									text[startIndex+config.DiffXIndex(diffs, index1+len(d.Text))-index2:]
							}
						}
						if d.Op != OpDelete {
							index1 += len(d.Text)
						}
					}
				}
			}
		}
		x++
	}
	// strip padding
	return text[len(nullPadding) : len(nullPadding)+(len(text)-2*len(nullPadding))], results
}

// PatchAddPadding adds some padding on text start and end so that edges can
// match something.  Intended to be called only from within patchApply.
func (config *Config) PatchAddPadding(patches []Patch) string {
	paddingLength := config.PatchMargin
	nullPadding := ""
	for x := 1; x <= paddingLength; x++ {
		nullPadding += string(rune(x))
	}
	// Bump all the patches forward.
	for i := range patches {
		patches[i].Start1 += paddingLength
		patches[i].Start2 += paddingLength
	}
	// Add some padding on start of first diff.
	if len(patches[0].Diffs) == 0 || patches[0].Diffs[0].Op != OpEqual {
		// Add nullPadding equality.
		patches[0].Diffs = append([]Diff{Diff{OpEqual, nullPadding}}, patches[0].Diffs...)
		patches[0].Start1 -= paddingLength // Should be 0.
		patches[0].Start2 -= paddingLength // Should be 0.
		patches[0].Length1 += paddingLength
		patches[0].Length2 += paddingLength
	} else if paddingLength > len(patches[0].Diffs[0].Text) {
		// Grow first equality.
		extraLength := paddingLength - len(patches[0].Diffs[0].Text)
		patches[0].Diffs[0].Text = nullPadding[len(patches[0].Diffs[0].Text):] + patches[0].Diffs[0].Text
		patches[0].Start1 -= extraLength
		patches[0].Start2 -= extraLength
		patches[0].Length1 += extraLength
		patches[0].Length2 += extraLength
	}
	// Add some padding on end of last diff.
	last := len(patches) - 1
	if len(patches[last].Diffs) == 0 || patches[last].Diffs[len(patches[last].Diffs)-1].Op != OpEqual {
		// Add nullPadding equality.
		patches[last].Diffs = append(patches[last].Diffs, Diff{OpEqual, nullPadding})
		patches[last].Length1 += paddingLength
		patches[last].Length2 += paddingLength
	} else if paddingLength > len(patches[last].Diffs[len(patches[last].Diffs)-1].Text) {
		// Grow last equality.
		lastDiff := patches[last].Diffs[len(patches[last].Diffs)-1]
		extraLength := paddingLength - len(lastDiff.Text)
		patches[last].Diffs[len(patches[last].Diffs)-1].Text += nullPadding[:extraLength]
		patches[last].Length1 += extraLength
		patches[last].Length2 += extraLength
	}
	return nullPadding
}

// PatchSplitMax looks through the patches and breaks up any which are longer
// than the maximum limit of the match algorithm.  Intended to be called only
// from within patchApply.
func (config *Config) PatchSplitMax(patches []Patch) []Patch {
	patchSize := config.MatchMaxBits
	for x := 0; x < len(patches); x++ {
		if patches[x].Length1 <= patchSize {
			continue
		}
		bigpatch := patches[x]
		// Remove the big old patch.
		patches = append(patches[:x], patches[x+1:]...)
		x--
		Start1 := bigpatch.Start1
		Start2 := bigpatch.Start2
		precontext := ""
		for len(bigpatch.Diffs) != 0 {
			// Create one of several smaller patches.
			patch := Patch{}
			empty := true
			patch.Start1 = Start1 - len(precontext)
			patch.Start2 = Start2 - len(precontext)
			if len(precontext) != 0 {
				patch.Length1 = len(precontext)
				patch.Length2 = len(precontext)
				patch.Diffs = append(patch.Diffs, Diff{OpEqual, precontext})
			}
			for len(bigpatch.Diffs) != 0 && patch.Length1 < patchSize-config.PatchMargin {
				diffType := bigpatch.Diffs[0].Op
				diffText := bigpatch.Diffs[0].Text
				if diffType == OpInsert {
					// Insertions are harmless.
					patch.Length2 += len(diffText)
					Start2 += len(diffText)
					patch.Diffs = append(patch.Diffs, bigpatch.Diffs[0])
					bigpatch.Diffs = bigpatch.Diffs[1:]
					empty = false
				} else if diffType == OpDelete && len(patch.Diffs) == 1 && patch.Diffs[0].Op == OpEqual && len(diffText) > 2*patchSize {
					// This is a large deletion.  Let it pass in one chunk.
					patch.Length1 += len(diffText)
					Start1 += len(diffText)
					empty = false
					patch.Diffs = append(patch.Diffs, Diff{diffType, diffText})
					bigpatch.Diffs = bigpatch.Diffs[1:]
				} else {
					// Deletion or equality.  Only take as much as we can stomach.
					diffText = diffText[:min(len(diffText), patchSize-patch.Length1-config.PatchMargin)]
					patch.Length1 += len(diffText)
					Start1 += len(diffText)
					if diffType == OpEqual {
						patch.Length2 += len(diffText)
						Start2 += len(diffText)
					} else {
						empty = false
					}
					patch.Diffs = append(patch.Diffs, Diff{diffType, diffText})
					if diffText == bigpatch.Diffs[0].Text {
						bigpatch.Diffs = bigpatch.Diffs[1:]
					} else {
						bigpatch.Diffs[0].Text =
							bigpatch.Diffs[0].Text[len(diffText):]
					}
				}
			}
			// Compute the head context for the next patch.
			precontext = config.DiffText2(patch.Diffs)
			precontext = precontext[max(0, len(precontext)-config.PatchMargin):]
			postcontext := ""
			// Append the end context for this patch.
			if len(config.DiffText1(bigpatch.Diffs)) > config.PatchMargin {
				postcontext = config.DiffText1(bigpatch.Diffs)[:config.PatchMargin]
			} else {
				postcontext = config.DiffText1(bigpatch.Diffs)
			}
			if len(postcontext) != 0 {
				patch.Length1 += len(postcontext)
				patch.Length2 += len(postcontext)
				if len(patch.Diffs) != 0 && patch.Diffs[len(patch.Diffs)-1].Op == OpEqual {
					patch.Diffs[len(patch.Diffs)-1].Text += postcontext
				} else {
					patch.Diffs = append(patch.Diffs, Diff{OpEqual, postcontext})
				}
			}
			if !empty {
				x++
				patches = append(patches[:x], append([]Patch{patch}, patches[x:]...)...)
			}
		}
	}
	return patches
}

// PatchToText takes a list of patches and returns a textual representation.
func (config *Config) PatchToText(patches []Patch) string {
	var buf bytes.Buffer
	for _, p := range patches {
		_, _ = buf.WriteString(p.String())
	}
	return buf.String()
}

// PatchFromText parses a textual representation of patches and returns a List
// of Patch objects.
func (config *Config) PatchFromText(textline string) ([]Patch, error) {
	patches := []Patch{}
	if len(textline) == 0 {
		return patches, nil
	}
	text := strings.Split(textline, "\n")
	textPointer := 0
	patchHeader := regexp.MustCompile(`^@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@$`)
	var patch Patch
	var sign uint8
	var line string
	for textPointer < len(text) {
		if !patchHeader.MatchString(text[textPointer]) {
			return patches, errors.New("Invalid patch string: " + text[textPointer])
		}
		patch = Patch{}
		m := patchHeader.FindStringSubmatch(text[textPointer])
		patch.Start1, _ = strconv.Atoi(m[1])
		if len(m[2]) == 0 {
			patch.Start1--
			patch.Length1 = 1
		} else if m[2] == "0" {
			patch.Length1 = 0
		} else {
			patch.Start1--
			patch.Length1, _ = strconv.Atoi(m[2])
		}
		patch.Start2, _ = strconv.Atoi(m[3])
		if len(m[4]) == 0 {
			patch.Start2--
			patch.Length2 = 1
		} else if m[4] == "0" {
			patch.Length2 = 0
		} else {
			patch.Start2--
			patch.Length2, _ = strconv.Atoi(m[4])
		}
		textPointer++
		for textPointer < len(text) {
			if len(text[textPointer]) > 0 {
				sign = text[textPointer][0]
			} else {
				textPointer++
				continue
			}
			line = text[textPointer][1:]
			line = strings.Replace(line, "+", "%2b", -1)
			line, _ = url.QueryUnescape(line)
			if sign == '-' {
				// Deletion.
				patch.Diffs = append(patch.Diffs, Diff{OpDelete, line})
			} else if sign == '+' {
				// Insertion.
				patch.Diffs = append(patch.Diffs, Diff{OpInsert, line})
			} else if sign == ' ' {
				// Minor equality.
				patch.Diffs = append(patch.Diffs, Diff{OpEqual, line})
			} else if sign == '@' {
				// Start of next patch.
				break
			} else {
				// WTF?
				return patches, errors.New("Invalid patch mode '" + string(sign) + "' in: " + string(line))
			}
			textPointer++
		}
		patches = append(patches, patch)
	}
	return patches, nil
}
