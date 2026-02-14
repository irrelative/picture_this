#!/usr/bin/env python3
import argparse
import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from generate_joke_audio import (
    DEFAULT_LANGUAGE,
    DEFAULT_MODEL,
    DEFAULT_XTTS_SPEAKER,
    allow_xtts_globals,
    build_tts_kwargs,
    choose_device,
    discover_speakers,
    encode_mp3,
)


# Display interlude text catalog. These lines are intentionally short and punchy.
INTERLUDE_LINES = [
    ("phase_drawings", "Time to draw!"),
    ("phase_guesses", "Pass it on. Write a fake prompt."),
    ("phase_votes", "Vote time. Pick the real prompt."),
    ("phase_results", "Let's reveal the answers."),
    ("reveal_guesses", "Here come the lies."),
    ("reveal_votes", "And now, the votes."),
    ("phase_complete", "Game over. Final scores!"),
]


def parse_args():
    parser = argparse.ArgumentParser(description="Generate display interlude voice-over audio files.")
    parser.add_argument("--output-dir", default="static/audio/interludes", help="Directory to write audio files")
    parser.add_argument("--public-prefix", default="/static/audio/interludes", help="Public URL prefix for manifest")
    parser.add_argument("--model", default=os.getenv("COQUI_TTS_MODEL", DEFAULT_MODEL), help="Coqui TTS model name")
    parser.add_argument(
        "--device",
        choices=["auto", "cpu", "mps", "cuda"],
        default=os.getenv("COQUI_TTS_DEVICE", "auto"),
        help="Torch device",
    )
    parser.add_argument("--speaker", default=os.getenv("COQUI_TTS_SPEAKER"), help="Optional speaker name/id")
    parser.add_argument("--speaker-wav", default=os.getenv("COQUI_TTS_SPEAKER_WAV"), help="Optional reference WAV path")
    parser.add_argument("--language", default=os.getenv("COQUI_TTS_LANGUAGE", DEFAULT_LANGUAGE), help="Language code")
    parser.add_argument("--speed", type=float, default=float(os.getenv("COQUI_TTS_SPEED", "0.98")), help="Speech speed")
    parser.add_argument("--temperature", type=float, default=float(os.getenv("COQUI_TTS_TEMPERATURE", "0.35")), help="Sampling temperature")
    parser.add_argument("--top-p", type=float, default=float(os.getenv("COQUI_TTS_TOP_P", "0.85")), help="Nucleus top_p")
    parser.add_argument("--top-k", type=int, default=int(os.getenv("COQUI_TTS_TOP_K", "50")), help="Sampling top_k")
    parser.add_argument(
        "--repetition-penalty",
        type=float,
        default=float(os.getenv("COQUI_TTS_REPETITION_PENALTY", "5.0")),
        help="Repetition penalty",
    )
    parser.add_argument("--split-sentences", action=argparse.BooleanOptionalAction, default=True, help="Enable sentence splitting")
    parser.add_argument("--output-format", choices=["mp3", "wav"], default="mp3", help="Output format")
    parser.add_argument("--mp3-bitrate", default=os.getenv("COQUI_TTS_MP3_BITRATE", "128k"), help="MP3 bitrate")
    parser.add_argument("--ffmpeg-bin", default=os.getenv("FFMPEG_BIN", "ffmpeg"), help="ffmpeg binary path")
    parser.add_argument("--normalize-loudness", action=argparse.BooleanOptionalAction, default=True, help="Apply loudness normalization")
    parser.add_argument("--force", action="store_true", help="Regenerate existing files")
    parser.add_argument("--dry-run", action="store_true", help="Print outputs without writing files")
    return parser.parse_args()


def resolve_speaker(tts, args):
    speaker = args.speaker
    if speaker or args.speaker_wav:
        return speaker
    available = discover_speakers(tts)
    if not available:
        return speaker
    preferred = os.getenv("COQUI_TTS_DEFAULT_SPEAKER")
    if preferred and preferred in available:
        return preferred
    if DEFAULT_XTTS_SPEAKER in available:
        return DEFAULT_XTTS_SPEAKER
    return available[0]


def main():
    args = parse_args()
    cache_root = Path(".cache").resolve()
    (cache_root / "matplotlib").mkdir(parents=True, exist_ok=True)
    (cache_root / "fontconfig").mkdir(parents=True, exist_ok=True)
    os.environ.setdefault("XDG_CACHE_HOME", str(cache_root))
    os.environ.setdefault("MPLCONFIGDIR", str(cache_root / "matplotlib"))
    os.environ.setdefault("FC_CACHEDIR", str(cache_root / "fontconfig"))

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

    if args.speaker_wav:
        speaker_wav = Path(args.speaker_wav).expanduser()
        if not speaker_wav.exists():
            raise SystemExit(f"speaker wav not found: {speaker_wav}")
        args.speaker_wav = str(speaker_wav)

    speaker = resolve_speaker(tts, args)
    if speaker:
        print(f"Using speaker: {speaker}")

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    extension = "mp3" if args.output_format == "mp3" else "wav"
    manifest = []

    for index, (key, text) in enumerate(INTERLUDE_LINES, start=1):
        filename = f"{key}.{extension}"
        file_path = output_dir / filename
        public_path = f"{args.public_prefix.rstrip('/')}/{filename}"
        manifest.append({"key": key, "text": text, "path": public_path})

        if file_path.exists() and not args.force:
            print(f"[{index}/{len(INTERLUDE_LINES)}] skip {key} (exists)")
            continue
        if args.dry_run:
            print(f"[{index}/{len(INTERLUDE_LINES)}] dry-run {key} -> {public_path} :: {text}")
            continue

        try:
            with tempfile.TemporaryDirectory(prefix=f"interlude_{key}_", dir=str(output_dir)) as tmpdir:
                wav_path = Path(tmpdir) / f"{key}.wav"
                kwargs = build_tts_kwargs(tts, args, text, wav_path, speaker)
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
            print(f"[{index}/{len(INTERLUDE_LINES)}] generated {key} -> {public_path}")
        except Exception as exc:
            raise SystemExit(f"failed to generate {key}: {exc}") from exc

    manifest_path = output_dir / "manifest.json"
    if not args.dry_run:
        manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
        print(f"Wrote manifest: {manifest_path}")


if __name__ == "__main__":
    main()
