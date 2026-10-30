#!/usr/bin/env python3
"""Aggregate privacy-safe Nginx logs and optionally post a Discord summary."""

from __future__ import annotations

import argparse
import json
import os
import ssl
import subprocess
import sys
from collections import Counter
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Iterable
from urllib import error as urllib_error
from urllib import request as urllib_request
from urllib.parse import quote, urlencode
from zoneinfo import ZoneInfo


LOCAL_ZONE = ZoneInfo("Asia/Tokyo")
ALLOWED_ROUTES = {"document", "schedule", "asset", "other"}
BAR_WIDTH = 20
MAX_DISCORD_MESSAGE_LENGTH = 1900
SERVICE_ACCOUNT_ROOT = Path("/var/run/secrets/kubernetes.io/serviceaccount")


class ReportError(Exception):
    """Expected user-facing report errors."""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Aggregate privacy-safe Nginx access logs and optionally notify Discord."
    )
    time_group = parser.add_mutually_exclusive_group(required=True)
    time_group.add_argument("--from", dest="from_time", help="Inclusive ISO 8601 start time")
    time_group.add_argument(
        "--last-minutes",
        type=int,
        help="Use the previous N minutes ending at the current time",
    )
    parser.add_argument("--to", dest="to_time", help="Exclusive ISO 8601 end time")
    parser.add_argument(
        "--input",
        type=Path,
        help="Read fixture or saved logs from this file; omit to call kubectl logs",
    )
    parser.add_argument("--namespace", default="ribonclock", help="Kubernetes namespace")
    parser.add_argument(
        "--selector",
        default="app.kubernetes.io/name=ribonclock",
        help="Kubernetes pod label selector",
    )
    parser.add_argument("--kubectl", default="kubectl", help="kubectl executable")
    parser.add_argument(
        "--kubernetes-api",
        action="store_true",
        help="Read pod logs through the in-cluster Kubernetes API instead of kubectl",
    )
    parser.add_argument(
        "--container",
        default="web",
        help="Container name when reading logs through the Kubernetes API",
    )
    parser.add_argument(
        "--webhook-env",
        default="DISCORD_WEBHOOK_URL",
        help="Environment variable containing the Discord Webhook URL",
    )
    parser.add_argument("--dry-run", action="store_true", help="Print the message without sending it")
    return parser.parse_args()


def parse_datetime(value: str) -> datetime:
    normalized = value.strip().replace("Z", "+00:00")
    try:
        parsed = datetime.fromisoformat(normalized)
    except ValueError as exc:
        raise ReportError(f"invalid ISO 8601 time: {value}") from exc
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=LOCAL_ZONE)
    return parsed.astimezone(timezone.utc)


def format_local_time(value: datetime) -> str:
    return value.astimezone(LOCAL_ZONE).strftime("%Y-%m-%d %H:%M")


def kubernetes_api_request(
    path: str, query: dict[str, str] | None = None
) -> bytes:
    token_path = Path(os.environ.get("KUBERNETES_TOKEN_FILE", SERVICE_ACCOUNT_ROOT / "token"))
    ca_path = Path(os.environ.get("KUBERNETES_CA_FILE", SERVICE_ACCOUNT_ROOT / "ca.crt"))
    try:
        token = token_path.read_text(encoding="utf-8").strip()
    except OSError as exc:
        raise ReportError("Kubernetes service account token is unavailable") from exc
    if not token:
        raise ReportError("Kubernetes service account token is empty")

    host = os.environ.get("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc")
    port = os.environ.get("KUBERNETES_SERVICE_PORT_HTTPS", "443")
    request_url = f"https://{host}:{port}{path}"
    if query:
        request_url = f"{request_url}?{urlencode(query)}"
    request = urllib_request.Request(
        request_url,
        headers={
            "Authorization": f"Bearer {token}",
            "User-Agent": "ribonclock-access-report/1.0",
        },
        method="GET",
    )
    try:
        context = ssl.create_default_context(cafile=str(ca_path))
        with urllib_request.urlopen(request, context=context, timeout=10) as response:
            return response.read()
    except urllib_error.HTTPError as exc:
        raise ReportError(f"Kubernetes API returned HTTP {exc.code}") from exc
    except (OSError, urllib_error.URLError) as exc:
        raise ReportError("Kubernetes API request failed") from exc


def read_in_cluster_log_lines(args: argparse.Namespace, start: datetime) -> Iterable[str]:
    namespace = quote(args.namespace, safe="")
    try:
        payload = json.loads(
            kubernetes_api_request(
                f"/api/v1/namespaces/{namespace}/pods",
                {"labelSelector": args.selector},
            ).decode("utf-8")
        )
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ReportError("Kubernetes API returned invalid pod data") from exc

    pods = []
    for item in payload.get("items", []):
        metadata = item.get("metadata", {})
        name = metadata.get("name")
        if name and not metadata.get("deletionTimestamp"):
            pods.append(name)
    if not pods:
        raise ReportError("no matching running pods found")

    since_time = start.isoformat().replace("+00:00", "Z")
    for pod_name in pods:
        encoded_pod_name = quote(pod_name, safe="")
        try:
            log_data = kubernetes_api_request(
                f"/api/v1/namespaces/{namespace}/pods/{encoded_pod_name}/log",
                {
                    "container": args.container,
                    "sinceTime": since_time,
                    "timestamps": "false",
                },
            ).decode("utf-8")
        except UnicodeDecodeError as exc:
            raise ReportError("Kubernetes API returned invalid log data") from exc
        yield from log_data.splitlines()


def read_log_lines(args: argparse.Namespace, start: datetime) -> Iterable[str]:
    if args.input:
        if str(args.input) == "-":
            yield from sys.stdin
            return
        try:
            with args.input.open("r", encoding="utf-8") as source:
                yield from source
        except OSError as exc:
            raise ReportError(f"cannot read input log: {args.input}") from exc
        return

    if args.kubernetes_api:
        yield from read_in_cluster_log_lines(args, start)
        return

    since_time = start.isoformat().replace("+00:00", "Z")
    command = [
        args.kubectl,
        "logs",
        "--namespace",
        args.namespace,
        "--selector",
        args.selector,
        "--all-containers=false",
        "--prefix=false",
        "--tail=-1",
        "--since-time",
        since_time,
    ]
    try:
        completed = subprocess.run(command, check=False, capture_output=True, text=True)
    except OSError as exc:
        raise ReportError(f"cannot execute {args.kubectl}") from exc
    if completed.returncode != 0:
        raise ReportError(f"kubectl logs failed with exit code {completed.returncode}")
    yield from completed.stdout.splitlines()


def parse_log_lines(
    lines: Iterable[str], start: datetime, end: datetime
) -> tuple[Counter[str], Counter[str], Counter[datetime], int]:
    routes: Counter[str] = Counter()
    statuses: Counter[str] = Counter()
    buckets: Counter[datetime] = Counter()
    malformed = 0

    for line in lines:
        stripped_line = line.strip()
        if not stripped_line or not stripped_line.startswith("{"):
            continue
        try:
            payload = json.loads(stripped_line)
            timestamp = parse_datetime(str(payload["ts"]))
            route = payload.get("route", "other")
            status = int(payload["status"])
            if route not in ALLOWED_ROUTES:
                route = "other"
            if not 100 <= status <= 599:
                raise ValueError("status out of range")
        except (KeyError, TypeError, ValueError, json.JSONDecodeError, ReportError):
            malformed += 1
            continue

        if not start <= timestamp < end:
            continue

        local_timestamp = timestamp.astimezone(LOCAL_ZONE)
        bucket = local_timestamp.replace(minute=0, second=0, microsecond=0)
        routes[route] += 1
        statuses[f"{status // 100}xx"] += 1
        buckets[bucket] += 1

    return routes, statuses, buckets, malformed


def iter_hour_buckets(start: datetime, end: datetime) -> Iterable[datetime]:
    """Yield every local-hour bucket touched by the half-open report window."""
    cursor = start.astimezone(LOCAL_ZONE).replace(minute=0, second=0, microsecond=0)
    local_end = end.astimezone(LOCAL_ZONE)
    while cursor < local_end:
        yield cursor
        cursor += timedelta(hours=1)


def render_bar(value: int, maximum: int, width: int = BAR_WIDTH) -> str:
    """Render a proportional ASCII bar without leaking any raw request data."""
    if value <= 0 or maximum <= 0:
        return ""
    filled = max(1, min(width, value * width // maximum))
    return "#" * filled


def format_graph_line(label: str, value: int, maximum: int) -> str:
    bar = render_bar(value, maximum)
    return f"{label:<16} | {bar:<{BAR_WIDTH}} {value:>4}"


def build_message(
    start: datetime,
    end: datetime,
    buckets: Counter[datetime],
    malformed: int,
) -> str:
    total = sum(buckets.values())
    hour_buckets = list(iter_hour_buckets(start, end))
    hour_values = [buckets.get(bucket, 0) for bucket in hour_buckets]
    hour_maximum = max(hour_values, default=0)
    hour_label_format = "%H:%M" if (end - start).total_seconds() <= 24 * 60 * 60 else "%m-%d %H:%M"

    lines = [
        "Ribbon Clock アクセス集計",
        f"対象: {format_local_time(start)}〜{format_local_time(end)} JST",
        "種別: origin request count",
        "",
        "```text",
        "時間帯別アクセス数",
    ]
    lines.extend(
        format_graph_line(bucket.strftime(hour_label_format), buckets.get(bucket, 0), hour_maximum)
        for bucket in hour_buckets
    )

    lines.extend(["```", "", f"合計: {total}"])

    if malformed:
        lines.extend(["", f"解析対象外の行: {malformed}"])
    return "\n".join(lines)


def post_to_discord(webhook_url: str, message: str) -> None:
    if len(message) > MAX_DISCORD_MESSAGE_LENGTH:
        raise ReportError("report is too long for Discord; use a shorter time range")
    payload = json.dumps({"content": message}, ensure_ascii=False).encode("utf-8")
    request = urllib_request.Request(
        webhook_url,
        data=payload,
        headers={
            "Content-Type": "application/json",
            "User-Agent": "ribonclock-access-report/1.0",
        },
        method="POST",
    )
    try:
        with urllib_request.urlopen(request, timeout=10) as response:
            if not 200 <= response.status < 300:
                raise ReportError(f"Discord Webhook returned HTTP {response.status}")
    except urllib_error.HTTPError as exc:
        raise ReportError(f"Discord Webhook returned HTTP {exc.code}") from exc
    except urllib_error.URLError as exc:
        raise ReportError("Discord Webhook request failed") from exc


def main() -> int:
    args = parse_args()
    try:
        if args.input and args.kubernetes_api:
            raise ReportError("--input and --kubernetes-api cannot be used together")
        if args.last_minutes is not None:
            if args.to_time:
                raise ReportError("--to cannot be used with --last-minutes")
            if args.last_minutes <= 0:
                raise ReportError("--last-minutes must be greater than zero")
            end = datetime.now(timezone.utc)
            start = end - timedelta(minutes=args.last_minutes)
        else:
            if not args.to_time:
                raise ReportError("--to is required when --from is used")
            start = parse_datetime(args.from_time)
            end = parse_datetime(args.to_time)
        if start >= end:
            raise ReportError("--from must be earlier than --to")

        routes, statuses, buckets, malformed = parse_log_lines(
            read_log_lines(args, start), start, end
        )
        message = build_message(start, end, buckets, malformed)

        if args.dry_run:
            print(message)
            return 0

        webhook_url = os.environ.get(args.webhook_env, "")
        if not webhook_url:
            raise ReportError(f"environment variable {args.webhook_env} is not set")
        post_to_discord(webhook_url, message)
        print("Access report sent to Discord.")
        return 0
    except ReportError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
