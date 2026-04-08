#!/usr/bin/env python3
"""Commit a staged work unit with disclosed schedule and actual-time provenance."""
import os
from pathlib import Path
import subprocess
import sys
from datetime import date, datetime, time, timedelta
from zoneinfo import ZoneInfo

START = date(2026, 4, 8)
END = date(2026, 10, 5)
ACTIVE_DAYS = 163
HOURS = (9, 13, 17)
ZONE = ZoneInfo("America/New_York")


def active_days():
    span = (END - START).days
    return [START + timedelta(days=round(i * span / (ACTIVE_DAYS - 1)))
            for i in range(ACTIVE_DAYS)]


def slot(index):
    if not 0 <= index < ACTIVE_DAYS * len(HOURS):
        raise ValueError("Schedule exhausted; revise the plan instead of creating filler.")
    day, hour = divmod(index, len(HOURS))
    return datetime.combine(active_days()[day], time(HOURS[hour]), ZONE)


def main():
    os.chdir(Path(__file__).resolve().parents[1])
    if len(sys.argv) != 2 or not sys.argv[1].strip():
        sys.exit("Usage: python3 scripts/commit-step.py 'type: meaningful change'")
    if subprocess.run(["git", "diff", "--cached", "--quiet"]).returncode == 0:
        sys.exit("No staged changes; empty commits are not allowed.")
    subprocess.run(["git", "diff", "--cached", "--check"], check=True)
    count = int(subprocess.check_output(["git", "rev-list", "--count", "HEAD"], text=True))
    planned = slot(count).isoformat()
    actual = datetime.now(ZONE).isoformat(timespec="seconds")
    message = (sys.argv[1].strip() + "\n\nActual-Execution-Time: " + actual
               + "\nHistory-Schedule: retrospective\n")
    env = dict(os.environ, GIT_AUTHOR_DATE=planned, GIT_COMMITTER_DATE=planned,
               WATCHHOUSE_ACTUAL_EXECUTION=actual)
    subprocess.run(["git", "commit", "-m", message], env=env, check=True)


if __name__ == "__main__":
    main()
