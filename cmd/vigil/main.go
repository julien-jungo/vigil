package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("vigil %s\n", version)
		return
	}

	fmt.Fprintln(os.Stderr, "usage: vigil [--version]")
	os.Exit(1)
}
