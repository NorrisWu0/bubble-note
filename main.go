package main

import (
	"fmt"
	"os"

	"github.com/norriswu0/bubble-note/internal/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "bubble-note:", err)
		os.Exit(1)
	}
}
