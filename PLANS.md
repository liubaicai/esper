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
  `c325027dc`; the next semantic change must preserve that baseline.
- Status: the four typed `ExprCoreCast.java` executions are implemented and
  replayed with zero-difference evidence alongside the four delivered Exists
  executions. The independent reviewer was nonresponsive; bounded local review
  and all required local gates are complete, with commit/push pending.
- Current work unit: `expr.core` / four typed `ExprCoreCast.java` executions
  within `case.expr-core-exists-cast`
- Exact next action: create the semantic commit, push `master`, and verify the
  remote ref read-only.
- Worktree notes: intended work-unit edits are present; preserve them and do not
  modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the required Cast scouts were started
  concurrently through collaboration tools before implementation, but both
  remained reported as running after three 30-second waits and bounded
  completion requests. They were closed without reports.
- Java contract scout: Mendel `01a01fc7-90c0-7ec3-9a93-46103aba7713` was
  assigned the fixed `ExprCoreCast.java` source and exact runtime inventory;
  no deliverable returned before closure.
- Go surface scout: Huygens `01a01fc7-8dfe-7561-9e69-d148f84a4a92` was
  assigned the typed Cast implementation and existing parity surfaces; no
  deliverable returned before closure.
- Independent parity reviewer: Gauss `01a01fe0-2023-79e1-ade9-1dec6ba1f6b7`
  was started after targeted validation, remained reported as running after
  repeated 30/60-second waits and a completion request, and was closed without
  a report. The primary agent completed a bounded local review of source order,
  exact payload metadata, runtime IDs, trace/evidence provenance, lifecycle,
  Null behavior, and mutation coverage with no concrete findings. This is a
  serial-review exception, not an independent approval.
- Serial exception: both required read-only scouts were launched concurrently
  through the collaboration facility, but neither returned findings after
  repeated waits and interrupt-style completion requests. The primary agent
  inspected the fixed source and Go surface locally, froze the contract below,
  and did not broaden scope or authorize scout writes.

## Work-unit contract

- Capability/subdomain: `expr.core`, typed `Cast[A,B]` conversion across
  numeric/primitive/boxed/dynamic values, Null behavior, and a small source-order
  slice from `ExprCoreCast.java`.
- Java source and executions/runtime IDs: fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`, with
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java`:
  `ExprCoreCastSimple` / `java-runtime-3fc2cde530f321dc3994`,
  `ExprCoreCastSimpleMoreTypes` / `java-runtime-5c9fdd17b5480f9d78e2`,
  `ExprCoreCastAsParse` / `java-runtime-52fb8d57dd380f5efe6e`, and
  `ExprCoreCastDoubleAndNullOM` / `java-runtime-45b9d5a2e216f8063b75`.
- Differential scope: strict replay of the existing four source-order
  `ExprCoreExists.java` executions followed by these four Cast executions,
  with fresh statement/runtime lifecycle per case. The Cast payloads are exact:
  `cast-simple` sends `theString=abc,intPrimitive=100,intBoxed=3,
  floatBoxed=9.5`, then `theString=null,intPrimitive=100,intBoxed=null,
  floatBoxed=null`; `cast-simple-more-types` sends
  `theString=true,intPrimitive=1,doublePrimitive=1`; `cast-as-parse` sends
  `theString=12,intPrimitive=1`; and `cast-double-null-om` sends tagged
  dynamic items `int=100`, `byte=2`, `double=77.7777`, `int64=6`, `null`,
  and `string=abc`.
- Observable contract: all cases emit one new row per send at virtual time
  zero, no old stream, timers, or runtime errors. Cast Simple projects
  `c0..c7` with string, Integer/Float/Long/Number conversions and preserves
  Null for nullable inputs. MoreTypes projects float/short/byte/char/bool,
  BigInteger and BigDecimal-equivalent exact Go values; `AsParse` projects
  boxed Integer `12`; Double/Null OM projects boxed Double values for numeric
  dynamic items and Null for Null/incompatible string. The Go trace uses the
  Java oracle's scalar numeric/character normalization while focused unit
  tests retain the typed result schema. Java EPL/SODA/compile entry details,
  date/time diagnostics, interface casts, arrays, generic casts, Boolean casts,
  and remaining Cast executions stay out of scope.
- Allowed production files: only the smallest directly responsible Cast helper
  or typed API surface in `internal/esper`, and its generated root facade if the
  public API changes.
- Allowed Go test/parity files: a new Cast parity runner plus minimal dispatcher
  and focused test additions in `internal/app/parity` and `internal/esper`.
- Allowed parity asset files: Cast-specific Java oracle/runner files and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java Cast oracle runner; focused Cast tests;
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
      the documented nonresponsive fallback.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands from fixed-source local inspection.
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
  inventoried runtime associations while the four Exists-source IDs and four
  Cast-source IDs are now differential-verified.
- 2026-08-20: Cast implementation and replay assets passed the pinned Java
  oracle, focused Cast/parity/mutation/manifest checks, `gofmt` verification,
  `go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`, the
  focused expression/parity race gate, JSON/schema and shell checks, and
  `git diff --check`. The regenerated Java trace is byte-identical to the
  checked-in trace (SHA-256
  `8199f6dfab4a35acbd2949830268eb104cd10aa4de9915509ccaec6edbcd10ab`);
  evidence reports 22 records and zero differences.

## Validation evidence

- Investigation baseline before the Cast slice on 2026-08-20: `/root/app/esper`
  is fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` was clean and
  matched `origin/master` at `c325027dc`. Manifest summary reported 522 cases,
  134 differential-verified cases, and 415 differential runtimes before the
  four Cast runtime IDs were added.
- Current unit result: `case.expr-core-exists-cast` is differential-verified
  across eight exact runtime IDs and 22 listener records. The typed Cast char
  fix, replay assets, evidence, manifest, roadmap, CHANGELOG, README, and
  checkpoint are complete; the Java/Go trace is byte-identical with zero
  differences. Full local gates pass. The review exception is documented above;
  remote delivery remains pending.

## Delivery

- The current Exists/Cast unit has passed replay, review fallback, final diff
  review, and required local gates and is ready for one semantic commit and
  push. Git will remain authoritative for the resulting commit identity.
- After the semantic commit, verify the pushed ref without editing tracked
  files or creating a checkpoint-only follow-up commit.

## Handoff

After delivery, verify the pushed ref read-only and select the next work unit
from the roadmap and manifest; do not edit this file only to add the new
commit hash.
