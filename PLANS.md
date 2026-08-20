# Esper Go migration active ExecPlan

This is the resumable plan for Codex migration work. Keep it accurate while a
work unit is in progress. It is an operational checkpoint, not a second
roadmap or an append-only history.

## Plan maintenance

- At task start, verify this checkpoint against Git, the manifest, evidence,
  and the roadmap. Memory or an older conversation is not current evidence.
- Before editing production code, fill in the work-unit contract and mark the
  first incomplete progress item.
- After investigation, implementation, validation, review, or a material
  decision, update the relevant section with concrete paths and commands.
- Keep only the current work unit and at most five one-line recent outcomes.
  Detailed completed history belongs in CHANGELOG and Git.
- Maintain the delegation checkpoint for the active unit. Parallel shell
  commands are not subagent delegation. Before implementation, record the
  Java/Go scout agent IDs or the exact serial-fallback reason; after targeted
  validation, record the independent reviewer ID and result.
- On interruption, leave the worktree state, exact next action, failures, and
  unverified assumptions explicit enough for a new Codex task to resume.
- Record validation and outcome before the semantic commit. Never edit this
  file after committing only to add the new commit hash; Git owns commit
  identity and post-push verification is read-only.

## Outcome

Continue the fixed-commit Esper 9.0.0 to Go migration until the final
acceptance criteria in `docs/esper-go-port-quality-strategy.md` all pass.
Progress is measured by verified work units and manifest evidence, not agent
activity or a single coverage percentage.

## Active checkpoint

- Updated: 2026-08-20
- Baseline: `master` is clean and matches `origin/master` at the current
  pushed checkpoint; the next semantic change must preserve that baseline.
- Status: logical-expression replay, review resolution, final diff review, and
  all required local gates are complete; the semantic delivery commit is next.
- Current work unit: `expr.core` / `case.expr-core-logical`
- Exact next action: review the final diff, create one semantic commit, push
  `master`, and verify the remote ref without tracked edits.
- Worktree notes: only the logical-expression work unit is modified; no
  unrelated user changes are present. Do not modify `/root/app/esper` or
  central facts outside this unit.

## Delegation checkpoint

- Collaboration facility: available; concurrent scout fan-out completed before
  implementation.
- Java oracle scout: `01a01d94-b622-7bd3-8172-72947b06e6cc` (`Goodall`),
  report received. The fixed source emits 3 combined rows, 4 variable rows,
  and 6 boxed/null rows in `c0...` insertion order; every row is new-only,
  there are no time steps or errors, and `thing` changes while `s0` remains
  deployed. The null matrix is `[null,T,T,T,T,F]`, `[null,F,F,F,F,T]`,
  `[null,F,F,null,null,null]`, `[null,null,null,T,T,null]`,
  `[null,F,F,T,T,T]`, `[null,F,F,T,T,F]` for E1..E6.
- Go surface scout: `01a01d94-d1d9-75e1-8b6d-3c8d949bbed8` (`Franklin`),
  report received. Existing `And`/`Or`/`Not`, `Contains`, `VariableRef`,
  nullable `Property`, and atomic variable snapshots require no production
  change; replay should extend the bitwise harness. The scout flags that
  direct variable-update protocol steps are not currently supported and that
  `Contains` null behavior is outside this non-null Java case.
- Parity asset worker: `01a01d95-8ae9-7c81-9734-ab5f9c70d3ce` (`Socrates`),
  report received and files reviewed. It changed only the Java oracle, runner,
  and scenario source files; no traces/evidence or central facts.
- Parity reviewer: `01a01da9-3bba-7973-a220-6f33a1cccde9` (`Rawls`),
  independent read-only review found one blocking scope issue: the Go
  scenario modeled Java's direct deployment variable-service update as a
  second event-driven setter statement, and evidence metadata was not pinned
  in the linkage test. Review also requested null-state and record-removal
  mutations.
- Review resolution: repaired. The protocol now has an explicit
  `set-variable` step; the Java oracle calls its deployment variable service,
  while Go uses direct `Engine.SetVariable` against a protected module's
  qualified deployment-owned variable. The linkage test pins commit/runtime/
  source/execution metadata and the mutation suite covers null-state and
  record removal. Follow-up review is clean.
- Follow-up parity reviewer: `01a01da9-3bba-7973-a220-6f33a1cccde9`
  (`Rawls`), read-only follow-up completed with no blocking findings. It
  confirmed direct variable lifecycle/ownership parity, metadata pinning, and
  independent Java-trace mutation authority; residual risks are field-map
  ordering and no dedicated undeploy/redeploy scenario.
- Serial exception: none; collaboration tools are available and both scout
  scopes are independent.

## Work-unit contract

- Capability/subdomain: `expr.core`, logical `and`/`or`/`not`
- Java source and executions/runtime IDs:
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreAndOrNot.java`;
  `ExprCoreAndOrNotCombined` / `java-runtime-e48bf14356e3aeb838b5`;
  `ExprCoreNotWithVariable` / `java-runtime-c63d599754acde7bb4dc`;
  `ExprCoreAndOrNotNull` / `java-runtime-b9d938f2dc52682b77c0`.
- Observable contract: the Java executions emit insert-stream rows for
  integer combined predicates, a variable-backed `not contains` predicate
  before and after an atomic variable update, and the boxed Boolean matrix
  for `and`, `or`, and `not`. Null is three-valued: false-and-null is false,
  true-and-null is null, true-or-null is true, false-or-null is null, and
  not-null is null. Result field order and boxed boolean/null values are
  observable; Java EPL/SODA syntax and exact boxed metadata/diagnostics are
  outside the Go fluent API. The replay must use fixed inputs and exercise
  normal values, explicit null boxed values, variable lifecycle, and row
  ordering.
- Allowed production files: existing typed logical expression code under
  `internal/esper` only if replay proves a semantic regression; prefer no
  production change because the unit tests already cover the core behavior.
- Allowed Go test/parity files: `internal/app/parity/expr_core_logical.go`,
  focused additions to `internal/esper/expr_logic_test.go`, and minimal
  dispatcher/test additions in `internal/app/parity/run.go` and `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreLogicalScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-logical.sh`, and
  `testdata/parity/expr-core-logical.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused
  `go test ./internal/esper -run 'Logical|And|Or|Not' -count=1`; focused
  parity replay and mutation tests; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.

## Progress

- [x] Reconfirm baseline, roadmap priorities, manifest summary, and relevant
      existing gates.
- [x] Select the next natural work unit from the `expr.core` manifest gap.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands using read-only investigation.
- [x] Run concurrent Java/Go read-only scouts and record their agent IDs and
      conclusions.
- [x] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [x] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [x] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts changed.
- [x] Run independent parity review, resolve findings, and run complete local
      gates plus the applicable milestone gate.
- [x] Repair review findings, regenerate independent traces/evidence, and rerun
      targeted validation.
- [x] Review the final diff, record actual validation, create one semantic
      commit, push `master`, and verify the remote ref without tracked edits.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The selected unit is the implemented but unverified logical
  expression trio. The fixed Java source has three executions covering
  ordinary integer predicates, variable updates, and boxed/null truth tables;
  the typed Go `And`/`Or`/`Not` and variable APIs already exist.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839b`; `master` is clean and matches
  `origin/master` at `072998d59cb051076527467c0c629039516f1f34`. Manifest
  summary reports 522 cases, 124 differential-verified cases, 378
  differential runtimes, 2,901 referenced runtimes, and 1,235 unreferenced
  runtime IDs. The target is now promoted to `differential-verified` with
  381 differential runtime IDs; aggregate association totals remain 3,067,
  2,901 referenced, and 1,235 unreferenced.
- Current unit result: Rawls found and blocked the initial event-driven
  variable-update model; the repaired direct `set-variable` replay now has
  13 matching listener records with zero differences, and the independently
  regenerated Java trace is byte-identical to the checked-in trace. Focused
  Esper/parity tests, metadata/null/removal mutation checks,
  compatibility/manifest tests, and `git diff --check` pass after repair.
  The earlier full suite, vet, make, and race results predate the repair and
  were rerun successfully after repair, and the follow-up review is clean.

## Delivery

- The previous unit is committed and pushed to `origin/master`; Git history is
  the authoritative source for its commit identity. This unit may be committed
  only after independent review and all required gates pass.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
