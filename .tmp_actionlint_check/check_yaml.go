package main

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
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
		var n any
		if err := yaml.Unmarshal(data, &n); err != nil {
			fmt.Printf("%s: YAML PARSE ERROR: %v\n", f, err)
			os.Exit(1)
		}
		fmt.Printf("%s: YAML PARSE OK (%d bytes)\n", f, len(data))
	}
	fmt.Println("All files parsed successfully with go.yaml.in/yaml/v4 v4.0.0-rc.3")
}
