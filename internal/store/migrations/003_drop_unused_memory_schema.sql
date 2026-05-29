DROP TRIGGER IF EXISTS memory_items_ai;
DROP TRIGGER IF EXISTS memory_items_ad;
DROP TRIGGER IF EXISTS memory_items_au;
DROP TRIGGER IF EXISTS artifacts_ai;
DROP TRIGGER IF EXISTS artifacts_ad;
DROP TRIGGER IF EXISTS artifacts_au;

DROP TABLE IF EXISTS memory_fts;
DROP TABLE IF EXISTS artifact_fts;

DROP TABLE IF EXISTS conversation_topics;
DROP TABLE IF EXISTS topics;
DROP TABLE IF EXISTS entity_mentions;
DROP TABLE IF EXISTS entities;
DROP TABLE IF EXISTS task_items;
DROP TABLE IF EXISTS memory_items;
DROP TABLE IF EXISTS patches;
DROP TABLE IF EXISTS artifacts;

ALTER TABLE tool_calls DROP COLUMN output_artifact_id;
