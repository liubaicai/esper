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
  `d51138c25`; the next semantic change must preserve that baseline.
- Status: the equality/identity unit is committed and pushed. The next unit is
  a frozen differential slice of the already implemented CASE capability; no
  production semantic change is indicated by the scouts.
- Current work unit: `expr.core` / `case.expr-core-case`
- Exact next action: stage and audit the complete CASE diff, create one semantic
  commit, push `master`, and verify the remote ref read-only without tracked
  edits.
- Worktree notes: CASE parity source/assets, central facts, and this checkpoint
  are modified as expected; do not modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the required scouts were sent concurrently
  through collaboration tools before implementation.
- Java contract scout: Boole `01a01f0b-d142-7002-806d-58e591acfd23`; completed.
  Confirmed source-order executions 3/6/7, exact vectors and result types,
  fresh listener lifecycle, nine new records, no timers/old stream, and the
  Java/Go vector mismatch in the existing supplemental Branches3 test.
- Go surface scout: Copernicus `01a01f0b-cf32-7b73-960e-f1620dc3e4de`;
  completed. Confirmed `expr_case.go` already supplies searched/simple CASE
  semantics and identified the parity runner, oracle, scenario, trace,
  evidence, dispatcher, and focused-test scope.
- Independent parity reviewer: Confucius `01a01f20-0c37-76b3-9dfc-2c63fe9a70bf`;
  initial review found two issues: oracle runner provenance did not reject a
  dirty/untracked fixed checkout, and Go accepted known extra CASE step
  metadata. Both were fixed in the runner and symmetric Java/Go validators;
  follow-up returned no findings. Residual risk: unknown JSON keys not modeled
  by `compat.Step` remain outside the known-metadata validator.
- N+1 Java contract scout: Carson `01a01f20-107f-7bb0-96d9-b8894e0fa940`,
  prefetching `case.expr-core-instanceof` read-only.
- N+1 Go surface scout: Helmholtz `01a01f20-0e5a-76a1-8854-aea5f6a1faf2`,
  prefetching `case.expr-core-instanceof` read-only.
- Previous equality unit: committed and pushed as `d51138c25`; Git remains the
  source of commit identity.
- Serial exception: none; both CASE scouts were sent through the collaboration
  facility with disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, searched and simple CASE evaluation,
  branch order, numeric matching, result coercion, and listener lifecycle.
- Java source and executions/runtime IDs:
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCase.java`;
  `ExprCoreCaseSyntax1WithElse` / `java-runtime-e9ae8d32b9f6155877ef`;
  `ExprCoreCaseSyntax1Branches3` / `java-runtime-725a9999f48d69d792a2`;
  `ExprCoreCaseSyntax2` / `java-runtime-0f635579498bb6de3a4c`.
- Differential scope: three ordered isolated cases named `case-with-else`,
  `case-branches3`, and `case-simple-numeric`, matching source ordinals 3/6/7.
  `case-with-else` sends `CSCO,4000` then `DELL,20`; `case-branches3` sends
  `DELL,10000`, `MSFT,10000`, then `GE,10000`; `case-simple-numeric` sends
  `(2,2,1,1)`, `(5,1,1,5)`, `(12,1,12,4)`, then `(1,2,3,4)`. Each fresh
  deployment emits one new row per send in input order, for nine records total;
  no timers, old-stream rows, time advances, or runtime errors are in scope.
- Observable contract: `p1` is Long with values `4000` and `60`; `c0` is
  Double with values `5000`, `3333.3333333333335`, `10000`, `4`, `25`, `3`,
  and `10` in the exact case/send order; searched CASE selects the first true
  branch and simple CASE uses cross-numeric matching. The existing Go `JOE`
  unmatched assertion remains supplemental and is excluded from the strict
  differential scenario.
- Allowed production files: none unless targeted replay proves a CASE semantic
  defect; any such change is limited to `internal/esper/expr_case.go` or the
  smallest directly responsible shared helper.
- Allowed Go test/parity files: new
  `internal/app/parity/expr_core_case.go`, focused additions under
  `internal/esper` only if needed, and minimal dispatcher/test additions in
  `run.go` and `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreCaseScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-case.sh`, and
  `testdata/parity/expr-core-case.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused CASE Esper tests;
  focused parity replay and mutation tests; and
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
- 2026-08-20: `case.expr-core-relop` was delivered as `a448dbd4f` with two
  zero-difference runtime IDs and 30 replay records. Its scenario-shape review
  tightened exact case order and send counts.
- 2026-08-20: `case.expr-core-like-regexp` reached zero-difference replay and
  was delivered as `896ed1312`; the full-match fix and Java-LIKE test-modeling
  correction passed the complete local gates.
- 2026-08-20: Fresh concurrent IN/BETWEEN scouts Russell/Kierkegaard froze a
  safe five-execution scalar slice and identified the endpoint-policy Plan
  identity defect and nullable replay schema requirement.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records and exact `s0`/`s1`/`s2` range lifecycle; checked-in trace,
  evidence, mutation tests, manifest, roadmap, and CHANGELOG updates are in
  place. The endpoint-policy Plan identity regression is covered by a focused
  Go test.
- 2026-08-20: The first parity review's payload finding was resolved by
  symmetric frozen vectors in the Go runner and Java oracle. Validation now
  requires exact case/send order, event type, field set, and JSON value spelling
  for all 164 inputs, including the ten range pairs; a Go payload mutation test
  covers the rejection path.
- 2026-08-20: Shared differential provenance flags remain an intentional
  runner-level override used by existing modes and are out of scope for this
  unit; checked-in evidence still asserts the exact fixed commit, source,
  execution, and runtime metadata. Result fields remain maps because the
  protocol compares field identity/value structurally and JSON encoding gives
  deterministic lexicographic key order; field-map insertion order is likewise
  out of scope for this unit.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records, passed independent review and local gates, and was pushed as
  `033786cbd`; Git is authoritative for that commit identity.
- 2026-08-20: The equality scouts confirmed the reusable typed
  `EqualOf`/`NotEqualOf`/`Is`/`IsNot` surface, deep slice equality, and typed-nil
  behavior. They also identified a Plan identity collision for interface-typed
  primitive literals and the missing direct differential replay.
- 2026-08-20: `case.expr-core-equals-is` implementation added a strict
  four-case scenario validator, fresh per-case Go deployments, and a pinned
  Esper Java oracle. The replay covers eight listener records in exact source
  order; `ExprCoreEqualsInvalid` remains a compile/build boundary.
- 2026-08-20: `Literal[T]` now includes the dynamic concrete type in Plan
  descriptions when `T` is an interface and the value is primitive. The
  focused regression keeps equivalent plans stable while distinguishing
  `Literal[any](1)` from `Literal[any]("1")`.
- 2026-08-20: Targeted validation passed: the pinned Java runner regenerated
  an eight-record trace byte-for-byte equal to the checked-in trace; Go
  differential replay/evidence reported zero differences; focused equality,
  parity, mutation, compat, and manifest tests passed. `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, focused expression and
  parity race tests, JSON/schema checks, shell syntax, layout, and
  `git diff --check` also passed.
- 2026-08-20: Independent parity review by Meitner returned no findings. The
  reviewer confirmed the Java source order, exact payload vectors, fresh case
  lifecycle, trace/evidence metadata, and central manifest references; residual
  risk is limited to semantic-vs-lexical payload validation and broader
  object-array coercion coverage.
- 2026-08-20: CASE scouts froze the three source-order executions and found no
  production semantic gap. The strict replay uses Java's DELL/MSFT/GE vector;
  the existing JOE unmatched assertion remains supplemental.
- 2026-08-20: CASE Java and Go traces are byte-identical for nine listener
  records. Evidence reports zero differences; focused CASE/parity, malformed
  scenario, trace mutation, compat, and manifest checks pass.
- 2026-08-20: Independent CASE review identified and the primary agent fixed
  clean-oracle provenance enforcement plus symmetric exact case/send metadata
  validation in the Java oracle and Go replay; a metadata mutation regression
  now covers the previously asymmetric path.
- 2026-08-20: CASE review follow-up by Confucius returned no findings. N+1
  read-only scouts Carson/Helmholtz identified the five-execution
  `case.expr-core-instanceof` candidate and its primitive-wrapper/interface
  risks; no N+1 writes started.

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is
  fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and
  matches `origin/master` at `d51138c25`. Manifest summary reports 522 cases,
  130 differential-verified cases, 402 differential runtimes, 2,901
  referenced runtimes, and 1,235 unreferenced runtime IDs. CASE was
  implemented-only with the three target runtime associations already
  inventoried.
- Current unit result: the pinned Java oracle regenerated the checked-in trace
  byte-for-byte. Go differential replay and evidence report nine records and
  zero differences. Focused CASE/parity, checked-in evidence,
  malformed-scenario, mutation, compat, and manifest tests pass. The manifest
  now reports 131 differential-verified cases and 405 differential runtimes;
  `go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`, focused
  expression/parity race tests, JSON/shell/layout, and `git diff --check` pass.
  Independent review and follow-up pass with no findings.

## Delivery

- The previous equality unit is committed and pushed to `origin/master` at
  `d51138c25`; Git history is the authoritative source for its identity. The
  current CASE unit has passed targeted validation, independent review, and all
  required local gates; the next action is one semantic commit and push.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After the CASE commit is pushed,
select the next closed-loop work unit and repeat concurrent scout fan-out.
