#!/usr/bin/env python3
"""Validate the explicit retrospective schedule, not simulated work duration."""
import os
import subprocess
import sys
from datetime import datetime, date
from zoneinfo import ZoneInfo

zone = ZoneInfo("America/New_York")
now = datetime.now(zone)
actual = os.environ.get("WATCHHOUSE_ACTUAL_EXECUTION")
if not actual:
    sys.exit("Use python3 scripts/commit-step.py MESSAGE to preserve actual execution time.")
try:
    actual_stamp = datetime.fromisoformat(actual)
    if actual_stamp.tzinfo is None or abs((actual_stamp - now).total_seconds()) > 120:
        raise ValueError("not a current timezone-aware execution timestamp")
except ValueError as exc:
    sys.exit(f"Invalid execution timestamp: {exc}")
stamps = []
for identity in ("GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"):
    # Git itself sets GIT_AUTHOR_DATE when running hooks; validate its value,
    # not the mere presence of that environment variable.
    value = subprocess.check_output(["git", "var", identity], text=True)
    stamp = datetime.fromtimestamp(int(value.rsplit(">", 1)[1].split()[0]), zone)
    if not date(2026, 4, 8) <= stamp.date() <= date(2026, 10, 5):
        sys.exit(f"{identity}: date outside the required window.")
    stamps.append(stamp)
if stamps[0] != stamps[1]:
    sys.exit("Author and committer timestamps must use the same schedule slot.")
previous = subprocess.run(["git", "log", "-1", "--format=%ct"], text=True, capture_output=True)
if previous.returncode == 0 and previous.stdout.strip():
    if stamps[0].timestamp() <= int(previous.stdout.strip()):
        sys.exit("Schedule timestamp must be later than the previous commit.")
