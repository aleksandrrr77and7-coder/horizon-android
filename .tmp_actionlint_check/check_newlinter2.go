package main

import (
	"fmt"
	"os"

	"github.com/rhysd/actionlint"
)

func main() {
	fmt.Println("Test 1: NewLinter with ColorOptionKindNever...")
	linter, err := actionlint.NewLinter(os.Stderr, &actionlint.LinterOptions{Color: actionlint.ColorOptionKindNever})
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  OK: %v\n", linter)
	}

	fmt.Println("Test 2: NewLinter with default options...")
	linter2, err := actionlint.NewLinter(os.Stderr, nil)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  OK: %v\n", linter2)
	}
}
