package summary

// summarizerSystemPrompt is the system prompt for the Haiku
// summarizer agent.
const summarizerSystemPrompt = `You are a concise activity summarizer ` +
	`for a multi-agent coding assistant system. Given a Claude Code ` +
	`session transcript, produce a brief summary.

Respond with EXACTLY two lines:
SUMMARY: <1-2 sentence description of what the agent is currently ` +
	`working on>
DELTA: <brief note on what changed since the previous summary, or ` +
	`"Initial summary" if no previous context>

Rules:
- Focus on WHAT the agent is doing, not implementation details
- Use present continuous tense ("Implementing...", "Debugging...")
- Keep each line under 150 characters
- If the transcript is empty or unclear, say "Agent idle"
- Do NOT use markdown, code blocks, or bullet points`

// buildSummaryPrompt constructs the user prompt for summarization,
// including the transcript and optional previous summary for delta
// tracking.
func buildSummaryPrompt(
	transcript string, previousSummary string,
) string {
	prompt := "Summarize this agent's current activity:\n\n"

	if previousSummary != "" {
		prompt += "Previous summary: " + previousSummary + "\n\n"
	}

	prompt += "--- TRANSCRIPT ---\n" + transcript + "\n--- END ---"

	return prompt
}
