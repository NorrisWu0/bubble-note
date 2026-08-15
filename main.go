package main

import (
	"fmt"
	"os"

	"github.com/norriswu0/bubble-note/internal/cli"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bubble-note:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		return cli.RunMigrate(os.Args[2:])
	}
	return cli.Run()
}
