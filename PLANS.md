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
Active: Draft 4.556 ('expr-script-threading').

- Contract frozen (JavaContract556 + GoSurface556): expr domain remainder —
  filter core/in-set/optimizable/variables/define/dt/enum/clazz all
  referenced. Bundle = EPLScriptExpression ord 1 QuoteEscape + ord 4
  InvalidRegardlessDialect (compile probes) + ExprFilterLargeThreading ord 0
  (pattern+like listener). Excluded: S5/S7 JS+MVEL dialects
  (intentionally-different, no runtime in Go), S6/S9 MVEL no-ops,
  F1-F6 PERFORMANCE-flagged execs.
- [x] Assets556 dispatched (runner expr_script_threading_556.go, scenario
  expr-script-threading-556.json, oracle ExprScriptThreading556ScenarioOracle).
- [x] Assets556 delivered + integrated: Java trace 13 records, Go replay
  13 (compile-ok/unrepresentable/build-error + types/listener), `-diff`
  passing / 0 differences; evidence + traces checked in.
- [x] ID correction: scout swapped ord 4/5 runtime IDs — ground truth is
  ord4 InvalidRegardlessDialect=`416f111d`, ord5 InvalidScriptJS=`0198690c`.
  Fixed scenario/runner/oracle/evidence; also corrected
  case.expr-script-provider javaRuntimeIds (it had `416f111d` under
  DocSamples's name; real DocSamples = `6f79531e`, now referenced).
- [x] Manifest: NEW case.expr-script-threading-556 born-DV (3 IDs);
  summary -> 790 cases / 410 DV / 1635 DV runtime IDs / 399 unreferenced.
- [x] Gates: `make check` GREEN (exit 0).
- [x] ParityRev556 + JavaContract557 + GoSurface557 dispatched.
- [ ] Review outcome, commit, push.

## Previous work units (shipped)
- Shipped: Draft 4.555 ('expr-define-locreport + inlined-class/cache
  dispositions') committed and pushed as `7d788e025`; Git owns identity.
  ExprDefineLambdaLocReport born-DV (2 records each, 0 differences); 3
  intentionally-different dispositions recorded; review PASS, P3 metadata
  pinning fixed.
- Shipped: Draft 4.554 ('expr-enum-remainder') committed and pushed as
  `912f0eb4b`; Git owns identity. 6 executions born-DV (32 records each,
  0 differences); review PASS, P3 doc nits + must-succeed probe tolerance
  fixed.
- Shipped: Draft 4.553 ('expr-dt-tail') committed and pushed as
  `680e7d76f`; Git owns identity. 5 executions born-DV (57 records, 0
  differences); review FAIL -> fixed -> confirm PASS (doc-set-month
  0->1-based arg, DEFAULT-leg exclusion docs + mutation needles).



- Shipped: Draft 4.552 ('expr-dt-remainder') committed and pushed as
  0302fcd36; Git owns identity. 6 executions born-DV (22 records, 0
  differences); review FAIL -> fixed -> confirm PASS (Microseconds
  join, Go-replay fixture, manifest goTests).


- Shipped: Draft 4.551 ('expr-dt-format') committed and pushed as
  701614c56; Git owns identity. ExprDTFormat 2/2 born-DV (6 records,
  0 differences); review FAIL -> fixed -> confirm PASS (P2
  getDateInstance padding + literal injection, P3 run-lengths/docs).


- Shipped: Draft 4.550 ('expr-dt-set-nested') committed and pushed as
  03e5bc6f1; Git owns identity. ExprDTSet 2/2 + ExprDTNested 1/1
  born-DV (8 records, 0 differences); review PASS; year-7 Julian
  -61933561200000 byte-exact on the hybrid model.


- Shipped: Draft 4.549 ('expr-dt-with-minmax') committed and pushed as
  f1c6e7599; Git owns identity. 4 executions born-DV (8 records each,
  0 differences); review PASS + confirmation PASS — real shared-core
  fix: hybrid GregorianCalendar (inverse-JDN decompose + forward JDN
  recompose + Julian week grid) across all calop applies;
  set('week') accepted.




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
