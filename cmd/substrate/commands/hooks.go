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
	Short: "Manage coding-agent hooks integration",
	Long: `Manage the integration between Subtrate and coding-agent hooks.

Subtrate hooks provide:
- SessionStart: Heartbeat + check inbox at session start
- UserPromptSubmit: Silent heartbeat + check mail on each prompt
- Stop: Long-poll to keep main agent alive, blocking exit while checking mail
- SubagentStop: One-shot check for subagents, then allow exit
- PreCompact: Save identity state before context compaction

The default target is Claude Code. Pass --codex to install, inspect, or remove
the corresponding Codex lifecycle hooks.`,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Subtrate hooks",
	Long: `Install hook scripts and update settings.json for Subtrate integration.

This command:
1. Creates ~/.claude/hooks/substrate/ with hook scripts
2. Updates ~/.claude/settings.json to register hooks
3. Installs the Subtrate skill to ~/.claude/skills/substrate/
4. Optionally installs task sync hooks for auto-syncing Claude Code tasks

Existing hooks in settings.json are preserved; Subtrate hooks are appended.`,
	RunE: runHooksInstall,
}

// Hook install flags.
var (
	installWithTasks bool
	installNoTasks   bool
	hooksCodex       bool
)

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove Subtrate hooks",
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
	hooksCmd.PersistentFlags().BoolVar(
		&hooksCodex, "codex", false,
		"Manage Codex hooks in ~/.codex instead of Claude Code hooks",
	)
	hooksInstallCmd.Flags().BoolVar(
		&installWithTasks, "with-tasks", false,
		"Install task sync hooks for auto-syncing Claude Code tasks",
	)
	hooksInstallCmd.Flags().BoolVar(
		&installNoTasks, "no-tasks", false,
		"Skip installing task sync hooks",
	)

	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)
	hooksCmd.AddCommand(hooksStatusCmd)

	rootCmd.AddCommand(hooksCmd)
}

func runHooksInstall(cmd *cobra.Command, args []string) error {
	if hooksCodex {
		if installWithTasks {
			return fmt.Errorf("--with-tasks is only supported for Claude Code hooks")
		}

		return runCodexHooksInstall()
	}

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

	// Always install plan mode hooks.
	hooks.InstallPlanHooks(settings)

	// Install task hooks if requested.
	if installWithTasks && !installNoTasks {
		hooks.InstallTaskHooks(settings)
	}

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
	fmt.Println("Plan mode hooks installed:")
	for event, def := range hooks.PlanHookDefinitions {
		fmt.Printf("  - %s (%s)\n", event, def.Matcher)
	}
	if installWithTasks && !installNoTasks {
		fmt.Println()
		fmt.Println("Task sync hooks installed:")
		for event := range hooks.TaskHookDefinitions {
			fmt.Printf("  - %s (TaskCreate|TaskUpdate|TaskList|TaskGet)\n", event)
		}
		fmt.Println()
		fmt.Println("Task sync requires CLAUDE_CODE_TASK_LIST_ID to be set.")
	}
	fmt.Println()
	fmt.Println("Start a new Claude Code session to activate the hooks.")

	return nil
}

func runHooksUninstall(cmd *cobra.Command, args []string) error {
	if hooksCodex {
		return runCodexHooksUninstall()
	}

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
	hooks.UninstallPlanHooks(settings)
	hooks.UninstallTaskHooks(settings)

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
	if hooksCodex {
		return runCodexHooksStatus()
	}

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

	// Check plan hooks.
	planHooksInstalled := hooks.IsPlanHooksInstalled(settings)

	// Check task hooks.
	taskHooksInstalled := hooks.IsTaskHooksInstalled(settings)

	switch outputFormat {
	case "json":
		return outputJSON(map[string]any{
			"installed":            hooks.IsInstalled(settings),
			"scripts_exist":        scriptFilesExist,
			"skill_exists":         skillExists,
			"plan_hooks_installed": planHooksInstalled,
			"task_hooks_installed": taskHooksInstalled,
			"hook_events":          installedEvents,
			"scripts_dir":          scriptsDir,
			"settings_path":        filepath.Join(claudeDir, "settings.json"),
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

		fmt.Println()
		fmt.Println("Plan Mode Hooks:")
		if planHooksInstalled {
			fmt.Println("  PostToolUse: INSTALLED (Write → track plan files)")
			fmt.Println("  PreToolUse:  INSTALLED (ExitPlanMode → submit for review)")
		} else {
			fmt.Println("  Not installed")
		}

		fmt.Println()
		fmt.Println("Task Sync Hooks:")
		if taskHooksInstalled {
			fmt.Println("  PostToolUse: INSTALLED (TaskCreate|TaskUpdate|TaskList|TaskGet)")
			fmt.Println("  Enable sync by setting CLAUDE_CODE_TASK_LIST_ID")
		} else {
			fmt.Println("  Not installed (use --with-tasks to enable)")
		}
	}

	return nil
}

var codexScriptNames = []string{
	"session_start",
	"user_prompt",
	"stop",
	"subagent_stop",
	"pre_compact",
}

func runCodexHooksInstall() error {
	codexDir := getCodexDir()
	scriptsDir := filepath.Join(codexDir, "hooks", "substrate")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	for _, name := range codexScriptNames {
		filename := hooks.ScriptNames[name]
		scriptPath := filepath.Join(scriptsDir, filename)
		if err := os.WriteFile(
			scriptPath, []byte(hooks.GetScript(name)), 0o755,
		); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	hooksPath := filepath.Join(codexDir, "hooks.json")
	settings, err := hooks.LoadSettingsFile(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to load Codex hooks: %w", err)
	}
	hooks.InstallCodexHooks(settings)
	if err := hooks.SaveSettingsFile(hooksPath, settings); err != nil {
		return fmt.Errorf("failed to save Codex hooks: %w", err)
	}

	if err := installCodexSkill(codexDir); err != nil {
		return fmt.Errorf("failed to install Codex skill: %w", err)
	}

	fmt.Println("Subtrate hooks installed for Codex successfully!")
	fmt.Println()
	fmt.Println("Installed components:")
	fmt.Printf("  - Hook scripts: %s\n", scriptsDir)
	fmt.Printf("  - Hooks config: %s\n", hooksPath)
	fmt.Printf("  - Skill: %s\n", filepath.Join(codexDir, "skills", "substrate"))
	fmt.Println()
	fmt.Println("Hooks installed:")
	for event := range hooks.CodexHookDefinitions {
		fmt.Printf("  - %s\n", event)
	}
	fmt.Println()
	fmt.Println("Start a new Codex session, then use /hooks to review and trust them.")

	return nil
}

func runCodexHooksUninstall() error {
	codexDir := getCodexDir()
	scriptsDir := filepath.Join(codexDir, "hooks", "substrate")
	if err := os.RemoveAll(scriptsDir); err != nil {
		return fmt.Errorf("failed to remove hooks directory: %w", err)
	}

	hooksPath := filepath.Join(codexDir, "hooks.json")
	settings, err := hooks.LoadSettingsFile(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to load Codex hooks: %w", err)
	}
	hooks.UninstallCodexHooks(settings)
	if err := hooks.SaveSettingsFile(hooksPath, settings); err != nil {
		return fmt.Errorf("failed to save Codex hooks: %w", err)
	}

	skillDir := filepath.Join(codexDir, "skills", "substrate")
	if err := os.RemoveAll(skillDir); err != nil {
		return fmt.Errorf("failed to remove Codex skill: %w", err)
	}

	fmt.Println("Subtrate hooks uninstalled from Codex.")
	fmt.Printf("  - Removed: %s\n", scriptsDir)
	fmt.Printf("  - Updated: %s\n", hooksPath)
	fmt.Println()
	fmt.Println("Restart Codex for changes to take effect.")

	return nil
}

func runCodexHooksStatus() error {
	codexDir := getCodexDir()
	hooksPath := filepath.Join(codexDir, "hooks.json")
	settings, err := hooks.LoadSettingsFile(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to load Codex hooks: %w", err)
	}

	scriptsDir := filepath.Join(codexDir, "hooks", "substrate")
	scriptsExist := true
	for _, name := range codexScriptNames {
		if _, err := os.Stat(filepath.Join(
			scriptsDir, hooks.ScriptNames[name],
		)); err != nil {
			scriptsExist = false
			break
		}
	}

	skillDir := filepath.Join(codexDir, "skills", "substrate")
	_, skillErr := os.Stat(filepath.Join(skillDir, "SKILL.md"))
	skillExists := skillErr == nil
	installedEvents := hooks.GetInstalledHookEvents(settings)
	installed := hooks.IsCodexInstalled(settings)

	if outputFormat == "json" {
		return outputJSON(map[string]any{
			"target":        "codex",
			"installed":     installed,
			"scripts_exist": scriptsExist,
			"skill_exists":  skillExists,
			"hook_events":   installedEvents,
			"scripts_dir":   scriptsDir,
			"hooks_path":    hooksPath,
		})
	}

	fmt.Println("Subtrate Hooks Status (Codex)")
	fmt.Println("===============================")
	fmt.Println()
	switch {
	case installed && scriptsExist:
		fmt.Println("Status: INSTALLED")
	case installed || scriptsExist:
		fmt.Println("Status: PARTIAL (run 'substrate hooks install --codex' to complete)")
	default:
		fmt.Println("Status: NOT INSTALLED")
	}
	fmt.Println()
	fmt.Printf("Scripts directory: %s\n", scriptsDir)
	fmt.Printf("  Scripts: %s\n", presentOrMissing(scriptsExist))
	fmt.Println()
	fmt.Printf("Skill: %s\n", skillDir)
	fmt.Printf("  SKILL.md: %s\n", presentOrMissing(skillExists))
	fmt.Println()
	fmt.Println("Hooks in hooks.json:")
	if len(installedEvents) == 0 {
		fmt.Println("  None")
	} else {
		sort.Strings(installedEvents)
		for _, event := range installedEvents {
			fmt.Printf("  - %s\n", event)
		}
	}

	return nil
}

func presentOrMissing(present bool) string {
	if present {
		return "Present"
	}

	return "Missing"
}

// getClaudeDir returns the path to the ~/.claude directory.
func getClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude"
	}
	return filepath.Join(home, ".claude")
}

// getCodexDir returns the path to the ~/.codex directory.
func getCodexDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
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

func installCodexSkill(codexDir string) error {
	skillDir := filepath.Join(codexDir, "skills", "substrate")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	skillPath := filepath.Join(skillDir, "SKILL.md")
	return os.WriteFile(skillPath, []byte(hooks.CodexSkillContent), 0o644)
}
