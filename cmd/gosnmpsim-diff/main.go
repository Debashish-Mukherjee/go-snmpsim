package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/debashish-mukherjee/go-snmpsim/internal/walkdiff"
)

func main() {
	left := flag.String("left", "", "Left walk/snmprec file")
	right := flag.String("right", "", "Right walk/snmprec file")
	showAll := flag.Bool("show-all", false, "Show all differences (default shows first 100)")
	flag.Parse()

	if *left == "" || *right == "" {
		fmt.Fprintln(os.Stderr, "usage: gosnmpsim-diff --left <fileA> --right <fileB>")
		os.Exit(2)
	}

	result, err := walkdiff.CompareFiles(*left, *right)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff failed: %v\n", err)
		os.Exit(1)
	}

	if result.Identical() {
		fmt.Printf("IDENTICAL: %d OIDs\n", result.LeftCount)
		return
	}

	fmt.Printf("DIFF: left=%d right=%d differences=%d\n", result.LeftCount, result.RightCount, len(result.Diffs))
	limit := len(result.Diffs)
	if !*showAll && limit > 100 {
		limit = 100
	}
	for i := 0; i < limit; i++ {
		d := result.Diffs[i]
		fmt.Printf("- %s [%s]\n", d.OID, d.Kind)
		if d.LeftType != "" || d.LeftValue != "" {
			fmt.Printf("  left : %s|%s\n", d.LeftType, d.LeftValue)
		}
		if d.RightType != "" || d.RightValue != "" {
			fmt.Printf("  right: %s|%s\n", d.RightType, d.RightValue)
		}
	}
	if !*showAll && len(result.Diffs) > limit {
		fmt.Printf("... %d more differences omitted (use --show-all)\n", len(result.Diffs)-limit)
	}
	os.Exit(1)
}
