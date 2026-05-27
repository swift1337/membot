ALTER TABLE files ADD COLUMN basename TEXT;

CREATE INDEX IF NOT EXISTS idx_files_project_basename ON files(project_id, basename);
