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
Active: Draft 4.492 ('infra-table-invalid').

- [x] Contract frozen (.omp/contract-492.md) by NextJavaContract492 +
  NextGoSurface492: InfraTableInvalid.java ALL 4 executions
  (InfraInvalidAggMatchSingleFunc ord 0 / InfraInvalidAggMatchMultiFunc
  ord 1 / InfraInvalidAnnotations ord 2 / InfraInvalid ord 3). Runtime IDs
  java-runtime-70e525638c5fbaf37752 / -7c97277eedfa76b7e0cf /
  -fd2a7de8e5606de63b80 / -b9b6435f81135ce15040.
- [x] Shared-core changes (primary agent): `TableAggDecl` extended with
  ParamType/Distinct/Filter/IgnoreNulls/NthSize/RateInterval/EventType +
  `WithTableAggDecl`; `intoTableAggInfo` unwraps filter/distinct wrappers
  and extracts signature details; `validateIntoTableCompatible` checks all
  signature details; `validateIntoTable` checks group-by count/type vs PK
  (plain grouping only), unidirectional joins, requires-aggregation;
  `NewTableDefinition` rejects PK-on-expression and PK-on-event-type;
  `RegisterTableInModule` rejects name collisions vs variables/schemas/
  named-windows; `RecordStream.Window` on a table rejected; `OnRecord
  (FromTable)` trigger input rejected; `RecordStream.MatchRecognize` on a
  table rejected; `RegisterSchema` rejects table-name collision.
  Engine suite green after fixing one test that reused a schema name for
  a table (Java-invalid pattern).
- [x] Assets integrated (TableInvalidAssets492): oracle + run script +
  scenario + runner + run.go wiring. Two asset pin bugs fixed by primary
  agent: `myaggsingle()`/`leaving()` declared renders must be `(*)` (Java's
  ExprAggregateNodeBase prints `*` for zero-param aggs); runner error check
  switched to errors.Is so DuplicateModuleObjectError (ErrorDependency) is
  recognized; Go group-by-count message gained the trailing " group-by
  expressions" (shared-core fix in plan.go); "skip"-pinned probes now omit
  the trace value field like Java.
- [x] Java trace regenerated via run script (md5 0074d825689a4a01c4246a5180ed6d6f,
  104 records); `-mode infra-table-invalid-diff` passing / 0 differences;
  evidence written to testdata/parity/infra-table-invalid.evidence.json.
- [x] Manifest + run_test.go pins + docs: case.infra-table-invalid added
  (4 runtime IDs, born-DV); mapping to trigger.table-named-window; summary
  731 cases / 355 DV / 1377 DV runtime IDs; six-test family in run_test.go
  green; roadmap + CHANGELOG entries prepended.
- [x] Regression fixes after first `make check`: (1) `RegisterSchema`
  table-name collision broke infra-nwtable-on-merge-flow-itv — runner
  registered schema `MyInfraITV` unconditionally though Java only declares
  that event type for the named-window variant; scoped registration to
  `!isTable` (diff still 0). (2) `sorted(expr)` into-table rejection broke
  infra-table-into-table `bound-unbound-sorted-minmaxby` — Java's bound
  projection is zero-arg `sorted()` inheriting the declared key; runner now
  uses `SortedEventsBy(eventValue(), intPrimitive, false)` (sortedValueKey
  wildcard form, diff still 0).
- [x] Independent parity review (ParityReview492): FAIL — 1 P1 + 4 P2 + 5 P3. All fixed:
  P1 `info.eventType` now populated via `intoTableProvidedEventType`/`streamEventTypeName`
  (join-event child or aggregate input stream; alias resolved through schema catalog) so the
  declared `@type` check fires; `window-event-type` probe promoted to verified goSub.
  P2 ignore-nulls message corrected to Java's "provided is ignore nulls" wording.
  P2 guards added: table unidirectional join source ("Tables cannot be marked as
  unidirectional"), update-istream ("Tables cannot be used in an update-istream statement"),
  context declaration ("Tables cannot be used in a context declaration"), pattern atom
  ("Tables cannot be used in pattern filter atoms") — all four probes promoted to verified
  goSub. Write-only and retain are unrepresentable in the fluent API; contract corrected.
  P2 group-by key check tightened to Java's boxed-subtype direction and applied to all
  grouping forms (rollup exemption removed — Java validates unconditionally; the rollup
  engine test was rewritten to a Java-valid 2-pk shape).
  P2 named-window collision check removed from RegisterTableInModule (Java permits it).
  P3 precedence reordered to Java's sequence (requires-aggregation → table lookup →
  group-by → column compat → unidirectional); filter arg included in provided render;
  count-distinct-param-type decl now sets Distinct; duplicated TableAggDecl comment removed
  (facade regenerated); capability rollup extended (goRefs + 4 runtime IDs); probe counts
  corrected to 43/6/5/50 in manifest/CHANGELOG/roadmap/contract; runner pins step.Case;
  scenario javaFlags pinned [INVALIDITY] + oracle script updated.
- [x] Post-fix gates: `make check` exit 0 (parity 80s, esper 110s); infra-table-invalid-diff
  passing / 0 differences; into-table diff passing / 0 differences.
- [x] Confirmation review (ParityReview492, round 2): caught stale oracle JAVA_FLAGS
  constant — fixed to {"INVALIDITY"}, Java trace regenerated byte-identical (md5
  0074d825689a4a01c4246a5180ed6d6f), ExpectContains pinned in all four matchers.
  Round 3: PASS — all 104 probes verified, IDs/traces/evidence/manifest consistent,
  regressions clean.

## Previous work units (shipped)
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
