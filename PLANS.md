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
- On interruption, leave the worktree state, exact next action, failures, and
  unverified assumptions explicit enough for a new Codex task to resume.

## Outcome

Continue the fixed-commit Esper 9.0.0 to Go migration until the final
acceptance criteria in `docs/esper-go-port-quality-strategy.md` all pass.
Progress is measured by verified work units and manifest evidence, not agent
activity or a single coverage percentage.

## Active checkpoint

- Updated: 2026-08-20
- Baseline: `master` at `57c2079bd` (verified clean and matching
  `origin/master` before this work unit).
- Status: Current-evaluation-context is differential-verified; parity assets,
  manifest/docs, canonical Java/Go traces, and all required local gates are
  complete. The semantic commit and push are the remaining delivery action.
- Current work unit: `expr.core` / `case.expr-core-current-evaluation-context`
- Exact next action: review the final diff, create the semantic commit, push
  `master`, and record the resulting commit in this checkpoint.
- Worktree notes: current work-unit files are modified; no unrelated files are
  in scope; `master` still matches `origin/master` before delivery.

## Work-unit contract

- Capability/subdomain: `expr.core`, current evaluation context metadata
- Java source and executions/runtime IDs:
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCurrentEvaluationContext.java`;
  `ExprCoreCurrentEvalCtx{soda=false}` /
  `java-runtime-2efbbb4fce55aa6ce513`;
  `ExprCoreCurrentEvalCtx{soda=true}` /
  `java-runtime-ca0799a6f2d163a49f7f`.
- Observable contract: each case deploys one statement named `s0`, projects
  `current_evaluation_context()` twice and its `getRuntimeURI()` accessor, and
  sends one `SupportBean` at virtual time 0 ms. The listener emits one new row
  with repeated equivalent context metadata and the runtime URI accessor:
  runtime URI `parity-expr-core-current-evaluation-context`, statement name
  `s0`, user object `my_user_object`, and non-context partition ID `-1`; there
  is no old stream. Java boxed metadata and EPL/SODA compilation are normalized
  to the typed Go `ExpressionEvaluationContext` representation. The direct
  evaluation default partition normalization remains covered by the existing
  Go unit test but is outside this replay trace.
- Allowed production files: `internal/app/parity/expr_core_current_evaluation_context.go`,
  `internal/app/parity/run.go`, and focused additions in
  `internal/app/parity/run_test.go`; modify `internal/esper` only if the
  frozen parity replay demonstrates a real regression.
- Allowed parity asset files: `tools/java-oracle/ExprCoreCurrentEvaluationContextScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-current-evaluation-context.sh`, and
  `testdata/parity/expr-core-current-evaluation-context.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: unrelated semantic surfaces; `goal.txt`;
  generated evidence before trace validation; and central facts outside this
  unit's manifest/roadmap/CHANGELOG updates. `PLANS.md`, the manifest,
  roadmap, CHANGELOG, traces, and evidence remain primary-agent owned.
- Targeted validation: the pinned Java oracle runner; `go test
  ./internal/esper -run 'CurrentEvaluationContext' -count=1`; `go test
  ./internal/app/parity -run 'ExprCoreCurrentEvaluationContext|CurrentEvaluationContext'
  -count=1`; differential mutation tests; and `go test
  ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`, `go test
  ./... -count=1`, manifest/evidence validation, `make check`, and `git
  diff --check`; the focused race gate remains applicable at the expr.core
  capability milestone.

## Progress

- [x] Reconfirm baseline, roadmap priorities, manifest summary, and relevant
      existing gates.
- [x] Select the next natural work unit from the `expr.core` manifest gap.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands using read-only investigation.
- [x] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [x] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [x] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts changed.
- [x] Run independent parity review, resolve findings, and run complete local
      gates plus the applicable milestone gate.
- [ ] Review the final diff, create one semantic commit, push `master`, and
      record the commit and actual validation.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The current-evaluation-context Java source has two execution
  variants that differ only by EPL/SODA compilation mode. Their shared
  observable projection is runtime URI, statement name, user object, `-1`
  partition ID, and the runtime URI accessor; existing typed Go implementation
  and unit tests already cover this path.
- 2026-08-20: The new two-case scenario produced matching Java and Go listener
  traces at virtual time 0. The trace normalizes the Java/Go context objects to
  four named metadata fields; differential comparison passed with zero
  differences, and value, order, field-name, and time mutations are rejected.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and matches
  `origin/master` at `57c2079bd`.
- Previous work-unit result: `case.expr-core-bitwise` is persisted as
  `differential-verified` with two runtime IDs and zero-difference evidence;
  details are retained in `CHANGELOG.md` and its evidence artifact.
- Previous work-unit result: `tools/java-oracle/run-expr-core-current-timestamp.sh`
  generated four Java listener records; `go run ./cmd/parity -mode
  expr-core-current-timestamp-diff ...` produced passing evidence with zero
  differences. Focused parity success and value/order/field/time mutation tests
  pass; the case is now `differential-verified` with three runtime IDs. Targeted
  commands passed: `go test ./internal/esper -run
  'CurrentTimestamp|ExpressionArithmeticConditionalAndTimeFunctions' -count=1`,
  `go test ./internal/app/parity -run 'ExprCoreCurrentTimestamp|CurrentTimestamp'
  -count=1`, and `go test ./internal/compat ./internal/app/manifest -count=1`.
- Current work-unit implementation result: the typed Go replay and Java oracle
  for `expr-core-current-evaluation-context` produced two listener records with
  matching runtime URI, statement name, user object, partition ID `-1`, and
  accessor value. The persisted differential evidence is passing with zero
  differences. Focused Esper and parity tests, including four trace mutations,
  pass.
- Manifest/evidence validation initially caught a stale unreferenced-runtime
  summary; the distinct-runtime denominator is `4,136 - 2,898 = 1,238`, and
  the corrected manifest and documentation validate cleanly.
- Independent parity review on 2026-08-20: compared the fixed Java execution
  source, Go replay, oracle case order, scenario steps, normalized trace,
  evidence metadata, and promoted manifest entry; no semantic findings. The
  persisted Java and Go traces compare equal, and value/order/field/time
  mutations are rejected.
- Complete local validation on 2026-08-20: the pinned Java runner, `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, `git diff --check`, and
  changed-file `gofmt` checks passed. The focused race gate
  `go test -race ./internal/app/parity ./internal/esper -count=1 -timeout 600s`
  passed (`internal/app/parity` 18.116s; `internal/esper` 271.908s).

## Delivery

- Current work-unit semantic commit and push are pending final diff review.
- Previous work-unit semantic commit `344f9ddf6` and checkpoint closure
  `65913ab0d` remain in history; no force-push or semantic-history rewrite is
  required for this unit.

## Handoff

Start or resume a Codex task from the repository root with the starter prompt
in `docs/esper-go-port-codex-workflows.md`. The first migration action is plan
selection and contract discovery, not an implementation guess.
