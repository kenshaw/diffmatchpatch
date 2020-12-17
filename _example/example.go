// _example/example.go
package main

import (
	"fmt"

	"github.com/kenshaw/diffmatchpatch"
)

const (
	text1 = "Lorem ipsum dolor."
	text2 = "Lorem dolor sit amet."
)

func main() {
	diffs := diffmatchpatch.Diff(text1, text2, false)
	fmt.Println(diffmatchpatch.DiffPretty(diffs))
}
