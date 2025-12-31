ALTER TABLE prompt_libraries
  ADD COLUMN joke varchar(280);

ALTER TABLE prompts
  ADD COLUMN joke varchar(280);
