# Non-negotiable migration rules

- Treat the fixed Java commit as oracle; never change it to match Go output.
- Never claim `differential-verified` without replayable scenario, Java/Go traces, zero-difference evidence, and runtime IDs.
- Never bypass failures with Skip, weaker assertions, deleted tests, fabricated evidence, or unjustified `intentionally-different`.
- Only one agent may write a shared `internal/esper` semantic surface at a time.
- Batch independent work: when two safe read-only or disjoint tasks exist, do not spawn them as singleton tasks.
- Review N and prefetch N+1 together; do not `hub wait` while safe read-only pipeline work remains.
- Keep at most one prefetched work unit, and do not start its writes until N passes review and is committed.
- A shared-core writer may overlap only with one file-disjoint asset writer under a frozen contract.
- Reuse the same worker/reviewer for follow-up; do not respawn an agent for the same work-unit scope.
- Group tightly related executions that share one runtime surface and oracle harness to amortize review and gates.
- Subagents do not update manifest, roadmap, CHANGELOG, Goal, commit, or push.
- Subagents never hand-author generated trace/evidence; the primary agent generates and verifies them.
- The primary agent runs all validation and reviews the integrated diff before committing.
- Preserve unrelated user changes and never submit or push a known failing state.
- Verified work units are committed and pushed directly to `master`; do not add GitLab CI or branch protection now.
