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
Active: Draft 4.546 ('event-render').

- Contract frozen (JavaContract546 + GoSurface546): 7 unreferenced
  EventRender* executions — EventRender ord0 CustomRenderer
  (`java-runtime-26bb69572227aa67230c`, SERDEREQUIRED,
  unrepresentable: needs JSONRenderingOptions.setRenderer hook),
  ord1 ObjectArray (`-eacece93880bd4448b7d`, representable:
  RenderJSON/XML title MyEvent, explicit-null props, order
  p0,p1,p3,p4,p2), ord2 POJOMap (`-6af1376411de38cffefe`,
  representable: bean + map field, JSON/XML/XML-attr);
  EventRenderJSON ord2 EmptyMap (`-96f1450787b11db671fd`,
  representable), ord3 Enquote (`-8cce94662734d4052c0e`,
  unrepresentable: pure OutputValueRendererJSONString.enquote loop);
  EventRenderXML ord2 SQLDate (`-f47d81ef0d1a48b66476`,
  representable w/ risk: pin xmlScalar 2010-01-31 not RFC3339),
  ord3 Enquote (`-539c60ae3344b3001b44`, unrepresentable:
  OutputValueRendererXMLString.xmlEncode loop). Global rule:
  removeNewline collapses ≥2 whitespace runs; XML always emits the
  `<?xml ...?>` header; renderJSON/XML first arg is a TITLE, not the
  event-type name.
- [x] Assets546 delivered (runner event_render_546.go, scenario
  event-render-546.json, oracle EventRender546ScenarioOracle.java,
  mode event-render-546, 6-test family). Two field-level honest
  unrepresentable pins added beyond the contract: OA XML + pojo-map
  attr-XML null-rendering divergences.
- [x] Integrated: Java trace 37 records, Go replay 37, `-diff`
  passing / 0 differences; evidence + traces checked in. Manifest
  case.event-render born-DV → event.property-access-render (777 cases
  / 400 DV / 1597 DV IDs / 441 unreferenced); roadmap + CHANGELOG
  prepended; `make check` GREEN (parity 99s, esper 116s).
- [x] ParityRev546 + JavaContract547 + GoSurface547 dispatched
  (4.547 candidate: next event file — EventMapNested or EventAvro).
- [ ] Review outcome, commit, push.


## Previous work units (shipped)

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
