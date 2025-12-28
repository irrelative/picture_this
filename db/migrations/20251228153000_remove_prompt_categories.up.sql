-- Remove prompt categories and de-duplicate by text
ALTER TABLE IF EXISTS games DROP COLUMN IF EXISTS prompt_category;

ALTER TABLE IF EXISTS prompt_library DROP COLUMN IF EXISTS category;
ALTER TABLE IF EXISTS prompt_libraries DROP COLUMN IF EXISTS category;

DROP INDEX IF EXISTS idx_prompt_library_category_text;

DO $$
BEGIN
  IF to_regclass('public.prompt_libraries') IS NOT NULL THEN
    EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_library_text ON prompt_libraries(text)';
  ELSIF to_regclass('public.prompt_library') IS NOT NULL THEN
    EXECUTE 'CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_library_text ON prompt_library(text)';
  END IF;
END
$$;
