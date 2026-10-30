#!/usr/bin/env python3

from __future__ import annotations

import json
import sys
import unittest
from collections import Counter
from datetime import datetime
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch


sys.path.insert(0, str(Path(__file__).parent))
import report_access_counts as report  # noqa: E402


class ReportAccessCountsTest(unittest.TestCase):
    def test_parse_lines_filters_window_and_buckets_in_jst(self) -> None:
        lines = [
            json.dumps({"ts": "2026-10-29T15:05:00Z", "route": "document", "status": 200}),
            json.dumps({"ts": "2026-10-29T15:20:00Z", "route": "schedule", "status": 304}),
            json.dumps({"ts": "2026-10-29T16:00:00Z", "route": "asset", "status": 404}),
            json.dumps({"ts": "2026-10-29T18:00:00Z", "route": "document", "status": 200}),
            json.dumps({"ts": "2026-10-29T16:30:00Z", "route": "/user/secret", "status": 200}),
            "{not-json",
        ]
        start = report.parse_datetime("2026-10-30T00:00:00+09:00")
        end = report.parse_datetime("2026-10-30T03:00:00+09:00")

        routes, statuses, buckets, malformed = report.parse_log_lines(lines, start, end)

        self.assertEqual(routes, Counter({"document": 1, "schedule": 1, "asset": 1, "other": 1}))
        self.assertEqual(statuses, Counter({"2xx": 2, "3xx": 1, "4xx": 1}))
        self.assertEqual(malformed, 1)
        self.assertEqual(buckets[datetime(2026, 10, 30, 0, tzinfo=report.LOCAL_ZONE)], 2)
        self.assertEqual(buckets[datetime(2026, 10, 30, 1, tzinfo=report.LOCAL_ZONE)], 2)

    def test_message_contains_aggregate_only(self) -> None:
        start = report.parse_datetime("2026-10-30T00:00:00+09:00")
        end = report.parse_datetime("2026-10-30T01:00:00+09:00")
        message = report.build_message(
            start,
            end,
            Counter({"document": 2}),
            Counter({"2xx": 2}),
            Counter({datetime(2026, 10, 30, 0, tzinfo=report.LOCAL_ZONE): 2}),
            0,
        )

        self.assertIn("```text", message)
        self.assertIn("時間帯別リクエスト数", message)
        self.assertIn("document", message)
        self.assertIn("#", message)
        self.assertIn("00:00", message)
        self.assertNotIn("https://", message)
        self.assertNotIn("User-Agent", message)
        self.assertNotIn("192.0.2.1", message)

    def test_message_shows_zero_hour_buckets(self) -> None:
        start = report.parse_datetime("2026-10-30T00:00:00+09:00")
        end = report.parse_datetime("2026-10-30T03:00:00+09:00")
        message = report.build_message(
            start,
            end,
            Counter(),
            Counter(),
            Counter({datetime(2026, 10, 30, 1, tzinfo=report.LOCAL_ZONE): 2}),
            0,
        )

        self.assertIn("00:00", message)
        self.assertIn("01:00", message)
        self.assertIn("02:00", message)
        self.assertEqual(message.count("|"), 11)

    def test_render_bar_scales_to_maximum(self) -> None:
        self.assertEqual(report.render_bar(0, 10), "")
        self.assertEqual(report.render_bar(10, 10), "#" * report.BAR_WIDTH)
        self.assertEqual(len(report.render_bar(1, 10)), 2)

    def test_last_minutes_argument_is_supported(self) -> None:
        with patch.object(
            sys,
            "argv",
            ["report_access_counts.py", "--last-minutes", "5", "--dry-run"],
        ):
            args = report.parse_args()

        self.assertEqual(args.last_minutes, 5)
        self.assertIsNone(args.from_time)
        self.assertIsNone(args.to_time)

    def test_in_cluster_reader_uses_pod_and_log_api(self) -> None:
        args = SimpleNamespace(
            namespace="ribonclock",
            selector="app.kubernetes.io/name=ribonclock",
            container="web",
        )
        start = report.parse_datetime("2026-10-30T00:00:00+09:00")
        pod_payload = json.dumps(
            {"items": [{"metadata": {"name": "ribonclock-abc"}}]}
        ).encode()
        log_payload = b'{"ts":"2026-10-29T15:05:00Z","route":"document","status":200}\n'

        with patch.object(
            report,
            "kubernetes_api_request",
            side_effect=[pod_payload, log_payload],
        ) as request:
            lines = list(report.read_in_cluster_log_lines(args, start))

        self.assertEqual(lines, [log_payload.decode().strip()])
        self.assertEqual(request.call_count, 2)

    def test_invalid_range_is_rejected(self) -> None:
        start = report.parse_datetime("2026-10-30T01:00:00+09:00")
        end = report.parse_datetime("2026-10-30T00:00:00+09:00")
        self.assertGreaterEqual(start, end)

    def test_discord_message_limit_is_explicit(self) -> None:
        self.assertEqual(report.MAX_DISCORD_MESSAGE_LENGTH, 1900)


if __name__ == "__main__":
    unittest.main()
