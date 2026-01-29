-- Drop triggers first.
DROP TRIGGER IF EXISTS messages_au;
DROP TRIGGER IF EXISTS messages_ad;
DROP TRIGGER IF EXISTS messages_ai;

-- Drop FTS table.
DROP TABLE IF EXISTS messages_fts;

-- Drop tables in reverse order of creation (respecting foreign keys).
DROP TABLE IF EXISTS session_identities;
DROP TABLE IF EXISTS consumer_offsets;
DROP TABLE IF EXISTS message_recipients;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS agents;
