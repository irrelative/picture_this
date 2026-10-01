CREATE TABLE IF NOT EXISTS prompts (
    id bigserial PRIMARY KEY,
    round_id bigint NOT NULL REFERENCES rounds(id), player_id bigint NOT NULL REFERENCES players(id),
    text varchar(280) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_prompts_round_id ON prompts(round_id);
CREATE INDEX IF NOT EXISTS idx_prompts_player_id ON prompts(player_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_prompts_round_player_text ON prompts(round_id, player_id, text);

CREATE TABLE IF NOT EXISTS prompt_libraries (
    id bigserial PRIMARY KEY, text varchar(280) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_library_text ON prompt_libraries(text);
