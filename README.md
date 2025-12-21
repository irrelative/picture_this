# Picture This

Picture This is a Drawful-style party game with a Go backend, templ-rendered UI, and Postgres persistence. Players join from their phones, receive prompts from a prompt library, draw, guess, and vote while the host runs the lobby on a shared screen.

## How It Works
- Host creates a game and shares the join code.
- Players join from `/join` and receive assigned prompts.
- Audience can join from the home page to vote during guessing rounds.
- The game advances through drawing, guessing, guess-voting, and results phases.
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

When the server starts, it will auto-migrate and load prompts from `prompts.csv` if available.

## Configuration
- `PROMPTS_PER_PLAYER` — number of rounds to play.
- `DRAW_SECONDS` — time limit per drawing phase.
- `GUESS_SECONDS` — time limit per guessing phase.
- `VOTE_SECONDS` — time limit per vote phase.
- `REVEAL_SECONDS` — time per reveal step in results.

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
- `POST /api/games/{game_id}/votes` — submit a vote for the current drawing prompt.
- `POST /api/games/{game_id}/settings` — update lobby settings (rounds, max players, prompt pack, lock).
- `POST /api/games/{game_id}/kick` — host removes a player from the lobby.
- `POST /api/games/{game_id}/rename` — player updates their display name in the lobby.
- `POST /api/games/{game_id}/audience` — join as an audience member.
- `POST /api/games/{game_id}/audience/votes` — submit audience votes for a drawing.
- `POST /api/games/{game_id}/advance` — host/admin advances phase if needed.
- `GET /api/games/{game_id}/results` — fetch round or final results.
- `GET /api/games/{game_id}/events` — fetch event log for replay.
- `GET /api/prompts/categories` — list available prompt pack categories.
- `GET /ws/games/{game_id}` — websocket for realtime state/events.

## Game State Transition Flow (Draft)
- Phases: `lobby` -> `drawings` -> `guesses` -> `guesses-votes` -> `results` -> `complete`.
- `POST /api/games/{game_id}/start` moves `lobby` to `drawings`.
- Each round assigns one prompt per player from the prompt library.
- Prompts do not repeat within a game session.
- When all drawings are in, the game moves to `guesses` and walks each guess turn sequentially.
- After guesses complete, either a new round starts (if `PROMPTS_PER_PLAYER` > round count) or the game moves to `guesses-votes`.
- During `guesses-votes`, each drawing is shown with the prompt plus all guesses as options; players vote on the true prompt.
- Results are shown in `results` (guesses and votes per drawing), then the game is marked `complete`.

## Roadmap (Drawful Parity)
### Priority 1: Core Game Scoring & Flow
- Scoring rules (correct guesses, fooled players, edge cases) and score display.
- Timed phases with countdowns and auto-advance on timeouts.
- Reveal sequence per drawing (guesses, then votes) instead of one-shot results.

### Priority 2: Lobby & Round Configuration
- Host controls for rounds, player limits, and prompt pack selection.
- Prompt pack filtering and variety rules (no repeats within a game).
- Player management (kick/rename) and lobby readiness UX.

### Priority 3: Audience & Replay Features
- Audience mode for non-players with voting.
- Game replay view using event log for round-by-round playback.

### Priority 4: UX
- Drawing tool enhancements (brush sizes/colors, undo).
- Assign each player a consistent drawing color across the game.
- Accessibility pass and mobile polish (screen reader labels, touch affordances).

## Database Schema & ORM Plan (Draft)
- ORM: use GORM with the Postgres driver.
- Tables (initial):
  - `games` — `id`, `join_code`, `phase`, `created_at`, `updated_at`.
  - `players` — `id`, `game_id`, `name`, `is_host`, `joined_at`.
  - `rounds` — `id`, `game_id`, `number`, `status`, `created_at`.
  - `prompts` — `id`, `round_id`, `player_id`, `text`.
  - `drawings` — `id`, `round_id`, `player_id`, `prompt_id`, `image_data`.
  - `guesses` — `id`, `round_id`, `player_id`, `drawing_id`, `text`.
- `votes` — `id`, `round_id`, `player_id`, `drawing_id`, `choice_text`, `choice_type`.
- `events` — `id`, `game_id`, `round_id`, `player_id`, `type`, `payload`, `created_at`.
- Migrations: store SQL migrations under `db/migrations/`.

For now, don't include:
* Sound effects/music
* Voiceover. Instead, have the instructions printed to the WebUI


## Ideas
* htmx approach, does that work with ws?