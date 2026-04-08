# Watchhouse development rules

- Read README.md and docs/requirements.md before changing this repository.
- Author and committer dates use the explicitly documented retrospective schedule within 2026-04-08 through 2026-10-05, America/New_York. Use scripts/commit-step.py. Preserve actual execution time in commit trailers; never claim the schedule proves past work or long-term operation. Target 163 active days with multiple meaningful commits, not empty filler.
- Do not push or add a remote unless requested.
- Develop small, tested vertical slices. Keep docs honest about implementation status.
- No arbitrary shell execution or privileged write operations in the initial read-only phase.
- Do not treat fixtures, cross-compilation, or containers as proof of real host-system integration.
- Keep secrets, raw production telemetry, backups, toolchains, and build artifacts out of Git.
- No proactive sub-agent delegation unless the user explicitly asks for it.
