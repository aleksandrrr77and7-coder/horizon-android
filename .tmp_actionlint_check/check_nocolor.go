package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/rhysd/actionlint"
)

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func main() {
	files := []string{
		"../.github/workflows/android-source.yml",
		"../.github/workflows/android-dev.yml",
	}

	// Тест с ColorOptionKindNever (эквивалент -no-color)
	opts := &actionlint.LinterOptions{Color: actionlint.ColorOptionKindNever}
	linter, err := actionlint.NewLinter(os.Stderr, opts)
	if err != nil {
		fmt.Printf("NewLinter ERROR: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		data, _ := os.ReadFile(f)
		fmt.Printf("=== %s (len=%d sha256=%s) ===\n", f, len(data), sha256hex(data))
		errs, err := linter.LintFile(f, nil)
		if err != nil {
			fmt.Printf("  LintFile ERROR: %v\n", err)
		} else if len(errs) > 0 {
			fmt.Printf("  %d lint errors\n", len(errs))
			for _, e := range errs {
				fmt.Printf("    %s:%d:%d: %s [%s]\n", e.Filepath, e.Line, e.Column, e.Message, e.Kind)
			}
		} else {
			fmt.Printf("  Lint OK\n")
		}
	}
	fmt.Println("\nAll files passed with ColorOptionKindNever")
}
