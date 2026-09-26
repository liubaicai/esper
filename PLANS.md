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
Active: Draft 4.545 ('event-infra-contained-renderer-sender-supertype').

- Contract frozen (JavaContract545 + GoSurface545): all 8 candidates
  genuinely unreferenced — ContainedSimple/Nested/NestedArray/
  IndexedWithIndex (`java-runtime-6de635c8b30a4103c24c` /
  `-7fbe252dc4d607cf0da6` / `-597b6eca244190805083` /
  `-d0c21881fb79f2ca6b4a`, flags []), EventRenderer
  (`java-runtime-788241891a0cf2f7b34c`, flags []), EventSender
  (`java-runtime-87613a44bc6e8ae3ffa1`, OBSERVEROPS), Manufacturer
  (`java-runtime-70823aef36342bc74b8b`, STATICHOOK, forge API
  unrepresentable — observable construct-and-assert or
  unrepresentable markers), SuperType
  (`java-runtime-c176a2422bef1680520b`, OBSERVEROPS, WithSchemaParent
  dispatch matrix). No overlap with case.epl-contained-event-example.
- [x] Assets545 dispatched (runner event_infra_contained_545.go +
  event_infra_sender_supertype_545.go, scenario event-infra-545.json,
  oracle EventInfra545ScenarioOracle.java, mode event-infra-545).
- [x] Integrated: Java trace 235 records, Go replay 235, `-diff`
  passing / 0 differences; evidence + traces checked in. Manifest
  case.event-infra-contained-render-sender-supertype born-DV mapped to
  event.contained + event.property-access-render (776 cases / 399 DV /
  1590 DV IDs / 448 unreferenced); roadmap + CHANGELOG prepended;
  worker added missing 3 evidence tests on request — all 6 pass;
  `make check` GREEN (parity 99s, esper 118s).
- [x] ParityRev545 + JavaContract546 + GoSurface546 dispatched
  (4.546 candidate: EventRender* family, 7 executions).
- [ ] Review outcome, commit, push.

## Previous work units (shipped)

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
