#!/usr/bin/env python3
import argparse
import inspect
import os
import re
import shutil
import subprocess
import tempfile
from collections.abc import Iterable
from pathlib import Path

import psycopg2 as psycopg
import torch

DEFAULT_MODEL = "tts_models/multilingual/multi-dataset/xtts_v2"
DEFAULT_LANGUAGE = "en"
DEFAULT_XTTS_SPEAKER = "Rosemary Okafor"


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
    parser = argparse.ArgumentParser(description="Generate narration audio for prompt jokes.")
    parser.add_argument("--db-url", default=os.getenv("DATABASE_URL"), help="Postgres connection string")
    parser.add_argument("--output-dir", default="static/audio/jokes", help="Directory to write audio files")
    parser.add_argument("--public-prefix", default="/static/audio/jokes", help="Path stored in DB")
    parser.add_argument("--model", default=os.getenv("COQUI_TTS_MODEL", DEFAULT_MODEL), help="Coqui TTS model name")
    parser.add_argument("--device", choices=["auto", "cpu", "mps", "cuda"], default=os.getenv("COQUI_TTS_DEVICE", "auto"), help="Torch device")
    parser.add_argument("--speaker", default=os.getenv("COQUI_TTS_SPEAKER"), help="Optional speaker name/id")
    parser.add_argument("--speaker-wav", default=os.getenv("COQUI_TTS_SPEAKER_WAV"), help="Optional reference WAV path for voice cloning")
    parser.add_argument("--list-speakers", action="store_true", help="List available speakers for the model and exit")
    parser.add_argument("--list-languages", action="store_true", help="List available languages for the model and exit")
    parser.add_argument("--language", default=os.getenv("COQUI_TTS_LANGUAGE", DEFAULT_LANGUAGE), help="Optional language code")
    parser.add_argument("--speed", type=float, default=float(os.getenv("COQUI_TTS_SPEED", "0.96")), help="Speech speed (if model supports it)")
    parser.add_argument("--temperature", type=float, default=float(os.getenv("COQUI_TTS_TEMPERATURE", "0.35")), help="Sampling temperature (if supported)")
    parser.add_argument("--top-p", type=float, default=float(os.getenv("COQUI_TTS_TOP_P", "0.85")), help="Nucleus sampling top_p (if supported)")
    parser.add_argument("--top-k", type=int, default=int(os.getenv("COQUI_TTS_TOP_K", "50")), help="Sampling top_k (if supported)")
    parser.add_argument("--repetition-penalty", type=float, default=float(os.getenv("COQUI_TTS_REPETITION_PENALTY", "5.0")), help="Repetition penalty (if supported)")
    parser.add_argument("--split-sentences", action=argparse.BooleanOptionalAction, default=True, help="Enable sentence splitting where supported")
    parser.add_argument("--output-format", choices=["mp3", "wav"], default=os.getenv("COQUI_TTS_OUTPUT_FORMAT", "mp3"), help="Output file format")
    parser.add_argument("--mp3-bitrate", default=os.getenv("COQUI_TTS_MP3_BITRATE", "128k"), help="MP3 bitrate for ffmpeg encoding")
    parser.add_argument("--ffmpeg-bin", default=os.getenv("FFMPEG_BIN", "ffmpeg"), help="ffmpeg binary path")
    parser.add_argument("--normalize-loudness", action=argparse.BooleanOptionalAction, default=True, help="Apply ffmpeg loudness normalization when writing mp3")
    parser.add_argument("--max-chars", type=int, default=220, help="Maximum joke characters to synthesize")
    parser.add_argument("--limit", type=int, default=0, help="Max prompts to process (0 = no limit)")
    parser.add_argument("--force", action="store_true", help="Regenerate even if joke_audio_path is already set")
    parser.add_argument("--ids", default="", help="Comma-separated prompt_library IDs to process")
    parser.add_argument("--dry-run", action="store_true", help="Show what would be processed without writing files/DB")
    parser.add_argument("--continue-on-error", action=argparse.BooleanOptionalAction, default=True, help="Continue processing even if one row fails")
    args = parser.parse_args()
    args.ids = parse_ids(args.ids)
    return args


def parse_ids(value):
    if not value:
        return []
    ids = []
    for raw in value.split(","):
        raw = raw.strip()
        if not raw:
            continue
        try:
            ids.append(int(raw))
        except ValueError as exc:
            raise SystemExit(f"invalid --ids value '{raw}': expected integers") from exc
    return ids


def choose_device(requested):
    if requested != "auto":
        return requested
    if hasattr(torch.backends, "mps") and torch.backends.mps.is_available():
        return "mps"
    if torch.cuda.is_available():
        return "cuda"
    return "cpu"


def normalize_joke_text(text, max_chars):
    cleaned = re.sub(r"\s+", " ", text or "").strip()
    cleaned = cleaned.strip("`\"' ")
    cleaned = cleaned.replace("“", "\"").replace("”", "\"").replace("’", "'")
    if len(cleaned) > max_chars:
        clipped = cleaned[:max_chars].rsplit(" ", 1)[0].strip()
        cleaned = clipped if clipped else cleaned[:max_chars]
    if cleaned and cleaned[-1] not in ".!?":
        cleaned += "."
    return cleaned


def build_select_query(args):
    clauses = [
        "joke IS NOT NULL",
        "trim(joke) <> ''",
    ]
    params = []
    if not args.force:
        clauses.append("(joke_audio_path IS NULL OR trim(joke_audio_path) = '')")
    if args.ids:
        clauses.append("id = ANY(%s)")
        params.append(args.ids)

    query = f"""
        SELECT id, text, joke
        FROM prompt_libraries
        WHERE {" AND ".join(clauses)}
        ORDER BY id ASC
    """
    if args.limit and args.limit > 0:
        query += " LIMIT %s"
        params.append(args.limit)
    return query, params


def build_tts_kwargs(tts, args, text, wav_path, speaker):
    accepted = set(inspect.signature(tts.tts_to_file).parameters)
    kwargs = {
        "text": text,
        "file_path": str(wav_path),
    }
    optional = {
        "speaker": speaker,
        "speaker_wav": args.speaker_wav,
        "language": args.language,
        "speed": args.speed,
        "split_sentences": args.split_sentences,
        "temperature": args.temperature,
        "top_p": args.top_p,
        "top_k": args.top_k,
        "repetition_penalty": args.repetition_penalty,
    }
    for key, value in optional.items():
        if key in accepted and value is not None:
            kwargs[key] = value
    return kwargs


def _iter_speaker_values(raw):
    if raw is None:
        return []
    if isinstance(raw, dict):
        return raw.keys()
    if isinstance(raw, (str, bytes)):
        return [raw]
    if isinstance(raw, Iterable):
        return raw
    return [raw]


def discover_speakers(tts):
    speakers = []
    seen = set()

    def add_many(values):
        for value in values:
            if value is None:
                continue
            candidate = str(value).strip()
            if not candidate or candidate in seen:
                continue
            seen.add(candidate)
            speakers.append(candidate)

    add_many(_iter_speaker_values(getattr(tts, "speakers", None)))
    synthesizer = getattr(tts, "synthesizer", None)
    tts_model = getattr(synthesizer, "tts_model", None)
    speaker_manager = getattr(tts_model, "speaker_manager", None)
    if speaker_manager is not None:
        add_many(_iter_speaker_values(getattr(speaker_manager, "speaker_names", None)))
        add_many(_iter_speaker_values(getattr(speaker_manager, "speakers", None)))
        add_many(_iter_speaker_values(getattr(speaker_manager, "name_to_id", None)))

    return speakers


def encode_mp3(ffmpeg_bin, input_wav, output_mp3, bitrate, normalize_loudness):
    cmd = [ffmpeg_bin, "-hide_banner", "-loglevel", "error", "-y", "-i", str(input_wav)]
    if normalize_loudness:
        cmd.extend(["-af", "loudnorm=I=-16:TP=-1.5:LRA=11"])
    cmd.extend(["-vn", "-ar", "44100", "-ac", "1", "-codec:a", "libmp3lame", "-b:a", bitrate, str(output_mp3)])
    subprocess.run(cmd, check=True)


def main():
    args = parse_args()
    cache_root = Path(".cache").resolve()
    (cache_root / "matplotlib").mkdir(parents=True, exist_ok=True)
    (cache_root / "fontconfig").mkdir(parents=True, exist_ok=True)
    os.environ.setdefault("XDG_CACHE_HOME", str(cache_root))
    os.environ.setdefault("MPLCONFIGDIR", str(cache_root / "matplotlib"))
    os.environ.setdefault("FC_CACHEDIR", str(cache_root / "fontconfig"))

    if args.speaker_wav:
        speaker_wav = Path(args.speaker_wav).expanduser()
        if not speaker_wav.exists():
            raise SystemExit(f"speaker wav not found: {speaker_wav}")
        args.speaker_wav = str(speaker_wav)

    ffmpeg_bin = None
    if args.output_format == "mp3":
        ffmpeg_bin = shutil.which(args.ffmpeg_bin) or args.ffmpeg_bin
        try:
            probe = subprocess.run(
                [ffmpeg_bin, "-version"],
                check=False,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        except FileNotFoundError as exc:
            raise SystemExit(
                f"ffmpeg is required for mp3 output (tried '{args.ffmpeg_bin}'). "
                "Install ffmpeg or use --output-format wav."
            ) from exc
        if probe.returncode != 0:
            raise SystemExit(
                f"ffmpeg is required for mp3 output (tried '{args.ffmpeg_bin}'). "
                "Install ffmpeg or use --output-format wav."
            )

    allow_xtts_globals()
    from TTS.api import TTS

    device = choose_device(args.device)
    print(f"Loading TTS model: {args.model} (device={device})")
    tts = TTS(model_name=args.model, progress_bar=False, gpu=(device == "cuda"))
    if device == "mps":
        try:
            tts.to("mps")
        except Exception as exc:
            print(f"Warning: could not move model to mps ({exc}); using cpu.")

    if args.list_speakers:
        available = discover_speakers(tts)
        if not available:
            print("No speakers exposed by this model.")
        else:
            for speaker in available:
                print(speaker)
        return
    if args.list_languages:
        available = getattr(tts, "languages", None) or []
        if not available:
            print("No languages exposed by this model.")
        else:
            for language in available:
                print(language)
        return

    if not args.db_url:
        raise SystemExit("DATABASE_URL is required (or pass --db-url)")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    speaker = args.speaker
    if not speaker and not args.speaker_wav:
        available = discover_speakers(tts)
        if available:
            preferred = os.getenv("COQUI_TTS_DEFAULT_SPEAKER")
            if preferred and preferred in available:
                speaker = preferred
            elif DEFAULT_XTTS_SPEAKER in available:
                speaker = DEFAULT_XTTS_SPEAKER
            else:
                speaker = available[0]
            print(f"Using default speaker: {speaker}")

    query, params = build_select_query(args)
    extension = "mp3" if args.output_format == "mp3" else "wav"

    with psycopg.connect(args.db_url) as conn:
        with conn.cursor() as cur:
            cur.execute(query, params)
            rows = cur.fetchall()

        total = len(rows)
        if total == 0:
            print("No prompt jokes matched the current filters.")
            return

        generated = 0
        skipped = 0
        failed = 0

        with conn.cursor() as cur:
            for i, (prompt_id, prompt_text, joke) in enumerate(rows, start=1):
                normalized_joke = normalize_joke_text(joke, args.max_chars)
                if not normalized_joke:
                    skipped += 1
                    print(f"[{i}/{total}] skip id={prompt_id} (empty joke after normalization)")
                    continue

                filename = f"promptlib_{prompt_id}.{extension}"
                file_path = output_dir / filename
                public_path = f"{args.public_prefix.rstrip('/')}/{filename}"

                if args.dry_run:
                    print(f"[{i}/{total}] dry-run id={prompt_id} -> {public_path} :: {normalized_joke}")
                    continue

                try:
                    with tempfile.TemporaryDirectory(prefix=f"joke_{prompt_id}_", dir=str(output_dir)) as tmpdir:
                        wav_path = Path(tmpdir) / f"promptlib_{prompt_id}.wav"
                        kwargs = build_tts_kwargs(tts, args, normalized_joke, wav_path, speaker)
                        tts.tts_to_file(**kwargs)

                        if args.output_format == "mp3":
                            encode_mp3(
                                ffmpeg_bin=ffmpeg_bin,
                                input_wav=wav_path,
                                output_mp3=file_path,
                                bitrate=args.mp3_bitrate,
                                normalize_loudness=args.normalize_loudness,
                            )
                        else:
                            shutil.move(str(wav_path), str(file_path))

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
                    generated += 1
                    print(f"[{i}/{total}] generated id={prompt_id} -> {public_path}")
                except Exception as exc:
                    conn.rollback()
                    failed += 1
                    print(f"[{i}/{total}] failed id={prompt_id}: {exc}")
                    if not args.continue_on_error:
                        raise

    print(f"Done. generated={generated} skipped={skipped} failed={failed} total={total}")


if __name__ == "__main__":
    main()
