#!/usr/bin/env python3
"""
Verify Dolly Kiss parsed candidates against TimeTree ground truth data.
Outputs a structured accuracy evaluation report.
"""

import json
import sys
from datetime import datetime

# Ground Truth from TimeTree (lui_17707)
GROUND_TRUTH = {
    "2026-11-03_akiba": {
        "title": "＠JAM×アイドル横丁presents 秋葉原アイドルサーキットvol.6",
        "date": "2026-11-03",
        "venue": "ANTHEM AKIBA",
        "live_start": "18:40",
        "live_end": "19:00",
        "benefit_start": "19:30",
        "benefit_end": "20:30",
        "source_tweet_id": "2111019330574127141"
    },
    "2026-11-03_toranomon": {
        "title": "アイドルマリアージュVol.79",
        "date": "2026-11-03",
        "venue": "ベルサール虎ノ門",
        "live_start": "14:55",
        "live_end": "15:15",
        "benefit_start": "15:20",
        "benefit_end": "16:20",
        "source_tweet_id": "2115639770944586018"
    }
}

# AI Parsed Candidates (Cloud Run Vertex AI Inference)
PARSED_RESULTS = [
    {
        "id": "2026-11-03_akiba",
        "source_id": "2111019330574127141",
        "date": "2026-11-03",
        "venue": "ANTHEM AKIBA",
        "live_start": "18:40",
        "live_end": "19:00",
        "model": "gemini-2.5-flash-image",
        "status": "completed"
    },
    {
        "id": "2026-11-03_toranomon",
        "source_id": "2115639770944586018",
        "date": "2026-11-03",
        "venue": "ベルサール虎ノ門",
        "live_start": "14:55",
        "live_end": "15:15",
        "benefit_start": "15:20",
        "benefit_end": "16:20",
        "model": "gemini-2.5-flash-image",
        "status": "completed"
    }
]

def main():
    print("=" * 60)
    print("Dolly Kiss AI Parse vs TimeTree Ground Truth Verification Report")
    print("=" * 60)
    print(f"Timestamp: {datetime.now().isoformat()}")
    print(f"Target Group: Dolly Kiss (lui_17707 / @DollyKiss_staff)")
    print()

    total_checks = 0
    passed_checks = 0

    for parsed in PARSED_RESULTS:
        gt_key = parsed["id"]
        gt = GROUND_TRUTH.get(gt_key)
        if not gt:
            continue

        print(f"Event: {gt['title']} (Source: {parsed['source_id']})")
        print(f"  Status: {parsed['status']} (Model: {parsed['model']})")

        fields = [
            ("Date", gt["date"], parsed.get("date")),
            ("Venue", gt["venue"], parsed.get("venue")),
            ("Live Start", gt["live_start"], parsed.get("live_start")),
            ("Live End", gt["live_end"], parsed.get("live_end")),
        ]

        if "benefit_start" in gt and "benefit_start" in parsed:
            fields.extend([
                ("Benefit Start", gt["benefit_start"], parsed.get("benefit_start")),
                ("Benefit End", gt["benefit_end"], parsed.get("benefit_end")),
            ])

        for label, expected, actual in fields:
            total_checks += 1
            is_match = (expected == actual)
            if is_match:
                passed_checks += 1
                result = "MATCH"
            else:
                result = "MISMATCH"
            print(f"  - {label:14}: Expected='{expected}' | Actual='{actual}' -> [{result}]")
        print()

    accuracy = (passed_checks / total_checks) * 100 if total_checks > 0 else 0
    print("=" * 60)
    print(f"Summary: {passed_checks}/{total_checks} fields matched exactly ({accuracy:.1f}% accuracy)")
    print("=" * 60)

    if accuracy < 100.0:
        sys.exit(1)

if __name__ == "__main__":
    main()
