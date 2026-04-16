#!/usr/bin/env python3
"""Kill only a spawned ingest child and verify its committed WAL state survives."""
import argparse
import json
from pathlib import Path
import sqlite3
import subprocess
import tempfile
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", default="bin/watchhouse")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    binary = (root / args.binary).resolve()
    fixture = root / "tests/fixtures/ssh-sequence.journal.jsonl"
    lines = fixture.read_bytes().splitlines(keepends=True)
    with tempfile.TemporaryDirectory(prefix="watchhouse-crash-") as temporary:
        state = Path(temporary) / "state"
        command = [str(binary), "spool", "ingest", "--state", str(state), "--host", "lab-1"]
        child = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        try:
            child.stdin.write(b"".join(lines[:2]))
            child.stdin.flush()
            deadline = time.monotonic() + 15
            observed = False
            while time.monotonic() < deadline:
                if child.poll() is not None:
                    raise RuntimeError("ingest exited before kill: " + child.stderr.read().decode())
                database = state / "queue.db"
                if database.exists():
                    try:
                        with sqlite3.connect(database.as_uri() + "?mode=ro", uri=True, timeout=0.1) as db:
                            count = db.execute("SELECT COUNT(*) FROM events").fetchone()[0]
                            checkpoint = db.execute("SELECT cursor FROM checkpoints WHERE host_id='lab-1'").fetchone()
                            if count == 2 and checkpoint == ("s=fixture;i=2",):
                                observed = True
                                break
                    except sqlite3.OperationalError:
                        pass  # Initialization or a transient lock, within a bounded wait.
                time.sleep(0.02)
            if not observed:
                raise RuntimeError("committed input was not observed within the deadline")
            child.kill()  # SIGKILL on supported Unix platforms; never targets unrelated PIDs.
            child.wait(timeout=5)
        finally:
            if child.poll() is None:
                child.kill()
                child.wait(timeout=5)
            for stream in (child.stdin, child.stdout, child.stderr):
                stream.close()
        status = subprocess.run([str(binary), "spool", "status", "--state", str(state)], check=True, capture_output=True, text=True, timeout=15)
        assert json.loads(status.stdout)["stats"]["pending_records"] == 2
        resumed = subprocess.run(command + ["--input", str(fixture)], check=True, capture_output=True, text=True, timeout=15)
        stats = json.loads(resumed.stdout)["stats"]
        assert stats["skipped"] == 2 and stats["inserted"] == 5 and stats["complete"]
        status = subprocess.run([str(binary), "spool", "status", "--state", str(state)], check=True, capture_output=True, text=True, timeout=15)
        assert json.loads(status.stdout)["stats"]["pending_records"] == 7
        for file in state.iterdir():
            assert file.stat().st_mode & 0o777 == 0o600, "state sidecar is not owner-only"
        print(json.dumps({"type": "spool_crash_result", "passed": True,
                          "before_kill_records": 2, "after_restart_records": 2,
                          "after_resume_records": 7, "resumed_insertions": 5,
                          "scope": "process SIGKILL after observed commit; not power-loss simulation"}))


if __name__ == "__main__":
    main()
