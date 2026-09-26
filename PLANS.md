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

Active: Draft 4.544 ('event-infra-property-non-dynamic').

- Selection: `EventInfraProperty*` non-dynamic cluster — 7 executions,
  all ord 0, no flags: IndexedKeyExpr (`java-runtime-f5d82c0cf0cf2516d0ef`,
  static `java-461ebcf502cbdf9ba15c`), IndexedRuntimeIndex
  (`java-runtime-10ce593e8d5ae6a9dd89`, static `java-ed0cc8c7b0867c0cd502`),
  MappedIndexed (`java-runtime-b25d2dc5c7eb2c968105`, static
  `java-287c4bf86e9464e7b595`), MappedRuntimeKey
  (`java-runtime-c4f7fe6163eff7d35461`, static `java-899877f666cfc36adcb1`),
  NestedIndexed (`java-runtime-594a57ed26499f1ccd30`, static
  `java-ff678fbbd673309b12b2`), NestedNestedEscaped
  (`java-runtime-0b17042e1ee4d75b7007`, static `java-2d629e45d1948a6770ee`),
  NestedSimple (`java-runtime-848063976a9d6485ec20`, static
  `java-eb62cc47423e42ec7671`).
- [x] Java contract scout (JavaContract544) + Go surface scout
  (GoSurface544) complete; contract frozen. All 7 representable, no
  unrepresentable spots, existing oracle harness suffices. Per-execution
  byte-exact EPL/payload/assertion table at agent://JavaContract544;
  surface map + file list at agent://GoSurface544 (runner file:
  internal/app/parity/event_infra_property_non_dynamic.go; scenario:
  testdata/parity/event-infra-property-non-dynamic.json).
- [x] Assets544 delivered (after one infra-crash resume): runner 3206
  lines, scenario JSON, oracle + run.sh, run.go/run_test.go wiring.
  Java trace 824 records, Go replay 824, `-diff` passing / 0
  differences; evidence + traces checked in; manifest
  case.event-infra-property-non-dynamic born-DV (775 cases / 398 DV /
  1582 DV IDs / 456 unreferenced); roadmap + CHANGELOG prepended;
  `make check` GREEN (parity 97s, esper 119s).
- [x] ParityRev544 + JavaContract545 + GoSurface545 dispatched.
- [ ] Review outcome, commit, push.

Next: Draft 4.545 — prefetch only.

- Candidate: `EventInfraContained*` (4) + EventInfraEventRenderer /
  EventInfraEventSender / EventInfraManufacturer / EventInfraSuperType
  leftovers; scouts must first drop any execution already covered by
  case.epl-contained-event-example (4.518). Contract freeze pending.

## Previous work units (shipped)

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
