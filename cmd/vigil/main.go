package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: vigil <command> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  run      Run spec-driven UI tests\n")
		fmt.Fprintf(os.Stderr, "  explore  Run exploratory UI tests\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "run":
		fmt.Fprintln(os.Stderr, "vigil run: not yet implemented")
		os.Exit(1)
	case "explore":
		fmt.Fprintln(os.Stderr, "vigil explore: not yet implemented")
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		flag.Usage()
		os.Exit(1)
	}
}
