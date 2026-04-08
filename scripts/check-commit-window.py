#!/usr/bin/env python3
"""Reject commits outside the documented real-time window. No backdating."""
import subprocess
import sys
from datetime import datetime, date
from zoneinfo import ZoneInfo

zone = ZoneInfo("America/New_York")
now = datetime.now(zone)
today = now.date()
if not date(2026, 4, 8) <= today <= date(2026, 10, 5):
    sys.exit("Commit window closed. Save work without committing; update the requirement first.")
for identity in ("GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"):
    # Git itself sets GIT_AUTHOR_DATE when running hooks; validate its value,
    # not the mere presence of that environment variable.
    value = subprocess.check_output(["git", "var", identity], text=True)
    stamp = datetime.fromtimestamp(int(value.rsplit(">", 1)[1].split()[0]), zone)
    if not date(2026, 4, 8) <= stamp.date() <= date(2026, 10, 5):
        sys.exit(f"{identity}: date outside the required window.")
    if abs((stamp - now).total_seconds()) > 120:
        sys.exit(f"{identity}: date differs from actual time; backdating is disabled.")
