package commands

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// validateEnum checks that value is one of the valid values for the given
// flag. If invalid, returns an error listing all valid options.
func validateEnum(value, flagName string, validValues []string) error {
	for _, v := range validValues {
		if v == value {
			return nil
		}
	}

	return NewValidationError(
		fmt.Sprintf(
			"invalid %s %q; valid values: %s",
			flagName, value,
			strings.Join(validValues, ", "),
		),
		nil,
	)
}

// suggestClosestMatch returns the closest match to input from candidates
// using Levenshtein distance. Returns empty string if no candidate is
// close enough (distance > half the input length).
func suggestClosestMatch(input string, candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}

	input = strings.ToLower(input)
	bestDist := math.MaxInt
	bestMatch := ""

	for _, c := range candidates {
		d := levenshtein(input, strings.ToLower(c))
		if d < bestDist {
			bestDist = d
			bestMatch = c
		}
	}

	// Only suggest if the distance is reasonable (within half the
	// input length or at most 3 edits).
	maxDist := len(input) / 2
	if maxDist < 3 {
		maxDist = 3
	}
	if bestDist > maxDist {
		return ""
	}

	return bestMatch
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use a single-row DP approach.
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i

		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			curr[j] = min(
				curr[j-1]+1,
				min(prev[j]+1, prev[j-1]+cost),
			)
		}

		prev = curr
	}

	return prev[len(b)]
}

// confirmAction prompts the user for confirmation unless --yes is set or
// stdin is not a terminal (agent mode). Returns true if confirmed.
func confirmAction(prompt string) bool {
	if autoYes {
		return true
	}

	// In non-TTY mode (agent), skip confirmation.
	if !isTerminal(os.Stdin) {
		return true
	}

	fmt.Fprintf(os.Stderr, "%s [y/N]: ", prompt)

	var response string
	fmt.Scanln(&response)

	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes"
}

// isTerminal returns true if the given file is a terminal (not a pipe or
// redirection). This avoids importing golang.org/x/term.
func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}

	return (stat.Mode() & os.ModeCharDevice) != 0
}
