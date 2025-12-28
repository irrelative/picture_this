-- Restore prompt categories
ALTER TABLE IF EXISTS games ADD COLUMN IF NOT EXISTS prompt_category varchar(64) NOT NULL DEFAULT '';

ALTER TABLE IF EXISTS prompt_library ADD COLUMN IF NOT EXISTS category varchar(64) NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS prompt_libraries ADD COLUMN IF NOT EXISTS category varchar(64) NOT NULL DEFAULT '';

DROP INDEX IF EXISTS idx_prompt_library_text;

DO $$
BEGIN
  IF to_regclass('public.prompt_libraries') IS NOT NULL THEN
    EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_library_category_text ON prompt_libraries(category, text)';
  ELSIF to_regclass('public.prompt_library') IS NOT NULL THEN
    EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_library_category_text ON prompt_library(category, text)';
  END IF;
END
$$;
