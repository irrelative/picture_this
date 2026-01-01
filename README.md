# Picture This

Picture This is a Drawful-style party game with a Go backend, templ-rendered UI, and Postgres persistence. Players join from their phones, receive prompts from a prompt library, draw, guess, and vote while the host runs the lobby on a shared screen.

## How It Works
- Host creates a game and shares the join code.
- Players join from `/join` and receive assigned prompts.
- Audience can join from the home page to vote during guessing rounds.
- The game advances through drawing, guessing, guess-voting, and per-drawing results phases.
- Results are shown after each drawing, with final results after all rounds and state synced via websockets.

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
3. Run initialization assets: `make init`
4. Run migrations: `make migrate`
5. Start the server: `make run`
6. Open `http://localhost:8080` to create a game.

When the server starts, it will auto-migrate and load prompts from `prompts.csv` if available.

## Configuration
- `PROMPTS_PER_PLAYER` — number of rounds to play.
- `DRAW_SECONDS` — time limit per drawing phase.
- `GUESS_SECONDS` — time limit per guessing phase.
- `VOTE_SECONDS` — time limit per vote phase.
- `REVEAL_SECONDS` — time per reveal step in results.

## Dev Commands
- `make init` — download local sound effects + vendor assets for the display view.
- `make run` — generate templ output and start the server.
- `make build` — generate templ output and build all packages.
- `make test` — run all tests.
- `make migrate` — apply SQL migrations in `db/migrations/`.
- `make migrate-create name=add_table` — create a new migration pair.

## Deployment (Ubuntu 24.04 VPS)
This repo includes a simple root-run setup script plus nginx/supervisor configs.

Prereqs:
- DNS A record for your domain points at the VPS.
- Ports 80/443 open in your firewall/security group.

From the repo on the server (as root):
```
DOMAIN=example.com \
DB_PASS='strong-password' \
./scripts/setup_vps.sh
```

Optional overrides:
- `APP_USER` (default `picturethis`)
- `APP_DIR` (default `/opt/picture-this`)
- `APP_PORT` (default `8080`)
- `DB_NAME` (default `picture_this`)
- `DB_USER` (default `picture_this`)
- `APP_ENV` (default `production`)
- `SKIP_BUILD=1` (skip the Go build step)

What it does:
- Installs nginx, postgres, supervisor, openssl, and Go.
- Generates a self-signed TLS certificate (valid for 10 years).
- Creates the app user and Postgres role/database.
- Writes `.env` with the configured `DATABASE_URL`.
- Builds `./cmd/server` into `bin/picture-this`.
- Installs nginx and supervisor configs from `deploy/`.
- Reloads nginx with the TLS config.

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
- Phases: `lobby` -> `drawings` -> (`guesses` -> `guesses-votes` -> `results`) per drawing -> `complete`.
- `POST /api/games/{game_id}/start` moves `lobby` to `drawings`.
- Each round assigns one prompt per player from the prompt library.
- Prompts do not repeat within a game session.
- When all drawings are in, the game moves to `guesses` and walks each guess turn per drawing.
- After a drawing's guesses complete, the game moves to `guesses-votes` for that drawing.
- After voting completes for that drawing, `results` reveals guesses and votes for that drawing (guesses first, then votes).
- When all drawings in the round have been revealed, a new round starts (if `PROMPTS_PER_PLAYER` > round count) or the game moves to `complete` for final results.

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

## Sound Effects
Display-mode sound effects are pulled from OpenGameArt:
- Join: https://opengameart.org/content/pop-sounds-0 (pop2.wav.ogg) -> `static/sounds/join.ogg`
- Round start: https://opengameart.org/content/pop-sounds-0 (pop1.wav) -> `static/sounds/round_start.ogg`
- Timer ending: https://opengameart.org/content/pop-sounds-0 (pop9.wav) -> `static/sounds/timer_end.ogg`
- Voting start: https://opengameart.org/content/ui-accept-or-forward (Accept.mp3) -> `static/sounds/voting_start.mp3`

Please review licensing requirements at the source before distribution.

For now, don't include:
* Voiceover. Instead, have the instructions printed to the WebUI


## TODO
* Music and SFX generation
* Make the jokes snarkier, and the prompts wittier
* Pagination metadata for navigation (# of pages, etc)
* Search for admin view (prompts)


## Ideas
* Vectors for prompts to ensure no overlaps
