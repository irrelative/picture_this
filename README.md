# Picture This

Picture This is a Drawful-style party game with a Go backend, templ-rendered UI, and Postgres persistence. Players join from their phones, receive prompts from a prompt library, draw, guess, and vote while the host runs the lobby on a shared screen.

## How It Works
- Host creates a game and shares the join code.
- Players join from `/join` and receive assigned prompts.
- The game advances through drawing, guessing, and voting phases.
- Results are shown after each round, with state synced via websockets.

## Tech Stack
This project uses the following technology:

* Golang backend (Go 1.25.5)
* Golang templ templating for WebUI and mobile interfaces
* Websockets used for server and client syncing
* Minimal javascipt, no frameworks
* WebUI frontend for managing the game, showing "secret" code
* Postgres for backend state storage of game, drawings, etc. The game should be able to crash and be restarted without losing game state.

## Getting Started
1. Copy the example env file: `cp .env.example .env`
2. Update `DATABASE_URL` in `.env`.
3. Run migrations: `make migrate`
4. Start the server: `make run`
5. Open `http://localhost:8080` to create a game.

## Dev Commands
- `make run` — generate templ output and start the server.
- `make build` — generate templ output and build all packages.
- `make test` — run all tests.
- `make migrate` — apply SQL migrations in `db/migrations/`.
- `make migrate-create name=add_table` — create a new migration pair.

## Planned Server Endpoints (Draft)
- `POST /api/games` — create a new game; returns `game_id` and `join_code`.
- `POST /api/games/{game_id}/join` — join a game with a player name.
- `GET /api/games/{game_id}` — fetch a state snapshot for reconnects.
- `POST /api/games/{game_id}/start` — host starts the game.
- `POST /api/games/{game_id}/drawings` — submit a drawing for a prompt.
- `POST /api/games/{game_id}/guesses` — submit a guess for a drawing.
- `POST /api/games/{game_id}/votes` — submit vote(s) for the round.
- `POST /api/games/{game_id}/advance` — host/admin advances phase if needed.
- `GET /api/games/{game_id}/results` — fetch round or final results.
- `GET /ws/games/{game_id}` — websocket for realtime state/events.

## Game State Transition Flow (Draft)
- Phases: `lobby` -> `drawings` -> `guesses` -> `votes` -> `results` -> `complete`.
- `POST /api/games/{game_id}/start` moves `lobby` to `drawings`.
- `POST /api/games/{game_id}/advance` moves to the next phase in order.
- When a phase is completed, submissions lock and the host advances.
- Results are shown in `results`, then the game is marked `complete`.

## Database Schema & ORM Plan (Draft)
- ORM: use GORM with the Postgres driver.
- Tables (initial):
  - `games` — `id`, `join_code`, `phase`, `created_at`, `updated_at`.
  - `players` — `id`, `game_id`, `name`, `is_host`, `joined_at`.
  - `rounds` — `id`, `game_id`, `number`, `status`, `created_at`.
  - `prompts` — `id`, `round_id`, `player_id`, `text`.
  - `drawings` — `id`, `round_id`, `player_id`, `prompt_id`, `image_data`.
  - `guesses` — `id`, `round_id`, `player_id`, `drawing_id`, `text`.
- `votes` — `id`, `round_id`, `player_id`, `guess_id`.
- `events` — `id`, `game_id`, `round_id`, `player_id`, `type`, `payload`, `created_at`.
- Migrations: store SQL migrations under `db/migrations/`.

For now, don't include:
* Sound effects/music
* Voiceover. Instead, have the instructions printed to the WebUI
