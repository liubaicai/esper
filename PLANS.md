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
- Baseline: `HEAD` == `origin/master` == `67514e7db` (`cast-warray` unit,
  pushed). Uncommitted: this PLANS.md closeout note (rides with the next
  unit's commit; master is protected, no post-push amend).
- Status: the delivered `ExprCoreCastWArray` unit is committed and pushed.
  The next expr.core unit extends `case.expr-core-exists-cast` with
  `ExprCoreCastGeneric` (`java-runtime-2fcd2094aaf18ca4ce03`, ordinal 11,
  SERDEREQUIRED).
- Current work unit: `expr.core` / generic Cast extension — schema MyEvent
  with 8 Object-typed columns (listOfString, listOfOptionalInteger,
  mapOfStringAndInteger, listArrayOfString, listOfStringArray,
  listArray2DimOfString, listOfStringArray2Dim, listOfT) and
  `@name('s0') select cast(<col>, <generic target>) as <col> ... from
  MyEvent`, per SupportGenericColUtil.NAMESANDTYPES.
- Exact next action: freeze the Generic contract (scouts
  `GenericCastJavaContract` + `GenericCastGoSurface` running), record it
  here, then implement oracle case + fixture sends + Go runner + run_test
  counts, regenerate Java trace, Go replay zero-difference diff, evidence,
  manifest/roadmap/CHANGELOG, full gates, parity review, one semantic
  commit, push `master`, verify remote ref.
- Worktree notes: do not modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; this unit's read-only scouts were
  launched concurrently before implementation: `GenericCastJavaContract`
  (java-oracle-scout) and `GenericCastGoSurface` (scout). Primary agent
  independently read `ExprCoreCastGeneric` (ExprCoreCast.java:97-131) and
  `SupportGenericColUtil` (names/types/sample/compare) to cross-check.
- Frozen contract (pending scout confirmation): one case `cast-generic`,
  one statement, one send. Sample values: listOfString=["a"],
  listOfOptionalInteger=[Optional.of(10)], mapOfStringAndInteger={k:20},
  listArrayOfString=[[b]], listOfStringArray=[[c]],
  listArray2DimOfString=[[[b]]], listOfStringArray2Dim=[[[c]]],
  listOfT=["x"]. Java asserts cast returns the same instances; the
  differential encodes per-cell normalized rendering with
  Optional.get()=10 unwrapped to "10".
- Scope decision: `ExprCoreCastDates` stays deferred (needs chained
  calendar-get expression, send-time error record protocol, Java8 distinct
  time types).
- Implementation writer: primary agent owns the exists-cast parity surface
  as one atomic set; singleton writer is the recorded exception for this
  harness.
- Independent parity reviewer: `GenericReview` scheduled after integration.


## Work-unit contract

- Capability/subdomain: `expr.core`, generic Cast extension of
  `case.expr-core-exists-cast`, one source-order `ExprCoreCast.java`
  execution (`ExprCoreCastGeneric`, ordinal 11) beside the fifteen
  already-verified executions.
- Java source and execution/runtime ID: fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`,
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java`
  lines 97-131 + `support/events/SupportGenericColUtil.java`,
  `ExprCoreCastGeneric` / `java-runtime-2fcd2094aaf18ca4ce03` (already in
  manifest case javaRuntimeIds; only DV status changes).
- Differential scope: extend `testdata/parity/expr-core-exists-cast.json`
  with one case `cast-generic`, one send of map event `MyEventGeneric` (8
  Object-typed props). Select = `select cast(listOfString,
  java.util.List<String>) as listOfString, cast(listOfOptionalInteger,
  java.util.List<java.util.Optional<Integer>>) as listOfOptionalInteger,
  cast(mapOfStringAndInteger, java.util.Map<String,Integer>) as
  mapOfStringAndInteger, cast(listArrayOfString, java.util.List<String>[]) as
  listArrayOfString, cast(listOfStringArray, java.util.List<String[]>) as
  listOfStringArray, cast(listArray2DimOfString, java.util.List<String>[][])
  as listArray2DimOfString, cast(listOfStringArray2Dim,
  java.util.List<String[][]>) as listOfStringArray2Dim, cast(listOfT,
  java.util.List<Object>) as listOfT from MyEventGeneric` (harness type
  renamed from MyEvent to avoid collision; same cast semantics).
- Observable contract: one row at virtual time zero; eight columns preserve
  the sent values elementwise (List/Map cast identity on erasure);
  Optional.get()=10 renders as "10". Deterministic rendering both sides:
  lists as JSON arrays of element tokens, maps as sorted-key JSON objects,
  Optional unwrapped.
- Allowed production files: none under `internal/esper` expected (List/Map
  assignable casts exist via castAnyToType); parity-layer only unless the
  Go surface scout flags a real gap.
- Allowed Go test/parity files: `internal/app/parity/expr_core_exists_cast.go`
  (case dispatch, MyEventGeneric decode branch, list/map/Optional
  normalizer cases), `internal/app/parity/run_test.go` (49->50 records,
  case count, mutations), scenario/trace/evidence fixtures, oracle +
  runner script guards.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreExistsCastScenarioOracle.java` (generic case,
  payload builders, List/Map/Optional token rendering in TraceWriter) and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  `internal/esper` semantic surfaces; `goal.txt`; generated evidence before
  trace validation; central facts outside this unit's manifest/roadmap/CHANGELOG
  updates. `PLANS.md`, manifest, roadmap, CHANGELOG, traces, evidence remain
  primary-agent owned.
- Targeted validation: pinned Java oracle runner; focused exists-cast
  parity + mutation tests; scenario shape validator; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.


## Progress

- [x] Reconfirm baseline (`HEAD`/`origin/master` clean at `2a6c7f9a4`),
      roadmap, manifest.
- [x] Launch concurrent Java/Go read-only scouts for the remaining cast
      executions (Generic / WArray / Dates).
- [x] Java contract scout `DatesGenericArrayJavaContract` and Go surface scout
      `DatesGenericArrayGoSurface` both returned (6m13s / 20m25s); contract
      frozen: WArray{soda=false}+{soda=true} = one clean closed-loop unit;
      Generic and Dates deferred with recorded reasons.
- [x] Record the frozen WArray contract in this checkpoint and delegation
      records.
- [x] Implement the two `cast-warray` cases end-to-end: Java oracle (EPL
      insert-into, array-element token rendering, payload builders), scenario
      fixture (2 cases x 2 sends), Go runner (MyEvent map schema, MyArrayEvent
      target, decode branches, array/bean normalizer), run_test coverage
      (49 records, mutation indices).
- [x] Generate pinned Java trace; Go replay is zero-difference; add
      boundary/mutation coverage (4 warray mutations all reject).
- [x] Update manifest (case DV runtime IDs 13->15, summary 424->426),
      regenerate evidence (passing), roadmap, CHANGELOG.
- [x] Run full local gates and independent parity review; resolve findings
      (`WArrayReview`, parity-reviewer, APPROVED at 0.93 confidence with one
      P3 dead-code nit — unused `myEventWArrayMap()` — removed and the trace
      re-verified byte-identical afterwards).
- [x] Review final diff, record validation, create one semantic commit
      (`67514e7db` "expr: verify array cast parity"), push `master`, verify
      remote ref read-only (local == origin/master == `67514e7db`).

### Generic unit (in flight)

- [x] Reconfirm baseline (`HEAD` == `origin/master` == `67514e7db`).
- [x] Launch concurrent read-only scouts `GenericCastJavaContract` +
      `GenericCastGoSurface`; primary agent independently read
      ExprCoreCastGeneric + SupportGenericColUtil.
- [x] Freeze the Generic contract in this checkpoint (1 case, 1 send,
      8 columns, runtime `java-runtime-2fcd2094aaf18ca4ce03`).
- [x] Confirm scout reports match the frozen contract; adjust if they
      surface gaps.
- [x] Implement `cast-generic` end-to-end: oracle case + payload builders +
      TraceWriter List/Map/Optional rendering, scenario fixture send, Go
      runner case + decode + normalizer, run_test 49->51.
- [x] Pinned Java trace; zero-difference diff; mutations (4 generic
      mutations added); evidence.
- [x] Full gates green (`make check`), `GenericReview` parity review APPROVED
      (one P2 fixed: evidence javaRuntimeIds/javaExecutions + Go fallback
      lists appended in lockstep; evidence regenerated, 16 IDs / 15
      executions); commit `f2e6c3771` pushed to `master`.

### Cast Dates unit (in flight)

Baseline: `HEAD` == `origin/master` == `f2e6c3771`.
Delegation checkpoint (parallel batch): `CastDatesJavaContract`
(java-oracle-scout, 20m52s) + `CastDatesGoSurface` (scout, 13m10s);
primary agent independently read ExprCoreCast.java:376-820.

Frozen work-unit contract (execution `ExprCoreCastDates`, runtime
`java-runtime-2ee2b8ab1bf9bb2c4e90`, inventory line 2157):
- 3 cases, 3 sends, 51 -> 54 records:
  - `cast-dates-base`: MyDateType map event `{yyyymmdd:"20100510"}`; 9
    columns (date/java.util.Date, long/java.lang.Long,
    calendar/java.util.Calendar targets, plus `.get("month")` chains);
    expected Date/Long epoch 1273449600000, month Integer 4 (0-based
    Calendar month).
  - `cast-dates-java8`: same map event with all 4 string props; 6 columns:
    localdate/java.time.LocalDate x2 (2010-05-10), localdatetime x2
    (2010-05-10T14:15:16), localtime x2 (14:15:16). zoneddatetime VV cells
    deferred (Go stdlib lacks zone-region parsing; genuine coercion gap).
  - `cast-dates-constant`: SupportBean("E1",1); 1 column constant-folded
    Date(1044057600000).
- Determinism: harness already pins `-Duser.timezone=UTC -Duser.language=en
  -Duser.country=US` (Java 17); Go parses layouts in time.UTC.
- Trace encoding (frozen): java.util.Date/Calendar/Long cast columns render
  epoch millis int; LocalDate/LocalDateTime/LocalTime columns render ISO
  strings ("2010-05-10" / "2010-05-10T14:15:16" / "14:15:16"); month
  columns render int 4. Oracle TraceWriter gains temporal branches; Go
  normalize mirrors.
- Go API mapping: `CastWithLayout[string,time.Time]` ("20060102" etc.),
  long targets via `UnixMillis(CastWithLayout[...])`, month via
  `Month(...) - 1` (Java 0-based). No internal/esper changes expected.
- Deferred follow-ups (recorded, not this unit): ISO8601 'iso' cases
  (XMLGregorianCalendar zone-id semantics), dynamic dateformat (runtime
  EPExceptions need new trace op), dateformat-nonstring (locale-sensitive
  literals), invalid compile diagnostics (needs error-record kind),
  render-outcol (needs column-name record kind), VV zoneddatetime cells.

Progress:
- [x] Baseline confirmed; parallel scouts returned; contract frozen here.
- [x] Oracle: 3 case branches + MyDateType schema registration + temporal
      TraceWriter branches (Date/Calendar -> epoch millis).
- [x] Go runner: 3 case arms, MyDateType decode branch, case-aware
      temporal normalizer (epoch vs ISO per column), metadata arrays
      (17 IDs / 16 executions).
- [x] Fixture +3 sends (73 steps); runner script guards 51->54 records,
      +cast-dates cases, MyDateType==2, SupportBean 8->9.
- [x] Pinned Java trace; zero-difference diff; 4 new dates mutations
      (epoch/month/java8/constant cells) all reject; evidence regenerated.
- [x] Manifest (DV 16->17, 427->428), roadmap, CHANGELOG 4.215, README.
- [ ] Full gates, `CastDatesReview`, one semantic commit, push, verify ref.

- 2026-08-20: GitLab DOES protect `master` (force-push rejected 2026-08-20,
  contradicting AGENTS.md/runbook claims). Amended commits cannot be repushed;
  finalize all file changes BEFORE the single unit commit/push. Post-commit
  PLANS closeout notes must ride with the next unit's commit.


## Discoveries and decisions

- 2026-08-20: Oracle harness fix: prepending `@name('s0')` to multi-statement
  EPL (cast-warray declares two schemas plus the insert) makes Esper name the
  first schema statement `s0` and the insert `s0-1`; the old first-`s0`
  statement finder attached the listener to the schema statement and the case
  produced zero records. The finder now selects the LAST `s0`-prefixed
  statement and the TraceWriter records a fixed `"s0"` statement name so Java
  and Go traces stay field-comparable.

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
- 2026-08-20: The `ExprCoreCastInterface` contract is frozen. The Java
  regression asserts bean identity (`event.get("tX") == bean`), so the
  differential encoding is per-target bean presence. The `cast(item?, T)`
  semantics: marker/super-interface targets succeed via assignability or
  Implements; a class target (t3 ISupportBaseABImpl) is exact-class only and
  never satisfied through an interface; an abstract superclass target (t6)
  matches its concrete subclass. Send 1 must double-wrap
  (`item = SupportBeanDynRoot("abc")`), because a plain String satisfies no
  target interface. Both sides render matched beans as the Java simple class
  name token (deterministic, byte-exact) instead of `String.valueOf`'s
  address hash. Go requires zero `internal/esper` changes (castAnyToType
  already implements assignable/Implements/else-null).
- 2026-08-20: The `cast-interface` implementation needed one
  `internal/esper` shared semantic fix (correcting the scouts' "zero change"
  expectation): `typeOf` resolved a nil interface-typed `T` to the empty
  interface `any`, so every bean satisfied every target cell (all t0..t7
  matched on first probe). The fix recovers the static interface type via
  `reflect.TypeOf((*T)(nil)).Elem()`, preserving concrete/pointer resolution.
  Full `internal/esper` unit suite plus the complete parity suite pass with
  the fix. Differential encoding renders matched beans as Java simple-class
  name tokens on both sides, and the traces confirm the exact frozen
  bean-target matrix (S1->t0, S2->t5, S3->t2+t4, S4->t1/t2/t4/t6/t7,
  S5->t2+t3).
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
  deployments, and the coercion coverage.
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
