# Audio Sources

This project uses short UI/gameplay SFX that are fetched locally (not committed).

## License Policy

- Default policy: **CC0 only** for sound effects.
- If a non-CC0 asset is used, include explicit attribution requirements in this file and in release notes.
- Never add assets with non-commercial (`NC`) or no-derivatives (`ND`) restrictions.

## Current SFX Pack

- Source page: `https://opengameart.org/content/pop-sounds-0`
- Author: Sara Garrard
- License: CC0

## File Mapping

The fetch script maps pack files to in-game names:

- `join.ogg` -> player joined / lightweight progress pings
- `round_start.ogg` -> stage transition stinger / reveal impact
- `timer_end.ogg` -> timer expiry and timeout transitions
- `voting_start.ogg` -> voting phase entry
- `drum_roll.ogg` -> lie/prompt sequential reveal drum roll
- `reveal_correct.ogg` -> correct prompt reveal
- `reveal_wrong.ogg` -> lie reveal

## How To Refresh

Run:

```bash
make fetch-sfx
```

This downloads into `static/sounds/` and overwrites local files.
