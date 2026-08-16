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
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			return cli.RunMigrate(os.Args[2:])
		case "help", "-h", "--help":
			cli.PrintHelp(os.Stdout)
			return nil
		}
	}
	return cli.Run(os.Args[1:])
}
