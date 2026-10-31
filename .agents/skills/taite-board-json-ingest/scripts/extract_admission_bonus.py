#!/usr/bin/env python3
"""Extract and normalize admission bonus and ticket platform from raw post text.

Responsibility: Deterministic mechanical parsing of admission bonus options and
ticket platform sanitization, avoiding common errors such as truncating options
at internal slashes or polluting the platform name with sales/status text.
"""

from __future__ import annotations

import argparse
import json
import re
import sys


def extract_admission_bonus(text: str) -> str | None:
    """Extract admission bonus from post text, preserving internal option slashes."""
    if not text:
        return None

    # Search for standard Ribbon Clock bonus section: 🪄お目当て特典 ... or 指名特典
    # Pattern: capture ①, ②, ③ options
    bonus_match = re.search(
        r"(?:(?:お目当て特典|指名入場特典|指名特典|入場特典)[：:][^\n]*\n)([\s\S]*?)(?=(?:\n\s*📸|\n\s*🕰️|\n\s*❤︎|\n\s*🎟️|\n\s*https?://|$))",
        text,
    )

    if bonus_match:
        section = bonus_match.group(1).strip()
        lines = [line.strip() for line in section.splitlines() if line.strip()]

        # Look for numbered options ①, ②, ③
        opt1 = None
        opt2 = None
        opt3 = None

        for line in lines:
            # Clean emoji/decorations from beginning
            cleaned = re.sub(r"^[🪄📸🕰️❤︎・\s]+", "", line).strip()
            if cleaned.startswith("①"):
                # e.g. ① 10秒動画券（複数使用可） -> ①10秒動画券
                opt1 = re.sub(r"（複数使用可）", "", cleaned).replace(" ", "").strip()
            elif cleaned.startswith("②"):
                # e.g. ② 写真＋30秒トーク（複数使用不可） -> ②写真+30秒トーク
                opt2 = re.sub(r"（複数使用不可）", "", cleaned).replace(" ", "").replace("＋", "+").strip()
            elif cleaned.startswith("③"):
                # Preserves both 5枚 and 10枚 across internal slash
                # e.g. ③ 券5枚⇒集合写真/券10枚⇒私物サイン
                opt3 = re.sub(r"^[③\s]+", "③", cleaned).replace(" ", "").strip()

        if opt1 and opt2 and opt3:
            return f"お目当て3択（{opt1} / {opt2} / {opt3}）"

        # If options not fully separated into 3, join lines with slash
        if lines:
            # Filter out non-bonus lines
            bonus_lines = [re.sub(r"^[🪄📸🕰️❤︎・\s]+", "", l).strip() for l in lines if not l.startswith("新規特典")]
            if bonus_lines:
                return " / ".join(bonus_lines)

    # Check for simple single-line admission bonus
    single_match = re.search(r"(?:お目当て特典|指名入場特典|指名特典|入場特典)[：:]\s*([^\n]+)", text)
    if single_match:
        val = single_match.group(1).strip()
        if val and val != "決まり次第お知らせ":
            return val

    return None


def sanitize_ticket_platform(platform: str | None) -> tuple[str, str | None]:
    """Sanitize ticket platform by stripping parentheses/status notes.

    Returns:
        (clean_platform, extracted_status_note)
    """
    if not platform:
        return "外部サイト", None

    # Check if contains status notes like （前売完売/当日券あり）
    match = re.search(r"[（(](.*?)[）)]", platform)
    status_note = match.group(1) if match else None

    # Cleaned platform name
    cleaned = re.sub(r"[（(].*?[）)]", "", platform).strip()
    if not cleaned:
        cleaned = "外部サイト"

    return cleaned, status_note


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Extract and normalize admission bonus and clean ticket platform."
    )
    parser.add_argument("--text", type=str, help="Raw tweet text to parse")
    parser.add_argument("--file", type=str, help="Path to text or JSON file containing tweet text")
    parser.add_argument("--platform", type=str, help="Ticket platform string to sanitize")
    parser.add_argument("--json", action="store_true", help="Output result as JSON")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    raw_text = ""

    if args.text:
        raw_text = args.text
    elif args.file:
        try:
            with open(args.file, "r", encoding="utf-8") as f:
                content = f.read()
                try:
                    data = json.loads(content)
                    if isinstance(data, dict):
                        raw_text = data.get("text", "")
                    elif isinstance(data, list) and data:
                        raw_text = data[0].get("text", "")
                except json.JSONDecodeError:
                    raw_text = content
        except OSError as e:
            print(f"Error reading file: {e}", file=sys.stderr)
            return 1
    elif not sys.stdin.isatty():
        raw_text = sys.stdin.read()

    bonus = extract_admission_bonus(raw_text) if raw_text else None
    platform, status = sanitize_ticket_platform(args.platform) if args.platform else (None, None)

    if args.json:
        result = {
            "admission_bonus": bonus,
            "ticket_platform": platform,
            "platform_status_note": status,
        }
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        if bonus:
            print(f"admission_bonus: {bonus}")
        if platform:
            print(f"ticket_platform: {platform}")
            if status:
                print(f"platform_status: {status}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
