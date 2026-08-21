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

- Updated: 2026-08-21
- Baseline: `HEAD` == `origin/master` == `f138c56f1` (`subselect: verify
  join-gated parity (Draft 4.221)` pushed).
- Current work unit: `epl.subselect.filtered`, where-previous slice of the
  same `EPLSubselectFiltered.java` — three behaviorally-equivalent
  executions (base/OM/Compile) across three cases extending
  `subselect-filtered` to twenty cases; zero `internal/esper` changes.
- Frozen Java contract (fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`):
  - WherePrevious trio (`java-runtime-78f47a5bcdf6b87c9e68`,
    `java-runtime-b1c2169572264f9fd646`,
    `java-runtime-87fd9e194fcd61e0aba5`): correlated gated subquery
    `(select prev(1, id) from SupportBean_S1#length(1000) where id=s0.id)`
    projected as value over SupportBean_S0; three-round vector null/1/2 per
    case. WHERE only filters candidate rows; prev(1) reads the unfiltered
    window arrival order (round2 prev=1 is the discriminating probe).
- Differential scope: `subselect-filtered` scenario extended to twenty
  cases (three where-previous cases replaying the shared six-send sequence);
  Java oracle gains the three case branches (WherePrevOracle worker); Go
  runner gains one Prev-in-SubqueryValue branch and two new trace mutations
  targeting om seq2/seq3.
- Allowed files: `internal/app/parity/subselect_filtered.go`,
  `internal/app/parity/run_test.go`,
  `tools/java-oracle/EPLSubselectFilteredScenarioOracle.java` +
  `tools/java-oracle/run-subselect-filtered.sh` (worker/primary fix),
  `testdata/parity/subselect-filtered.json`, regenerated trace/evidence, and
  primary-owned `PLANS.md`, manifest, roadmap, CHANGELOG, README.
- Forbidden: changes under `/root/app/esper`, `goal.txt`, `internal/esper`
  production semantics, hand-authored generated trace/evidence.
- Targeted validation: pinned Java oracle runner, filtered diff/mutation
  tests, `go test ./internal/compat ./internal/app/manifest -count=1`, full
  local gates, independent review.

## Delegation checkpoint

- Coercion unit agents: prefetch scouts `CoercionJavaContract`
  (java-oracle-scout) and `CoercionGoSurface` (scout) ran concurrently with
  the Draft 4.219 review; asset writer `CoercionOracle`
  (parity-asset-worker, isolated) authored the oracle extension while the
  primary agent wrote the scenario extension, runner branch, and mutations.
  File ownership was disjoint.
- Filtered unit agents: prefetch scouts `FilteredJavaContract`
  (java-oracle-scout) and `FilteredGoSurface` (scout) ran concurrently with
  the Draft 4.217 review; asset writer `FilteredOracle` (parity-asset-worker,
  isolated) authored the new Java oracle and runner script on the frozen
  scenario contract while the primary agent wrote the Go runner, dispatch,
  scenario, and tests. File ownership was disjoint.
- Where-previous unit agents: prefetch scouts `WherePrevJavaContract`
  (java-oracle-scout) and `WherePrevGoSurface` (scout) ran concurrently with
  the Draft 4.221 review; asset writer `WherePrevOracle`
  (parity-asset-worker, isolated) authored the oracle case branches while
  the primary agent wrote the scenario extension, runner branch, and
  mutations. File ownership was disjoint.
- Join-gated unit agents: prefetch scouts `JoinFilteredJavaContract`
  (java-oracle-scout) and `JoinFilteredGoSurface` (scout) ran concurrently
  with the Draft 4.220 review; asset writer `JoinFilteredOracle`
  (parity-asset-worker, isolated) authored the oracle extension while the
  primary agent wrote the scenario extension, runner branch, and mutations.
  File ownership was disjoint.
- Multikey unit agents: prefetch scouts `MultikeyJavaContract`
  (java-oracle-scout) and `MultikeyGoSurface` (scout) ran concurrently with
  the Draft 4.218 review; asset writer `MultikeyOracle`
  (parity-asset-worker, isolated) authored the oracle extension (its compile
  failure against the regression-lib-only classpath was fixed by the primary
  agent adding the two event sources to the runner javac step after the
  parked worker could not be revived). File ownership disjoint except that
  one primary-owned script fix.
- Quantified-null unit agents: read-only scouts `JavaContract`
  (java-oracle-scout) and `GoSurface` (scout) ran concurrently before
  implementation; asset writer `OracleExtend` (parity-asset-worker,
  isolated) authored the Java oracle extension on the frozen scenario
  contract while the primary agent wrote the shared engine fix, Go runner,
  scenario, and tests. File ownership was disjoint.
- Cast Dates repair review: `CastDatesReview-2`, APPROVED after canonical
  metadata/evidence alignment; repair commit `c29aab39f` pushed.
- Primary agent owns shared semantics, parity assets, generated trace/evidence,
  manifest, roadmap, CHANGELOG, PLANS, validation, review, commit, and push.

## Quantified null unit (closed; committed as `f8b6f203f`)

- [x] Concurrent read-only scouts (`JavaContract`, `GoSurface`) froze the two
  null/empty-set executions, the exact event vectors, and the reusable Go
  surface (no new builders; `Field[any, any]` + existing quantified builders).
- [x] Extend `subselect-quantified` scenario with `relational-null-no-rows`
  and `equals-in-null-no-rows` (boxed nullable payloads); extend the Java
  oracle via the `OracleExtend` asset worker; extend the Go runner metadata,
  case order, and query branches.
- [x] Fix `evaluateQuantifiedSubquery` relational ANY to keep a decisive false
  when a non-null row exists; regenerate the Java trace (40 records) and the
  passing evidence with zero differences.
- [x] Add four trace mutations for the new cases including the
  false-dominates guard; all reject.
- [x] Promote `case.subquery-empty-quantifiers` to differential-verified;
  extend `case.subquery-quantified-comparisons` to six runtime IDs; manifest
  summary is 136 differential cases, 432 differential runtime IDs, 3071
  associations, 2901 unique referenced runtimes, and 1235 unreferenced.
- [x] Complete final full gates and independent parity review
  (`QuantifiedNullReview` verdict: pass, zero findings); delivery is ready for
  the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `a5d4703760c4c5c166e9fef357422eb50370780a57781d7ef93fa56fcdf6af2d`;
the extended differential evidence is passing with 40 Java records, 40 Go
records, 0 differences, six frozen runtime IDs, and six execution names.
Focused quantified diff/mutation tests (14 mutations all reject), full
`internal/esper` regression after the engine fix, `go vet ./...`, `go test
./... -count=1 -timeout 240s`, `make check`, and `git diff --check` pass.

## Filtered scalar unit (closed; committed as `a5cff7b23`)

- [x] Prefetch scouts (`FilteredJavaContract`, `FilteredGoSurface`) froze the
  five-execution slice, the exact event vectors, and the reusable Go surface
  (existing `SubqueryValue[WithOptions]` builders; zero engine changes).
- [x] Create the `subselect-filtered` scenario (7 cases, 3 event types), the
  Java oracle and runner script via the `FilteredOracle` asset worker, and
  the Go runner, dispatch modes, and diff/mutation tests (9 mutations all
  reject).
- [x] Regenerate the pinned-commit Java trace (24 records) and the passing
  evidence with zero differences; register `case.subselect-filtered-scalar-filter`
  and promote `epl.subselect.filtered` to differential-verified (5/27
  runtimes); manifest summary is 137 differential cases, 437 differential
  runtime IDs, 3076 associations, 2901 unique referenced runtimes, and 1235
- [x] Complete final full gates and independent parity review
  (`FilteredScalarReview` verdict: pass; the single P3 formatting finding in
  the manifest mappings tail was fixed and re-validated); delivery is ready
  for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `ffd54beb15c38c3884cb777b48afdde6c031dc47c6b8168d2dd62b48e6be0656`;
the differential evidence is passing with 24 Java records, 24 Go records,
0 differences, five frozen runtime IDs, and five execution names. Focused
filtered diff/mutation tests (9 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Multikey wArray unit (closed; committed as `4d931c979`)

- [x] Prefetch scouts (`MultikeyJavaContract`, `MultikeyGoSurface`) froze the
  three-execution slice, the exact event vectors, and the reusable Go surface
  (existing `Is`/`And`/`OuterField` builders; zero engine changes).
- [x] Extend `subselect-filtered` scenario to ten cases with array event
  payloads; extend the Java oracle via the `MultikeyOracle` asset worker
  (classpath fix applied by the primary agent); extend the Go runner with
  two array structs, three branches, and four new trace mutations.
- [x] Regenerate the pinned-commit Java trace (38 records) and the passing
  evidence with zero differences; register `case.subselect-filtered-multikey-array`;
  manifest summary is 138 differential cases, 440 differential runtime IDs,
  3079 associations, 2901 unique referenced runtimes, and 1235 unreferenced.
- [x] Complete final full gates and independent parity review
  (`MultikeyReview` verdict: pass, zero findings); delivery is ready for the
  single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `2946762534ffedd7e1e42890abd71c7f132d5d12cdea9dbbd8f6c4be33525a75`;
the differential evidence is passing with 38 Java records, 38 Go records,
0 differences, eight frozen runtime IDs, and eight execution names. Focused
filtered diff/mutation tests (13 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Joined coercion unit (closed; committed as `dcec9e1e0`)

- [x] Prefetch scouts (`CoercionJavaContract`, `CoercionGoSurface`) froze the
  two-execution slice, the five-round vectors, exact rejection boundaries,
  and the reusable Go surface (`JoinMany`/`JoinField`; zero engine changes).
- [x] Extend `subselect-filtered` scenario to fifteen cases with boxed
  SupportBean payloads; extend the Java oracle via the `CoercionOracle`
  asset worker; extend the Go runner with the JoinMany branch, boxed bean
  fields, and four new trace mutations.
- [x] Regenerate the pinned-commit Java trace (63 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-joined-coercion`; manifest summary is 139
  differential cases, 442 differential runtime IDs, 3081 associations,
  2901 unique referenced runtimes, 1235 unreferenced; and
  `epl.subselect.filtered` at 10/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`CoercionReview` verdict: pass; the single P3 finding — the
  predicate-order mutation targeting a structural null record — was fixed by
  retargeting it to the p3 seq2 match record and re-validated); delivery is
  ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `05e3ea6dc541f87e84a233ecd4dec630573fb163437a2f97fd05b825b9452d56`;
the differential evidence is passing with 63 Java records, 63 Go records,
0 differences, ten frozen runtime IDs, and ten execution names. Focused
filtered diff/mutation tests (17 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Join-gated unit (closed; committed as `f138c56f1`)

- [x] Prefetch scouts (`JoinFilteredJavaContract`, `JoinFilteredGoSurface`)
  froze the two-execution slice, the fire/no-fire vectors, prior/prev
  unfiltered-window semantics, and the reusable Go surface (`Join`/`OnEqual`
  builders; zero engine changes).
- [x] Extend `subselect-filtered` scenario to seventeen cases; extend the
  Java oracle via the `JoinFilteredOracle` asset worker; extend the Go runner
  with the S2 struct, Join branch, and three new trace mutations.
- [x] Regenerate the pinned-commit Java trace (67 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-join-gated`; manifest summary is 140 differential
  cases, 444 differential runtime IDs, 3083 associations, 2901 unique
  referenced runtimes, 1235 unreferenced; `epl.subselect.filtered` reaches
  12/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`JoinGatedReview` verdict: pass, zero findings); delivery is ready for the
  single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `74cd9542e22d059247ae94f3e46bbbc7ede54491a4a3343fd271bbef05b64117`;
the differential evidence is passing with 67 Java records, 67 Go records,
0 differences, twelve frozen runtime IDs, and twelve execution names. Focused
filtered diff/mutation tests (20 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Where-previous unit (active)

- [x] Prefetch scouts (`WherePrevJavaContract`, `WherePrevGoSurface`) froze
  the three-execution slice, the three-round vectors, and the reusable Go
  surface (`Prev` inside `SubqueryValue`; zero engine changes).
- [x] Extend `subselect-filtered` scenario to twenty cases; extend the Java
  oracle via the `WherePrevOracle` asset worker; extend the Go runner with
  the Prev branch and two new trace mutations (om seq2 null-flip and seq3
  offset probe).
- [x] Regenerate the pinned-commit Java trace (76 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-where-previous`; manifest summary is 141
  differential cases, 447 differential runtime IDs, 3086 associations,
  2901 unique referenced runtimes, 1235 unreferenced;
  `epl.subselect.filtered` reaches 15/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`WherePrevReview` initial verdict flagged one P2 stale doc comment; the
  fix was re-checked by the same reviewer: pass, zero findings); delivery is
  ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `08ce7300b5fea5f0c896f70a779fb5c780b273b2fd12fc4d0f951c40b9091511`;
the differential evidence is passing with 76 Java records, 76 Go records,
0 differences, fifteen frozen runtime IDs, and fifteen execution names.
Focused filtered diff/mutation tests (22 mutations all reject), `go vet
./...`, `go test ./... -count=1 -timeout 240s`, `make check`, and
`git diff --check` pass.

## Historical work-unit contract (Generic Cast; closed)

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


## Direct multirow unit (closed; committed as `b5599f1ba`)

- [x] Freeze the two `EPLSubselectMultirow.java` executions and exact runtime
  IDs with concurrent scouts `SubqueryMultirowJavaContract` and
  `SubqueryMultirowGoSurface`.
- [x] Implement the dedicated Go runner, dispatch modes, pinned Java oracle,
  scenario, and checked-in six-record trace/evidence.
- [x] Correct the staged single-column replay to accept eight sends with the
  Java 5-send/3-send redeploy split; focused value, window-retention, null,
  underlying-field, case, and record-count mutations reject.
- [x] Normalize only correlated `val` arrays in the direct multirow runner and
  its differential boundary. Stable JSON row keys mirror the Java oracle's
  `SupportBean[]`/`EventBean[]` sorting; shared `compat.normalizeValue` and
  global trace comparison remain unchanged. Swapped-row differential mutation
  passes while semantic mutations reject.
- [x] Register `case.subquery-multirow` and `query.subquery-basic` mapping;
  manifest summary is 523 cases, 135 differential cases, 430 differential
  runtime IDs, 3069 associations, 2901 unique referenced runtimes, and 1235
  unreferenced runtimes.
- [x] Complete final full gates and independent parity review; delivery is ready
  for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `0bffc4f680c631cdb5958e2a650a1d174f0cc00f3e2cf0581d20082cea50dcf1`;
the direct differential evidence is passing with 6 Java records, 6 Go records,
0 differences, both frozen runtime IDs, and both execution names. Focused
direct replay/mutation tests, `go test ./internal/compat ./internal/app/manifest
-count=1`, `go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`,
and `git diff --check` pass. `DirectMultirowRunnerReview` approved the scoped
correlated-array normalization and its separate swapped-row acceptance test.

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

### Generic unit (closed)

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

### Cast Dates unit (closed; metadata repair)

Baseline: implementation commit `500d8db84`; metadata repair follows on the
same committed behavior and does not modify the pinned trace.
Delegation checkpoint: `CastDatesJavaContract` (java-oracle-scout) and
`CastDatesGoSurface` (scout) were launched concurrently; primary agent read
`ExprCoreCast.java:376-820` independently.

Frozen work-unit contract (execution `ExprCoreCastDates`, runtime
`java-runtime-2ee2b8ab1bf9bb2c4e90`, inventory line 2157):
- 3 cases, 3 sends, 51 -> 54 records:
  - `cast-dates-base`: MyDateType map event `{yyyymmdd:"20100510"}`; 9
    columns (date/java.util.Date, long/java.lang.Long,
    calendar/java.util.Calendar targets, plus `.get("month")` chains);
    expected Date/Long epoch 1273449600000, month Integer 4 (zero-based).
  - `cast-dates-java8`: LocalDate, LocalDateTime, and LocalTime alias/FQCN
    targets render deterministic ISO values; zoneddatetime VV cells remain
    deferred because Go stdlib lacks zone-region parsing.
  - `cast-dates-constant`: SupportBean("E1",1); one constant-folded Date
    column with epoch 1044057600000.
- Go mapping uses `CastWithLayout[string,time.Time]`, `UnixMillis`, and
  `Month(...) - 1`; no `internal/esper` change was needed.
- Deferred follow-ups: ISO8601 `iso`, dynamic dateformat, non-string formatter
  parameters, invalid compile diagnostics, render-out-column metadata, and VV
  zoneddatetime cells.

Progress:
- [x] Oracle, Go runner, scenario, pinned trace, and zero-difference replay.
- [x] Four date mutations reject (epoch, month, Java 8 cell, constant cell).
- [x] Manifest and Draft 4.215 docs record 17 differential runtime IDs.
- [x] Post-commit repair aligns Go/evidence runtime IDs and execution names to
  Java inventory ordinal order; manifest prose now states four Exists plus
  thirteen typed Cast executions.
- [x] Regenerate evidence from the unchanged trace; targeted parity/compat
  tests, JSON checks, full gates, and `CastDatesReview-2` re-review all pass.
  One semantic repair commit and push remain.
- 2026-08-21: Repair validation completed against the unchanged pinned trace;
  `case.expr-core-exists-cast` now has 17 inventory-ordered runtime IDs, 17
  execution names, and 54 records with passing evidence and zero differences.
  The trace checksum is `f0efd3239e22fbb9533a648d7b2cee9ac321df6e45a94a77712418ffe8f7f40d`.
  `go vet ./...`, `go test ./... -count=1`, `make check`, targeted parity and
  compat tests, JSON validation, and `git diff --check` passed; the repair is
  ready for its semantic commit and push.

- 2026-08-20: GitLab DOES protect `master` (force-push rejected 2026-08-20,
  contradicting AGENTS.md/runbook claims). Amended commits cannot be repushed;
  finalize all file changes BEFORE the single unit commit/push. Post-commit
  `PLANS.md` closeout notes must ride with the next unit's commit.

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
- 2026-08-21: Cast metadata repair validation passed the pinned Java trace
  replay, focused parity/mutation/manifest checks, JSON/schema checks, `gofmt`,
  `go vet ./...`, `go test ./... -count=1`, `make check`, and `git diff --check`.
  The regenerated evidence is passing with 17 inventory-ordered runtime IDs,
  17 execution names, 54 records, and zero differences; the checked-in trace
  remains byte-identical (SHA-256
  `f0efd3239e22fbb9533a648d7b2cee9ac321df6e45a94a77712418ffe8f7f40d`).
- Current unit result: `case.expr-core-exists-cast` is differential-verified
  across 17 exact inventory-ordered runtime IDs and 54 listener records. The
  Cast Dates metadata repair aligns Go/evidence runtime IDs and execution names
  with the manifest and Java inventory; the regenerated evidence is passing with
  zero differences. The pinned trace remains byte-identical. Full local gates
  and the independent `CastDatesReview-2` re-review pass; remote delivery is
  ready for the semantic repair commit and push.

## Validation evidence

- Investigation baseline before the Cast slice on 2026-08-20: `/root/app/esper`
  is fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` was clean and
  matched `origin/master` at `c325027dc`. Manifest summary reported 522 cases,
  134 differential-verified cases, and 415 differential runtimes before the
  four Cast runtime IDs were added.
- Current unit result: `case.expr-core-exists-cast` is differential-verified
  across 17 exact inventory-ordered runtime IDs and 54 listener records. The
  Cast Dates metadata repair aligns Go/evidence runtime IDs and execution names
  with the manifest and Java inventory; the regenerated evidence is passing with
  zero differences. The pinned trace remains byte-identical. Full local gates
  and the independent `CastDatesReview-2` re-review pass; remote delivery is
  ready for the semantic repair commit and push.
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
