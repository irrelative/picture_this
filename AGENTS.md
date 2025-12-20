# Repository Guidelines

## Project Structure & Module Organization
- Current structure: `cmd/server` for the entrypoint, `README.md` for scope, `AGENTS.md` for contributor guidance, and `Makefile` for dev targets.
- Planned stack (per README): Go backend, `templ` templates for WebUI/mobile, minimal JavaScript, and Postgres for persistent game state.
- As code is added, keep clear top-level separation, e.g. `internal/` for app logic, `web/` or `ui/` for templates/assets, and `db/` for schema/migrations.

## Build, Test, and Development Commands
- `make run` — run all Go packages (updates to target `./cmd/server` can follow once the entrypoint stabilizes).
- `make build` — build all Go packages.
- `go test ./...` — run all Go tests.
- `templ generate` — regenerate templates if `templ` is used.

## Coding Style & Naming Conventions
- Go code should be formatted with `gofmt` and follow standard Go naming (exported `CamelCase`, unexported `camelCase`).
- Keep JavaScript minimal and framework-free as stated in `README.md`.
- Prefer clear package names that match responsibility (e.g., `game`, `storage`, `web`).

## Testing Guidelines
- Run `make test` and `make e2e-test` before committing changes.
- Use Go’s `_test.go` naming and table-driven tests where appropriate.
- Favor integration tests for Postgres-backed flows once persistence is implemented.

## Commit & Pull Request Guidelines
- Use Conventional Commits (e.g., `feat: add game state model`, `chore: update Makefile`).
- When working as Codex, run `make test` and `make e2e-test`, then `git add` and `git commit` after each change to make rollback easy.
- PRs should describe the change, link any related issues, and include screenshots for WebUI changes.

## Security & Configuration Tips
- Do not hardcode credentials; use environment variables for Postgres connection details.
- Ensure the game can restart without losing state, per README scope.
