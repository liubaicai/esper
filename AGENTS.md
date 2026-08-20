# bigsoc-esper agent instructions

## Mission

Port the fixed Esper 9.0.0 source at `/root/app/esper` to this Go module.
Treat Java as the observable-behavior oracle. The public Go rule API must stay
typed, fluent, Go-idiomatic, and Flink-inspired; EPL text is not the primary
public interface.

## Start and resume

For migration work:

1. Inspect the branch, worktree, and recent relevant history. Preserve all
   unrelated user changes.
2. Read `goal.txt`, then resume from `PLANS.md`. Update the active checkpoint
   in `PLANS.md` before implementation and after every material milestone.
3. Read the current priorities in `docs/esper-go-port-roadmap.md` and the
   target entries plus `summary` in
   `testdata/compat/capability-manifest.json`.
4. Read only the relevant parts of `docs/esper-go-port-runbook.md`,
   `docs/esper-go-port-quality-strategy.md`, the implementation plan, Java
   sources, Go sources, tests, scenarios, evidence, and Git history.
5. Use `docs/esper-go-port-codex-workflows.md` for Codex orchestration and
   `docs/esper-go-port-omp-workflows.md` for OMP-specific mechanics.

Validated manifest and evidence are current facts. The roadmap is current
priority. The implementation plan and ADRs define stable design. CHANGELOG and
Git are history. Correct prose in the same work unit when it conflicts with
validated machine evidence.

## Work units and ownership

Use one closed-loop work unit at a time, normally 1 to 5 tightly related Java
executions from one capability subdomain. Freeze the Java source/runtime IDs,
observable contract, allowed files, forbidden files, and targeted validation
before implementation.

The primary agent owns work-unit selection, the active plan, shared runtime
integration, generated traces/evidence, manifest status, roadmap, CHANGELOG,
formatting, tests, review, commits, and pushes.

Delegation is allowed for concrete, bounded tasks that can run independently.
Use 2 to 3 agents normally and never exceed 4 active agents. Parallelize
read-only Java/Go investigation and genuinely file-disjoint work after the
contract is frozen. There may be only one writer for any shared
`internal/esper` semantic surface. A second writer may edit assigned oracle,
scenario, or isolated parity-test source files only when ownership is disjoint.

Every delegated task must state `Target`, `Context`, `Allowed files`,
`Forbidden actions`, and `Deliverable`. Subagents do not update `PLANS.md`,
manifest, roadmap, CHANGELOG, or `goal.txt`; do not generate traces/evidence;
do not format, build, test, commit, or push. The primary agent reviews every
patch and performs integration and validation. If no subagent facility is
available, follow the same stages serially without weakening acceptance.

Review work unit N while prefetching at most one future unit N+1 read-only.
Do not start N+1 writes until N passes review and is committed. Reuse the same
subagent for follow-up on its scope when the runtime supports that continuity.

## Quality gates

- Never change the fixed Java oracle to make Go output pass.
- Never bypass a failure with Skip, weaker assertions, deleted tests,
  fabricated evidence, or an unjustified `intentionally-different` status.
- `implemented` is not parity. Use `differential-verified` only with a
  replayable scenario, Java and Go traces, zero-difference evidence, and exact
  runtime IDs.
- Cover values and ordering plus applicable old/new streams, Null/Missing/type
  behavior, virtual time, lifecycle, state transitions, and error phase.
- Stop new feature work when an existing gate is red. Reproduce and repair the
  baseline first, while distinguishing pre-existing failures from regressions.

Validate from narrow to broad: affected package tests, target Java/Go scenario
diff and required mutation, manifest/evidence checks, `make check`, and
`git diff --check`. At milestone boundaries also run the applicable race,
stress, Docker integration, and benchmark gates from the quality strategy.

## Delivery

The repository currently works directly on `master`; do not add GitLab CI or
branch protection. After a complete work unit passes all required gates and
independent parity review, create one semantic, revertible commit and push it
to `master`. Never commit or push a known failing state. Record the commit and
commands actually run in the active plan and CHANGELOG as appropriate.

Do not add project-level model or provider bindings. Model choice belongs to
the user's Codex/OMP settings. Never claim complete Esper parity until every
final acceptance condition in `docs/esper-go-port-quality-strategy.md` passes.
