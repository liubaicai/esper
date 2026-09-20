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
Active: Draft 4.489 ('infra-table-faf-execute-query').

- [x] Contract frozen (.omp/contract-489.md) by NextJavaContract489 +
  NextGoSurface489: InfraTableFAFExecuteQuery.java ALL 4 executions —
  InfraFAFInsert (ord 0, unkeyed table + empty FAF result + ordered
  iterator), InfraFAFDelete (ord 1, delete-all + iteratorCount 10->0),
  InfraFAFUpdate (ord 2, update-all + @Name != table name), InfraFAFSelect
  (ord 3, select-star FAF result array). Runtime IDs
  java-runtime-a79e19dc5f135bb8e628 / -a68109b2bb91de4ce1cd /
  -196ff792f0f739c8d97c / -b995c40f3c052bcc277e. Fully asset-only.
- [x] Assets (TableFAFAssets489): oracle + run script + scenario
  (4 cases / 44 steps / 12 records) + runner + run.go wiring. Java trace
  regenerated; `-mode infra-table-faf-execute-query-diff` passing / 0
  differences; evidence + both traces checked in.
- [x] Manifest updated: new born-DV case.infra-table-faf-execute-query
  (728 cases / 352 DV / 1366 DV runtime IDs); capability
  trigger.table-named-window DV list + goRefs + mapping extended.
- [x] run_test.go: six pinned tests all green.
- [x] Independent parity review (ParityReview489): PASS, one P3 fixed
  (undeployAll now clears s.snapshots to mirror the oracle's
  statements.clear()).
- [x] Full local gates GREEN: make check exit 0.

## Previous work units (shipped)
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
