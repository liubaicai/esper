# Non-negotiable migration rules

- Treat the fixed Java commit as oracle; never change it to match Go output.
- Never claim `differential-verified` without replayable scenario, Java/Go traces, zero-difference evidence, and runtime IDs.
- Never bypass failures with Skip, weaker assertions, deleted tests, fabricated evidence, or unjustified `intentionally-different`.
- Only one agent may write a shared `internal/esper` semantic surface at a time.
- Subagents do not update manifest, roadmap, CHANGELOG, Goal, commit, or push.
- The primary agent runs all validation and reviews the integrated diff before committing.
- Preserve unrelated user changes and never submit or push a known failing state.
- Verified work units are committed and pushed directly to `master`; do not add GitLab CI or branch protection now.
