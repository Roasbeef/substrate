package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/roasbeef/subtrate/internal/build"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// enumRegistry maps flag names to their valid enum values. This is the
// single source of truth for constrained fields across the CLI.
var enumRegistry = map[string][]string{
	"format":    {"text", "json", "context", "hook"},
	"priority":  {"urgent", "normal", "low"},
	"type":      {"full", "security", "performance", "architecture"},
	"transport": {"streamable-http", "sse", "stdio"},
	"state": {
		"new", "pending_review", "under_review",
		"changes_requested", "re_review",
		"approved", "rejected", "cancelled",
	},
	"status": {"pending", "in_progress", "completed", "deleted"},
	"in":     {"inbox", "sent", "all"},
}

// CLISchema is the top-level schema output.
type CLISchema struct {
	Version  string          `json:"version"`
	Binary   string          `json:"binary"`
	Commands []CommandSchema `json:"commands"`
}

// CommandSchema describes a single CLI command.
type CommandSchema struct {
	Name        string              `json:"name"`
	Path        string              `json:"path"`
	Description string              `json:"description"`
	Aliases     []string            `json:"aliases,omitempty"`
	Flags       []FlagSchema        `json:"flags,omitempty"`
	Args        *ArgsSchema         `json:"args,omitempty"`
	Subcommands []CommandSchema     `json:"subcommands,omitempty"`
	Enums       map[string][]string `json:"enum_constraints,omitempty"`
}

// FlagSchema describes a single flag.
type FlagSchema struct {
	Name        string   `json:"name"`
	Shorthand   string   `json:"shorthand,omitempty"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Description string   `json:"description"`
	Required    bool     `json:"required,omitempty"`
	EnumValues  []string `json:"enum_values,omitempty"`
}

// ArgsSchema describes positional argument constraints.
type ArgsSchema struct {
	Min int    `json:"min"`
	Max int    `json:"max"` // -1 means unlimited.
	Use string `json:"usage,omitempty"`
}

// schemaCmd outputs the CLI schema as JSON for agent discovery.
var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Output CLI schema as JSON for agent discovery",
	Long: `Outputs a machine-readable JSON schema describing all commands,
their flags, argument requirements, and enum constraints.

This enables AI agents to discover available operations without
parsing --help text.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		schema := buildSchema(rootCmd)

		data, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}

		fmt.Println(string(data))

		return nil
	},
}

// buildSchema walks the cobra command tree and builds a CLISchema.
func buildSchema(root *cobra.Command) CLISchema {
	schema := CLISchema{
		Version: build.Version(),
		Binary:  "substrate",
	}

	for _, cmd := range root.Commands() {
		// Skip help and completion commands.
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}

		schema.Commands = append(
			schema.Commands,
			buildCommandSchema(cmd, "substrate"),
		)
	}

	// Sort commands by name for stable output.
	sort.Slice(schema.Commands, func(i, j int) bool {
		return schema.Commands[i].Name < schema.Commands[j].Name
	})

	return schema
}

// buildCommandSchema builds a CommandSchema for a single command.
func buildCommandSchema(
	cmd *cobra.Command, parentPath string,
) CommandSchema {
	path := parentPath + " " + cmd.Name()

	cs := CommandSchema{
		Name:        cmd.Name(),
		Path:        path,
		Description: cmd.Short,
		Aliases:     cmd.Aliases,
		Enums:       make(map[string][]string),
	}

	// Extract flags.
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Skip inherited persistent flags (they're on the parent).
		if cmd.Parent() != nil {
			if pf := cmd.Parent().PersistentFlags().Lookup(
				f.Name,
			); pf != nil {
				return
			}
		}

		fs := FlagSchema{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		}

		// Check if this flag has enum constraints.
		if vals, ok := enumRegistry[f.Name]; ok {
			fs.EnumValues = vals
			cs.Enums[f.Name] = vals
		}

		// Check required annotation.
		if ann, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
			if len(ann) > 0 && ann[0] == "true" {
				fs.Required = true
			}
		}

		cs.Flags = append(cs.Flags, fs)
	})

	// Extract argument constraints from Use string.
	if useArgs := extractArgsFromUse(cmd.Use); useArgs != nil {
		cs.Args = useArgs
	}

	// Recurse into subcommands.
	for _, sub := range cmd.Commands() {
		if sub.Name() == "help" {
			continue
		}

		cs.Subcommands = append(
			cs.Subcommands,
			buildCommandSchema(sub, path),
		)
	}

	// Clean up empty maps.
	if len(cs.Enums) == 0 {
		cs.Enums = nil
	}

	return cs
}

// extractArgsFromUse parses the Use string to determine argument
// requirements. Returns nil if no args are specified.
func extractArgsFromUse(use string) *ArgsSchema {
	parts := strings.Fields(use)
	if len(parts) <= 1 {
		return nil
	}

	// Count required and optional args.
	minArgs := 0
	maxArgs := 0

	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "<") || strings.HasPrefix(p, "[") {
			maxArgs++
			if strings.HasPrefix(p, "<") {
				minArgs++
			}
		}
	}

	if maxArgs == 0 {
		return nil
	}

	return &ArgsSchema{
		Min: minArgs,
		Max: maxArgs,
		Use: strings.Join(parts[1:], " "),
	}
}
