#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST_DIR="${ROOT_DIR}/static/sounds"

mkdir -p "${DEST_DIR}"

# Source: OpenGameArt "Pop sounds 0" by Sara Garrard.
# License: CC0.
# https://opengameart.org/content/pop-sounds-0
declare -a FILES=(
  "join.ogg|https://opengameart.org/sites/default/files/audio_preview/pop2.wav.ogg"
  "round_start.ogg|https://opengameart.org/sites/default/files/audio_preview/pop1.wav.ogg"
  "timer_end.ogg|https://opengameart.org/sites/default/files/audio_preview/pop9.wav.ogg"
  "voting_start.ogg|https://opengameart.org/sites/default/files/audio_preview/pop6.wav.ogg"
  "drum_roll.ogg|https://opengameart.org/sites/default/files/audio_preview/pop7.wav.ogg"
  "reveal_correct.ogg|https://opengameart.org/sites/default/files/audio_preview/pop3.wav.ogg"
  "reveal_wrong.ogg|https://opengameart.org/sites/default/files/audio_preview/pop8.wav.ogg"
)

echo "Fetching SFX into ${DEST_DIR}"
for item in "${FILES[@]}"; do
  name="${item%%|*}"
  url="${item##*|}"
  out="${DEST_DIR}/${name}"
  echo "  - ${name}"
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 15 "${url}" -o "${out}"
done

echo "Done."
