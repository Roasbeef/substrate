-- Add discovery metadata columns to agents table.
-- These columns enable agent discovery by providing richer context about
-- what each agent is doing and where it's located.
ALTER TABLE agents ADD COLUMN purpose TEXT DEFAULT '';
ALTER TABLE agents ADD COLUMN working_dir TEXT DEFAULT '';
ALTER TABLE agents ADD COLUMN hostname TEXT DEFAULT '';

-- Add compound index on message_recipients for efficient unread count
-- lookups used by the DiscoverAgents query.
CREATE INDEX IF NOT EXISTS idx_recipients_agent_state
    ON message_recipients(agent_id, state);
