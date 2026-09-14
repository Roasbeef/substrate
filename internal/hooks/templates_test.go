package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStopScriptArmsWithoutEndingTheCurrentTurn protects the watcher contract.
// The Stop hook is reached when an agent tries to finish a turn, but its
// watcher is only an idle-notification mechanism. Arming it must not direct
// an agent to abandon work that is still in progress.
func TestStopScriptArmsWithoutEndingTheCurrentTurn(t *testing.T) {
	require.Contains(t, StopScript,
		"it does not require ending your turn. Continue the current task if work remains",
	)
	require.NotContains(t, StopScript, "then end your turn")

	installed, ok := AllScripts()["stop"]
	require.True(t, ok)
	require.Equal(t, StopScript, installed)
	require.Equal(t, StopScript, GetScript("stop"))
}
