ALTER TABLE prompt_libraries
  ADD COLUMN joke_audio_path varchar(280);

ALTER TABLE prompts
  ADD COLUMN joke_audio_path varchar(280);
