# Watchhouse development rules

- Read README.md and docs/requirements.md before changing this repository.
- Commits must use real author and committer timestamps within 2026-04-08 through 2026-10-05 inclusive, America/New_York. After the window, leave work uncommitted until the user revises the requirement. Never backdate or fabricate development history.
- Do not push or add a remote unless requested.
- Develop small, tested vertical slices. Keep docs honest about implementation status.
- No arbitrary shell execution or privileged write operations in the initial read-only phase.
- Do not treat fixtures, cross-compilation, or containers as proof of real host-system integration.
- Keep secrets, raw production telemetry, backups, toolchains, and build artifacts out of Git.
- No proactive sub-agent delegation unless the user explicitly asks for it.
