#!/usr/bin/env python3
import argparse
import os
from pathlib import Path

import psycopg2 as psycopg
import torch


def allow_xtts_globals():
    try:
        from TTS.tts.configs.xtts_config import XttsConfig
        from TTS.tts.models.xtts import XttsArgs, XttsAudioConfig
        from TTS.config.shared_configs import BaseDatasetConfig
    except Exception:
        return
    if hasattr(torch.serialization, "add_safe_globals"):
        torch.serialization.add_safe_globals([XttsConfig, XttsArgs, XttsAudioConfig, BaseDatasetConfig])


def parse_args():
    parser = argparse.ArgumentParser(description="Generate MP3 audio for prompt jokes.")
    parser.add_argument("--db-url", default=os.getenv("DATABASE_URL"), help="Postgres connection string")
    parser.add_argument("--output-dir", default="static/audio/jokes", help="Directory to write MP3 files")
    parser.add_argument("--public-prefix", default="/static/audio/jokes", help="Path stored in DB")
    parser.add_argument("--model", default=os.getenv("COQUI_TTS_MODEL", "tts_models/en/vctk/vits"), help="Coqui TTS model name")
    parser.add_argument("--speaker", default=os.getenv("COQUI_TTS_SPEAKER"), help="Optional speaker name/id")
    parser.add_argument("--list-speakers", action="store_true", help="List available speakers for the model and exit")
    parser.add_argument("--language", default=os.getenv("COQUI_TTS_LANGUAGE"), help="Optional language code")
    parser.add_argument("--limit", type=int, default=0, help="Max prompts to process (0 = no limit)")
    return parser.parse_args()


def main():
    args = parse_args()
    cache_root = Path(".cache").resolve()
    (cache_root / "matplotlib").mkdir(parents=True, exist_ok=True)
    (cache_root / "fontconfig").mkdir(parents=True, exist_ok=True)
    os.environ.setdefault("XDG_CACHE_HOME", str(cache_root))
    os.environ.setdefault("MPLCONFIGDIR", str(cache_root / "matplotlib"))
    os.environ.setdefault("FC_CACHEDIR", str(cache_root / "fontconfig"))

    allow_xtts_globals()
    from TTS.api import TTS
    tts = TTS(model_name=args.model, progress_bar=False, gpu=False)
    if args.list_speakers:
        available = getattr(tts, "speakers", None) or []
        if not available:
            print("No speakers exposed by this model.")
        else:
            for speaker in available:
                print(speaker)
        return

    if not args.db_url:
        raise SystemExit("DATABASE_URL is required (or pass --db-url)")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    speaker = args.speaker
    if not speaker:
        available = getattr(tts, "speakers", None)
        if available:
            speaker = available[0]
            print(f"Using default speaker: {speaker}")

    query = """
        SELECT id, text, joke
        FROM prompt_libraries
        WHERE joke IS NOT NULL AND trim(joke) <> ''
          AND (joke_audio_path IS NULL OR trim(joke_audio_path) = '')
        ORDER BY id ASC
    """
    if args.limit and args.limit > 0:
        query += " LIMIT %s"

    with psycopg.connect(args.db_url) as conn:
        with conn.cursor() as cur:
            if args.limit and args.limit > 0:
                cur.execute(query, (args.limit,))
            else:
                cur.execute(query)
            rows = cur.fetchall()
            for prompt_id, prompt_text, joke in rows:
                filename = f"promptlib_{prompt_id}.mp3"
                file_path = output_dir / filename
                public_path = f"{args.public_prefix.rstrip('/')}/{filename}"

                tts.tts_to_file(
                    text=joke,
                    file_path=str(file_path),
                    speaker=speaker,
                    language=args.language,
                )

                cur.execute(
                    "UPDATE prompt_libraries SET joke_audio_path = %s WHERE id = %s",
                    (public_path, prompt_id),
                )
                cur.execute(
                    """
                    UPDATE prompts
                    SET joke_audio_path = %s
                    WHERE joke_audio_path IS NULL
                      AND text = %s
                    """,
                    (public_path, prompt_text),
                )
            conn.commit()

    print(f"Generated {len(rows)} audio file(s).")


if __name__ == "__main__":
    main()
