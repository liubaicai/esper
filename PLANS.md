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
  `ed3d292aa` (scalar cast parity); the next semantic change must preserve
  that baseline.
- Status: the delivered scalar-Cast unit is committed. The next expr.core
  unit extends `case.expr-core-exists-cast` with
  `ExprCoreCastBigDecimalBigInt` (running at
  `java-runtime-f44847213060b8eb3949`).
- Current work unit: `expr.core` / BigDecimal-BigInteger Cast —
  `Cast[any, big.Rat]` (BigDecimal) + `Cast[any, big.Int]` (BigInteger) on a
  map event `MyEvent(value java.lang.Object)`.
- Exact next action: review the final diff, create one semantic commit,
  push `master`, and verify the remote ref read-only.
- Worktree notes: delivered baseline is clean; preserve it and do not
  modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; the next-unit read-only scouts were
  launched concurrently through collaboration tools before implementation.
- Java contract scout: `CastRemainingJavaContract` returned the frozen
  contract for the remaining non-date Cast executions: BigDecimalBigInt
  (`java-runtime-f44847213060b8eb3949`), Interface
  (`java-runtime-2012a048edc6511a33e0`), WArray both deployment modes
  (`java-runtime-53d0455ef4e377c9c2f9` / `58852773df609efe7d69`).
- Go surface scout: `CastRemainingGoSurface` returned a degenerate stub
  ("probe") with no real assessment; rejected. The primary agent completed
  the focused Go surface investigation locally (read-only): `Cast[any,
  big.Rat]`/`Cast[any, big.Int]` exist and `castToBigInt` is correct, but
  `castToBigRat`'s float branch used `SetFloat64` (binary-exact) and
  needed a shortest-decimal round-trip (`strconv.FormatFloat(v,'g',-1,
  bits)` + `big.Rat.SetString`) to match Java `BigDecimal.valueOf(double)`;
  big.Rat exact-decimal trace rendering needed a harness normalizer
  addition; WArray requires a bean `insert-into` production surface (no
  `RegisterBean`/`InsertInto` exists in internal/esper) and Interface
  requires Go bean-hierarchy identity modeling — both are separate
  production-surface units.
- Scope decision: this unit covers only `ExprCoreCastBigDecimalBigInt`
  (one execution). WArray and Interface each need dedicated
  `internal/esper` production-surface work (bean insert-into with array
  fields; dynamic dirty-bean interface hierarchy), so they are deferred
  to their own focused units — this is the recorded serial reason, not a
  padded split.
- Implementation writer: primary agent owns the whole exists-cast parity
  surface (scenario, oracle, Go runner, tests) as one atomic set; singleton
  writer is the recorded exception for this harness.
- Independent parity reviewer: `BigDecimalBigIntReview` (parity-reviewer)
  returned APPROVED with no findings — evidence integrity, trace honesty
  (including 2.4 -> "2.4" and the 2^500500 / 2^500500+0.1 vectors), the
  `castToBigRat` float round-trip fix, the 13-mutation table, the Java
  oracle, the runner jq, and the manifest/doc facts all verified.

## Work-unit contract

- Capability/subdomain: `expr.core`, BigDecimal/BigInteger Cast extension of
  `case.expr-core-exists-cast`, one source-order `ExprCoreCast.java`
  execution (`ExprCoreCastBigDecimalBigInt`) beside the eleven
  already-verified executions.
- Java source and execution/runtime ID: fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`,
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java`
  lines 61-95, `ExprCoreCastBigDecimalBigInt` /
  `java-runtime-f44847213060b8eb3949` (already inventoried in the manifest
  case; only differential-verified status changes).
- Differential scope: extend `testdata/parity/expr-core-exists-cast.json`
  with `cast-bigdecimal-bigint` (8 sends of a map `MyEvent(value)` tagged by
  kind): int 1, long 2, double 2.4, decimal "156.78", bigint "200",
  bigint 2^500500, decimal 2^500500+0.1, null. Select
  `cast(value, BigDecimal) as c0, cast(value, BigInteger) as c1`.
  Expected rows: c0/c1 both big numbers (big.Rat/big.Int in Go;
  BigDecimal/BigInteger in Java); double 2.4 -> "2.4" (Java
  `BigDecimal.valueOf(double)` shortest-decimal round-trip, not binary
  exact) and c1=2 (truncation toward zero); decimal/bigint round-trip;
  2^500500+0.1 preserves trailing ".1" and 2^500500 the integer part;
  null -> null.
- Observable contract: one new row per send at virtual time zero, no old
  stream/timers/errors, fresh runtime+statement per execution. Java
  BigDecimal/BigInteger toString is the exact decimal/integer string; Go
  must render big.Rat (non-integer) as its exact terminating decimal
  expansion (not FloatString/RatString). BigInteger from double truncates
  toward zero ((long)2.4 -> 2).
- Allowed production files: `internal/esper/expr_cast.go` — the diff
  required a real semantic fix: `castToBigRat`'s float branch now
  round-trips the double through its shortest decimal
  (`strconv.FormatFloat(v,'g',-1,bits)` + `big.Rat.SetString`) instead of
  `SetFloat64`, matching Java `BigDecimal.valueOf(double)` (2.4 -> "2.4").
  Plus an exact big.Rat -> decimal rendering helper in the trace
  normalizer.
- Allowed Go test/parity files: `internal/app/parity/expr_core_exists_cast.go`
  (`cast-bigdecimal-bigint` case, exact-decimal normalizer path),
  `internal/app/parity/run_test.go` (record/case counts, new mutations).
- Allowed parity asset files: `tools/java-oracle/ExprCoreExistsCastScenarioOracle.java`
  (`MyEvent` map schema, `cast-bigdecimal-bigint` case) and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  `internal/esper` semantic surfaces; `goal.txt`; generated evidence before
  trace validation; central facts outside this unit's manifest/roadmap/CHANGELOG
  updates. `PLANS.md`, manifest, roadmap, CHANGELOG, traces, evidence remain
  primary-agent owned.
- Targeted validation: pinned Java BigDecimal-bigint oracle runner; focused
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
- [x] Review the final diff, record actual validation, create one semantic
      commit, push `master`, verify the remote ref read-only.
- [x] Implement the `cast-bigdecimal-bigint` case in the Go runner, Java
      oracle, and scenario (with exact big.Rat decimal rendering).
- [x] Generate pinned Java trace and run zero-difference Go replay (40
      records, 0 differences); fix `castToBigRat` float branch to match
      `BigDecimal.valueOf(double)`; add boundary/mutation coverage (13
      mutations pass).
- [x] Update manifest (423 runtime IDs), regenerate evidence (passing),
      roadmap, CHANGELOG, README.
- [ ] Run full local gates and independent parity review; resolve findings.
- [ ] Review final diff, record validation, create one semantic commit,
      push `master`, verify remote ref read-only.


## Discoveries and decisions

- 2026-08-20: The `cast-bigdecimal-bigint` unit passed the independent
  parity review (`BigDecimalBigIntReview`, parity-reviewer, APPROVED, no
  findings). The reviewer independently re-verified every evidence,
  trace, mutation, oracle, and doc claim, including a Python bigint-pow
  exact-match of the 2^500500 / 2^500500+0.1 vectors and the
  `Double.toString` round-trip for 2.4.
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
- 2026-08-20: The `cast-bigdecimal-bigint` unit reached zero-difference
  replay with 40 records (was 32). The diff exposed a genuine
  `internal/esper` semantic defect: `castToBigRat`'s float branch used
  `big.Rat.SetFloat64` (binary-exact), but Java `BigDecimal.valueOf(double)`
  round-trips through `Double.toString` — fixed to
  `strconv.FormatFloat(v,'g',-1,bits)` + `big.Rat.SetString`, with
  exact-decimal trace rendering for non-integer big.Rat. Manifest case DV
  runtime IDs 11→12, summary 422→423, cases stay 134.
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
