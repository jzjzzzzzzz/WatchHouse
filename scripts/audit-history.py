#!/usr/bin/env python3
"""Audit disclosed retrospective Git history and meaningful tree changes."""
import argparse
import datetime
import json
import math
import subprocess
from zoneinfo import ZoneInfo

START = datetime.date(2026, 4, 8)
END = datetime.date(2026, 10, 5)
ZONE = ZoneInfo("America/New_York")


def git(*args):
    return subprocess.run(["git", *args], check=True, capture_output=True, text=True).stdout


def audit(require_complete=False):
    commits = git("rev-list", "--reverse", "HEAD").splitlines()
    if not commits:
        raise RuntimeError("history is empty")
    dates, actual_times = [], []
    for index, commit in enumerate(commits):
        fields = git("show", "-s", "--format=%aI%n%cI%n%B", commit).splitlines()
        author, committer = datetime.datetime.fromisoformat(fields[0]), datetime.datetime.fromisoformat(fields[1])
        if author != committer or author.tzinfo is None or not START <= author.astimezone(ZONE).date() <= END:
            raise RuntimeError(f"invalid scheduled identity date at {commit}")
        dates.append(author)
        trailers = [line.split(": ", 1)[1] for line in fields[2:] if line.startswith("Actual-Execution-Time: ")]
        if len(trailers) != 1:
            raise RuntimeError(f"missing unique actual execution trailer at {commit}")
        actual = datetime.datetime.fromisoformat(trailers[0])
        if actual.tzinfo is None:
            raise RuntimeError(f"naive actual execution time at {commit}")
        actual_times.append(actual)
        if index:
            changed = subprocess.run(["git", "diff-tree", "--quiet", commits[index - 1], commit]).returncode
            if changed == 0:
                raise RuntimeError(f"empty tree commit at {commit}")
    if any(left >= right for left, right in zip(dates, dates[1:])):
        raise RuntimeError("scheduled commit timestamps are not strictly increasing")
    unique_days = len({value.astimezone(ZONE).date() for value in dates})
    required_days = math.ceil(((END - START).days + 1) * 0.90)
    complete = dates[-1].astimezone(ZONE).date() == END and unique_days >= required_days
    if require_complete and not complete:
        raise RuntimeError(f"history incomplete: last={dates[-1].date()} unique_days={unique_days}/{required_days}")
    return {"schema_version": 1, "commits": len(commits), "unique_scheduled_days": unique_days,
            "required_days": required_days, "first_scheduled": dates[0].isoformat(),
            "last_scheduled": dates[-1].isoformat(), "complete": complete,
            "actual_execution_first": min(actual_times).isoformat(),
            "actual_execution_last": max(actual_times).isoformat()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--require-complete", action="store_true")
    args = parser.parse_args()
    print(json.dumps(audit(args.require_complete), sort_keys=True))


if __name__ == "__main__":
    main()
