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
  `1a5824a53`; the next semantic change must preserve that baseline.
- Status: the next bounded expr.core unit extends the delivered Cast scenario
  with three scalar executions (raw string/boolean/static-parse casts) inside
  the same `case.expr-core-exists-cast`.
- Current work unit: `expr.core` / typed scalar Cast — `ExprCoreCastStringAndNullCompile`,
  `ExprCoreCastBoolean`, `ExprCoreCastWStaticType`
- Exact next action: append the three executions to the parity harness
  (scenario, oracle, Go runner, tests), regenerate traces/evidence, update the
  manifest case to differential-verified for the three new runtime IDs.
- Worktree notes: the delivered baseline is clean; preserve it and do not
  modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the unit's read-only scouts were launched
  concurrently through collaboration tools and delivered frozen contracts.
- Java contract scout: `CastScalarJavaContract` returned exact EPL,
  event-type/property types, per-execution input/output vectors,
  null/type/lifecycle semantics, and runtime-ID mapping (`9957cb6d9cd9ea836d4d`
  / `0b91403db9a899efde99` / `9babbbb6f96faf389bb7`).
- Go surface scout: `CastScalarGoSurface` confirmed no `internal/esper`
  production change is needed; all three executions are covered by existing
  `castToString`/`castToBool`/`parseStringNumber`, and reuse the exists-cast
  parity harness.
- Implementation writer: primary agent owns the whole exists-cast parity
  surface because the case-order/payload/event-type contract spans the
  scenario, oracle, Go runner, and tests as one atomic set; a second parallel
  writer would risk inconsistent case lists, so a singleton writer is the
  recorded exception.
- Independent parity reviewer: `ScalarCastReview` returned a conditional
  fail on one minor evidence-integrity defect — `javaExecutions[2]` mislabeled
  runtime `8fa7f5076dde9d791d08` as `ExprCoreCastStringAndNullCompile` when
  the execution inventory maps it to `ExprCoreCastDoubleAndNullOM`. The label
  was restored to match the inventory and the evidence was regenerated so the
  Go array and evidence `javaExecutions` agree; the review finding is resolved
  and all local gates pass.

## Work-unit contract

- Capability/subdomain: `expr.core`, typed scalar `Cast` extension of
  `case.expr-core-exists-cast` for three source-order `ExprCoreCast.java`
  executions alongside the eight already-verified executions.
- Java source and executions/runtime IDs: fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`, with
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java`:
  `ExprCoreCastStringAndNullCompile` / `java-runtime-9957cb6d9cd9ea836d4d`,
  `ExprCoreCastBoolean` / `java-runtime-0b91403db9a899efde99`, and
  `ExprCoreCastWStaticType` / `java-runtime-9babbbb6f96faf389bb7`. These three
  runtime IDs are already inventoried in the manifest case; only their
  differential-verified status changes.
- Differential scope: extend `testdata/parity/expr-core-exists-cast.json` with
  three new cases after `cast-double-null-om`:
  - `cast-string-and-null` sends six `SupportBeanDynRoot` tagged items
    (`itemType` int/byte/double/int64/null/string with itemValue
    100/2/77.7777/6/-/"abc") projecting `Cast(item?,String)` →
    `"100"`,`"2"`,`"77.7777"`,`"6"`,null,`"abc"`;
  - `cast-boolean` sends three `SupportBean` (theString/intPrimitive/
    boolPrimitive/boolBoxed) projecting `Cast(boolPrimitive,Boolean)`,
    `Cast(boolBoxed|boolPrimitive,boolean)` (null-propagates on null boolBoxed),
    `Cast(boolBoxed,String)`;
  - `cast-w-static-type` sends one `StaticTypeMapEvent` map
    (anInt="100", anDouble="1.4E-1", anLong="-10", anFloat="1.001",
    anByte="0x0A", anShort="223", intPrimitive=10, intBoxed=11) projecting ten
    typed casts int/double/long/float/byte/short/int/int/long/long.
- Observable contract: all cases emit one new row per send at virtual time
  zero, no old stream, timers, or errors; fresh statement/runtime lifecycle per
  case. Java number-to-String is `Double.toString`; Java numeric string parse
  is `Byte.decode` (hex allowed, `"0x0A"`→10), Short/Long/Integer decimal
  (`LongValue.parseString` strips trailing L and leading +), Float/Double
  `parseFloat/parseDouble`. Existing Go `parseStringNumber` covers these
  exact vectors. `StaticTypeMapEvent` is a map event with `anInt..anShort`
  as String and `intPrimitive` int / `intBoxed` Integer; the Go `RegisterMap`
  FieldDefs mirror it. Dates, interface casts, array casts, generic casts, and
  the remaining BigDecimal/BigInt executions stay out of scope.
- Allowed production files: none — no `internal/esper` change is authorized;
  this unit is parity/asset-only.
- Allowed Go test/parity files: `internal/app/parity/expr_core_exists_cast.go`
  (three new cases), `internal/app/parity/run_test.go` (record-count and
  case-count updates, extra payload-mutation).
- Allowed parity asset files: `tools/java-oracle/ExprCoreExistsCastScenarioOracle.java`
  (`StaticTypeMapEvent` config, bool fields, new cases) and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces of `internal/esper`; `goal.txt`; generated evidence before
  trace validation; central facts outside this unit's manifest/roadmap/CHANGELOG
  updates. `PLANS.md`, manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned.
- Targeted validation: pinned Java scalar-cast oracle runner; focused
  exists-cast parity + mutation tests; scenario shape validator; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.

## Progress

- [x] Reconfirm baseline (gofmt/vet/full tests green), roadmap, manifest.
- [x] Select the next bounded unit: three scalar `ExprCoreCast.java`
      executions from the remaining Cast inventory (string/bool/static-parse).
- [x] Run concurrent Java/Go read-only scouts; record agent IDs and frozen
      contract (no `internal/esper` production change needed).
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands.
- [x] Extend the exists-cast parity harness (scenario, oracle, Go runner,
      tests) with the three executions.
- [x] Generate and compare Java/Go traces; run targeted mutation/parity checks
      (32 records, zero differences; Go matches pinned Java oracle).
- [x] Update manifest (three runtime IDs → differential-verified), evidence,
      roadmap, CHANGELOG, README where verified facts change.
- [x] Run independent parity review, resolve findings, run complete local
      gates.
- [ ] Review the final diff, record actual validation, create one semantic
      commit, push `master`, verify the remote ref read-only.


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
- 2026-08-20: `case.expr-core-exists-cast` scalar-Cast extension reached
  zero-difference replay with 32 records (was 22) by reusing existing
  `castToString`/`castToBool`/`parseStringNumber` with no `internal/esper`
  production change. The independent parity review found one minor
  evidence-integrity defect: `javaExecutions[2]` relabeled the pre-existing
  `ExprCoreCastDoubleAndNullOM` runtime (`8fa7f5076dde9d791d08`) as
  `ExprCoreCastStringAndNullCompile`; the label was restored to match the
  execution inventory and evidence regenerated. Manifest case DV runtime IDs
  8→11, summary 419→422, cases stay 134.
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
