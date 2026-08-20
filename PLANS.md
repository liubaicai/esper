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
- Baseline: `master` at `95b4f7b6b` (verified clean before this work unit).
- Status: Current-timestamp case is promoted with passing four-record evidence;
  review, full gates, and the focused race gate are green; commit/push remains.
- Current work unit: `expr.core` / `case.expr-core-current-timestamp`
- Exact next action: review the final staged diff, create the semantic commit,
  push `master`, and record the resulting commit.
- Worktree notes: implementation and parity assets for this unit are modified;
  unrelated files remain out of scope; `master` still matches `origin/master`.

## Work-unit contract

- Capability/subdomain: `expr.core`, current timestamp and virtual clock
- Java source and executions/runtime IDs:
  `ExprCoreCurrentTimestampGet` /
  `java-runtime-c1c1fd3dc31af4864a50`;
  `ExprCoreCurrentTimestampOM` /
  `java-runtime-96c8b8cb4cf36a523669`;
  `ExprCoreCurrentTimestampCompile` /
  `java-runtime-5b126fe7fb865be8b293`.
- Observable contract: `current-timestamp-get` deploys `s0` with the
  unaliased `current_timestamp()` field plus `t0`, `t1`, and `t2`; sends at
  virtual times 100 ms and 999 ms produce rows `{100,100,100,101}` and
  `{999,999,999,1000}` in that field order, with no old stream. The OM and
  compile cases each deploy the `t0` projection and send one event at 777 ms,
  producing `{777}`. Java exposes boxed Long metadata; Go exposes typed int64
  values and normalizes both as exact epoch-millisecond numbers.
- Allowed production files: `internal/app/parity/expr_core_current_timestamp.go`,
  `internal/app/parity/run.go`, and the focused additions in
  `internal/app/parity/run_test.go`; modify `internal/esper` only if the
  frozen parity contract exposes a real regression.
- Allowed parity asset files: `tools/java-oracle/ExprCoreCurrentTimestampScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-current-timestamp.sh`, and
  `testdata/parity/expr-core-current-timestamp.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: unrelated semantic surfaces; `goal.txt`;
  generated evidence before trace validation; and central facts outside this
  unit's manifest/roadmap/CHANGELOG updates. `PLANS.md`, the manifest,
  roadmap, CHANGELOG, traces, and evidence remain primary-agent owned.
- Targeted validation: the pinned Java oracle runner; `go test
  ./internal/esper -run 'CurrentTimestamp|ExpressionArithmeticConditionalAndTimeFunctions'
  -count=1`; `go test ./internal/app/parity -run
  'ExprCoreCurrentTimestamp|CurrentTimestamp' -count=1`; differential mutation
  tests; and `go test ./internal/compat ./internal/app/manifest -count=1`.
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
- [x] Review the final diff, create one semantic commit, push `master`, and
      record the commit and actual validation.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and matches
  `origin/master` at `95b4f7b6b`.
- Previous work-unit result: `case.expr-core-bitwise` is persisted as
  `differential-verified` with two runtime IDs and zero-difference evidence;
  details are retained in `CHANGELOG.md` and its evidence artifact.
- Current work-unit implementation result: `tools/java-oracle/run-expr-core-current-timestamp.sh`
  generated four Java listener records; `go run ./cmd/parity -mode
  expr-core-current-timestamp-diff ...` produced passing evidence with zero
  differences. Focused parity success and value/order/field/time mutation tests
  pass; the case is now `differential-verified` with three runtime IDs. Targeted
  commands passed: `go test ./internal/esper -run
  'CurrentTimestamp|ExpressionArithmeticConditionalAndTimeFunctions' -count=1`,
  `go test ./internal/app/parity -run 'ExprCoreCurrentTimestamp|CurrentTimestamp'
  -count=1`, and `go test ./internal/compat ./internal/app/manifest -count=1`.
- Independent parity review on 2026-08-20: compared the fixed Java execution
  source, Go replay, oracle case order, scenario steps, normalized trace,
  evidence metadata, and promoted manifest entry; no semantic findings. The
  persisted Java and Go traces compare equal, and value/order/field/time
  mutations are rejected.
- Complete local validation on 2026-08-20: the pinned Java runner, `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, `git diff --check`, and
  changed-file `gofmt` checks passed. The focused race gate
  `go test -race ./internal/app/parity ./internal/esper -count=1 -timeout 600s`
  passed (`internal/app/parity` 18.020s; `internal/esper` 269.233s).

## Delivery

- Semantic commit `344f9ddf6`: `expr: verify current timestamp parity`, pushed
  to the protected `origin/master` branch.
- The checkpoint-only closure is being recorded in a normal follow-up commit;
  no force-push or semantic-history rewrite is required.

## Handoff

Start or resume a Codex task from the repository root with the starter prompt
in `docs/esper-go-port-codex-workflows.md`. The first migration action is plan
selection and contract discovery, not an implementation guess.
