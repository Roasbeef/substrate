package main

import (
	"os"

	"github.com/roasbeef/subtrate/cmd/substrate/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		exitCode := commands.OutputError(err)
		os.Exit(exitCode)
	}
}
