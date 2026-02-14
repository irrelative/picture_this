DROP INDEX IF EXISTS idx_prompt_libraries_embedding;

ALTER TABLE prompt_libraries
  DROP COLUMN IF EXISTS embedding;
