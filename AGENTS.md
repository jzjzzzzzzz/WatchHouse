# Watchhouse development rules

- Read README.md and docs/requirements.md before changing this repository.
- Do not push or add a remote unless requested.
- Develop small, tested vertical slices. Keep docs honest about implementation status.
- No arbitrary shell execution or privileged write operations in the initial read-only phase.
- Do not treat fixtures, cross-compilation, or containers as proof of real host-system integration.
- Keep secrets, raw production telemetry, backups, toolchains, and build artifacts out of Git.
- No proactive sub-agent delegation unless the user explicitly asks for it.
