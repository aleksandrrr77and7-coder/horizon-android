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
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("%s: READ ERROR: %v\n", f, err)
			os.Exit(1)
		}
		_, parseErrs := actionlint.Parse(data)
		if len(parseErrs) > 0 {
			for _, e := range parseErrs {
				fmt.Printf("%s: PARSE ERROR: %s (line %d, col %d, kind %s)\n", f, e.Message, e.Line, e.Column, e.Kind)
			}
			os.Exit(1)
		}
		fmt.Printf("%s: ACTIONLINT PARSE OK (%d bytes)\n", f, len(data))
	}
	fmt.Println("All files parsed successfully with actionlint.Parse")
}
