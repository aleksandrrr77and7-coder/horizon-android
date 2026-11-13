package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-colorable"
	"github.com/fatih/color"
)

func main() {
	var out io.Writer = os.Stderr
	fmt.Printf("out type: %T\n", out)
	
	if f, ok := out.(*os.File); ok {
		fmt.Printf("type assertion ok, f=%v\n", f)
		out2 := colorable.NewColorable(f)
		fmt.Printf("out2 type: %T\n", out2)
		color.NoColor = false
		fmt.Printf("color.NoColor set to false\n")
		fmt.Println("All steps OK")
	}
}
