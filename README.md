# Picture This

This is a port of the game Drawful, as best as possible, using the following technology:

* Golang backend (Go 1.25.5)
* Golang templ templating for WebUI and mobile interfaces
* Websockets used for server and client syncing
* Minimal javascipt, no frameworks
* WebUI frontend for managing the game, showing "secret" code
* Postgres for backend state storage of game, drawings, etc. The game should be able to crash and be restarted without losing game state.

## Planned Server Endpoints (Draft)
- `POST /api/games` — create a new game; returns `game_id` and `join_code`.
- `POST /api/games/{game_id}/join` — join a game with a player name.
- `GET /api/games/{game_id}` — fetch a state snapshot for reconnects.
- `POST /api/games/{game_id}/start` — host starts the game.
- `POST /api/games/{game_id}/prompts` — submit prompts for the round.
- `POST /api/games/{game_id}/drawings` — submit a drawing for a prompt.
- `POST /api/games/{game_id}/guesses` — submit a guess for a drawing.
- `POST /api/games/{game_id}/votes` — submit vote(s) for the round.
- `POST /api/games/{game_id}/advance` — host/admin advances phase if needed.
- `GET /api/games/{game_id}/results` — fetch round or final results.
- `GET /ws/games/{game_id}` — websocket for realtime state/events.

## Database Schema & ORM Plan (Draft)
- ORM: use `sqlc` (type-safe SQL) over a full ORM to keep queries explicit and fast.
- Tables (initial):
  - `games` — `id`, `join_code`, `phase`, `created_at`, `updated_at`.
  - `players` — `id`, `game_id`, `name`, `is_host`, `joined_at`.
  - `rounds` — `id`, `game_id`, `number`, `status`, `created_at`.
  - `prompts` — `id`, `round_id`, `player_id`, `text`.
  - `drawings` — `id`, `round_id`, `player_id`, `prompt_id`, `image_data`.
  - `guesses` — `id`, `round_id`, `player_id`, `drawing_id`, `text`.
  - `votes` — `id`, `round_id`, `player_id`, `guess_id`.
- Migrations: store SQL migrations under `db/migrations/`.

For now, don't include:
* Sound effects/music
* Voiceover. Instead, have the instructions printed to the WebUI
