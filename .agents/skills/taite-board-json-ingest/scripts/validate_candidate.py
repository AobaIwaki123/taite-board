#!/usr/bin/env python3
"""Validate a taite-board candidate intake JSON without publishing it."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlparse


DATE_RE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
TIME_RE = re.compile(r"^(\d{2}:\d{2}|--:--)$")
ALLOWED_STATUSES = {
    "needs_review",
    "unconfirmed",
    "scheduled",
    "timetable_pending",
    "postponed",
    "cancelled",
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate a taite-board candidate intake JSON without publishing it."
    )
    parser.add_argument("--file", required=True, type=Path, help="Candidate JSON path")
    parser.add_argument("--repo-root", type=Path, default=Path.cwd(), help="Repository root for relative image paths")
    parser.add_argument(
        "--allow-missing-local",
        action="store_true",
        help="Warn instead of failing when staged images are absent",
    )
    parser.add_argument("--require-session", action="store_true", help="Require at least one candidate session")
    return parser.parse_args()


def is_http_url(value: object) -> bool:
    if not isinstance(value, str):
        return False
    parsed = urlparse(value)
    return parsed.scheme in {"http", "https"} and bool(parsed.netloc)


def file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def validate(
    payload: object,
    repo_root: Path,
    allow_missing_local: bool,
    require_session: bool,
) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []
    if not isinstance(payload, dict):
        return ["root must be an object"], warnings

    if payload.get("contract") != "taite-board.candidate-intake/v1":
        errors.append("contract must be taite-board.candidate-intake/v1")
    if not isinstance(payload.get("run_id"), str) or not payload["run_id"]:
        errors.append("run_id is required")
    if not isinstance(payload.get("created_at"), str) or not payload["created_at"]:
        errors.append("created_at is required")
    if payload.get("status") not in {"needs_review", "ready"}:
        errors.append("root status must be needs_review or ready")

    inputs = payload.get("inputs")
    if not isinstance(inputs, list) or not inputs:
        errors.append("inputs must contain at least one item")
        inputs = []

    input_ids: set[str] = set()
    for index, item in enumerate(inputs, 1):
        prefix = f"inputs[{index - 1}]"
        if not isinstance(item, dict):
            errors.append(f"{prefix} must be an object")
            continue
        input_id = item.get("input_id")
        if not isinstance(input_id, str) or not input_id:
            errors.append(f"{prefix}.input_id is required")
        elif input_id in input_ids:
            errors.append(f"{prefix}.input_id is duplicated: {input_id}")
        else:
            input_ids.add(input_id)

        kind = item.get("kind")
        if kind == "url":
            if not is_http_url(item.get("url")):
                errors.append(f"{prefix}.url must be an http(s) URL")
        elif kind == "image":
            image_path = item.get("path")
            if not isinstance(image_path, str) or not image_path:
                errors.append(f"{prefix}.path is required for images")
                continue
            target = Path(image_path).expanduser()
            if not target.is_absolute():
                target = repo_root / target
            if not target.is_file():
                message = f"{prefix}.path does not exist: {image_path}"
                (warnings if allow_missing_local else errors).append(message)
                continue
            expected_hash = item.get("sha256")
            if isinstance(expected_hash, str) and expected_hash:
                actual_hash = file_sha256(target)
                if actual_hash != expected_hash:
                    errors.append(f"{prefix}.sha256 does not match {image_path}")
        else:
            errors.append(f"{prefix}.kind must be url or image")

    sessions = payload.get("sessions")
    if not isinstance(sessions, list):
        errors.append("sessions must be an array")
        sessions = []
    if require_session and not sessions:
        errors.append("at least one candidate session is required")
    if not sessions:
        warnings.append("no candidate sessions are present; this is still an input-only manifest")

    session_ids: set[str] = set()
    for index, session in enumerate(sessions, 1):
        prefix = f"sessions[{index - 1}]"
        if not isinstance(session, dict):
            errors.append(f"{prefix} must be an object")
            continue
        session_id = session.get("id")
        if not isinstance(session_id, str) or not session_id:
            errors.append(f"{prefix}.id is required")
        elif session_id in session_ids:
            errors.append(f"{prefix}.id is duplicated: {session_id}")
        else:
            session_ids.add(session_id)

        if session.get("status") not in ALLOWED_STATUSES:
            errors.append(f"{prefix}.status must describe review/publication state")
        if not isinstance(session.get("title"), str) or not session["title"]:
            errors.append(f"{prefix}.title is required")
        if "date" in session and session["date"] is not None and not DATE_RE.fullmatch(str(session["date"])):
            errors.append(f"{prefix}.date must be YYYY-MM-DD when present")

        for section_name in ("live", "benefit"):
            section = session.get(section_name)
            if not isinstance(section, dict):
                continue
            for field in ("start_time", "end_time"):
                value = section.get(field)
                if value is not None and not TIME_RE.fullmatch(str(value)):
                    errors.append(f"{prefix}.{section_name}.{field} must be HH:MM or --:--")

        # Discrepancy warning: live is pending but benefit has time
        live_start = session.get("live", {}).get("start_time")
        benefit_start = session.get("benefit", {}).get("start_time")
        if live_start == "--:--" and benefit_start and benefit_start != "--:--":
            warnings.append(f"{prefix}: live.start_time is --:-- but benefit.start_time is {benefit_start}")

        # Check admission_bonus option truncation (e.g. truncated at internal slash)
        bonus = session.get("admission_bonus")
        if isinstance(bonus, str):
            if "券5枚" in bonus and "10枚" not in bonus:
                warnings.append(
                    f"{prefix}.admission_bonus contains '券5枚' but lacks '10枚' option (likely truncated at '/')"
                )

        # Check ticket.platform cleanliness
        ticket = session.get("ticket")
        if isinstance(ticket, dict):
            platform = ticket.get("platform")
            if isinstance(platform, str):
                if "完売" in platform or "当日券" in platform:
                    warnings.append(
                        f"{prefix}.ticket.platform '{platform}' contains sales/status text which breaks CTA button layout; keep platform clean (e.g. 'イープラス') and move status to prices/special_notes"
                    )

        # Check source_tweet_url if present
        source_tweet = session.get("source_tweet_url")
        if source_tweet and not re.match(r"^https?://(x\.com|twitter\.com)/[^/]+/status/\d+", str(source_tweet)):
            errors.append(f"{prefix}.source_tweet_url must be an X/Twitter status URL: {source_tweet}")

        # Check images
        images = session.get("images")
        if isinstance(images, list):
            for img_index, img in enumerate(images, 1):
                img_prefix = f"{prefix}.images[{img_index - 1}]"
                if not isinstance(img, dict):
                    continue
                img_url = img.get("url")
                if img_url:
                    if not is_http_url(img_url):
                        errors.append(f"{img_prefix}.url must be an absolute http(s) URL (relative path forbidden): {img_url}")
                img_type = img.get("type")
                if img_type and img_type not in {"flyer", "timetable"}:
                    warnings.append(f"{img_prefix}.type '{img_type}' is unusual (expected 'flyer' or 'timetable')")

        refs = session.get("source_refs")
        if not isinstance(refs, list) or not refs:
            errors.append(f"{prefix}.source_refs must contain at least one source")
            continue
        for ref_index, ref in enumerate(refs, 1):
            if not isinstance(ref, dict):
                errors.append(f"{prefix}.source_refs[{ref_index - 1}] must be an object")
                continue
            if ref.get("input_id") not in input_ids:
                errors.append(f"{prefix}.source_refs[{ref_index - 1}].input_id is not in inputs")
            if "url" in ref and not is_http_url(ref["url"]):
                errors.append(f"{prefix}.source_refs[{ref_index - 1}].url must be an http(s) URL")

    return errors, warnings


def main() -> int:
    args = parse_args()
    try:
        payload = json.loads(args.file.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        print(f"error: cannot read candidate JSON: {error}", file=sys.stderr)
        return 2

    errors, warnings = validate(
        payload,
        args.repo_root.expanduser().resolve(),
        args.allow_missing_local,
        args.require_session,
    )
    for warning in warnings:
        print(f"warning: {warning}", file=sys.stderr)
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"Candidate validation passed: {args.file}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
