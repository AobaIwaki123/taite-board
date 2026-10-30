#!/usr/bin/env python3
"""Create a traceable intake manifest from URLs and local image files."""

from __future__ import annotations

import argparse
import hashlib
import json
import mimetypes
import re
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse


RUN_ID_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]+$")
SAFE_NAME_RE = re.compile(r"[^A-Za-z0-9._-]+")


def is_url(value: str) -> bool:
    parsed = urlparse(value)
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Create a taite-board candidate intake manifest from links and images."
    )
    parser.add_argument("sources", nargs="*", help="URLs or local image paths")
    parser.add_argument("--url", action="append", default=[], help="URL input; repeatable")
    parser.add_argument("--image", action="append", default=[], help="Local image path; repeatable")
    parser.add_argument("--input", type=Path, help="File containing one URL or image path per line")
    parser.add_argument("--output", default="-", help="JSON output path, or '-' for stdout")
    parser.add_argument("--run-id", help="Stable run identifier; generated when omitted")
    parser.add_argument("--repo-root", type=Path, default=Path.cwd(), help="Repository root for relative paths")
    parser.add_argument(
        "--stage-dir",
        type=Path,
        help="Copy local images into this directory before recording them",
    )
    parser.add_argument("--force", action="store_true", help="Allow replacing an existing output file")
    return parser.parse_args()


def read_input_file(path: Path) -> list[tuple[str, str]]:
    entries: list[tuple[str, str]] = []
    for line_number, raw_line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        value = raw_line.strip()
        if not value or value.startswith("#"):
            continue
        if value.startswith("url:"):
            entries.append(("url", value[4:].strip()))
        elif value.startswith("image:"):
            entries.append(("image", value[6:].strip()))
        elif is_url(value):
            entries.append(("url", value))
        else:
            entries.append(("image", value))
        if not entries[-1][1]:
            raise ValueError(f"{path}:{line_number}: empty input")
    return entries


def unique_entries(entries: list[tuple[str, str]]) -> list[tuple[str, str]]:
    seen: set[tuple[str, str]] = set()
    unique: list[tuple[str, str]] = []
    for entry in entries:
        if entry not in seen:
            seen.add(entry)
            unique.append(entry)
    return unique


def safe_filename(name: str, index: int) -> str:
    cleaned = SAFE_NAME_RE.sub("-", Path(name).name).strip("-.") or "image"
    return f"{index:02d}-{cleaned}"


def relative_or_absolute(path: Path, repo_root: Path) -> str:
    try:
        return path.resolve().relative_to(repo_root.resolve()).as_posix()
    except ValueError:
        return str(path.resolve())


def stage_image(
    value: str,
    input_id: str,
    image_index: int,
    repo_root: Path,
    stage_dir: Path | None,
) -> dict[str, object]:
    source = Path(value).expanduser()
    if not source.is_file():
        raise ValueError(f"image input is not a file: {value}")

    source = source.resolve()
    media_type = mimetypes.guess_type(source.name)[0] or "application/octet-stream"
    if not media_type.startswith("image/"):
        raise ValueError(f"image input has a non-image MIME type ({media_type}): {value}")

    digest = sha256_file(source)
    recorded_path = source
    if stage_dir is not None:
        stage_dir.mkdir(parents=True, exist_ok=True)
        destination = stage_dir / safe_filename(source.name, image_index)
        if destination.exists():
            if sha256_file(destination) != digest:
                raise ValueError(f"staged image already exists with a different hash: {destination}")
        else:
            shutil.copy2(source, destination)
        recorded_path = destination.resolve()

    return {
        "input_id": input_id,
        "kind": "image",
        "path": relative_or_absolute(recorded_path, repo_root),
        "original_name": source.name,
        "sha256": digest,
        "media_type": media_type,
        "size_bytes": source.stat().st_size,
    }


def build_manifest(args: argparse.Namespace) -> dict[str, object]:
    entries: list[tuple[str, str]] = []
    if args.input:
        entries.extend(read_input_file(args.input.expanduser()))
    entries.extend(("url", value) for value in args.url)
    entries.extend(("image", value) for value in args.image)
    for value in args.sources:
        entries.append(("url", value) if is_url(value) else ("image", value))
    entries = unique_entries(entries)
    if not entries:
        raise ValueError("no inputs were provided")

    repo_root = args.repo_root.expanduser().resolve()
    run_id = args.run_id or datetime.now(timezone.utc).strftime("manual-%Y%m%dT%H%M%SZ")
    if not RUN_ID_RE.fullmatch(run_id):
        raise ValueError("run-id must contain only letters, numbers, '.', '_' or '-'")

    stage_dir = None
    if args.stage_dir:
        stage_dir = args.stage_dir.expanduser()
        if not stage_dir.is_absolute():
            stage_dir = repo_root / stage_dir

    inputs: list[dict[str, object]] = []
    image_index = 1
    for index, (kind, value) in enumerate(entries, 1):
        input_id = f"input-{index:02d}"
        if kind == "url":
            if not is_url(value):
                raise ValueError(f"URL must use http or https: {value}")
            hostname = (urlparse(value).hostname or "").lower()
            is_x_host = hostname == "x.com" or hostname.endswith(".x.com")
            is_twitter_host = hostname == "twitter.com" or hostname.endswith(".twitter.com")
            source_type = "x_post" if is_x_host or is_twitter_host else "web_page"
            inputs.append({"input_id": input_id, "kind": "url", "source_type": source_type, "url": value})
        else:
            inputs.append(stage_image(value, input_id, image_index, repo_root, stage_dir))
            image_index += 1

    return {
        "contract": "taite-board.candidate-intake/v1",
        "run_id": run_id,
        "created_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "status": "needs_review",
        "inputs": inputs,
        "sessions": [],
    }


def write_output(payload: dict[str, object], output: str, force: bool) -> None:
    encoded = json.dumps(payload, ensure_ascii=False, indent=2) + "\n"
    if output == "-":
        sys.stdout.write(encoded)
        return

    target = Path(output).expanduser()
    if target.exists() and not force:
        raise ValueError(f"output already exists; use --force to replace it: {target}")
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(encoded, encoding="utf-8")
    print(f"Wrote intake manifest: {target}", file=sys.stderr)


def main() -> int:
    args = parse_args()
    try:
        write_output(build_manifest(args), args.output, args.force)
    except (OSError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
