-- Down migration for 000005_agent_tasks
DROP VIEW IF EXISTS available_tasks;
DROP TABLE IF EXISTS agent_tasks;
DROP TABLE IF EXISTS task_lists;
