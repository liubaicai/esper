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
  `f76c618fa`; the next semantic change must preserve that baseline.
- Status: the InstanceOf unit is implemented, differentially verified, reviewed,
  and ready for its semantic commit. The production fix is limited to the
  confirmed interface-target reflection defect.
- Current work unit: `expr.core` / `case.expr-core-instanceof`
- Exact next action: perform the final diff audit, create one semantic commit,
  push `master`, and verify the remote ref read-only.
- Worktree notes: the InstanceOf implementation, replay assets, focused tests,
  and this checkpoint are modified as expected; do not modify `/root/app/esper`
  or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the required scouts were sent concurrently
  through collaboration tools before implementation.
- Java contract scout: Sartre `01a01f33-5212-7c82-accd-5fae896c045b`; completed.
  Confirmed source-order execution/runtime mapping, exact vectors, Boolean
  result types, one fresh `s0` listener per execution, 17 new records, and no
  timer/old-stream/error behavior. Missing-property behavior is outside scope.
- Go surface scout: Euler `01a01f33-4fd3-7142-8038-2ac9bab7c3ab`; completed.
  Confirmed `InstanceOf[T]`, `Property`, dynamic map schemas, and existing
  interface/pointer tests are sufficient; no production semantic change is
  indicated. Explicit numeric type decoding and binary `Or` composition are
  required for the replay.
- Independent parity reviewer: Laplace `01a01f54-8770-7bb1-81bd-c96c658e11b7`;
  completed with no findings. Residual risks recorded: OM/SODA serialization
  is not exercised by the oracle, unknown JSON step keys are outside the
  generic `compat.Step` validator, and the broader hierarchy/missing matrix is
  unverified. Aristotle `01a01f4c-a2db-7a71-acb1-94c7ad9c11ab` was an earlier
  bounded review attempt that was shut down without a report and is not used
  as acceptance evidence.
- Serial exception: none; both scouts have disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, typed `instanceof` predicates,
  primitive/wrapper matching, dynamic optional properties, null handling, and
  interface/supertype hierarchy evaluation.
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreInstanceOf.java`;
  `ExprCoreInstanceofSimple` / `java-runtime-f57ec2f2f04aad28961d`;
  `ExprCoreInstanceofStringAndNullOM` / `java-runtime-fc4c87ba7677b72cab9a`;
  `ExprCoreInstanceofStringAndNullCompile` / `java-runtime-c5af420270464e5633f7`;
  `ExprCoreDynamicPropertyJavaTypes` / `java-runtime-0423d81802ef0e7eb910`;
  `ExprCoreDynamicSuperTypeAndInterface` / `java-runtime-f446c95462162907b0c6`.
- Differential scope: five ordered isolated cases named
  `instanceof-simple`, `instanceof-string-om`, `instanceof-string-compile`,
  `instanceof-dynamic-types`, and `instanceof-dynamic-hierarchy`, matching
  source order. The first case sends two `SupportBean` payloads and emits
  `c0..c7`; the OM and compile cases each send the same non-null and null
  string payloads and emit `t0,t1`; dynamic types sends string, Float, null,
  Integer, and Long items and emits `t0..t7`; dynamic hierarchy sends the six
  source objects and emits `t0..t7`. Each fresh deployment emits one new row
  per send in input order, for 17 records total; no timers, old-stream rows,
  time advances, or runtime errors are in scope.
- Observable contract: every result field is Boolean. Simple outputs are
  `[true,false,true,false,true,false,true,true]` and
  `[false,false,false,false,true,false,true,false]`; OM and compile outputs are
  `[true,true]` then `[false,false]`; dynamic Java-type outputs are
  `[true,false,false,false,false,false,false,false]`,
  `[false,false,true,true,false,false,true,true]`,
  `[false,false,false,false,false,false,false,false]`,
  `[false,true,false,false,true,false,true,false]`, and
  `[false,false,false,false,false,true,true,true]`; hierarchy outputs are the
  six exact boolean vectors asserted by the Java source. Null is false and
  multi-target checks use OR semantics.
- Allowed production files: none unless targeted replay proves an
  `InstanceOf` semantic defect; any such change is limited to
  `internal/esper/expr.go` or the smallest directly responsible helper.
- Allowed Go test/parity files: new `internal/app/parity/expr_core_instanceof.go`
  and minimal dispatcher/test additions in `run.go` and `run_test.go`; focused
  `internal/esper/expr_instanceof_test.go` changes only if a regression is
  required.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreInstanceOfScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-instanceof.sh`, and
  `testdata/parity/expr-core-instanceof.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused InstanceOf Esper
  tests; focused parity replay and mutation tests; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.

## Progress

- [x] Reconfirm baseline, roadmap priorities, manifest summary, and relevant
      existing gates.
- [x] Select the next natural work unit from the `expr.core` manifest gap.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands after both read-only scout results return.
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
- 2026-08-20: The `instanceof` source review confirmed five source-order
  executions, 17 listener records, Boolean-only results, fresh lifecycle per
  execution, and no timer/old-stream behavior. The Go `InstanceOf[T]` surface
  is already present; replay modeling must preserve dynamic numeric types and
  custom interface hierarchy values.
- 2026-08-20: Sartre/Euler scouts completed concurrently. The strict scenario
  uses `SupportBean` payloads with explicit nullable fields and tagged
  `SupportBeanDynRoot` payloads (`itemType` plus optional `itemValue` or
  hierarchy constructor fields). Go decodes tags to `string`, `float32`,
  `int`, `int64`, or local pointer/interface values; Java constructs the
  corresponding fixed support objects. No production write is authorized by
  the frozen contract.
- 2026-08-20: The pinned InstanceOf oracle regenerated the checked-in trace
  byte-for-byte; Go replay and evidence reported 17 records and zero
  differences. The malformed-scenario test now mutates the first dynamic send
  at `Steps[10]`, rather than the preceding case marker.
- 2026-08-20: Independent review by Laplace returned no findings. Residual
  risk remains limited to the unexercised OM/SODA serialization path, unknown
  JSON keys ignored by the generic step decoder, and broader optional/missing
  and hierarchy matrices outside this five-execution slice.

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is
  fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and
  matches `origin/master` at `f76c618fa`. Manifest summary reports 522 cases,
  131 differential-verified cases, 405 differential runtimes, 2,901
  referenced runtimes, and 1,235 unreferenced runtime IDs. `case.expr-core-instanceof`
  is implemented-only with five target runtime associations inventoried.
- Current unit result: focused tests pass; the pinned Java oracle regenerated a
  17-record trace byte-for-byte equal to the checked-in trace; Go differential
  replay/evidence reports zero differences; malformed-scenario and trace
  mutation tests pass. Manifest/evidence invariants pass. `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, layout, shell/JSON
  validation, `git diff --check`, and `go test -race ./internal/esper
  ./internal/app/parity -count=1 -timeout 600s` all pass.

## Delivery

- CASE is committed and pushed to `origin/master` at `f76c618fa`; Git history
  is the authoritative source for that identity. The current InstanceOf unit
  has passed replay, review, and required local gates; commit and push it once
  the final diff audit remains clean.
- After the semantic commit, verify the pushed ref without editing tracked
  files or creating a checkpoint-only follow-up commit.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After the CASE commit is pushed,
select the next closed-loop work unit and repeat concurrent scout fan-out.
