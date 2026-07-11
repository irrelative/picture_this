DROP TABLE IF EXISTS likes;
ALTER TABLE rounds
  DROP COLUMN IF EXISTS reveal_stage,
  DROP COLUMN IF EXISTS active_drawing_index;
ALTER TABLE games
  DROP COLUMN IF EXISTS public_replay,
  DROP COLUMN IF EXISTS jokes_enabled,
  DROP COLUMN IF EXISTS audience_enabled,
  DROP COLUMN IF EXISTS avatars_enabled,
  DROP COLUMN IF EXISTS ruleset;
