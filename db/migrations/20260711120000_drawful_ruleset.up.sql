ALTER TABLE games
  ADD COLUMN IF NOT EXISTS ruleset varchar(32) NOT NULL DEFAULT 'picture_this_v1',
  ADD COLUMN IF NOT EXISTS avatars_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS audience_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS jokes_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS public_replay boolean NOT NULL DEFAULT false;

ALTER TABLE rounds
  ADD COLUMN IF NOT EXISTS active_drawing_index integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS reveal_stage varchar(32) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS likes (
  id bigserial PRIMARY KEY,
  round_id bigint NOT NULL REFERENCES rounds(id) ON DELETE CASCADE,
  player_id bigint NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  drawing_id bigint NOT NULL REFERENCES drawings(id) ON DELETE CASCADE,
  guess_owner_id bigint NOT NULL REFERENCES players(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT idx_likes_round_player_drawing_owner UNIQUE
    (round_id, player_id, drawing_id, guess_owner_id)
);

CREATE INDEX IF NOT EXISTS idx_likes_round_id ON likes(round_id);
CREATE INDEX IF NOT EXISTS idx_likes_drawing_id ON likes(drawing_id);
