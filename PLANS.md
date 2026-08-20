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
- Baseline: `HEAD` and `origin/master` are clean and match at pushed commit
  `6a29940a1`; the next semantic change must preserve that baseline.
- Status: TypeName fragment parity is delivered. Exists/Cast implementation and
  the pinned Java replay now pass targeted validation with byte-identical traces;
  central metadata is updated and awaits independent review and full gates.
- Current work unit: `expr.core` / `case.expr-core-exists-cast`
- Exact next action: validate the updated manifest/evidence, run the broad local
  gates, obtain the independent parity review, then record the final result
  before the semantic commit.
- Worktree notes: current work-unit edits are present in `internal/app/parity`,
  `PLANS.md`, and the scenario; preserve them and do not modify `/root/app/esper` or
  `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the required scouts were sent concurrently
  through collaboration tools before implementation.
- Java contract scout: Turing `01a01f9c-a9a0-7662-8970-70a021400a6a` was
  started concurrently but returned no deliverable after two 30-second waits,
  an interrupt request, and closure while still reported running. The primary
  agent inspected the fixed `ExprCoreExists.java` source locally and froze the
  contract below.
- Go surface scout: Plato `01a01f5f-39a9-72d0-9c70-fff51776455c` was asked
  concurrently to inspect the typed Exists/Cast surface, but returned no
  deliverable after the same wait/interrupt/closure sequence. The primary
  agent inspected `expr_exists_test.go`, `expr_cast.go`, and existing parity
  helpers locally.
- Serial exception: both required read-only scouts were launched through the
  collaboration facility, but neither returned findings; local source
  inspection is the recorded fallback. No semantic scope or file ownership was
  broadened, and no scout wrote files or ran tests.
- Independent parity reviewer: Kuhn `01a01fbb-81cd-71b0-8bcf-500794697bd0`
  was started after targeted validation, but remained reported as running after
  repeated 10/30-second waits, a completion prompt, and an interrupt request;
  it was closed without a report. The primary agent completed a bounded local
  review of source order, exact metadata, scenario shape, evidence freshness,
  trace counts, Null/Missing boundaries, lifecycle, and mutation coverage with
  no concrete findings. This is a serial-review exception, not an independent
  approval.
- N+1 prefetch: Boyle `01a01fbb-8468-7871-8f78-f469cff59684` confirmed that
  `ExprCoreExists.java` is exhausted at the four verified executions; remaining
  Cast inventory entries belong to `ExprCoreCast.java` and require a new
  explicitly scoped unit. Singer `01a01fbb-7ef0-7113-a12b-7b6e1fdfb0e2` was
  closed after no Go-surface report; no N+1 writes started.

## Work-unit contract

- Capability/subdomain: `expr.core`, typed runtime `Exists` predicates,
  dynamic nested property presence, explicit Null versus Missing, and the
  source-order SODA/compile lifecycle equivalents.
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreExists.java`;
  `ExprCoreExistsSimple` / `java-runtime-d77c035088ce635538c7`;
  `ExprCoreExistsInner` / `java-runtime-4202039fb2f1ea65fd49`;
  `ExprCoreCastDoubleAndNullOM` / `java-runtime-8fa7f5076dde9d791d08`;
  `ExprCoreCastStringAndNullCompile` / `java-runtime-afd826c7d955eb538001`.
- Differential scope: strict replay of these four source-order executions,
  using 1, 5, 3, and 3 sends respectively. `ExprCoreExistsSimple` projects
  `c0..c4` as `true,true,false,true,true` from a SupportBean with
  `theString=abc`, `intPrimitive=100`, `intBoxed=3`, and `floatBoxed=9.5`.
  `ExprCoreExistsInner` projects `t0..t10` for null, the default complex
  property bean twice, nested SupportBean, and SupportBean_A(id=10), with the
  exact vectors frozen from the Java assertions. The OM and compile executions
  each project `t0` for nested SupportBean, null, and string `abc`, producing
  `true,false,false`; their language-entry difference is normalized to the
  same typed Go plan and fresh lifecycle.
- Observable contract: every result field is Boolean; rows are ordered by
  source case and send order, have new-stream only, no timers, no time advance,
  and no runtime errors. `Exists` is true for a present property even when its
  value is null, false for Missing, null intermediate objects, and incompatible
  dynamic values. Java bean inputs are represented by deterministic Go structs
  and a typed dynamic root while preserving the observed presence/value
  contract; no Java class identity is claimed. Source/runtime IDs are exact and
  pinned to commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`.
- Allowed production files: none unless the frozen replay proves a semantic
  regression; any such change is limited to the smallest directly responsible
  helper in `internal/esper`.
- Allowed Go test/parity files: a new Exists/Cast parity runner file and minimal
  dispatcher/test additions in `internal/app/parity`; focused `internal/esper`
  tests only if the replay proves a regression.
- Allowed parity asset files:
  Exists-specific Java oracle/runner files and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused Exists/Cast
  tests; focused parity replay and mutation tests; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.

## Progress

- [x] Reconfirm baseline, roadmap priorities, manifest summary, and relevant
      existing gates.
- [x] Select the next natural work unit from the `expr.core` manifest gap.
- [x] Run concurrent Java/Go read-only scouts and record their agent IDs and
      conclusions.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands after both read-only scout results return.
- [x] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [x] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [x] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts change.
- [x] Run independent parity review, resolve findings, and run complete local
      gates plus the applicable milestone gate; the reviewer was nonresponsive,
      so the documented bounded local-review exception applies.
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
- 2026-08-20: TypeName implementation now consumes declared nested fragment
  metadata without changing ordinary Go reflection names; non-Avro nil-like
  fragments remain null while Avro retains declared metadata.
- 2026-08-20: The pinned TypeName oracle regenerated successfully after passing
  the shared Avro configuration and Jackson classpath requirements. Temporary
  Java/Go replay produced 18 records and zero differences across all six
  representation branches.
- 2026-08-20: Independent parity review by Planck was attempted after targeted
  validation but the agent remained nonresponsive and was closed without a
  report. The primary agent performed a bounded local review of the complete
  TypeName diff and found no concrete issues; the serial-review exception is
  retained here rather than represented as an independent approval.
- 2026-08-20: Exists/Cast scouts Turing and Plato were started concurrently for
  the next unit but produced no deliverables after waits and interruption; they
  were closed. Local inspection froze the four `ExprCoreExists` executions,
  their exact runtime IDs, boolean vectors, and Null/Missing contract. The
  unit has no authorized production change unless replay proves a regression.
- 2026-08-20: The initial Go replay produced 12 records but differed from Java
  for null dynamic roots, missing indexed/mapped paths on incompatible values,
  and the OM/compile null vectors. Ordinary `Exists` and explicit nullable
  fields must retain Null-as-present semantics, so the authorized production
  fix is typed `OptionalProperty` plus `?` path-boundary propagation rather
  than a global Exists change.
- 2026-08-20: The pinned Java oracle regenerated a 12-record trace byte-identical
  to the checked-in trace (SHA-256
  `20c4ca6c1ff48bf2f12ec4a267003726535ace672d38291a522ec444b04e678a`). Go
  differential replay/evidence reports 12 records and zero differences.
- 2026-08-20: Focused Exists/Cast, malformed-scenario, checked-in-evidence,
  trace-mutation, compat/manifest, JSON/schema, shell syntax, changed-file
  formatting, `go vet ./...`, `go test ./... -count=1 -timeout 240s`, focused
  expression/parity race tests, `make check`, and `git diff --check` passed.
- 2026-08-20: Independent review by Kuhn was attempted after targeted
  validation but remained nonresponsive and was closed without a report. The
  primary agent's bounded local review found no concrete findings; the exact
  serial-review exception is retained here rather than represented as an
  independent approval. N+1 Java scout Boyle confirmed the remaining Cast
  executions belong to `ExprCoreCast.java`, so no future writes began.
- 2026-08-20: Final manifest audit retained both `ExprCoreExists.java` and
  `ExprCoreCast.java` as source references because the case preserves all 17
  inventoried runtime associations while only the four Exists-source IDs are
  differential-verified in this unit.

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is
  fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and
  matches `origin/master` at `3390ed16b`. Manifest summary reports 522 cases,
  132 differential-verified cases, 410 differential runtimes, 2,901
  referenced runtimes, and 1,235 unreferenced runtime IDs. Before this unit,
  `case.expr-core-type-name` was implemented-only with five target runtime
  associations inventoried.
- Current unit result: `case.expr-core-exists-cast` is differential-verified
  for four exact runtime IDs. The typed OptionalProperty fix, replay assets,
  evidence, manifest, roadmap, CHANGELOG, README, and checkpoint are complete;
  the Java/Go trace is byte-identical with zero differences. Full local gates
  pass. The review exception is documented above; remote delivery remains
  pending.

## Delivery

- TypeName is committed and pushed to `origin/master` at `6a29940a1`; Git
  history is authoritative for that identity. The current Exists/Cast unit has
  passed replay, review fallback, and required local gates and is ready for one
  semantic commit and push.
- After the semantic commit, verify the pushed ref without editing tracked
  files or creating a checkpoint-only follow-up commit.

## Handoff

After delivery, verify the pushed ref read-only and select the next work unit
from the roadmap and manifest; do not edit this file only to add the new
commit hash.
