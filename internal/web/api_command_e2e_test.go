package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCommandFeedEndpoint verifies the aggregated command feed over
// HTTP: lanes group classified agent traffic and actionable items land
// in the attention queue.
func TestCommandFeedEndpoint(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	// The feed keys off the human User agent's inbox.
	h.createTestAgent(UserAgentName)
	workerID := h.createTestAgent("Worker")
	h.createTestAgent("Quiet")

	// An urgent question and a status update from Worker to User.
	sendReq := map[string]interface{}{
		"sender_id":       workerID,
		"recipient_names": []string{UserAgentName},
		"subject":         "Decision needed: cache strategy",
		"body":            "LRU or LFU?",
		"priority":        3, // PRIORITY_URGENT.
	}
	status, body, err := h.httpPost(
		h.apiURL("/api/v1/messages"), sendReq,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	statusReq := map[string]interface{}{
		"sender_id":       workerID,
		"recipient_names": []string{UserAgentName},
		"subject":         "[Status] Worker",
		"body": "Implemented cache layer.\n\n" +
			"Waiting for: cache strategy decision",
		"priority": 2, // PRIORITY_NORMAL.
	}
	status, body, err = h.httpPost(
		h.apiURL("/api/v1/messages"), statusReq,
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	// Fetch the command feed.
	status, body, err = h.httpGet(h.apiURL("/api/v1/command/feed"))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status, "body=%s", string(body))

	var feed CommandFeedResponse
	require.NoError(t, json.Unmarshal(body, &feed))

	// The User agent must not get a lane; Worker and Quiet do.
	require.Len(t, feed.Lanes, 2)
	for _, lane := range feed.Lanes {
		require.NotEqual(t, UserAgentName, lane.Agent.Name)
	}

	// Worker sorts first: it has an actionable question.
	worker := feed.Lanes[0]
	require.Equal(t, "Worker", worker.Agent.Name)
	require.Len(t, worker.Events, 2)
	require.Equal(t, 1, worker.NeedsAction)
	require.Equal(
		t, "cache strategy decision", worker.WaitingFor,
	)

	// Both events are present with the right classification. Creation
	// timestamps can tie at second granularity, so match by kind
	// instead of position.
	kinds := make(map[string]CommandEvent, 2)
	for _, ev := range worker.Events {
		kinds[ev.Kind] = ev
	}
	require.Contains(t, kinds, eventKindStatus)
	require.Contains(t, kinds, eventKindQuestion)
	require.True(t, kinds[eventKindQuestion].NeedsAction)

	// The question surfaces in the attention queue.
	require.NotEmpty(t, feed.Attention)
	require.Equal(t, attentionKindQuestion, feed.Attention[0].Kind)
	require.Equal(t, "Worker", feed.Attention[0].AgentName)

	// Method guard: POST is rejected.
	status, _, err = h.httpPost(
		h.apiURL("/api/v1/command/feed"), map[string]string{},
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusMethodNotAllowed, status)
}

// TestCommandFeedEndpointNoUser verifies the feed degrades to an empty
// response when no User agent exists yet.
func TestCommandFeedEndpointNoUser(t *testing.T) {
	h := newGatewayTestHarness(t)
	defer h.Close()

	status, body, err := h.httpGet(
		h.apiURL("/api/v1/command/feed"),
	)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	var feed CommandFeedResponse
	require.NoError(t, json.Unmarshal(body, &feed))
	require.Empty(t, feed.Lanes)
	require.Empty(t, feed.Attention)
}
