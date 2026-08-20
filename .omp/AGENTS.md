# OMP adapter for the Esper Go migration

Read the repository-root `AGENTS.md` first. It is the shared source for the
migration mission, evidence hierarchy, work-unit ownership, quality gates, and
delivery rules used by both Codex and OMP. This file adds only OMP mechanics.

## Sources of truth

Load only the context needed for the current work unit:

1. `AGENTS.md` and `goal.txt` for the shared contract.
2. `docs/esper-go-port-roadmap.md` for current priorities and remaining work.
3. `testdata/compat/capability-manifest.json` for machine-verified status.
4. `docs/esper-go-port-runbook.md` and the relevant part of
   `docs/esper-go-port-quality-strategy.md` for execution and acceptance.
5. Use `rg` to locate only the relevant implementation, Java/Go sources,
   tests, scenarios, evidence, and history.
6. Use `docs/esper-go-port-omp-workflows.md` for OMP task batches and agents.

If prose and validated manifest/evidence conflict, machine evidence wins and
the prose must be corrected in the same work unit.

## Work ownership

Use OMP `task` batches for safe sibling tasks within the four-agent project
limit. Use `local://<path>` for large context and `schemaMode: strict` for
structured results. Every task uses `# Target`, `# Change`, and `# Acceptance`.
Use `isolated: true` only for a frozen, file-disjoint writing contract. OMP
patch merging is not proof of semantic correctness.

Optimize for verified work-unit throughput with a two-unit pipeline. Keep at
most one future work unit prefetched read-only. Once work unit N is integrated
and narrowly validated, launch one batch containing N's `parity-reviewer`, the
Java contract scout for N+1, and the Go surface scout for N+1. While that batch
runs, the primary agent may consolidate N+1's read-only contract, but must not
start N+1 writes until N passes review and is committed. Do not call `hub wait`
while safe read-only pipeline work remains.

After a contract is frozen, one shared-core writer and one
`parity-asset-worker` may run concurrently only when their allowed files are
disjoint. The asset lane may author assigned oracle/scenario/test sources; it
must not hand-author generated traces or evidence. A singleton task is allowed
only when no safe sibling task exists, and the primary agent must state that
reason before spawning it. `maxConcurrency` is a ceiling, not evidence that a
turn was parallel.

Subagents start blank and must receive all scoped context explicitly. Steer the
same worker with exact failures when repair is needed. Before `hub wait`, the
primary agent completes every safe read-only integration or prefetch action.

## Completion

A work unit is complete only after Java/Go differential evidence, affected
tests, manifest consistency, full local gates, independent parity review, and
documentation consistency pass. Then commit and push directly to `master`.
The project currently has no GitLab CI and does not protect `master`.
