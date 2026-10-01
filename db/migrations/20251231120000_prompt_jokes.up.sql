ALTER TABLE prompt_libraries
  ADD COLUMN IF NOT EXISTS joke varchar(280);

ALTER TABLE prompts
  ADD COLUMN IF NOT EXISTS joke varchar(280);
