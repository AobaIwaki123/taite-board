#!/usr/bin/env python3
"""update_photo_rule.py: Deterministically update session photo rules and sync copies."""

import argparse
import json
import re
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

def main():
    parser = argparse.ArgumentParser(description="Update session photo rule and sync schedule files")
    parser.add_argument("--session", required=True, help="Session ID (e.g. 20261101-1)")
    parser.add_argument("--rule", required=True, help="Clean photo rule text (e.g. 5曲目『75秒の沈黙』撮影可能)")
    parser.add_argument("--raw", default="", help="Raw announcement text snippet")
    parser.add_argument("--not-allowed", action="store_true", help="Set photo available to false")
    parser.add_argument("--tweet-url", default="", help="Source tweet URL to attach to source_refs")
    parser.add_argument("--notes", default="", help="Prepend or update special notes")
    args = parser.parse_args()

    root_dir = Path(__file__).resolve().parent.parent
    schedule_path = root_dir / "data" / "generated" / "schedule.json"

    with open(schedule_path, "r", encoding="utf-8") as f:
        data = json.load(f)

    target_session = None
    for s in data.get("sessions", []):
        if s.get("id") == args.session:
            target_session = s
            break

    if not target_session:
        print(f"Error: Session '{args.session}' not found in {schedule_path}", file=sys.stderr)
        sys.exit(1)

    target_session["photo"] = {
        "available": not args.not_allowed,
        "rule": args.rule,
        "raw_text": args.raw or args.rule
    }

    if args.notes:
        existing_notes = target_session.get("special_notes") or ""
        if existing_notes and args.notes not in existing_notes:
            target_session["special_notes"] = f"{args.notes} / {existing_notes}"
        elif not existing_notes:
            target_session["special_notes"] = args.notes

    if args.tweet_url:
        m = re.search(r"status/(\d+)", args.tweet_url)
        source_id = m.group(1) if m else ""
        refs = target_session.get("source_refs", [])
        if source_id and not any(r.get("source_id") == source_id for r in refs):
            refs.append({
                "type": "x_post",
                "source_id": source_id,
                "url": args.tweet_url
            })
        target_session["source_refs"] = refs

    data["generated_at"] = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")

    with open(schedule_path, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)
        f.write("\n")

    print(f"Updated session {args.session} in {schedule_path}")

    # Synchronize to web public directories
    sync_script = root_dir / "scripts" / "sync-schedule.sh"
    subprocess.run([str(sync_script)], check=True)

if __name__ == "__main__":
    main()
