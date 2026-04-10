#!/bin/sh
# Real Linux binary execution, not a systemd-host integration test.
set -eu
cd "$(dirname "$0")/.."
GO=${GO:-go}
arch=$(docker info --format '{{.Architecture}}')
case "$arch" in
  aarch64|arm64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) echo "Unsupported Docker architecture: $arch" >&2; exit 1 ;;
esac
work=$(mktemp -d)
image="watchhouse-smoke:$(git rev-parse --short HEAD)-$$"
cleanup() {
  docker image rm "$image" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$GO" build -trimpath -o "$work/watchhouse" ./cmd/watchhouse
printf 'FROM scratch\nCOPY watchhouse /watchhouse\nUSER 65534:65534\nENTRYPOINT ["/watchhouse"]\n' > "$work/Dockerfile"
docker build --network none -q -t "$image" "$work" >/dev/null
docker run --rm --network none --read-only --cap-drop ALL \
  --security-opt no-new-privileges --memory 128m --pids-limit 32 \
  --mount "type=bind,source=$(pwd)/tests/fixtures/ssh-sequence.journal.jsonl,target=/fixture,readonly" \
  "$image" replay --host lab-1 --input /fixture > "$work/output.jsonl" 2> "$work/summary.json"
python3 - "$work" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
items = [json.loads(line) for line in (p / 'output.jsonl').read_text().splitlines()]
summary = json.loads((p / 'summary.json').read_text())
assert summary['stats'] == dict(records=8, matched=7, unmatched=1, findings=1, complete=True)
events = {item['event']['event_id'] for item in items if item['type'] == 'event'}
findings = [item['finding'] for item in items if item['type'] == 'finding']
assert len(events) == 7 and len(findings) == 1
assert len(findings[0]['evidence_event_ids']) == 6
assert all(identity in events for identity in findings[0]['evidence_event_ids'])
print('Linux scratch smoke PASS: non-root, read-only, no network; 7 events, 1 finding, 6 resolved evidence IDs.')
PY
if docker run --rm --network none --read-only --cap-drop ALL \
  --security-opt no-new-privileges --memory 128m --pids-limit 32 \
  "$image" snapshot --host lab-1 > "$work/snapshot.out" 2> "$work/snapshot.err"; then
  echo "snapshot unexpectedly succeeded without journalctl" >&2
  exit 1
fi
grep -q 'journalctl failed' "$work/snapshot.err"
echo 'Missing native journal rejected; this container does not validate systemd integration.'
