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
- Baseline: `HEAD` and `origin/master` remain at the clean baseline commit
  `3390ed16b`; the worktree contains only the current TypeName unit changes.
- Status: the TypeName fragment implementation, checked-in trace/evidence, and
  pinned Java/Go replay pass with zero differences. Planck's independent review
  agent was nonresponsive and closed; a bounded local review found no concrete
  issues. The final diff review is complete; semantic delivery remains pending.
- Current work unit: `expr.core` / `case.expr-core-type-name`
- Exact next action: run the last read-only consistency check, then create and
  push the semantic TypeName commit.
- Worktree notes: TypeName implementation, parity runner/tests, scenario, and
  Java oracle/runner are modified or new; do not modify `/root/app/esper` or
  `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the required scouts were sent concurrently
  through collaboration tools before implementation.
- Java contract scout: Epicurus `01a01f5f-3760-7f93-b045-bdcdc38c2397`;
  completed. It froze all five source-order executions and identified the
  fragment, POJO, dynamic wrapper-name, variant/match-recognize, and invalid
  compile boundaries.
- Go surface scout: Plato `01a01f5f-39a9-72d0-9c70-fff51776455c`; completed.
  It confirmed the typed `TypeName` surface, the existing Go-name contract for
  ordinary values, and the missing declared fragment metadata path.
- Independent parity reviewer: Planck `01a01f85-d751-7580-b79c-1598d2001cfb`
  was started after targeted validation, but remained nonresponsive after
  several waits and a completion prompt; it was closed without a report.
  A bounded local review found no concrete findings.
- Asset-writer handoff: Java scout Epicurus was asked to write only the
  disjoint TypeName oracle/scenario assets after contract freeze, but did not
  return a patch after three 30-second waits and two stop requests. It was
  closed; the primary agent is taking over that exact allowed asset scope.
- Serial exception: asset writing is serial in this handoff because the
  delegated writer remained nonresponsive; no semantic scope or file ownership
  was broadened.

## Work-unit contract

- Capability/subdomain: `expr.core`, typed runtime type-name predicates,
  event envelope identity, dynamic values, fragments, variants, Null/Missing,
  and invalid type-name diagnostics.
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreTypeOf.java`;
  `ExprCoreTypeOfFragment` / `java-runtime-eeeacbe2c7669e1e1207`;
  `ExprCoreTypeOfNamedUnnamedPOJO` / `java-runtime-e2b1dfb02a3ef7fb8ba3`;
  `ExprCoreTypeOfInvalid` / `java-runtime-7811d3bb8fcadbda3937`;
  `ExprCoreTypeOfDynamicProps` / `java-runtime-a4ad7a90bf65d6ae817f`;
  `ExprCoreTypeOfVariantStream` / `java-runtime-c3573f2b2f4f979ab963`.
- Differential scope: strict replay of the complete
  `ExprCoreTypeOfFragment` execution across `OBJECTARRAY`, `MAP`, `AVRO`,
  `JSON`, `JSONCLASSPROVIDED`, and `DEFAULT` representation branches. The
  POJO execution remains implemented-only because the current typed Go event
  envelope does not preserve the concrete bean subtype name. The invalid compile
  execution remains implemented-only because the typed API has no text parser;
  dynamic properties remain implemented-only because Java emits `Integer` and
  `String` while the public Go contract intentionally emits `int` and
  `string`; variant remains implemented-only because its source execution also
  requires the unsupported match-recognize tail.
- Observable contract: fragment projects `t0`/`t1` as nullable strings, with
  `InnerSchema` and `InnerSchema[]` for populated non-Avro fragments and
  representation metadata (including empty/null values) for Avro. Rows are ordered by
  representation then payload, use fresh deployment/lifecycle per execution,
  and have no timers, old stream, or runtime errors. Null/Missing remains
  null for non-Avro fragments. Java source/runtime IDs are exact and pinned to
  commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`.
- Allowed production files: `internal/esper/expr.go` and the smallest directly
  related focused TypeName test file only. No global Go reflection-name change
  is authorized.
- Allowed Go test/parity files: a new TypeName parity runner file and minimal
  dispatcher/test additions in `internal/app/parity`; focused `internal/esper`
  tests only if the frozen replay proves a regression.
- Allowed parity asset files:
  TypeName-specific Java oracle/runner files and
  `testdata/parity/expr-core-type-name.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused TypeName tests;
  focused parity replay and mutation tests; and
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
      where verified facts changed.
- [x] Run the independent parity-review attempt, record the serial exception,
      resolve any findings, and run complete local
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

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is
  fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and
  matches `origin/master` at `3390ed16b`. Manifest summary reports 522 cases,
  132 differential-verified cases, 410 differential runtimes, 2,901
  referenced runtimes, and 1,235 unreferenced runtime IDs. Before this unit,
  `case.expr-core-type-name` was implemented-only with five target runtime
  associations inventoried.
- Current unit result: changed-file formatting, checked-in trace/evidence,
  pinned Java replay, Go differential replay, focused TypeName/parity/mutation
  tests, `go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`,
  focused expression/parity race tests, JSON/schema checks, shell syntax, and
  `git diff --check` pass. The independent-review attempt has no report; the
  bounded local review found no concrete issues. The final diff inspection found
  no issues; remote verification remains pending.

## Delivery

- InstanceOf is committed and pushed to `origin/master` at `3390ed16b`; Git
  history is authoritative for that identity. The TypeName unit has passed its
  replay and required local gates and is ready for the final diff inspection
  and semantic commit.
- After the semantic commit, verify the pushed ref without editing tracked
  files or creating a checkpoint-only follow-up commit.

## Handoff

After delivery, verify the pushed ref read-only and select the next work unit
from the roadmap and manifest; do not edit this file only to add the new
commit hash.
