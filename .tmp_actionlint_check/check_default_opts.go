package main

import (
	"fmt"
	"os"

	"github.com/rhysd/actionlint"
)

func main() {
	files := []string{
		"../.github/workflows/android-source.yml",
		"../.github/workflows/android-dev.yml",
	}

	// Тест 1: NewLinter с дефолтными опциями (Color = 0 = Auto)
	fmt.Println("=== Тест 1: NewLinter с дефолтными опциями ===")
	linter, err := actionlint.NewLinter(os.Stderr, &actionlint.LinterOptions{})
	if err != nil {
		fmt.Printf("  NewLinter ERROR: %v\n", err)
	} else {
		fmt.Printf("  NewLinter OK: %v\n", linter)
		for _, f := range files {
			errs, err := linter.LintFile(f, nil)
			if err != nil {
				fmt.Printf("    LintFile(%s) ERROR: %v\n", f, err)
			} else if len(errs) > 0 {
				fmt.Printf("    LintFile(%s): %d errors\n", f, len(errs))
				for _, e := range errs {
					fmt.Printf("      %s:%d:%d: %s [%s]\n", e.Filepath, e.Line, e.Column, e.Message, e.Kind)
				}
			} else {
				fmt.Printf("    LintFile(%s): OK\n", f)
			}
		}
	}

	// Тест 2: NewLinter с ColorOptionKindAlways
	fmt.Println("\n=== Тест 2: NewLinter с ColorOptionKindAlways ===")
	linter2, err := actionlint.NewLinter(os.Stderr, &actionlint.LinterOptions{Color: actionlint.ColorOptionKindAlways})
	if err != nil {
		fmt.Printf("  NewLinter ERROR: %v\n", err)
	} else {
		fmt.Printf("  NewLinter OK: %v\n", linter2)
		for _, f := range files {
			errs, err := linter2.LintFile(f, nil)
			if err != nil {
				fmt.Printf("    LintFile(%s) ERROR: %v\n", f, err)
			} else if len(errs) > 0 {
				fmt.Printf("    LintFile(%s): %d errors\n", f, len(errs))
				for _, e := range errs {
					fmt.Printf("      %s:%d:%d: %s [%s]\n", e.Filepath, e.Line, e.Column, e.Message, e.Kind)
				}
			} else {
				fmt.Printf("    LintFile(%s): OK\n", f)
			}
		}
	}
}
