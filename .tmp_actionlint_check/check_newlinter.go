package main

import (
	"fmt"
	"os"

	"github.com/rhysd/actionlint"
)

func main() {
	fmt.Println("Creating Linter with os.Stderr and default options...")
	linter, err := actionlint.NewLinter(os.Stderr, nil)
	if err != nil {
		fmt.Printf("NewLinter ERROR: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Linter created: %v\n", linter)
	fmt.Println("OK")
}
