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
Active: Draft 4.588 ('matchuntil ord1 52-case W-harness').

- Contract frozen (JavaContract588 + GoSurface588): ord1
 PatternOp (java-runtime-0383244f373c8a0ffc8b, static
 java-d4cb4534ca508be87595) — PatternTestHarness deploys ALL 52
 EventExpressionCases SIMULTANEOUSLY (S0..S51), advanceTime-
 before-send over the 12-event mixed set (A1..D3, t=1000..12000),
 per-(stmt,event) multiset compareLists, timer legs 40/44/45
 attribute to the upcoming event, leg52 start-fire ignored,
 undeployAll silence check. Go side: ONE DeployPlans deployment
 with 52 plans + per-statement Subscribe buckets; existing
 test-corpus legs map 1:1, only the harness shape is new
 (no engine gaps per GoSurface588).
- After this unit the file is DV-complete except ord6
 (dynamic-bounds subdomain).
- [x] Assets588 delivered + integrated: Java trace 45 records,
  Go replay 45, `-diff` passing / 0 differences; evidence +
  traces checked in. Bucket-sort normalizer matches the harness's
  compareLists multiset contract (per stmt,event).
- [x] Manifest: case +1 DV ID (8/9 — only ord6 dynamic-bounds
  remains). Summary -> 427 DV cases / 1734 DV runtime IDs.
- [x] Gates: `make check` GREEN (exit 0).
- [x] ParityRev588 + JavaContract589 + GoSurface589 dispatched.
- [ ] Review outcome, commit, push.

## Previous work units (shipped)
- Shipped: Draft 4.587 ('matchuntil repeat-use-tags trio')
 committed and pushed as `7702eaa32`; Git owns identity. case
 promoted to DV 7/9. Review PASS after P2 (leg-3 EPL whitespace
 `)-> [2] C`) fixed.
- Shipped: Draft 4.586 ('matchuntil static remainder') committed
 and pushed as `cad3fdb4b`; Git owns identity. case 6/7+ DV;
 review PASS (1 P3).
- Shipped: Draft 4.585 ('matchuntil untimed until-array quad')
  committed and pushed as `92b59784b`; Git owns identity. case
  promoted to differential-verified 4/7; review PASS (clean).
- Shipped: Draft 4.584 ('followed-by ord2 file closer') committed
  and pushed as `5466ae3ae`; Git owns identity. case
  pattern-operator-followed-by CLOSED 10/10; review PASS after P2
  bare-seconds unit fix.
- Shipped: Draft 4.583 ('followed-by W-harness') committed and
  pushed as `151d8327a`; Git owns identity. case 9/10 DV; review
  PASS (clean; LIFO normalization ruled faithful).
- Shipped: Draft 4.582 ('followed-by chain pair') committed and
  pushed as `c19d843a4`; Git owns identity. case 8/10 DV; review
  PASS (2 P3s fixed).
- Shipped: Draft 4.581 ('followed-by timer+not trio') committed and
  pushed as `6e0a8758d`; Git owns identity. case 6/10 DV (4 records,
  0 differences); review PASS after P2 topology fix (Or inside Then)
  + P3 prose sync.

- Shipped: Draft 4.580 ('followed-by RFID trio') committed and
  pushed as `28c0eb996`; Git owns identity. case promoted to
  differential-verified 3/10 (4 records, 0 differences); review PASS
  (clean).
- Shipped: Draft 4.579 ('everydistinct follow-up triplet') committed
  and pushed as `3fc86a883`; Git owns identity. case closed to 16/17
  DV (20 records, 0 differences); review PASS (1 P3).
- Shipped: Draft 4.578 ('everydistinct nested quintet') committed
  and pushed as `3dd21924f`; Git owns identity. case extended to
  13/17 DV (22 records, 0 differences) + ords 5/7 dual association;
  review PASS (2 P3s).
- Shipped: Draft 4.577 ('everydistinct compound-operator quintet')
  committed and pushed as `fdc715fc4`; Git owns identity. case
  extended to 8/17 DV (34 records, 0 differences); review PASS (1 P3).
- Shipped: Draft 4.576 ('everydistinct single-filter expiry')
  committed and pushed as `f6ee9a51e`; Git owns identity. case
  promoted DV 3/17 ords (12 records, 0 differences); review PASS (2
  P3s fixed).
- Shipped: Draft 4.575 ('pattern-complex-property-access') committed
  and pushed as `abdc6f4f7`; Git owns identity. case promoted DV 3/5
  ords (11 records/18 cases, 0 differences); review PASS (2 P3s).
- Shipped: Draft 4.574 ('subquery-within-pattern') committed and
  pushed as `8adab5d3a`; Git owns identity. case promoted DV 4/5
  ords (19 records/9 spellings, 0 differences) + subquery tag-
  propagation + untagged-atom core fixes; review PASS (1 P3).
- Shipped: Draft 4.573 ('view-expression remainder closer') committed
  and pushed as `f0f1149d9`; Git owns identity. Window ord3 DV (17
  records, 0 differences); Invalids intentionally-different; review
  PASS. View-expression DV-complete except compile-only probes.
- Shipped: Draft 4.572 ('expression UDF + Prev quartet + Func3Ctx')
  committed and pushed as `6d9da36a2`; Git owns identity. batch 12/13
  + window 11/13 DV (16 records, 0 differences) + shared-core
  predicate-binding fix (keep->oldest, trigger->arriving, re-eval->
  absent); review FAIL->fixed->PASS (NW expr_batch consumer missed).
- Shipped: Draft 4.571 ('expression variable quartet') committed and
  pushed as `d1bd95178`; Git owns identity. batch 10/13 + window 9/13
  DV (30 records, 0 differences); review PASS (2 P3s fixed).
- Shipped: Draft 4.570 ('view-expression-batch core quartet') committed
  and pushed as `d36d601fc`; Git owns identity. case extended to 8/13
  DV (18 records, 0 differences); review PASS (no findings).
- Shipped: Draft 4.569 ('view-expression-batch aggregate quartet')
  committed and pushed as `a57fecfd6`; Git owns identity. case promoted
  implemented->DV 4/13 (20 records, 0 differences); review PASS (1 P3).
- Shipped: Draft 4.568 ('view-expression-window aggregate quartet')
  committed and pushed as `7acacb6b0`; Git owns identity. case extended
  to 7/13 DV (43 records, 0 differences); review PASS (1 P3).
- Shipped: Draft 4.567 ('view-expression-window trio') committed and
  pushed as `018811630`; Git owns identity. case promoted
  implemented->DV (36 records total, 0 differences); review PASS (1 P3).
- Shipped: Draft 4.566 ('infra-namedwindow-explicit-index') committed
  and pushed as `a60036a4e`; Git owns identity. 2 executions born-DV
  (33 records each, 0 differences) + shared-core unique-index post-
  retention-validation fix; review PASS.
- Shipped: Draft 4.565 ('infra-namedwindow-update-propagation')
  committed and pushed as `d439b2cb5`; Git owns identity. 4 executions
  born-DV (37 records each, 0 differences); review PASS (4 P3s fixed).
- Shipped: Draft 4.564 ('infra-namedwindow-om, file closed') committed
  and pushed as `b345a2b95`; Git owns identity. 3 executions born-DV
  (62 records each, 0 differences); review FAIL->fixed->PASS (P2
  manifest javaStaticIds pinned the deduplicated inventory id).
- Shipped: Draft 4.563 ('infra-nwtable-create-ddl') committed and pushed
  as `d291e4665`; Git owns identity. 3 executions born-DV (16 records
  each, 0 differences); review FAIL->fixed->PASS (P1 mangled staticIds,
  P2 non-byte-exact module EPL pins).
- Shipped: Draft 4.562 ('infra-nwtable-create-index-mrak, file closed')
  committed and pushed as `ac405900a`; Git owns identity. 2 executions
  born-DV (20 records each, 0 differences); umbrella 22/22 dispositioned
  (20 DV + 2 intentionally-different); review PASS (2 P3s fixed).
- Shipped: Draft 4.561 ('infra-nwtable-index-remainder + drop
  disposition') committed and pushed as `9bd01f337`; Git owns identity.
  6 executions born-DV (92 records each, 0 differences) + DropCreate
  intentionally-different.
- Shipped: Draft 4.560 ('infra-nwtable-late-index') committed and pushed
  as `24416543a`; Git owns identity. 4 executions born-DV (16 records
  each, 0 differences); review PASS (P3 rationale wording).
- Shipped: Draft 4.559 ('infra-nwtable-index-faf') committed and pushed as
  `272065ad4`; Git owns identity. 4 executions born-DV (28 records each,
  0 differences); review PASS (clean).
- Shipped: Draft 4.558 ('infra-nwtable-widening') committed and pushed as
  `802991380`; Git owns identity. 4 executions born-DV (36 records each,
  0 differences); review FAIL->fixed->PASS (P1: probes now exercise the
  declared index path — plan.indexPlan is frozen at env.Build, indexes
  declared at creation time + ForSource(0) non-FullScan assertion).
- Shipped: Draft 4.557 ('infra-nwtable-comparative') committed and pushed as
  `253dbe630`; Git owns identity. 2 executions born-DV (2002 records each,
  0 differences); review PASS (P3 complexity wording unified).
- Shipped: Draft 4.556 ('expr-script-threading') committed and pushed as
  `184230e54`; Git owns identity. 3 executions born-DV (13 records each,
  0 differences); also corrected case.expr-script-provider DocSamples
  runtimeId (416f111d->6f79531e); review FAIL->fixed (goTests names).
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
