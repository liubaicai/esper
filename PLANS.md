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

- Shipped: Draft 4.542 ('event-infra-getter-nested-fragment') committed and pushed as 4d497d022; Git owns identity. All 5 executions born-DV; nested-array property[].id always-null fragment quirk pinned.
- Shipped: Draft 4.541 ('event-infra-getter-dynamic') committed and pushed as df006948c; Git owns identity. All 5 getter-dynamic `?` executions born-DV.
- Shipped: Draft 4.540 ('expr-filter-optimizable-lookupable-limited') committed and pushed as ec613de94; Git owns identity. Ords 1/2/7/8 DV + ord 6 intentionally-different (EPL-text disqualify surface).
- Shipped: Draft 4.539 ('epl-other-invalid') committed and pushed as 3d2dd2720; Git owns identity. 2 DV + 2 intentionally-different (EPL-text-only rejection surface).
- Shipped: Draft 4.538 ('infra-named-window-insert-from') committed and pushed as 99440f9f7; Git owns identity. Full file, 7 executions, 30 records each, 0 differences.

## Current work unit

Active: Draft 4.543 ('event-infra-property-dynamic') — assets dispatched.

- Selection: `EventInfraPropertyDynamic*` cluster — all 6 executions across
  6 sibling files, all ord 0; only Nested has no EXCLUDEWHENINSTRUMENTED flag:
  `EventInfraPropertyDynamicSimple` (`java-runtime-9f5b65c0125c70ea8760`),
  `EventInfraPropertyDynamicNested` (`java-runtime-f686347226e7681f0325`),
  `EventInfraPropertyDynamicNestedDeep` (`java-runtime-0285612baa5f5321fae2`),
  `EventInfraPropertyDynamicNestedRootedNonSimple`
  (`java-runtime-a1c916427bb1c85757f8`),
  `EventInfraPropertyDynamicNestedRootedSimple`
  (`java-runtime-4cd646a570fcd69e826a`),
  `EventInfraPropertyDynamicNonSimple`
  (`java-runtime-d6c08cf3d40f13126684`).
- [x] Java contract scout (JavaContract543) + Go surface scout
  (GoSurface543) complete; contract frozen in the Assets543 batch
  context. All 5 mostly representable: dynamic-VALUE properties
  (`any`-typed declared field, runtime type decides sub-property
  existence); `?` root/leaf placement matrix; JSON-provided
  instantiates nulls vs dynamic-schema absent; XML attribute
  navigation; Avro unions; Avro-dynamic unrepresentable where
  AllowDynamicFields needed. Nested has NO flags (only one in
  cluster).
- [x] Assets543 dispatched for scenario/oracle/runner/wiring. Worker
  reported research complete, writing files (runner first).
- [x] Java trace 879 records via run-event-infra-property-dynamic.sh;
  Go replay 879 records; `-diff` passing / 0 differences; evidence +
  both traces checked into testdata/parity/ (flat path). Manifest
  case.event-infra-property-dynamic born-DV (6 runtime IDs) + mapping
  + cap dvRuntimeIds/goRefs/javaRefs; summary recomputed (774 cases /
  397 DV / 1575 DV IDs / 463 unreferenced). Roadmap + CHANGELOG
  entries prepended. `make check` GREEN (parity 96s, esper 118s).
- [x] ParityRev543 dispatched (read-only, same reviewer family).
- [x] ParityRev543: initial FAIL (P1 sixth execution
  EventInfraPropertyDynamicNonSimple orphaned — now covered as 6th
  case 'nonsimple', 879 records each side, 0 differences; P3 stale
  AllowDynamicFields claim removed) -> confirmation PASS; 2 residual
  P3s fixed (dead eipdAvroNestedName deleted; emulation caveat
  documented). `make check` re-verified green on 6-case state.
  Committing + pushing.

Next: Draft 4.544 ('event-infra-property-static') — prefetch only.

- Selection: `EventInfraProperty*` non-dynamic cluster — 7 executions,
  all ord 0, no flags: IndexedKeyExpr (`java-runtime-f5d82c0cf0cf2516d0ef`),
  IndexedRuntimeIndex (`java-runtime-10ce593e8d5ae6a9dd89`),
  MappedIndexed (`java-runtime-b25d2dc5c7eb2c968105`),
  MappedRuntimeKey (`java-runtime-c4f7fe6163eff7d35461`),
  NestedIndexed (`java-runtime-594a57ed26499f1ccd30`),
  NestedNestedEscaped (`java-runtime-0b17042e1ee4d75b7007`),
  NestedSimple (`java-runtime-848063976a9d6485ec20`).
- Scouts dispatched: JavaContract544 (Java oracle) + GoSurface544 (Go
  surface); contract frozen (all 7 representable, harness suffices;
  see agent://JavaContract544 + agent://GoSurface544). Runner file
  name: event_infra_property_non_dynamic.go; scenario
  event-infra-property-non-dynamic.json. Scout noted an EPL
  discrepancy to verify during implementation (IndexedKeyExpr schema
  declares intarray/mapped but one probe references 'indexed'/'dummy').

## Previous work units (shipped)

- Shipped: Draft 4.542 ('event-infra-getter-nested-fragment') committed and
  pushed as 4d497d022; Git owns identity. Parity review PASS after one P3
  fix (epl made optional on xml-mode schema deploy steps so
  evidence-embedded scenario round-trips; Java trace + Go replay 381
  records each, 0 differences).
- Shipped: Draft 4.541 ('event-infra-getter-dynamic') committed and pushed
  as df006948c; Git owns identity.
- Shipped: Draft 4.540 ('expr-filter-optimizable-lookupable-limited')
  committed and pushed as ec613de94; Git owns identity.
- Shipped: Draft 4.539 ('epl-other-invalid') committed and pushed as
  3d2dd2720; Git owns identity.
- Shipped: Draft 4.538 ('infra-named-window-insert-from') committed and
  pushed as 99440f9f7; Git owns identity.

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
