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

- Shipped: Draft 4.378 ('database-slices') committed; Git owns identity. The `epl-database-join-2` chain adds five more executions (case dv list 5->10).
- Shipped: Draft 4.379 ('database-slices') committed; Git owns identity. The eleven remaining EPLDatabaseJoin executions are enumerated as manifest dispositions (no trace change).
- Shipped: Draft 4.380 ('timebatch-historical-release') committed as 58fa99c5f; Git owns identity. Engine fix releases joined rows at time-batch boundaries over historical joins; `EPLDatabaseTimeBatch` differential-verified.
- Shipped by this commit: Draft 4.381 ('infra-namedwindow-views-keepall-delete'); Git owns identity. Second differential coverage of `InfraNamedWindowViews.java` (ord 1, 39-42, 5 runtime IDs; ord 3 was already verified via `case.time-window-long-running`); engine fix orders the window statement's output before tail-view consumer dispatches on the mutation path, guarded so mutation-preprocessing statement output keeps precedence (review finding); 637 cases, 260 DV cases, 969 DV runtime IDs.
- Shipped by this commit: Draft 4.383 ('infra-named-window-unique-views'); Git owns identity. Third InfraNamedWindowViews slice (ords 32/33/34) differential-verified; new any-mode snapshot protocol.
- Shipped by this commit: Draft 4.384 ('infra-named-window-length-views'); Git owns identity. Fourth InfraNamedWindowViews slice (ords 11/12/13/25) differential-verified.
- Shipped by this commit: Draft 4.385 ('infra-named-window-lengthbatch-sort-views'); Git owns identity. Fifth InfraNamedWindowViews slice (ords 19-22) differential-verified.
- Shipped by this commit: Draft 4.386 ('infra-named-window-time-views'); Git owns identity. Sixth InfraNamedWindowViews slice (ords 4/5/8) differential-verified under virtual time; chain gains the advance-time step protocol.
- Shipped by this commit: Draft 4.387 ('infra-named-window-ext-time-views'); Git owns identity. Seventh InfraNamedWindowViews slice (ords 6/7/53) differential-verified (event-time, no engine change).
- Shipped by this commit: Draft 4.388 ('infra-named-window-time-order-accum-views'); Git owns identity. Eighth InfraNamedWindowViews slice (ords 9/10/14/15) differential-verified under virtual time.
- Shipped by this commit: Draft 4.389 ('infra-named-window-time-batch-views'); Git owns identity. Ninth InfraNamedWindowViews slice (ords 16/17/18/23/24) differential-verified under virtual time: batch flush semantics (new+old single delivery, silent deletes, no callback for an all-empty flush, anchor surviving an empty flush, time_length_batch size/time dual trigger) and the late consumer whose preload is skipped; no engine change.
- Shipped by this commit: Draft 4.390 ('infra-named-window-groupwin-views'); Git owns identity. Tenth InfraNamedWindowViews slice (ords 26/27/44/55) differential-verified: per-group length/time_batch retention and late-started grouped consumers with iterate-time variable having; no engine change.
- Shipped by this commit: Draft 4.391 ('infra-named-window-consumer-views'); Git owns identity. Eleventh InfraNamedWindowViews slice (ords 43/45/49/50/51) differential-verified: filtered/prior/univariate/late consumers and the left-outer late join; no engine change.
- Shipped by this commit: Draft 4.392 ('infra-named-window-bean-views'); Git owns identity. Twelfth InfraNamedWindowViews slice (ords 2/35/37/38) differential-verified: bean/schema representations, nested fragment, supertype insert and FAF observation; the ord-2 update-wave adjacency is oracle-pinned and normalized through the established hook.

- Shipped: Draft 4.405 ('event-json-adapter') committed; Git owns identity. EventJsonAdapter observable slice differential-verified; invalid execution split to its own intentionally-different case (652 cases, 278 DV cases, 1023 DV runtime IDs).
- Shipped by commit 24a2631ec: Draft 4.394 ('infra-named-window-insert-shape'); Git owns identity. Thirteenth InfraNamedWindowViews slice (ords 28/36/54/56) differential-verified, 8/8 records, 0 differences; engine fix captures one namedWindowInsertBoundary per direct insert trigger. InfraNamedWindowViews 53/58 executions differential (649 cases, 272 DV cases, 1016 DV runtime IDs).
- Shipped: Draft 4.483 ('infra-nwtable-on-merge-pattern-nowhere') committed and pushed as 36378f03f; Git owns identity.

- Shipped: Draft 4.484 ('infra-nwtable-on-merge-flow-itv') committed and pushed as ef141897e; Git owns identity. Parity review initial FAIL on one P2 (#unique windows created with keep-all retention), fixed and re-reviewed PASS.

- Shipped: Draft 4.485 ('infra-nwtable-on-merge-invalid-insertonly') committed and pushed as 9ac32d036; Git owns identity. Parity review PASS, two P3s fixed.

## Current work unit
Active: Draft 4.526 ('expr-dt-data-sources').

- Selection: `ExprDTDataSources.java` all 4 executions, no flags —
 `ExprDTDataSourcesStartEndTS` (`java-runtime-ab467628bb158bf8ce6c`),
 `ExprDTDataSourcesFieldWValue` (`java-runtime-0ee5536b7a3d3d75f2eb`),
 `ExprDTDataSourcesAllCombinations` (`java-runtime-e1d8bce93d3743e9137f`),
 `ExprDTDataSourcesMinMax` (`java-runtime-fb5317848fcedbabf016`).
 Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`.
- [x] Contract frozen: JavaContract526 (4 executions: MinMax windowed+row
  min/max as interval endpoints, AllCombinations 5-field x 11-getter
  fan-out with java8 asymmetries, FieldWValue 16-col current_timestamp+
  field getters, StartEndTS compile-only schema inheritance + 3 invalid
  probes) + GoSurface526 (small delta: 3 DateTimeGet arms day_of_year/
  era/weekyear + joinSourceEvent single-stream fallback; MinMax uses
  existing Interval+BeforeThreshold+MinOf; no new API).
- [x] Parallel lanes dispatched: SharedCore526 (expr_dt_resolution.go
  DateTimeGet arms + joinSourceEvent fallback + tests) + Assets526
  (runner/scenario/oracle/wiring only). Frozen API names in batch context.
- [x] SharedCore526 integrated: `internal/esper/expr_dt_resolution.go`
  DateTimeGet arms (day_of_year/era/weekyear + week/millis_of_second
  aliases) + joinSourceEvent single-stream fallback + tests.
- [x] Assets526 integrated: `internal/app/parity/expr_dt_data_sources.go` +
  `testdata/parity/expr-dt-data-sources.json` (4 cases / 49 steps) +
  oracle `tools/java-oracle/ExprDTDataSourcesScenarioOracle.java` +
  `run-expr-dt-data-sources.sh` + run.go/run_test.go wiring. Worker
  verified zero-diff end-to-end before handoff.
- [x] Differential replay: Java 16 records; Go 16 records. `-mode
  expr-dt-data-sources-diff` status `passing` / 0 differences (identity
  normalizer).
- [x] Manifest: NEW case `case.expr-dt-data-sources` born-DV with the 4
  IDs; `expr.core` mapping + goRefs extended. Summary 757 cases / 383 DV /
  1506 DV runtime IDs / unreferenced 526.
- [ ] Gates + parity review + commit: pending `make check`, independent
  reviewer, then commit/push.

## Previous work units (shipped)

- Shipped: Draft 4.525 ('expr-dt-resolution') committed and pushed as
 7bff4591a; Git owns identity. Born-DV `case.expr-dt-resolution` under
 `expr.core` (all 4 executions, 8 records, 0 differences). Shared core:
 WithTimeUnit(Microseconds) + resolution-aware datetime ops +
 EventIntervalBounds/IntervalBefore + Engine threading through join
 conditions/filterJoinTuples/dataflow joins. Parity review PASS (one P2
 fixed: dataflow join engine threading).

- Shipped: Draft 4.524 ('expr-enum-select-from') committed and pushed as
 0c9075639; Git owns identity. Born-DV `case.expr-enum-select-from` under
 `expr.enum` (ords 1/3/4, 12 records, 0 differences). Zero engine changes.
 Parity review PASS, zero findings.


- Shipped: Draft 4.523 ('expr-dt-interval-ops') committed and pushed as
  8a898757e; Git owns identity. Born-DV `case.expr-dt-interval-ops-calops`
  under `expr.core` (ords 0/1/17, 60 records, 0 differences). Shared core:
  DateTimeSet/WithDate/WithTime/Before + duration-preserving bounds
  transforms. Parity review PASS (three P3 fixes).

- Shipped: Draft 4.522 ('epl-other-istream-rstream-keywords') committed and
  pushed as d9c514379; Git owns identity. `case.istream-rstream-keywords`
  extended to all 10 executions (ords 0/1/9 added; 2 records, 0 differences,
  zero engine changes). Parity review PASS, no findings.

- Shipped: Draft 4.521 ('event-object-array-core') committed and pushed as
  f7a24357d; Git owns identity. Born-DV `case.event-object-array-core` under
  `event.object-array` (ords 0/1/3/4, 6 records, 0 differences, zero engine
  changes). Parity review PASS (two P3 fixes).


- Shipped: Draft 4.520 ('event-map-properties') committed and pushed as
  c3449869a; Git owns identity. Born-DV `case.event-map-properties` under
  `event.map-core` (4 runtime IDs, 8 records, 0 differences, zero engine
  changes). Parity review PASS (one P3 comment fix).

- Shipped: Draft 4.519 ('epl-other-pattern-event-properties') committed and
  pushed as 0d93d2984; Git owns identity. Born-DV
  `case.epl-other-pattern-event-properties` under `pattern.basic` (4 runtime
  IDs, 6 records, 0 differences, zero engine changes). Parity review PASS
  (one P3 comment fix).


- Shipped: Draft 4.518 ('epl-contained-event-example') committed and pushed
  as 65f16b572; Git owns identity. Born-DV `case.epl-contained-event-example`
  (6 runtime IDs, 33 records, 0 differences). Parity review initial FAIL
  (1 P1 + 3 P3) -> all fixed -> confirmation PASS. Shared core: same-parent
  contained-join constraint on every join path, scalar-as-single-element
  [property] expansion, ContainedParentField Null for parentless rows,
  keyless table select = full scan.

## Current work unit
Active: Draft 4.513 ('context-key-segmented-infra-prioritized').

- Selection: `ContextKeySegmentedInfra.java` ord 0
  `ContextKeySegmentedInfraAggregatedSubquery` (`java-runtime-f297e13be96337235ae0`,
  static `java-46365e0a7205d91894a0`), ord 2 `ContextKeySegmentedInfraCreateIndex`
  (`java-runtime-f49875a427fb00323404`, static `java-4124e1ac4a7995762796`), and
  `ContextKeySegmentedPrioritized.java` `ContextKeySegmentedPrioritized`
  (`java-runtime-3b57cf3ad453555fb0b5`, static `java-fc89ee858ab88751c211`) — three
  unreferenced lifecycle-context executions sharing the init-term partition
  surface. Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`, no flags.
- [x] Shared core: (1) `executeInitiatedTerminatedFireAndForget` — init-term FAF
  iterates live partition descriptors in allocation order, snapshots the
  named-window/table source per partition, evaluates the projection against the
  descriptor's stored context properties; replaces the old InvalidRule rejection
  (obsolete test `TestInitiatedTerminatedContextFireAndForgetIsRejected`
  deleted). (2) `routePartitionInsertLocked` extracted and wired into every
  partition output point of `processInitiatedTerminated`,
  `processPatternInitiatedTerminated`, `processNestedInitiatedTerminated`,
  `processNestedInitiatedParent`; the generic post-dispatch route queue is
  skipped for all lifecycle contexts. (3) `insertWithVariables` broadcasts a
  context-free insert into a lifecycle-contexted window to every live partition
  (Esper context-controller delivery; routed event type never matches the start
  condition so no partition is allocated).
- [x] Parity assets: runner `internal/app/parity/context_key_segmented_infra_prioritized.go`
  (bean widened to the full 20-field SupportBean shape; the create-index
  insert-into routes the whole decoded event because Esper fills unprojected
  window columns with bean defaults — charPrimitive NUL — which a two-column
  projection would re-zero), scenario (5 cases / 38 steps), oracle
  `tools/java-oracle/ContextKeySegmentedInfraPrioritizedScenarioOracle.java` +
  run.sh, run.go/run_test.go wiring.
- [x] Differential replay: Java 9 records / Go 9 records, `passing` / 0
  differences; evidence `context-key-segmented-infra-prioritized.evidence.json`.
- [x] Manifest/roadmap/CHANGELOG: `case.context-key-segmented-infra-prioritized`
  born-DV (3 runtime IDs); `context.partition` goRefs extended. Summary 745
  cases / 371 DV / 1463 DV runtime IDs / unreferenced 567.
- [x] Gates + parity review: `make check` green (parity ~98s, esper ~110s).
  Reviewer ParityReview513 first pass FAIL on 1 P2 + 4 P3 — P2: extending the
  route-queue skip to init-term contexts dropped batches from timer expiry,
  named-window consumer waves, and output.When startBatch; fixed by wiring
  routePartitionInsertLocked into expireContext (lifecycle-gated),
  processPatternContextTime, expireMixedEndPatternsLocked, both nested paths'
  initiation/termination sites, and processNamedWindowContextLocked
  (lifecycle-gated). P3s fixed: eventPrecedence evaluation in the partition
  route, descriptor-property overlay for broadcast inserts, prose corrections
  (max not sum; keyed-prioritized is a keyed context; 5 cases), per-execution
  static IDs in scenario+oracle. Confirmation review: PASS.
- [x] Shipped; Git owns identity. Draft 4.513 committed and pushed as `20522d48b`.

## Current work unit
Active: Draft 4.514 ('context-key-segmented-named-window').

- [x] Contract frozen (prefetched scouts NextJavaContract514 + NextGoSurface514):
 `ContextKeySegmentedNamedWindow.java` ord 0 `ContextKeyedNamedWindowBasic`
 (`java-runtime-18f8400337cdcd1dffd3`, static `java-0554274bf96ee5ab94ce`) +
 ord 3 `ContextKeyedNamedWindowFAF` (`java-runtime-73bbdb8596de168d6d94`,
 static `java-bf2785808c45262f93ce`, flag FIREANDFORGET). Ords 1/2 already
 referenced; ords 4/5 (subquery over context-free window) out of scope.
- [x] Scenario `testdata/parity/context-key-segmented-named-window.json` (2
 cases / 13 steps), oracle
 `tools/java-oracle/ContextKeySegmentedNamedWindowScenarioOracle.java` +
 run.sh (byte-exact EPLs, pinned steps/cases, EXPECTED_RECORDS=3), runner
 `internal/app/parity/context_key_segmented_named_window.go` + run.go wiring.
- [x] Java trace generated (3 records: 1 listener + 2 faf); Go replay 3
 records; `-mode context-key-segmented-named-window-diff` passing / 0
 differences. Zero engine work — all surfaces already existed.
- [x] Test family: passing-evidence, 3 trace mutations, 7 raw-scenario
 mutations, runtime-ID mapping, help listing — all green.
- [x] Manifest: `case.context-key-segmented-named-window` born-DV (2 runtime
 IDs) + `context.partition` mapping/goRefs; summary 746 cases / 372 DV /
 1465 DV runtime IDs / unreferenced 565. Roadmap + CHANGELOG entries added.
- [x] Gates + parity review: `make check` green (parity ~90s, esper ~111s).
  Reviewer ParityReview514 PASS with one P3 (FAF deploy EPLs carried a
  trailing ';' absent from the Java source); fixed in scenario + oracle,
  traces regenerated, confirmation PASS.
- [x] Shipped; Git owns identity. Draft 4.514 committed and pushed as `0bbedbc7d`.

## Current work unit
Active: Draft 4.515 ('context-key-segmented-named-window-subquery').

- [x] Contract frozen (scouts NextJavaContract515 + NextGoSurface515):
 `ContextKeySegmentedNamedWindow.java` ords 4 `ContextKeyedSubqueryNamedWindowIndexUnShared`
 (`java-runtime-7347c7d16d52e5ea0d30`, static `java-b6c806b84e6ac1511ac5`) +
 5 `ContextKeyedSubqueryNamedWindowIndexShared` (`java-runtime-af7bcf071474f57227fb`,
 static `java-79ea11275b4e65d10869`). Correlated scalar subquery over a
 context-FREE window: not partition-scoped; indexshare hint observably
 identical. Go surface expressible today (SubqueryValue+OuterField global
 snapshot; NamedWindowSubqueryIndexSharing for the hint).
- [x] Scenario `testdata/parity/context-key-segmented-named-window-subquery.json`
 (2 cases / 23 steps), oracle
 `tools/java-oracle/ContextKeySegmentedNamedWindowSubqueryScenarioOracle.java`
 + run.sh (EXPECTED_RECORDS=10), runner
 `internal/app/parity/context_key_segmented_named_window_subquery.go` +
 run.go wiring. Java/Go 10 records each, 0 differences. Zero engine work.
- [x] Test family: passing-evidence, 3 trace mutations, 7 raw-scenario
 mutations, runtime-ID mapping, help listing — all green.
- [x] Manifest: `case.context-key-segmented-named-window-subquery` born-DV
 (2 runtime IDs) + `context.partition` mapping/goRefs; summary 747 cases /
 373 DV / 1467 DV runtime IDs / unreferenced 565. Roadmap + CHANGELOG added.
- [x] Gates + parity review: `make check` green (parity ~98s, esper ~110s).
  Reviewer ParityReview515 PASS, no findings.
- [x] Shipped; Git owns identity. Draft 4.515 committed and pushed as `ff186afec`.

## Current work unit
Active: Draft 4.516 ('context-init-term-prioritized').

- Selection: `ContextInitTermPrioritized.java` ord 0
  `ContextInitTermPrioNonOverlappingSubqueryAndInvalid` (`java-runtime-bb247dc87cf118eb8661`,
  static `java-41c13254dc50886c2dd2`) + ord 1 `ContextInitTermPrioAtNowWithSelectedEventEnding`
  (`java-runtime-0c822c80cf402d017d61`, static `java-517175a60c987d2e2397`). Contract frozen by
  NextJavaContract516 + NextGoSurface516; asset contract at `.omp/contract-516.md`.
- [x] Shared core (primary agent, verified by probes):
  (1) `queueStatementRoutesLocked` skip extended to `definition.isTemporal()` and the temporal
  branch of `process` now calls `routePartitionInsertLocked` — contexted `insert into` a
  contexted named window under a cron/daily context previously routed via the generic queue
  with statement-level variables and landed in a bogus window partition (probe: `out` saw
  nothing; now emits one row).
  (2) `validateSubqueryWindowContext` added in `Build` BEFORE insert-into context inheritance —
  a context-free `insert into` whose subquery reads a contexted window is now rejected with
  `ErrorInvalidRule` ("has been declared for context"), matching Java's
  `Failed to validate subquery ... can only be used within the same context`.
  (3) Terminating-event re-initiation NOT needed: Go already delivers the terminating event to
  the old partition for `select *` (no output clause → `OutputNoTermination` → no skip), so
  E1/E2 each emit one row identically to Java's new-partition delivery. Verified by probe.
- [x] Regression check: `go test ./internal/esper -run 'TestContext|TestSubquery|TestNamedWindow|TestRoute|TestFAF'` green (58s).
- [x] Parity assets (AssetWorker512 + primary fix): scenario
  `context-init-term-prioritized.json` (2 cases / 18 steps — ctx+s0 merged into
  one deploy step because non-@public C1 is invisible across Java modules),
  oracle `ContextInitTermPrioritizedScenarioOracle.java` + run.sh, runner
  `context_init_term_prioritized.go`, run.go wiring.
- [x] Differential replay: Java 4 records / Go 4 records, `passing` / 0
  differences; evidence `context-init-term-prioritized.evidence.json`.
  FAF test `context-mismatch` updated to assert Build-time rejection (Java
  compileFAF runs the same context validation as compileDeploy).
- [x] Manifest/roadmap/CHANGELOG: `case.context-init-term-prioritized` born-DV
  (2 runtime IDs) + `context.partition` mapping/goRefs/DV IDs. Summary 748
  cases / 374 DV / 1469 DV runtime IDs / unreferenced 563.
- [x] Gates: `make check` green (parity ~90s, esper ~108s, compat clean).
- [x] Parity review: ParityReview516 initial FAIL (1 P2 + 3 P3) — P2: the
  isTemporal() route-queue skip dropped expire-path and trigger-select routed
  output; fixed by extending expireContext routePerPartition to temporal,
  routing syncTemporalContextLocked termination/expire batches per partition,
  and routing the trigger batch per-partition. P3s fixed: route moved after
  the iterator-only early return; nested-subquery recursion added (symmetric
  direction deliberately NOT applied — contexted statements may read
  context-free windows via projection subqueries, proven by DV'd ord 4/5);
  run_test.go gained the 5-test family and the loader now whitelists
  case-marker fields. Confirmation review: PASS.
- [x] Shipped; Git owns identity. Draft 4.516 committed and pushed as `aef870d9a`.

## Current work unit
Active: Draft 4.517 ('infra-nwtable-context').

- Selection: `InfraNWTableContext.java` ord 0 `InfraContext{namedWindow=true}`
  (`java-runtime-dfaacac6bb82c21d6d47`) + ord 1 `InfraContext{namedWindow=false}`
  (`java-runtime-913c09693fb262b75d75`). Contract frozen by NextJavaContract517 +
  NextGoSurface517; asset contract at `.omp/contract-517.md`.
- [x] Shared core (primary agent, verified by probes — both variants emit the
  exact Java-asserted rows for all six statements):
  (1) `adoptInitiatedTerminatedPartitionsLocked` added at deploy (after
  materializeCategoryContextLocked): a statement deployed into an
  initiated-terminated context AFTER the initiating event now instantiates an
  agent instance per live partition, reusing the partition's original
  allocation ID and contextProperties from the descriptor. Without it the
  statement's partition map stayed empty and `output snapshot when terminated`
  fired nothing on the end event.
  (2) `snapshotAggregateBatch` extended: a named-window aggregate whose own
  state is empty (late deploy or empty window) now replays the window's
  current rows through a fresh aggregate runtime — the same model
  `snapshotAggregateFromTable` already used for tables. Fixes s2..s6
  returning zero/empty for late-deployed window aggregates.
- [x] Parity assets (InfraNWTableCtx517): scenario (2 cases / 28 steps),
  oracle, runner, run.go/run_test.go wiring, 6-test family. Java trace via
  oracle; Go trace + diff `passing` / 0 differences (12 records each).
- [x] Shared core (post-review fixes): `infraLeafEvents` walks wrapper nodes
  (streamWindow/streamFilter/streamContained/streamDerived/streamPattern) to
  the infra leaf so preload/replay reach stored rows; adoption guard requires
  non-overlapping, non-distinct, no start/end pattern, ALL contextKeys
  literal; preload failure skips the partition; `snapshotAggregateFromTable`
  falls through to `snapshotAggregateStateBatch` on empty sources (restores
  Esper's {count=0} empty-group row); `.go.trace.json` checked in;
  `TestRunInfraNWTableContextDiffAcceptsJavaTerminationOrder` exercises the
  s6->s1 canonicalization end-to-end.
- [x] Manifest/roadmap/CHANGELOG updated: `case.infra-nwtable-context` +
  `infra.nwtable-context` born-DV (2 runtime IDs). Summary 671 cases / 298 DV
  / 1093 DV runtime IDs, unreferenced 816.
- [x] Gates: `make check` green (parity ~97s, esper ~107s, compat clean).
- [x] Parity review (ParityReview517): PASS after five rounds — P1 missing
  .go.trace.json; P2s preload-only-zero-delta, empty-source early return,
  adoption for overlapping/keyed contexts, wrapped-input leaf enumeration,
  pattern-end fresh timer, pattern-start unreproducible key; P3s composite-key
  guard, silent preload error, unreachable endPattern block, normalizer test,
  comment accuracy. All fixed; residual P3 scope notes only (non-snapshot
  output, join secondary streams, keyed contexts — pre-existing divergences).

## Current work unit
Active: Draft 4.512 ('context-start-end-trio').

- Selection: `ContextStartEndContextPartitionSelection` (ord 0) +
  `ContextStartEndPrevPriorAndAggregation` (ord 7) +
  `ContextInitTermScheduleFilterResources` (ContextInitTerm ord 10) — 3
  unreferenced, initiated/terminated-context family. Contract frozen via
  NextJavaContract512 + NextGoSurface512.
- [x] Shared core: `ScheduleCountOverall` now counts per-partition
  context-end schedules (terminatedAfter deadline / end-pattern timer)
  once per (context, partition) pair — matches Java's per-agent-instance
  termination callback.
- [x] Go runner `internal/app/parity/context_start_end_trio.go` + run.go
  dispatch + run_test.go 2-test family (3 cases / 25 records).
- [x] Parity assets (Assets512 + primary): oracle
  `tools/java-oracle/ContextStartEndTrioScenarioOracle.java` (fixed:
  context-deployment-id lookup, null initiating properties), run.sh,
  scenario `testdata/parity/context-start-end-trio.json` (3 cases / 53
  steps, generated from the oracle's pinned step keys).
- [x] Differential replay: Java 25 records / Go 25 records, `passing` /
  0 differences. Evidence `context-start-end-trio.evidence.json` + 2-test
  family (passing-evidence + 3 mutations).
- [x] Manifest/roadmap/CHANGELOG: `case.context-start-end-trio` born-DV
  (3 runtime IDs); `context.partition` goRefs + DV IDs extended. Summary
  744 cases / 370 DV / 1460 DV runtime IDs / unreferenced 570.
- [x] Shipped; Git owns identity. Draft 4.512 committed and pushed as `1b0523add`.

## Current work unit
Active: Draft 4.494 ('view-group-closure').

- Selection: `ViewGroup.java` ordinals 7 `ViewGroupInvalid`
  (`java-runtime-a72aa6eebc4ce8d21115`), 8 `ViewGroupLengthWinWeightAvg`
  (`java-runtime-e718af611543b6d40436`), 19 `ViewGroupEscapedPropertyText`
  (`java-runtime-d9b2e762c318369875e5`) — the last three unreferenced
  executions of the file; closes it. (The earlier prefetched candidate
  `ResultSetLocalGroupedSolutionPattern` ord 12 turned out already shipped as
- [x] Java contract freeze (NextJavaContract494b) + Go surface freeze
  (NextGoSurface494b); contract at .omp/contract-494.md. Ord 8 carries
  PERFORMANCE flag -> thin trace (deployed + sent-count markers); ord 7
  probe 4 (merge view) unrepresentable -> pinned-only record.
- [x] Shared core: WeightedAvg emits NaN (not Null) when usable rows exist
  but total weight is 0 (Java weighted_avg contract); `weighted-avg` added
  to groupwin implicit-grouping detection; groupwin key validation +
  null-typed-key rejection in validateNode. `go test ./internal/esper` green.
- [x] Parity assets (ViewGroupAssets494): runner `view_group_closure.go`,
  scenario (3 cases / 18 steps), oracle `ViewGroupClosureScenarioOracle.java`
  + `run-view-group-closure.sh`, run.go/run_test.go wiring. Worker transcription
  clean; shared-core fixes found by replay: `GroupWindowSpec.validate` nil-Inner
  now uses Java's child-views wording; groupwin first-position check added in
  validateNode; WeightedAvg NaN-on-zero-weight; weighted-avg implicit grouping.
- [x] Java trace via oracle (11 records), Go trace, diff `passing` / 0
  differences; evidence `view-group-closure.evidence.json`. The 10k-event
  ord-8 send replays in ~280s (O(group) aggregate eval); the two diff tests
  diff the checked-in Go trace directly to stay inside the package timeout.
- [x] Manifest: `case.view-group-closure` + `view.group-closure` born-DV
  (3 runtime IDs). Summary +1 case/capability, +3 DV runtime IDs/associations,
  unreferenced -3 (630).
- [x] Gates + parity review: `make check` green (parity 78s, esper 114s).
  Reviewer ParityReview494 first pass FAIL on one P2 — WeightedAvg emitted
  ±Inf when totalWeight==0 but weighted!=0; Java emits NaN for every
  sumW==0 (incl. empty/all-unusable). Fixed to `totalWeight==0 -> NaN`,
  three P3s fixed (stale comments, record count, nil-state guard).
  Confirmation review: PASS. Evidence + go.trace regenerated byte-identical.
- [x] Shipped; Git owns identity. Draft 4.494 committed and pushed as `bdeb93e4a`.

## Current work unit
Active: Draft 4.495 ('resultset-output-when-then-closure').

- Selection: `ResultSetOutputLimitCrontabWhen.java` ordinals 9
  `ResultSetOutputWhenThenExpressionSODA` (`java-runtime-c8af5811fef1d7f582d6`),
  10 `ResultSetOutputWhenThenSameVarTwice` (`java-runtime-feee4a26544010fc7fed`),
  13 `ResultSetInvalid` (`java-runtime-54ed0a4e11d7714b8289`) — the last three
  unreferenced executions of the file.
- [x] Contract frozen (JavaContract495 + GoSurface495): ord 9 deploy smoke
  (toEPL unrepresentable), ord 10 same-var-twice output-last-when (engine gap:
  whenPending must reduce to last row), ord 13 eight invalid probes (3 new
  validations + 1 unrepresentable).
- [x] Shared core: `output last when` reduces whenPending to last row per key;
  aggregate-in-when / aggregate-in-then-set / prev-in-when validations added.
  Smoke tests green (4 new in when_valid_test.go).
- [x] Parity assets (Assets495): runner, scenario (3 cases), oracle, run.sh,
  run.go/run_test.go wiring. Java trace via oracle, Go trace, diff `passing` /
  0 differences (15 records each).
- [x] Manifest: `case.resultset-output-limit-crontab-when-closure` +
  `resultset.output-when-then-closure` born-DV (3 runtime IDs). Summary +1
  case/capability, +3 DV runtime IDs/associations, unreferenced -3 (627).
- [x] Gates + parity review: `make check` green (parity 82s, esper 114s).
  Reviewer ParityReview495 first pass FAIL on P2 — appendWhenPending dropped
  outputKeysNew/outputKeysOld, breaking keyed output-last-when; fixed by
  propagating all key slices + clone() copying outputKeysOld. Second pass FAIL
  on P1 — oracle step pins stale after scenario gained a step; fixed
  EXPECTED_STEPS 31→32 + CASE_STEPS. Third pass: PASS. Evidence + traces
  regenerated byte-identical (15 records).
- [x] Shipped; Git owns identity. Draft 4.495 committed and pushed as `98d55664e`.

## Current work unit
Active: Draft 4.502 ('expr-enum-sumof-remainder').

- Selection: `ExprEnumSumOf` ordinals 1 `ExprEnumSumEventsPlus`
  (`java-runtime-8497175e9fc13501285c`, static `java-7443fe2db668140640df`),
  3 `ExprEnumSumScalarStringValue` (`java-runtime-93ec466957ff19bc38a6`,
  static `java-ce9a9b8124ed9b9cda09`), 4 `ExprEnumSumInvalid`
  (`java-runtime-bbe116cdf8ad7e13172f`, static `java-3bdebdfafa650da60077`),
  5 `ExprEnumSumArray` (`java-runtime-b0455a5d1e4b447f34b1`, static
  `java-2c99e8eabfc5cdb0b018`). One file, one enum method, no flags, no
  virtual time, listener-only SupportEvalBuilder assertions + 2 build-error
  probes.
- [x] Contract frozen (JavaContract502 + GoSurface502): all 4 ordinals
  replayable; zero engine change predicted (EnumSum/SumOf + Func1/Func2 UDFs
  + big-number collections already exist). Pitfalls: null-vs-empty
  collection both yield null; null elements/lambda-nulls skipped; result
  type follows input (Integer/Long/Double/BigDecimal/BigInteger); BigDecimal
  scale fidelity; invalid probes pin message prefixes only (JVM FQNs
  diverge).
- [x] Parity assets (Assets502): runner
  `internal/app/parity/expr_enum_sumof_remainder.go`, scenario
  `testdata/parity/expr-enum-sumof-remainder.json` (4 cases / 22 steps),
  oracle `tools/java-oracle/ExprEnumSumOfRemainderScenarioOracle.java` +
  `run-expr-enum-sumof-remainder.sh`, run.go dispatch + 6-test family in
  run_test.go.
- [x] Oracle fix: deploy compiles must use `new CompilerArguments(configuration)`
  (carries compiler-level plug-in single-row functions extractNum/
  extractBigDecimal); `runtime.getRuntimePath()` does not. Precedent:
  ContextHashScenarioOracle. First trace run failed with "Unknown single-row
  function 'extractNum'" before the fix.
- [x] Differential replay: Java 11 records / Go 11 records, `passing` /
  0 differences. Checked-in traces + evidence under testdata/parity/.
- [x] Manifest/roadmap: `case.expr-enum-minmax-sum-avg` extended to
  differential-verified with the 4 new runtime IDs (javaRuntimeIds now 11,
  javaNames +4, goTests +6, `difference` text records remaining partial
  coverage); summary 361 DV / 1411 DV runtime IDs / unreferenced 619.
  Roadmap Draft 4.502 entry added. CHANGELOG intentionally untouched (it
  lags the roadmap; top entry is 4.496).
- [x] Gates + parity review: `make check` exit 0 twice (before and after the
  P3 fix). Independent parity review (ParityReview502): OVERALL PASS, one P3 —
  the sumof-null-lambda probe tolerated either Build outcome and verified
  nothing; FIXED: the probe now requires Go's typed NullLiteral selector to
  build (Build failure = Go regression), header comment corrected. Diff
  re-run passing / 0 differences, Go trace byte-identical.
- [x] N+1 prefetch: ord 12 solution-pattern turned out already DV'd (4.413);
  re-selected `EPLOtherSelectWildcardWAdditional` ords 0/2/3/4/5/6 (6
  unreferenced, wildcard+additional-column family). Java contract frozen by
  NextJavaContract503; Go surface scout NextGoSurface503b running.

## Current work unit
Active: Draft 4.507 ('rowrecog-greedyness-ops').

- Selection: `RowRecogGreedyness` ords 0-2 + `RowRecogOps` ords 5/7/9 (6
  unreferenced, pattern-semantics cluster). No flags, no virtual time.
- [x] Contract frozen (NextJavaContract507 + NextGoSurface507): verbatim
  EPLs, reluctant minimum-binding semantics, 2000-send partition stress,
  ord9 JVM-only pinned as unrepresentable. Predicted zero shared-core —
  confirmed.
- [x] Parity assets (Assets507): oracle
  `tools/java-oracle/RowRecogGreedynessOpsScenarioOracle.java`, run.sh,
  scenario `testdata/parity/rowrecog-greedyness-ops.json` (6 cases / 2054
  steps, 1015 records). Runner `internal/app/parity/rowrecog_greedyness_ops.go`
  + run.go dispatch + 2-test family (primary agent).
- [x] Runner decisions: empty-iterator snapshot omits 'new' (Java oracle
  shape); unrepresentable op emits pinned note; FirstMatch for
  non-all-matches EPLs, AllMatches for ord7.
- [x] Differential replay: Java 1015 records / Go 1015 records, `passing`
  / 0 differences. Zero shared-core changes.
- [x] Manifest/roadmap/CHANGELOG: `case.rowrecog-greedyness-ops` born-DV
  (6 runtime IDs); `case.rowrecog-unlimited-partition` note points at the
  DV case. Summary 739 cases / 365 DV / 1441 DV runtime IDs /
  unreferenced 589.

## Current work unit
Active: Draft 4.506 ('rowrecog-repetition').

- Selection: `RowRecogRepetition` ords 0-5 (6 unreferenced, quantifier
  family). flags=[INVALIDITY] for ord3; no virtual time.
- [x] Contract frozen (NextJavaContract506 + NextGoSurface506): verbatim
  EPLs, ord0/ord1 share one trace (soda = compile path only), ord3 nine
  probes (4 unrepresentable + 5 gated), ord5 62 expansion pairs.
- [x] Parity assets (Assets506): oracle
  `tools/java-oracle/RowRecogRepetitionScenarioOracle.java`, run.sh,
  scenario `testdata/parity/rowrecog-repetition.json` (6 cases / 542 steps,
  127 records). Runner `internal/app/parity/rowrecog_repetition.go` + run.go
  dispatch + 2-test family (primary agent).
- [x] Runner decisions: `compile-text` op + `expectExpansion` added to
  compat.Step/Validate (shared compat surface, not engine); ord5 verified
  via runner-local AST expander mirroring RowRecogPatternExpandUtil
  (Go RowPattern cannot model atom-type+repeat combos like A+{2}); scalar
  vs array measures = PatternEvent vs TagEvents by pattern multiplicity;
  doc-samples use RegisterObjectArray + SendObjectArray positional sends.
- [x] Differential replay: Java 127 records / Go 127 records, `passing` /
  0 differences. Zero shared-core engine changes.
- [x] Manifest/roadmap/CHANGELOG: `case.rowrecog-repetition` born-DV (6
  runtime IDs); summary 738 cases / 364 DV / 1435 DV runtime IDs /
  unreferenced 595.

## Current work unit
Active: Draft 4.505 ('rowrecog-after').

- Selection: `RowRecogAfter` ords 0-5 (6 unreferenced, AFTER MATCH SKIP
  family). No flags, no virtual time, listener+iterator dual assertions.
- [x] Contract frozen (NextJavaContract505 + NextGoSurface505): verbatim
  EPLs incl. irregular whitespace, ord1 listener-suppression pitfall, ord5
  skip-past-last non-overlap contrast. Predicted zero shared-core change —
  confirmed.
- [x] Parity assets (Assets505): oracle
  `tools/java-oracle/RowRecogAfterScenarioOracle.java`, run.sh, scenario
  `testdata/parity/rowrecog-after.json` (6 cases / 102 steps, 47 records).
  Runner `internal/app/parity/rowrecog_after.go` + run.go dispatch + 2-test
  family (primary agent).
- [x] Runner fixes during integration: `.FirstMatch()` required for ord0/ord1
  (MatchRecognize defaults allMatches=true; non-all-matches EPLs emit only
  the first/longest match per start); listener sequence resets per deploy
  cycle (Java attaches a fresh listener per deployment). No engine change.
- [x] Differential replay: Java 47 records / Go 47 records, `passing` /
  0 differences. Checked-in traces + evidence under testdata/parity/.
- [x] Manifest/roadmap/CHANGELOG: `case.rowrecog-after` born-DV (6 runtime
  IDs); `rowrecog.match-recognize` goRefs + `full Java mapping and trace
  parity` removed from remaining. Summary 737 cases / 363 DV / 1429 DV
  runtime IDs / unreferenced 601.

## Current work unit
Active: Draft 4.504 ('event-objectarray-nested').

- Selection: `EventObjectArrayEventNested` ords 0-4 + `EventObjectArrayEventNestedPojo`
  ord 0 (6 unreferenced, object-array nested property family). No flags, no
  virtual time.
- [x] Contract frozen (NextJavaContract504 + NextGoSurface504): verbatim EPLs
  incl. the byte-exact 22-column pojo projection (missing space after `as c2,`),
  per-case Configuration event types, `_bean`/`_long` payload tags, oa-nested
  iterator-only (no listener). Predicted zero shared-core change — confirmed.
- [x] Parity assets (Assets504): oracle
  `tools/java-oracle/EventObjectArrayNestedScenarioOracle.java`, run.sh,
  scenario `testdata/parity/event-objectarray-nested.json` (6 cases / 43
  steps, 12 records). Runner `internal/app/parity/event_objectarray_nested.go`
  + run.go dispatch + 2-test family (primary agent).
- [x] Runner fixes during integration: OA fields declared `any` (strict
  assignability rejects `[]any` for typed slices); `esper.Select` idiom →
  `FromAny(...).Select(...)`; `CoalesceOf[any]` explicit type arg; `nil` →
  `esper.Query{}` return; `Result` has no `Schema()` — use `Event()`/`Row()`;
  oa-nested skips listener (Java asserts iterator only); fragment rendering
  via `GetFragment`/`GetFragments` + `NestedSchema` fallback (`TypeLev0` for
  `p0`); `_bean`/`_long` decode + `map[string]oaNestedLevTwo` render case.
- [x] Differential replay: Java 12 records / Go 12 records, `passing` /
  0 differences. Checked-in traces + evidence under testdata/parity/.
- [x] Manifest/roadmap/CHANGELOG: `case.event-objectarray-nested` born-DV
  (6 runtime IDs); `event.object-array` → differential-verified (runtime IDs
  added, `Java parity trace` removed from remaining). Summary 736 cases /
  362 DV / 1423 DV runtime IDs / unreferenced 607. Roadmap + CHANGELOG
  entries added.
- [x] Gates + parity review: `make check` exit 0 (twice — before and after
  P3 fixes). Independent parity review (ParityReview504): OVERALL PASS, three
  P3s — mutation retargeted to pojo a3 (was mislabeled map-name-nested),
  header comment now notes the mapped('k') -> mapprop('k') adaptation, and
  oaRenderNested disambiguates []any-of-maps as array-of-rows (latent shape
  bug, not exercised by the pinned scenario). Diff re-run passing /
  0 differences, Go trace byte-identical.
- [x] N+1 prefetch: `RowRecogAfter` ords 0-5 (6 unreferenced, AFTER MATCH
  SKIP family). Java contract frozen by NextJavaContract505 (verbatim EPLs,
  listener+iterator dual assertions, ord1 listener-suppression pitfall,
  ord5 skip-past-last non-overlap contrast); Go surface scout NextGoSurface505
  confirms zero shared-core change (all skip strategies, tag measures,
  partition-by, all-matches, order-by, snapshot iterator exist).
- [x] Shipped; Git owns identity. Draft 4.504 committed and pushed as `1f08478c4`.

## Current work unit
Active: Draft 4.503 ('epl-other-wildcard-additional').

- Selection: `EPLOtherSelectWildcardWAdditional` ordinals 0/2/3/4/5/6 (the
  six unreferenced executions; ords 1/7/8 already dispositioned). One file,
  wildcard+additional-column family, no flags, no virtual time.
- [x] Contract frozen (NextJavaContract503 + NextGoSurface503b): verbatim
  EPLs, Pair underlying, join stream-name props, ambiguous shared props
  suppressed, combined-props nested materialization. Predicted zero
  shared-core change — confirmed.
- [x] Oracle fixes: join-no-common gained the s1 where variant (the original
  oracle dropped Java's second deploy cycle); rows() emits `<unreadable>`
  for indexed-only properties (Java get("indexed") throws
  PropertyAccessException inside the listener, which Esper swallows —
  combined-props produced zero records before the fix).
- [x] Runner extended to 9 cases; scenario 9 cases / 22 steps; Java/Go 13
  records each, `passing` / 0 differences. Test family extended to the
  standard six tests.
- [x] Manifest/roadmap: case.epl-other-wildcard-additional javaRuntimeIds
  9, DV IDs 8 (ord 8 stays intentionally-different under its own case);
  summary 361 DV / 1417 DV runtime IDs / unreferenced 613.
- [x] Gates + parity review: `make check` exit 0 (twice — before and after
  P3 fixes). Independent parity review (ParityReview503): OVERALL PASS, two
  P3s — combined-props-nested-drift now drifts a real nested leaf
  (array[0].mapprop['0ma'].value), and CheckedInEvidence upgraded to the full
  sibling convention (DiffTraces, metadata constants, scenario embedding,
  canonical rebuild, fresh replay diff). Confirmation re-check clean.
- [x] N+1 prefetch: `EventObjectArrayEventNested` ords 0-4 +
  EventObjectArrayEventNestedPojo (6 unreferenced, object-array nested
  property family). Java contract frozen by NextJavaContract504; Go surface
  scout NextGoSurface504 running.

## Current work unit
Active: Draft 4.501 ('resultset-outputlimit-simple-none').

- Selection: `ResultSetOutputLimitSimple` ordinals 0-3 (none variants:
  no-having/having x no-join/join). All replayable, no flags, virtual time.
- [x] Contract frozen (JavaContract501-2 + GoSurface501-2): all 4 ordinals
  replayable; none output-limit fully expressible.
- [x] Parity assets (Assets501): runner, scenario (4 cases / 320 steps),
  oracle, run.sh, run.go/run_test.go wiring. Java trace via oracle, Go
  trace, diff `passing` / 0 differences (62 records each).
- [x] Manifest: `case.output-simple-core` extended (+4 net-new DV runtime
  IDs). Summary 735 cases / 360 DV / 1407 DV runtime IDs, unreferenced 623.
- [x] Gates + parity review: strict raw-bytes loader added after first
  `make check` failure (malformed-scenario rejections + test needle fixes);
  test family green. Reviewer ParityReview501-2: PASS with 1 P3 (stale DV
  count here, fixed).

## Current work unit
Active: Draft 4.500 ('resultset-orderby-simple-no-output-invalid').

- Selection: `ResultSetOrderBySimple` ordinals 15-17 (no-output-clause-view,
  no-output-clause-join, invalid). Ords 15-16 replayable; ord 17 build-error
  probes requiring new plan.go validation.
- [x] Contract frozen (JavaContract500 + GoSurface500): ords 15-16
  replayable; ord 17 needs aggregate-order-by validation rule.
- [x] Parity assets (Assets500): runner, scenario (5 cases / 51 steps),
  oracle, run.sh, run.go/run_test.go wiring, plan.go
  `validateOrderByAggregates` rule. Java trace via oracle, Go trace, diff
  `passing` / 0 differences (20 records each).
- [x] Manifest: `case.resultset-orderby-simple` extended (+3 net-new DV
  runtime IDs). Summary 735 cases / 359 DV / 1403 DV runtime IDs,
  unreferenced 623.
- [x] Gates + parity review: `make check` green (parity 84s, esper 110s).
  Reviewer ParityReview500: PASS with 2 P3 findings (pre-existing manifest
  staleness, not blockers).

## Current work unit
Active: Draft 4.499 ('resultset-orderby-simple-join-wildcard').

- Selection: `ResultSetOrderBySimple` ordinals 10-14 (multiple-keys-join,
  simple, simple-join, wildcard, wildcard-join). All replayable, no flags,
  no virtual time.
- [x] Contract frozen (JavaContract499 + GoSurface499): all 5 ordinals
  replayable; join/wildcard order-by fully expressible.
- [x] Parity assets (Assets499): runner, scenario (19 cases / 188 steps),
  oracle, run.sh, run.go/run_test.go wiring. Java trace via oracle, Go
  trace, diff `passing` / 0 differences (19 records each).
- [x] Manifest: `case.resultset-orderby-simple` extended (+4 net-new DV
  runtime IDs; ord 12 already listed). Summary 735 cases / 359 DV / 1400 DV
  runtime IDs, unreferenced 623.
- [x] Gates + parity review: `make check` green (parity 84s, esper 111s).
  Reviewer ParityReview499: PASS with 2 P3 findings (manifest prose
  staleness, both fixed).

## Current work unit
Active: Draft 4.498 ('resultset-orderby-simple-expressions-aliases').

- Selection: `ResultSetOrderBySimple` ordinals 5-9 (expressions,
  aliases-simple, expressions-join, multiple-keys, aliases). All replayable,
  no flags, no virtual time.
- [x] Contract frozen (JavaContract498 + GoSurface498): all 5 ordinals
  replayable; expression/alias/multikey/join order-by fully expressible.
- [x] Parity assets (Assets498): runner, scenario (20 cases / 161 steps),
  oracle, run.sh, run.go/run_test.go wiring. Java trace via oracle, Go
  trace, diff `passing` / 0 differences (26 records each).
- [x] Manifest: `case.resultset-orderby-simple` extended (+4 net-new DV
  runtime IDs; ord 8 already listed). Summary 735 cases / 359 DV / 1396 DV
  runtime IDs, unreferenced 623.
- [x] Gates + parity review: `make check` green (parity 76s, esper 117s).
  Reviewer ParityReview498: PASS with 1 P3 finding (manifest count wording,
  fixed).
## Current work unit
Active: Draft 4.497 ('resultset-orderby-simple-descending-om').

- Selection: `ResultSetOrderBySimple` ordinals 3 `ResultSetDescendingOM`
  (`java-runtime-df8ea61ff025609ce309`) and 4 `ResultSetDescending`
  (`java-runtime-9668909b2b2f00769dab`, variants 2-6). Ord 3 runtime flow
  identical to ord 4 variant 1; OM text/serialization unrepresentable.
- [x] Contract frozen (JavaContract497-2 + GoSurface497-2): all 5 ordinals
  replayable; ord 3 OM assertions unrepresentable; ord 4 needs 5 more
  variants, zero engine work.
- [x] Parity assets (Assets497): runner, scenario (6 cases / 44 steps),
  oracle, run.sh, run.go/run_test.go wiring. Java trace via oracle, Go
  trace, diff `passing` / 0 differences (8 records each).
- [x] Manifest: `case.resultset-orderby-simple` upgraded to
  differential-verified (+2 DV runtime IDs). Summary 735 cases / 358 DV /
  1414 DV runtime IDs, unreferenced 623.
- [x] Gates + parity review: `make check` green (parity 79s, esper 114s).
  Reviewer ParityReview497: PASS with 3 P3-only findings (all fixed:
  manifest notes leading space, missing goTests entries, static-ID
  convention documented).

## Current work unit
Active: Draft 4.496 ('resultset-aggregate-invalid-closure').

- Selection: 4 invalid-form executions across 4 files, all aggregate-function
  compile-time validation (10 probes): `ResultSetAggregateExtInvalid`
  (`java-runtime-98f0ac5299780e2d6656`, single-execution file),
  `ResultSetAggregateInvalid` (SortedMinMaxBy ord 7,
  `java-runtime-09530eec55a704b01c96`), `ResultSetAggregateWindowInvalid`
  (MethodWindow ord 4, `java-runtime-fbbdc48ea2da6d4bc426`),
  `ResultSetAggregateFilterNamedParamInvalid` (FilterNamedParameter ord 20,
  `java-runtime-50935b7efcc1fdced314`).
- [x] Contract frozen (JavaContract496 + GoSurface496): 7 unrepresentable
  (rate toggle, windowcol Method escape, 3 typed-API filter, create-table
  filter), 3 build-error (maxby cross-stream, sorted no-window, correlated
  subquery covered).
- [x] Shared core: sorted requires data window (into-table/create-table
  exempt), min-by/max-by/sorted same-stream criteria check, create-table
  filter unrepresentable. Existing test needed LengthWindow(4) for sorted.
  Smoke tests green (2 new in agg_invalid_test.go).
- [x] Parity assets (Assets496): runner, scenario (4 cases / 17 steps),
  oracle, run.sh, run.go/run_test.go wiring. Oracle import fix (SupportBean_S0/S1
  package). Java trace via oracle, Go trace, diff `passing` / 0 differences
  (11 records each).
- [x] Manifest: `case.resultset-aggregate-invalid-closure` +
  `resultset.aggregate-invalid-closure` born-DV (4 runtime IDs). Summary +1
  case/capability, +4 DV runtime IDs/associations, unreferenced -4 (623).
- [x] Gates + parity review: `make check` green (parity 77s, esper 113s).
  Reviewer ParityReview496: first pass FAIL on 2 P1 + 3 P2 + 2 P3; second
  pass FAIL on 3 P2 + 3 P3; third pass PASS with 4 P3-only findings (all
  fixed post-review: grouped-subquery having, outer-field criteria,
  PLANS.md prose, plain-select aggregate gap documented).

## Previous work units (shipped)

- Shipped: Draft 4.493 ('infra-table-count-min-sketch') committed and pushed as
  2433f0bb1; Git owns identity. New capability `infra.table-count-min-sketch` +
  `case.infra-table-count-min-sketch` born-DV (4 runtime IDs, 57 records, 0
  differences). Parity review initial FAIL (2 P2 + 5 P3) -> all fixed ->
  confirmation PASS. Shared core: CountMinSketchValue counts/lastBump/TopK
  lazy admission+eviction, TableAggDecl TopK/Agent, countMinSketchAdd
  into-table-only, pattern-triggered table reads, restored join-scoped
  subquery validation.


- Shipped: Draft 4.492 ('infra-table-invalid') committed and pushed as a136a2f7f;
  Git owns identity. Parity review PASS after two fix rounds (dead eventType
  check, missing table-misuse guards, over-strict named-window collision,
  lenient group-by check, stale oracle JAVA_FLAGS).
- Draft 4.491 ('infra-table-context') committed and pushed as
  6b410228b; Git owns identity. Parity review PASS after two P2 fixes
  (RegisterTableInModule context-existence check; validateTableContext
  recurses into expression children).
- Draft 4.490 ('infra-table-subquery') committed and pushed as
  50bed7548; Git owns identity. Parity review PASS (P3s precedent-consistent).

- Draft 4.489 ('infra-table-faf-execute-query') committed and pushed as
  da1ab468f; Git owns identity. Parity review PASS (one P3 fixed).

- Draft 4.488 ('infra-table-update-and-index') committed and pushed as
  195a2a472; Git owns identity. Parity review PASS after one P2 fix
  (deploy-time merge unique-key re-check against live table indexes).
- Draft 4.487 ('infra-table-select-enum-multikey') committed and pushed as
  a60220d7b; Git owns identity. Parity review PASS (two P3s fixed).


## Deferred work items

- `ResultSetQueryTypeGroupByWithComputation` + `MixedAccessAggregation`:
  groupby-computation requires engine fix for non-aggregated non-grouped
  column null-out at coarser rollup levels; mixed-access requires window(*)
  event-reference trace normalization. Both investigated via
  `GrpCompMixedJavaContract`/`GrpCompMixedGoSurface` scouts; contracts
  frozen in session transcripts.
- `BoundRollup2Dim` join variant: join+rollup+window-expiry produces extra
  IR-pair rows in Go's new array vs Java's newData-only listener; needs
  deeper engine investigation of grouped-rollup IR-pair row emission.
- `SelectWildcardNoName`: auto-name `subselect_1` has no Go chain-API form
  (approved difference, already documented).
