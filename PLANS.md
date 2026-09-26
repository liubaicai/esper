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
- On interruption, leave the worktree state, exact failing command, and the
  next safe step.
- Record validation and outcome before the semantic commit. Never edit this
  file to claim results that have not run.

## Outcome

Continue the fixed-commit Esper 9.0.0 to Go migration until the final
acceptance criteria in `docs/esper-go-port-quality-strategy.md` all pass.
Progress is measured by verified work units and manifest evidence, not agent
activity or a single coverage percentage.

- Shipped: Draft 4.544 ('event-infra-property-non-dynamic') committed; Git owns identity. All 7 executions born-DV (824 records each, 0 differences); review PASS, one P3 (dead identity wrapper) fixed.
- Shipped: Draft 4.543 ('event-infra-property-dynamic') committed and pushed as 8ddf1704a; Git owns identity. All 6 executions born-DV (879 records each, 0 differences); review initial FAIL (orphaned NonSimple) -> covered + confirmation PASS.
- Shipped: Draft 4.542 ('event-infra-getter-nested-fragment') committed and pushed as 4d497d022; Git owns identity. All 5 executions born-DV.
- Shipped: Draft 4.541 ('event-infra-getter-dynamic') committed and pushed as df006948c; Git owns identity. All 5 getter-dynamic `?` executions born-DV.
- Shipped: Draft 4.540 ('expr-filter-optimizable-lookupable-limited') committed and pushed as ec613de94; Git owns identity.
- Shipped: Draft 4.539 ('epl-other-invalid') committed and pushed as 3d2dd2720; Git owns identity.

## Current work unit
Active: Draft 4.549 ('expr-dt-with-minmax').

- Contract frozen (JavaContract549 + GoSurface549): ExprDTWithMax
  (ords 0-1) + ExprDTWithMin (ords 0-1), 4 executions, 2 files,
  expr/datetime — the largest verified single-domain remainder
  (~20 unreferenced/13 files). Runtime java-runtime-097a5db3742ae25c3b0b
  / -c6bba43af400a7375075 / -3f79b4bf903cca74fe6a / -8ca773479bf038c94597;
  static java-d4afe3864ba41bfd7918 / java-d87f60ced41e2a52719f /
  java-2224592eef4aca9b4e7b / java-f3bf884f776301d91900; flags [] all.
  SupportDateTime bean (longdate/utildate/caldate/localdate/zoneddate);
  'week' clamps keep day-of-week (WEEK_OF_YEAR); 'year' hits
  GregorianCalendar actual-max 292278994 → int64-wrapped millis
  9223372030035600000, replayed raw.
- [x] Shared core (primary): dateTimeCalWeek + dateTimeCalOpWithMinMax
  + DateTimeWithMax/DateTimeWithMin[V int64|time.Time] in
  expr_dt_calops.go; hybrid GregorianCalendar model
  (dateTimeJavaDecompose inverse-JDN + dateTimeJavaToMillis forward-JDN
  + Julian week grid) applied to ALL four applies (set/withDate/
  withTime/withMinMax); 'week' accepted by the shared resolver —
  set('week',n) matches Java WEEK_OF_YEAR (needed by the next unit's
  ExprDTSet contract); facade regenerated; all pinned values
  JDK-verified incl. pre-cutover -14831121600000 / -14800276800000 and
  wrapped year-max millis; TestDateTimeWithMinMaxTransforms added.
- [x] Assets549 dispatched (runner expr_dt_with_minmax_549.go, scenario
- [x] Assets549 delivered + integrated: Java trace 8 records, Go
  replay 8, `-diff` passing / 0 differences; evidence + traces
  checked in. Shared-core fix found by oracle verification:
  withMin('year') needed Julian-day-number epoch-millis
  (dateTimeJavaToMillis, JDN*86400000-210866803200000+offset);
  confirmed -62122863537997 byte-exact. Manifest
  case.expr-dt-with-minmax born-DV → expr.core (780 cases / 403 DV /
  1609 DV IDs / 429 unreferenced); `make check` GREEN.
- [x] ParityRev549 + JavaContract550 + GoSurface550 dispatched.
- [ ] Review outcome, commit, push.



## Previous work units (shipped)

- Shipped: Draft 4.548 ('event-avro-hook') committed and pushed as
  9c98f57cd; Git owns identity. EventAvroHook 4/4 born-DV (14 records
  each, 0 differences); review PASS, two P3s fixed (build-error kind
  check, schema record-name assertion); event.avro-record gained
  differential-verified.

- Shipped: Draft 4.547 ('event-map-nested') committed and pushed as
  88ee76ee4; Git owns identity. EventMapNested 4/4 born-DV (6 records
  each, 0 differences); review PASS, two P3s fixed (dead statements
  map, overstated deploy doc).

- Shipped: Draft 4.546 ('event-render') committed and pushed as
  a13138036; Git owns identity. All 7 EventRender* executions born-DV
  (37 records each, 0 differences); review PASS, two P3s fixed (null-key
  send comment, misleading mutation-subtest names).




- Shipped: Draft 4.545 ('event-infra-contained-render-sender-supertype')
  committed and pushed as 8c07533b2; Git owns identity. All 8 executions
  born-DV (235 records each, 0 differences); review PASS, three P3s fixed
  (header wording, dead var, manifest scope note).

- Shipped: Draft 4.544 ('event-infra-property-non-dynamic') committed
  and pushed as 756f32910; Git owns identity. Parity review PASS, one
  P3 (dead identity wrapper) fixed. Java/Go 824 records each, 0
  differences; manifest 775 cases / 398 DV / 1582 DV IDs / 456
  unreferenced.
- Shipped: Draft 4.543 ('event-infra-property-dynamic') committed and
  pushed as 8ddf1704a; Git owns identity. Parity review initial FAIL
  (P1 orphaned sixth execution NonSimple -> covered as 6th case; P3
  stale AllowDynamicFields claim removed) -> confirmation PASS with 2
  residual P3s fixed. Java/Go 879 records each, 0 differences.
- Shipped: Draft 4.542 ('event-infra-getter-nested-fragment') committed
  and pushed as 4d497d022; Git owns identity. PASS after one P3 fix.
- Shipped: Draft 4.541 ('event-infra-getter-dynamic') committed and
  pushed as df006948c; Git owns identity.
- Shipped: Draft 4.540 ('expr-filter-optimizable-lookupable-limited')
  committed and pushed as ec613de94; Git owns identity.
- Shipped: Draft 4.539 ('epl-other-invalid') committed and pushed as
  3d2dd2720; Git owns identity.

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
