# Esper Go migration context

This repository ports the fixed Esper 9.0.0 source at `/root/app/esper` to Go.
Java is the observable-behavior oracle. Public rule construction is a typed,
fluent, Flink-inspired Go API; EPL is not the primary public Go interface.

## Sources of truth

Load only the context needed for the current work unit:

1. `docs/esper-go-port-roadmap.md` for current priorities and remaining work.
2. `testdata/compat/capability-manifest.json` for machine-verified status.
3. `docs/esper-go-port-runbook.md` and the relevant part of
   `docs/esper-go-port-quality-strategy.md` for execution and acceptance.
4. Use `rg` to locate only the relevant section of the large implementation
   plan, Java source, Go implementation, tests, scenarios, and recent history.
5. Use `docs/esper-go-port-omp-workflows.md` when spawning OMP tasks.

If prose and validated manifest/evidence conflict, machine evidence wins and
the prose must be corrected in the same work unit.

## Work ownership

The primary agent owns work-unit selection, shared runtime integration,
manifest/evidence status, roadmap, CHANGELOG, validation, commits, and pushes.
Use parallel read-only scouts freely within the four-agent project limit.
There may be only one writer for a shared `internal/esper` semantic surface.
Use isolated writing agents only for independent components with a frozen
contract. OMP text conflict resolution is not proof of semantic correctness.

Subagents start blank. Give them `local://` references instead of pasting large
documents. Every task must use `# Target`, `# Change`, and `# Acceptance`.
Subagents skip formatters, linters, builds, tests, commits, and pushes; the
primary agent validates once after integration and steers the same worker with
specific failures when repair is needed.

## Completion

A work unit is complete only after Java/Go differential evidence, affected
tests, manifest consistency, full local gates, independent parity review, and
documentation consistency pass. Then commit and push directly to `master`.
The project currently has no GitLab CI and does not protect `master`.
