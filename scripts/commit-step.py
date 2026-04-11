#!/usr/bin/env python3
"""Commit a staged work unit with disclosed schedule and actual-time provenance."""
import os
import hashlib
import random
from pathlib import Path
import subprocess
import sys
from datetime import date, datetime, time, timedelta
from zoneinfo import ZoneInfo

START = date(2026, 4, 8)
END = date(2026, 10, 5)
ACTIVE_DAYS = 163
SLOTS_PER_DAY = 3
MIN_SPACING_SECONDS = 20 * 60
ZONE = ZoneInfo("America/New_York")


def active_days():
    span = (END - START).days
    return [START + timedelta(days=round(i * span / (ACTIVE_DAYS - 1)))
            for i in range(ACTIVE_DAYS)]


def day_seconds(day):
    # Reproducible random scheduling: inspecting/retrying the same slot must
    # not silently change its timestamp. This is not an execution-time claim.
    seed = hashlib.sha256(("watchhouse.schedule.v2:" + day.isoformat()).encode()).digest()
    rng = random.Random(int.from_bytes(seed, "big"))
    while True:
        seconds = sorted(rng.sample(range(8 * 3600, 23 * 3600), SLOTS_PER_DAY))
        if all(b - a >= MIN_SPACING_SECONDS for a, b in zip(seconds, seconds[1:])):
            return seconds


def slot(index):
    if not 0 <= index < ACTIVE_DAYS * SLOTS_PER_DAY:
        raise ValueError("Schedule exhausted; revise the plan instead of creating filler.")
    day_index, position = divmod(index, SLOTS_PER_DAY)
    day = active_days()[day_index]
    # Keep the nine existing commits unchanged. Only future slots use v2.
    if index < 9:
        seconds = (9, 13, 17)[position] * 3600
    else:
        seconds = day_seconds(day)[position]
    return datetime.combine(day, time(), ZONE) + timedelta(seconds=seconds)


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
