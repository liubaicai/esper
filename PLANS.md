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
- Baseline: `master` is clean and matches `origin/master` at pushed commit
  `96a2aadf3`; the next semantic change must preserve that baseline.
- Status: coalesce is committed and pushed. Relop replay implementation,
  independent review, targeted validation, and all local milestone gates are
  complete; final delivery remains.
- Current work unit: `expr.core` / `case.expr-core-relop`
- Exact next action: create one semantic commit, push `master`, and verify the
  remote ref without tracked edits.
- Worktree notes: only the expected relop implementation, parity assets,
  central facts, and this checkpoint are modified/untracked; do not modify
  `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; relop scout fan-out started concurrently
  before implementation.
- Java oracle scout: `01a01e04-304f-7631-ae3f-7449fcef4e76` (`Nash`), completed.
  Confirmed nine independent value statements plus one Null statement, 30
  listener records, fields `c0..c3`/`c0..c7`, boxed Boolean results, fresh
  deploy/undeploy boundaries, and no timers/windows/old-stream rows.
- Go surface scout: `01a01e04-2f3d-7272-9680-e0c785039915` (`McClintock`),
  completed. Confirmed the existing four mixed relational builders and exact
  numeric/string/Null coverage; no production semantic change is expected.
- Independent parity reviewer: `01a01e1c-33d8-73a2-9ea6-1a58bda362c7`
  (`Tesla`), completed read-only review. It found one medium scenario-shape
  validation gap; the primary agent tightened Go and shell validation to the
  exact ten-case order/three-send contract and added a malformed-shape test.
  No semantic, trace, metadata, or manifest mismatch remained.
- Previous coalesce scouts/review: retained in Git history and the prior
  semantic checkpoint; the unit was pushed as `96a2aadf3`.
- Serial exception: none; both required relop scouts were started through the
  collaboration facility with disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, mixed relational operators and Null
  propagation
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreRelOp.java`;
  `ExprCoreRelOpTypes` / `java-runtime-615cb125ab25e488f40c`;
  `ExprCoreRelOpNull` / `java-runtime-402d95bd69d700735100`.
- Differential scope: both executions are replayed with ten deterministic
  case markers: `relop-string`, `relop-int`, `relop-long`, `relop-float`,
  `relop-double`, `relop-big-decimal`, `relop-int-big-decimal`,
  `relop-big-integer`, `relop-int-big-integer`, and `relop-null`. Each value
  marker sends three events and emits one new row; the Null marker sends E1,
  E2, and E3. This yields 30 ordered records. There is no separate compile-
  error runtime; invalid-builder behavior remains Go-unit evidence.
- Observable contract: mixed relational predicates must preserve Java's
  numeric/string comparison, result field order and boxed Boolean type,
  Null/Missing and typed-nil propagation, record order, and fresh
  execution-local lifecycle boundaries. Value rows are `[false,false,true,true]`,
  `[true,false,true,false]`, and `[true,true,false,false]`; Null rows are
  `[null,true,null,null,null,false,true,true]`,
  `[null,true,null,null,null,true,false,false]`, and eight Nulls. No time
  advances, old-stream rows, windows, or timers occur.
  The exact EPL/SODA/compile syntax and parser diagnostics remain outside the
  typed fluent API. The replay must use fixed inputs and include normal mixed
  values, Null/Missing values, coercion-sensitive boundaries, record order, and
  the relevant invalid/build boundary.
- Allowed production files: existing typed relational code under
  `internal/esper` only if differential replay proves a semantic regression;
  prefer no production change.
- Allowed Go test/parity files: a new
  `internal/app/parity/expr_core_relop.go`, focused additions only if needed
  under `internal/esper`, and minimal dispatcher/test additions in
  `internal/app/parity/run.go` and `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreRelOpScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-relop.sh`, and
  `testdata/parity/expr-core-relop.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused relational
  Esper tests; focused parity replay and mutation tests; and
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
- [x] Review the final diff and record actual validation.
- [ ] Create one semantic commit, push `master`, and verify the remote ref
      without tracked edits.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The previous coalesce unit was delivered as `96a2aadf3`, with
  six zero-difference runtimes and 22 replay records; its invalid compile case
  remains implemented-only.
- 2026-08-20: `case.expr-core-relop` is the next natural expression unit: two
  implemented-only runtimes with existing typed mixed relational builders and
  focused tests. Nash confirmed the ten-marker/30-record Java contract;
  McClintock confirmed no production semantic gap in the Go surface.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` was clean and matched
  `origin/master` at `96a2aadf366bdcb77814224aea09cc76e395f81b`. The manifest
  then reported 522 cases, 126 differential-verified cases, 387 differential
  runtimes, 2,901 referenced runtimes, and 1,235 unreferenced runtime IDs.
- Current unit result: relop replay, review, and complete local validation are
  complete. The pinned Java runner produced 30 records; the checked-in Java
  trace and Go replay both hash to
  `04af6cdc70dc929c85aa308374521cc47c91f6145566e4845784ef49041ad34f`.
  Differential evidence reports `passing` with zero differences and exact
  runtime/source/execution metadata. Focused relational Esper/parity/compat/
  manifest tests, the malformed-scenario and trace mutation checks,
  `go vet ./...`, `go test ./... -count=1 -timeout 240s`,
  `go test -race ./internal/app/parity ./internal/esper -count=1 -timeout 600s`,
  `make check`, `jq`, `sh -n`, and `git diff --check` all pass.
- Final diff audit: only the expected relop parity implementation, dispatcher
  and tests, fixed Java oracle/runner, scenario/trace/evidence, manifest,
  roadmap, CHANGELOG, and this checkpoint are changed; no `internal/esper`
  production semantic file, `/root/app/esper`, or `goal.txt` was modified.

## Delivery

- The previous unit is committed and pushed to `origin/master` at
  `96a2aadf3`; Git history is the authoritative source for its identity. The
  relop unit may be committed only after independent review and all required
  gates pass.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
