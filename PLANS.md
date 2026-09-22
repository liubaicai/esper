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
