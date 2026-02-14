CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE prompt_libraries
  ADD COLUMN IF NOT EXISTS embedding vector(1536);

CREATE INDEX IF NOT EXISTS idx_prompt_libraries_embedding
  ON prompt_libraries
  USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 100);
