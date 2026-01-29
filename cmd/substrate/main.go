package main

import (
	"fmt"
	"os"

	"github.com/roasbeef/subtrate/cmd/substrate/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
