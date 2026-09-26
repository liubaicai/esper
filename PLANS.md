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
Active: Draft 4.548 ('event-avro-hook').

- Contract frozen (JavaContract548 + GoSurface548): EventAvroHook.java
  4/4 unreferenced, static java-281ee8b5adc379b62b2c, one harness
  (TestSuiteEventAvroWConfig). Singleton cluster rejected (4 files /
  disjoint surfaces / AllowDynamicFields + inherits unrepresentable).
  ord0 SimpleWriteablePropertyCoerce `java-runtime-34ec4dc8abe4f2443930`:
  (a) tryInvalidCompile ZonedDateTime→CharSequence msg oracle-only
  (intentionally-different error kind) + (b) LDT→ISO string via
  explicit Func1 UDF, exact-match (now captured pre-send).
  ord1 SchemaFromClass `-c80f13d8b1a53d4ff458`: UDF makeLocalDateTime
  (now() INSIDE call — shape-only: schema JSON +
  avroToJson isodate length>10, never exact string).
  ord2 Populate `-20190c427227521c092b` STATICHOOK: Func1
  SupportBean→*AvroRecord + InsertInto into predeclared
  MyEventPopulate; flat avroToJson `{"sb":{"theString":"E1",...}}`.
  ord3 NamedWindowPropertyAssignment `-85c9c44d4baae883febe`
  STATICHOOK: create window + on-update set sb=thebean; union-null
  seed; DOUBLE-NESTED `{"sb":{"SupportBeanSchema":{...}}}` JSON.
- [x] Assets548 delivered + integrated: Java trace 14 records, Go
  replay 14, `-diff` passing / 0 differences; evidence + traces
  checked in. Manifest case.event-avro-hook born-DV →
  event.avro-record (779 cases / 402 DV / 1605 DV IDs / 433
  unreferenced / caps 126-125-50-11); capability gained
  `differential-verified` in verification (precedent
  event.contained); summary counters recomputed (validator caught
  stale counts); roadmap + CHANGELOG prepended; `make check` GREEN.
- [x] ParityRev548 + JavaContract549 + GoSurface549 dispatched.
- [ ] Review outcome, commit, push.


## Previous work units (shipped)

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
