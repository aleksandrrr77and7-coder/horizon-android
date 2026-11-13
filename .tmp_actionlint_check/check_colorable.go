package main

import (
	"fmt"
	"os"

	"github.com/mattn/go-colorable"
)

func main() {
	fmt.Println("Testing colorable.NewColorable(os.Stderr)...")
	w := colorable.NewColorable(os.Stderr)
	fmt.Printf("Result: %v (type: %T)\n", w, w)
	fmt.Println("OK")
}
