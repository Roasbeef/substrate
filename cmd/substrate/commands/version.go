package commands

import (
	"fmt"

	"github.com/roasbeef/subtrate/internal/build"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version, commit hash, and build metadata for substrate.`,
	Run:   runVersion,
}

// runVersion prints the version and build information.
func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("substrate version %s", build.Version())

	if build.Commit != "" {
		fmt.Printf(" commit=%s", build.Commit)
	} else if build.CommitHash != "" {
		fmt.Printf(" commit=%s", build.CommitHash)
	}

	if build.GoVersion != "" {
		fmt.Printf(" go=%s", build.GoVersion)
	}

	if tags := build.Tags(); len(tags) > 0 {
		fmt.Printf(" tags=%s", build.RawTags)
	}

	fmt.Println()
}
