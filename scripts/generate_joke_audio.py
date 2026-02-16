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
DEFAULT_AB_MODEL = "tts_models/multilingual/multi-dataset/bark"
DEFAULT_LANGUAGE = "en"
DEFAULT_XTTS_SPEAKER = "Rosemary Okafor"
DEFAULT_SPEED = "0.92"
DEFAULT_TEMPERATURE = "0.55"
DEFAULT_TOP_P = "0.92"
DEFAULT_REPETITION_PENALTY = "3.0"

PUNCHLINE_PIVOTS = (
    "but",
    "except",
    "until",
    "yet",
    "though",
    "although",
    "however",
    "because",
    "so",
    "then",
    "when",
    "while",
    "after",
    "before",
    "unless",
    "instead",
)


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
    parser.add_argument("--ab-model", default=os.getenv("COQUI_TTS_AB_MODEL", ""), help="Optional second model for A/B generation (e.g. Bark)")
    parser.add_argument("--ab-speaker", default=os.getenv("COQUI_TTS_AB_SPEAKER"), help="Optional speaker name/id for --ab-model")
    parser.add_argument("--ab-language", default=os.getenv("COQUI_TTS_AB_LANGUAGE"), help="Optional language code for --ab-model")
    parser.add_argument("--ab-output-dir", default=os.getenv("COQUI_TTS_AB_OUTPUT_DIR", "static/audio/jokes_ab"), help="Output directory for A/B files")
    parser.add_argument("--ab-public-prefix", default=os.getenv("COQUI_TTS_AB_PUBLIC_PREFIX", "/static/audio/jokes_ab"), help="Public path prefix for A/B output")
    parser.add_argument("--device", choices=["auto", "cpu", "mps", "cuda"], default=os.getenv("COQUI_TTS_DEVICE", "auto"), help="Torch device")
    parser.add_argument("--speaker", default=os.getenv("COQUI_TTS_SPEAKER"), help="Optional speaker name/id")
    parser.add_argument("--speaker-wav", default=os.getenv("COQUI_TTS_SPEAKER_WAV"), help="Optional reference WAV path for voice cloning")
    parser.add_argument("--list-speakers", action="store_true", help="List available speakers for the model and exit")
    parser.add_argument("--list-languages", action="store_true", help="List available languages for the model and exit")
    parser.add_argument("--language", default=os.getenv("COQUI_TTS_LANGUAGE", DEFAULT_LANGUAGE), help="Optional language code")
    parser.add_argument("--speed", type=float, default=float(os.getenv("COQUI_TTS_SPEED", DEFAULT_SPEED)), help="Speech speed (if model supports it)")
    parser.add_argument("--temperature", type=float, default=float(os.getenv("COQUI_TTS_TEMPERATURE", DEFAULT_TEMPERATURE)), help="Sampling temperature (if supported)")
    parser.add_argument("--top-p", type=float, default=float(os.getenv("COQUI_TTS_TOP_P", DEFAULT_TOP_P)), help="Nucleus sampling top_p (if supported)")
    parser.add_argument("--top-k", type=int, default=int(os.getenv("COQUI_TTS_TOP_K", "50")), help="Sampling top_k (if supported)")
    parser.add_argument("--repetition-penalty", type=float, default=float(os.getenv("COQUI_TTS_REPETITION_PENALTY", DEFAULT_REPETITION_PENALTY)), help="Repetition penalty (if supported)")
    parser.add_argument("--split-sentences", action=argparse.BooleanOptionalAction, default=False, help="Enable sentence splitting where supported")
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
    args.ab_model = (args.ab_model or "").strip()
    if args.ab_model.lower() in {"1", "true", "yes", "on"}:
        args.ab_model = DEFAULT_AB_MODEL
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
    cleaned = insert_delivery_pause(cleaned)
    if len(cleaned) > max_chars:
        clipped = cleaned[:max_chars].rsplit(" ", 1)[0].strip()
        cleaned = clipped if clipped else cleaned[:max_chars]
    if cleaned and cleaned[-1] not in ".!?":
        cleaned += "."
    return cleaned


def insert_delivery_pause(text):
    if not text:
        return text
    words = text.split()
    if len(words) < 5:
        return text

    for pivot in PUNCHLINE_PIVOTS:
        match = re.search(rf"(?i)\b{re.escape(pivot)}\b", text)
        if match is None or match.start() == 0:
            continue
        prefix = text[:match.start()].rstrip()
        suffix = text[match.start():].lstrip()
        if prefix and prefix[-1] in ",;:.!?-":
            return text
        return f"{prefix}, {suffix}"

    if re.search(r"[,:;!?]", text):
        return text
    if len(words) >= 8:
        split_at = len(words) - 3
        return f"{' '.join(words[:split_at])}, {' '.join(words[split_at:])}"
    return text


def build_select_query(args):
    clauses = [
        "joke IS NOT NULL",
        "trim(joke) <> ''",
    ]
    params = []
    if not args.force and not args.ab_model:
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
    filter_chain = [
        "highpass=f=80",
        "lowpass=f=12000",
        "acompressor=threshold=0.08:ratio=3:attack=5:release=120:makeup=2",
    ]
    if normalize_loudness:
        filter_chain.append("loudnorm=I=-16:TP=-1.5:LRA=9")
    filter_chain.append("alimiter=limit=0.95")
    cmd.extend(["-af", ",".join(filter_chain)])
    cmd.extend(["-vn", "-ar", "44100", "-ac", "1", "-codec:a", "libmp3lame", "-b:a", bitrate, str(output_mp3)])
    subprocess.run(cmd, check=True)


def model_slug(name):
    cleaned = re.sub(r"[^a-z0-9]+", "-", (name or "").lower()).strip("-")
    return cleaned or "model"


def load_tts_model(tts_class, model_name, device):
    print(f"Loading TTS model: {model_name} (device={device})")
    tts = tts_class(model_name=model_name, progress_bar=False, gpu=(device == "cuda"))
    if device == "mps":
        try:
            tts.to("mps")
        except Exception as exc:
            print(f"Warning: could not move model to mps ({exc}); using cpu.")
    return tts


def resolve_speaker(tts, requested_speaker, speaker_wav, model_name):
    speaker = (requested_speaker or "").strip()
    if speaker or speaker_wav:
        return speaker
    available = discover_speakers(tts)
    if not available:
        return ""
    preferred = os.getenv("COQUI_TTS_DEFAULT_SPEAKER")
    if preferred and preferred in available:
        return preferred
    if "xtts" in (model_name or "").lower() and DEFAULT_XTTS_SPEAKER in available:
        return DEFAULT_XTTS_SPEAKER
    return available[0]


def synthesize_to_file(tts, args, text, output_path, ffmpeg_bin, speaker, language):
    with tempfile.TemporaryDirectory(prefix="joke_", dir=str(output_path.parent)) as tmpdir:
        wav_path = Path(tmpdir) / f"{output_path.stem}.wav"
        original_language = args.language
        original_speaker = args.speaker
        try:
            args.language = language
            args.speaker = speaker
            kwargs = build_tts_kwargs(tts, args, text, wav_path, speaker)
        finally:
            args.language = original_language
            args.speaker = original_speaker

        tts.tts_to_file(**kwargs)
        if args.output_format == "mp3":
            encode_mp3(
                ffmpeg_bin=ffmpeg_bin,
                input_wav=wav_path,
                output_mp3=output_path,
                bitrate=args.mp3_bitrate,
                normalize_loudness=args.normalize_loudness,
            )
        else:
            shutil.move(str(wav_path), str(output_path))


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
    tts = load_tts_model(TTS, args.model, device)
    tts_ab = None
    if args.ab_model:
        tts_ab = load_tts_model(TTS, args.ab_model, device)

    if args.list_speakers:
        available = discover_speakers(tts)
        if args.ab_model and tts_ab is not None:
            print(f"[A] {args.model}")
        if not available:
            print("No speakers exposed by this model.")
        else:
            for speaker in available:
                print(speaker)
        if args.ab_model and tts_ab is not None:
            print("")
            print(f"[B] {args.ab_model}")
            available_ab = discover_speakers(tts_ab)
            if not available_ab:
                print("No speakers exposed by this model.")
            else:
                for speaker in available_ab:
                    print(speaker)
        return
    if args.list_languages:
        available = getattr(tts, "languages", None) or []
        if args.ab_model and tts_ab is not None:
            print(f"[A] {args.model}")
        if not available:
            print("No languages exposed by this model.")
        else:
            for language in available:
                print(language)
        if args.ab_model and tts_ab is not None:
            print("")
            print(f"[B] {args.ab_model}")
            available_ab = getattr(tts_ab, "languages", None) or []
            if not available_ab:
                print("No languages exposed by this model.")
            else:
                for language in available_ab:
                    print(language)
        return

    if not args.db_url:
        raise SystemExit("DATABASE_URL is required (or pass --db-url)")

    output_dir = Path(args.ab_output_dir if args.ab_model else args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    public_prefix = args.ab_public_prefix if args.ab_model else args.public_prefix
    speaker = resolve_speaker(tts, args.speaker, args.speaker_wav, args.model)
    if speaker:
        print(f"Using speaker for {args.model}: {speaker}")
    speaker_ab = ""
    language_ab = args.ab_language or args.language
    if args.ab_model and tts_ab is not None:
        speaker_ab = resolve_speaker(tts_ab, args.ab_speaker, args.speaker_wav, args.ab_model)
        if speaker_ab:
            print(f"Using speaker for {args.ab_model}: {speaker_ab}")
        print("A/B mode enabled: writing comparison files only (database paths are unchanged).")

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
                public_path = f"{public_prefix.rstrip('/')}/{filename}"

                if args.dry_run:
                    if args.ab_model:
                        model_a = model_slug(args.model)
                        model_b = model_slug(args.ab_model)
                        file_path_a = output_dir / f"promptlib_{prompt_id}__{model_a}.{extension}"
                        file_path_b = output_dir / f"promptlib_{prompt_id}__{model_b}.{extension}"
                        public_path_a = f"{public_prefix.rstrip('/')}/{file_path_a.name}"
                        public_path_b = f"{public_prefix.rstrip('/')}/{file_path_b.name}"
                        print(f"[{i}/{total}] dry-run id={prompt_id} -> A:{public_path_a} B:{public_path_b} :: {normalized_joke}")
                    else:
                        print(f"[{i}/{total}] dry-run id={prompt_id} -> {public_path} :: {normalized_joke}")
                    continue

                try:
                    if args.ab_model and tts_ab is not None:
                        model_a = model_slug(args.model)
                        model_b = model_slug(args.ab_model)
                        file_path_a = output_dir / f"promptlib_{prompt_id}__{model_a}.{extension}"
                        file_path_b = output_dir / f"promptlib_{prompt_id}__{model_b}.{extension}"
                        public_path_a = f"{public_prefix.rstrip('/')}/{file_path_a.name}"
                        public_path_b = f"{public_prefix.rstrip('/')}/{file_path_b.name}"
                        synthesize_to_file(tts, args, normalized_joke, file_path_a, ffmpeg_bin, speaker, args.language)
                        synthesize_to_file(tts_ab, args, normalized_joke, file_path_b, ffmpeg_bin, speaker_ab, language_ab)
                        print(f"[{i}/{total}] generated id={prompt_id} -> A:{public_path_a} B:{public_path_b}")
                    else:
                        synthesize_to_file(tts, args, normalized_joke, file_path, ffmpeg_bin, speaker, args.language)
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
                        print(f"[{i}/{total}] generated id={prompt_id} -> {public_path}")
                    generated += 1
                except Exception as exc:
                    conn.rollback()
                    failed += 1
                    print(f"[{i}/{total}] failed id={prompt_id}: {exc}")
                    if not args.continue_on_error:
                        raise

    print(f"Done. generated={generated} skipped={skipped} failed={failed} total={total}")


if __name__ == "__main__":
    main()
