-- Add discovery metadata columns to agents table.
-- These columns enable agent discovery by providing richer context about
-- what each agent is doing and where it's located.
ALTER TABLE agents ADD COLUMN purpose TEXT DEFAULT '';
ALTER TABLE agents ADD COLUMN working_dir TEXT DEFAULT '';
ALTER TABLE agents ADD COLUMN hostname TEXT DEFAULT '';
