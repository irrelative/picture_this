-- Bootstrap the original schema, including databases previously created by GORM.

CREATE TABLE IF NOT EXISTS games (
    id bigserial PRIMARY KEY,
    join_code varchar(12) NOT NULL, phase varchar(32) NOT NULL,
    prompts_per_player bigint NOT NULL DEFAULT 2, max_players bigint NOT NULL DEFAULT 0,
    lobby_locked boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS players (
    id bigserial PRIMARY KEY,
    game_id bigint NOT NULL REFERENCES games(id), name varchar(64) NOT NULL,
    avatar_image bytea, color varchar(16) NOT NULL DEFAULT '', is_host boolean NOT NULL DEFAULT false,
    joined_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rounds (
    id bigserial PRIMARY KEY,
    game_id bigint NOT NULL REFERENCES games(id), number bigint NOT NULL, status varchar(32) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS drawings (
    id bigserial PRIMARY KEY,
    round_id bigint NOT NULL REFERENCES rounds(id), player_id bigint NOT NULL REFERENCES players(id),
    prompt_id bigint NOT NULL, image_data bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS guesses (
    id bigserial PRIMARY KEY,
    round_id bigint NOT NULL REFERENCES rounds(id), player_id bigint NOT NULL REFERENCES players(id),
    drawing_id bigint NOT NULL, text varchar(280) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS votes (
    id bigserial PRIMARY KEY,
    round_id bigint NOT NULL REFERENCES rounds(id), player_id bigint NOT NULL REFERENCES players(id),
    drawing_id bigint NOT NULL DEFAULT 0, guess_id bigint NOT NULL DEFAULT 0,
    choice_text varchar(280) NOT NULL DEFAULT '', choice_type varchar(32) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS events (
    id bigserial PRIMARY KEY,
    game_id bigint NOT NULL REFERENCES games(id), round_id bigint REFERENCES rounds(id),
    player_id bigint REFERENCES players(id), type varchar(64) NOT NULL, payload jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id varchar(64) PRIMARY KEY, flash varchar(280), player_name varchar(64),
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_games_join_code ON games (join_code);

CREATE INDEX IF NOT EXISTS idx_players_game_id ON players (game_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_players_game_name ON players (game_id, name);

CREATE INDEX IF NOT EXISTS idx_rounds_game_id ON rounds (game_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_rounds_game_number ON rounds (game_id, number);

CREATE INDEX IF NOT EXISTS idx_drawings_round_id ON drawings (round_id);

CREATE INDEX IF NOT EXISTS idx_drawings_player_id ON drawings (player_id);

CREATE INDEX IF NOT EXISTS idx_drawings_prompt_id ON drawings (prompt_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drawings_round_player ON drawings (round_id, player_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_drawings_round_prompt ON drawings (round_id, prompt_id);

CREATE INDEX IF NOT EXISTS idx_guesses_round_id ON guesses (round_id);

CREATE INDEX IF NOT EXISTS idx_guesses_player_id ON guesses (player_id);

CREATE INDEX IF NOT EXISTS idx_guesses_drawing_id ON guesses (drawing_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_guesses_round_player_text ON guesses (round_id, player_id, text);

CREATE INDEX IF NOT EXISTS idx_votes_round_id ON votes (round_id);

CREATE INDEX IF NOT EXISTS idx_votes_player_id ON votes (player_id);

CREATE INDEX IF NOT EXISTS idx_votes_drawing_id ON votes (drawing_id);

CREATE INDEX IF NOT EXISTS idx_votes_guess_id ON votes (guess_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_round_player_drawing ON votes (round_id, player_id, drawing_id);

CREATE INDEX IF NOT EXISTS idx_events_game_id ON events (game_id);

CREATE INDEX IF NOT EXISTS idx_events_round_id ON events (round_id);

CREATE INDEX IF NOT EXISTS idx_events_player_id ON events (player_id);
