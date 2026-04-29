package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vigil %s\n", version)
		return
	}

	fmt.Fprintln(os.Stderr, "usage: vigil [--version]")
	os.Exit(1)
}
