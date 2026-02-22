package main

import (
	"os"

	"github.com/glennswest/rosecicd/internal/builder"
)

func main() {
	specPath := builder.DefaultSpecPath
	if len(os.Args) > 1 {
		specPath = os.Args[1]
	}
	os.Exit(builder.Run(specPath))
}
