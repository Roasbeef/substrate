package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/roasbeef/subtrate/internal/hooks"
	"github.com/spf13/cobra"
)

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Claude Code hooks integration",
	Long: `Manage the integration between Subtrate and Claude Code hooks.

Subtrate hooks provide:
- SessionStart: Heartbeat + check inbox at session start
- UserPromptSubmit: Silent heartbeat + check mail on each prompt
- Stop: Long-poll to keep main agent alive, blocking exit while checking mail
- SubagentStop: One-shot check for subagents, then allow exit
- PreCompact: Save identity state before context compaction

The Stop hook implements the "persistent agent" pattern - it keeps the agent
alive indefinitely, continuously checking for work. Use Ctrl+C to force exit.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Subtrate hooks into ~/.claude",
	Long: `Install hook scripts and update settings.json for Subtrate integration.

This command:
1. Creates ~/.claude/hooks/substrate/ with hook scripts
2. Updates ~/.claude/settings.json to register hooks
3. Installs the Subtrate skill to ~/.claude/skills/substrate/

Existing hooks in settings.json are preserved; Subtrate hooks are appended.`,
	RunE: runHooksInstall,
}

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Subtrate hooks from ~/.claude",
	Long:  `Remove Subtrate hooks from settings.json and delete hook scripts.`,
	RunE:  runHooksUninstall,
}

var hooksStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Subtrate hooks installation status",
	Long:  `Check whether Subtrate hooks are installed and show their status.`,
	RunE:  runHooksStatus,
}

func init() {
	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)
	hooksCmd.AddCommand(hooksStatusCmd)

	rootCmd.AddCommand(hooksCmd)
}

func runHooksInstall(cmd *cobra.Command, args []string) error {
	claudeDir := getClaudeDir()

	// 1. Create hook scripts directory.
	scriptsDir := filepath.Join(claudeDir, "hooks", "substrate")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// 2. Write hook scripts.
	allScripts := hooks.AllScripts()
	for name, content := range allScripts {
		filename := hooks.ScriptNames[name]
		scriptPath := filepath.Join(scriptsDir, filename)

		if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// 3. Update settings.json.
	settings, err := hooks.LoadSettings(claudeDir)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	hooks.InstallHooks(settings)

	if err := hooks.SaveSettings(claudeDir, settings); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	// 4. Install skill.
	if err := installSkill(claudeDir); err != nil {
		return fmt.Errorf("failed to install skill: %w", err)
	}

	fmt.Println("Subtrate hooks installed successfully!")
	fmt.Println()
	fmt.Println("Installed components:")
	fmt.Printf("  - Hook scripts: %s\n", scriptsDir)
	fmt.Printf("  - Settings: %s\n", filepath.Join(claudeDir, "settings.json"))
	fmt.Printf("  - Skill: %s\n", filepath.Join(claudeDir, "skills", "substrate"))
	fmt.Println()
	fmt.Println("Hooks installed:")
	for event := range hooks.HookDefinitions {
		fmt.Printf("  - %s\n", event)
	}
	fmt.Println()
	fmt.Println("Start a new Claude Code session to activate the hooks.")

	return nil
}

func runHooksUninstall(cmd *cobra.Command, args []string) error {
	claudeDir := getClaudeDir()

	// 1. Remove hook scripts directory.
	scriptsDir := filepath.Join(claudeDir, "hooks", "substrate")
	if err := os.RemoveAll(scriptsDir); err != nil {
		return fmt.Errorf("failed to remove hooks directory: %w", err)
	}

	// 2. Update settings.json to remove hooks.
	settings, err := hooks.LoadSettings(claudeDir)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	hooks.UninstallHooks(settings)

	if err := hooks.SaveSettings(claudeDir, settings); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	// 3. Remove skill.
	skillDir := filepath.Join(claudeDir, "skills", "substrate")
	if err := os.RemoveAll(skillDir); err != nil {
		// Ignore errors removing skill directory.
		_ = err
	}

	fmt.Println("Subtrate hooks uninstalled.")
	fmt.Printf("  - Removed: %s\n", scriptsDir)
	fmt.Printf("  - Updated: %s\n", filepath.Join(claudeDir, "settings.json"))
	fmt.Println()
	fmt.Println("Restart your Claude Code session for changes to take effect.")

	return nil
}

func runHooksStatus(cmd *cobra.Command, args []string) error {
	claudeDir := getClaudeDir()

	settings, err := hooks.LoadSettings(claudeDir)
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Check script files.
	scriptsDir := filepath.Join(claudeDir, "hooks", "substrate")
	scriptFilesExist := true
	for name := range hooks.ScriptNames {
		filename := hooks.ScriptNames[name]
		scriptPath := filepath.Join(scriptsDir, filename)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			scriptFilesExist = false
			break
		}
	}

	// Check skill.
	skillDir := filepath.Join(claudeDir, "skills", "substrate")
	skillExists := false
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err == nil {
		skillExists = true
	}

	// Check settings.
	installedEvents := hooks.GetInstalledHookEvents(settings)

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"installed":     hooks.IsInstalled(settings),
			"scripts_exist": scriptFilesExist,
			"skill_exists":  skillExists,
			"hook_events":   installedEvents,
			"scripts_dir":   scriptsDir,
			"settings_path": filepath.Join(claudeDir, "settings.json"),
		})
	default:
		fmt.Println("Subtrate Hooks Status")
		fmt.Println("=====================")
		fmt.Println()

		if hooks.IsInstalled(settings) && scriptFilesExist {
			fmt.Println("Status: INSTALLED")
		} else if hooks.IsInstalled(settings) || scriptFilesExist {
			fmt.Println("Status: PARTIAL (run 'substrate hooks install' to complete)")
		} else {
			fmt.Println("Status: NOT INSTALLED")
		}

		fmt.Println()
		fmt.Printf("Scripts directory: %s\n", scriptsDir)
		if scriptFilesExist {
			fmt.Println("  Scripts: All present")
		} else {
			fmt.Println("  Scripts: Missing")
		}

		fmt.Println()
		fmt.Printf("Skill: %s\n", skillDir)
		if skillExists {
			fmt.Println("  SKILL.md: Present")
		} else {
			fmt.Println("  SKILL.md: Missing")
		}

		fmt.Println()
		fmt.Println("Hooks in settings.json:")
		if len(installedEvents) == 0 {
			fmt.Println("  None")
		} else {
			sort.Strings(installedEvents)
			for _, event := range installedEvents {
				fmt.Printf("  - %s\n", event)
			}
		}
	}

	return nil
}

// getClaudeDir returns the path to the ~/.claude directory.
func getClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// installSkill installs the Subtrate skill to ~/.claude/skills/substrate/.
func installSkill(claudeDir string) error {
	skillDir := filepath.Join(claudeDir, "skills", "substrate")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	return os.WriteFile(skillPath, []byte(hooks.SkillContent), 0o644)
}
