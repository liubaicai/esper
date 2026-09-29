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








## Previous work units (shipped)

- Shipped: Draft 4.592 (timer-interval W-harness) committed and pushed as
 df78702b7 + plan marker d2dabe48e.

## Current work unit
Active: Draft 4.594 ('timer:within 34-leg W-harness — PatternGuardTimerWithin ord0').

- [x] Contract frozen (agents JavaContract594b + GoSurface594b): ord0 PatternOp,
 java-runtime-bb8113cb979826cff927, static java-0dec801a426fed297402 /
 static-manifest java-811b1a1fcf834ee01db8, no flags, Java commit 9e1b9f1cc9117f.
 34 legs S0..S33 over EventSetOne A1@1s..D3@12s (advance-before-send), 64 expected
 Java assertion rows collapsing to 47 listener invocations (records); bucket totals
 B1=11 B2=8 D1=12 D2=4 B3=18 D3=9; silent legs S0,S2,S5,S6,S17,S20,S21,S24,S27,S28,
 S33. SODA leg S3 = timer:within(10.001d) == S4's 10001ms. Exclusive deadline: event
 at exactly arm+P loses to the quit callback.
 Go fluent mapping: X.Within(d)=`X where within(d)`; X.Every().Within(d)=
 `(every X) where within`; X.Within(d).Every()=`every (X where within)`;
 X.Every().Within(d).Every()=`every ((every X) where within)`.
 Key engine semantics verified: guard-expiry respawn under every (restartable
 evaluateFalse), and-side expiry kills branch, or survives a side's expiry,
 nested-every accumulation (S11/S12 doubling 1,2,4), and-side completed-partial
 retention across later sends (S18/S29 I3 {B3,D2}).
- [x] Assets: scenario `testdata/parity/pattern-guard-timerwithin-wharness-594.json`
 (34 epls, 15 steps, 12 advance-before-send sends + deploy-all + undeploy-all);
 runner `internal/app/parity/pattern_guard_timerwithin_wharness_594.go` + run.go
 wiring; six-test file `pattern_guard_timerwithin_wharness_594_test.go` (521/527
 expiry variants plus artifacts/ID/malformed probes). Oracle assets by agent
 OracleAssets594: `tools/java-oracle/PatternGuardTimerWithinWHarness594ScenarioOracle.java`
 + `run-pattern-guard-timerwithin-wharness-594.sh` (ATOMS byte-verified vs both Go
 const and Java ord0 source; SODA leg asserted via toEPL). Contract deviation
 verified: EXPECTED_RECORDS=47 per-invocation records, not 64 per-add rows.
- [x] **Engine fix (shared core, internal/esper/runtime.go)**: expired `within`
 guards now disarm (quit path) instead of post-firing stale rows, and restartable
 evaluateFalse respawns `every` at the expiry-callback instant
 (EvalGuardStateNode/EvalEveryStateNode parity). Two tests that pinned the old
 buggy semantics were re-based on the Java contract:
 `TestPatternIndependentWithinGuardsComposeWithEveryAndMatchesEsper` (now expects
 the {B@10s,D@13s} row Java emits after every respawns the and at the
 expiry-callback instant t=10s) and
 `TestPatternWithinExpressionUsesDeploymentParameter` (permanent death of a
 non-every within at deadline; B2 emits nothing).
 Post-fix regression found by `make check`: dropping the `continue` after the
 `transition.complete` block in `advanceContextPattern` double-admitted
 completed-but-active transitions into `nextActive` (duplicate context
 partitions; three `context_test.go` failures + the context-init-term diff
 test). Restored the original control flow — completed transitions `continue`
 after the complete-block, satisfied-terminal ones set `terminal` there — while
 the quit/disarm semantics stay in `advancePatternNodeTrigger`. All failures
- [x] Gates/review: `make check` green (parity 154s, internal/esper 120s) after
 the `continue` restoration; independent review (agent ParityReview594) PASS on
 areas A-G, two P3s fixed (dead `terminal` branch removed at runtime.go; stale
 comment arithmetic corrected — Java dispatches due callbacks at the advance
 target, so the respawn arms at t=10s with 12001/16001 deadlines) plus one DV
 list-consistency add (`bb8113cb` on case.pattern-every). Post-P3 targeted tests
 re-run green; diff re-verified `passing`/[].
- [x] Java trace 47 records via oracle (commit 9e1b9f1cc9117f); normalized
 zero-diff compare `-mode pattern-guard-timerwithin-wharness-594-diff` -> status
 `passing`, differences `[]`; evidence
 `testdata/parity/pattern-guard-timerwithin-wharness-594.evidence.json`.
- Shipped; Git owns identity. Draft 4.594 committed and pushed as `d9e0ed92b`;
 review PASS (2 P3s fixed: dead terminal branch, stale respawn arithmetic).

## Current work unit
Active: Draft 4.595 ('timer:within remainder — PatternGuardTimerWithin ords 1-6').

- [x] Contract frozen (agents JavaContract595 + GoSurface595, read-only):
 PatternGuardTimerWithin ords 1-6, all static `java-0dec801a426fed297402`,
 flags [], commit 9e1b9f1cc9117f. Per-ord runtime IDs: ord1 PatternInterval10Min
 `java-runtime-f0649272cfb528ce731b` (within(93784005ms), tryAssertion fire@0,
 fire@93784004, silent@93784005); ord2 PatternInterval10MinVariable
 `java-runtime-2fe5370a7c1599bfb424` (suite variables D/H/M/S/MS=1..5, same
 schedule; eplToModel round-trip = approved difference); ord3
 PatternIntervalPrepared `java-runtime-66d65309509fac58bbfc` (5 positional `?`
 params -> Parameter + DeployWithParameters); ord4 PatternWithinFromExpression
 `java-runtime-36ef6f15f14a0e86ab93` (`a=SB -> (every b=SB) where
 within(a.intPrimitive seconds)` event-correlated deadline: E1(3)@0 arms 3000,
 E2@2000/E3@2999 emit {id}, expiry@3000 silent); ord5
 PatternPatternNotFollowedBy `java-runtime-6a5e8128ae184e8a7249`
 (`every(SB -> (SMDB where within(5s)))`: E1/E2 branches die @6000, every
 respawns during advance, E4 branch + E5 match -> ONE empty-payload emission);
 ord6 PatternWithinMayMaxMonthScoped `java-runtime-34555c4a9823a346d710`
 (two rounds: within(1 month) then withinmax(1 month,10); arm 2002-02-01 09:00,
 fire E1 + E2 at 03-01 09:00-1ms, silent at boundary).
- GoSurface595: ALL six expressible with existing API — Within/WithinExpr/
 WithinCalendar/WithinOrMaxCalendar, DurationSum+VariableRef, Parameter+
 DeployWithParameters, tag-correlated duration (patternDurationDeadline).
 No engine changes expected: pure asset unit (594-style runner clone).
- [x] Assets: scenario `testdata/parity/pattern-guard-timerwithin-forms-595.json`
 (6 cases / 43 steps, deploy steps may carry `at` = advance-before-deploy,
 ord 6 only), runner `internal/app/parity/pattern_guard_timerwithin_forms_595.go`
 + run.go wiring, six-test family `pattern_guard_timerwithin_forms_595_test.go`
 (replay pin 13 records, 7 raw mutations, pinned artifacts, runtime mapping,
 byte-exact EPLs, arm-order) all green.
- [x] **Runner fixes found by replay** (Go engine correct — throwaway repros
 proved 1-row fire + boundary silence on a shared stream):
 (1) ord4's two `esper.From[bean](env,"SupportBean")` calls duplicated the
 event feed → doubled `{id}` rows; fixed by sharing one stream for a and b;
 (2) ord6 deploys must run on the already-advanced clock (Java calls
 sendCurrentTime BEFORE compileDeploy) — deploy steps now pin an optional
 `at` the runner advances to before compiling; without it the `within` arms
 at epoch 0 and dies before 2002 while `withinmax` mis-fires through the
 backward replay.
- [x] Oracle + differential evidence: Java oracle authored by
 OracleAssets595 (`tools/java-oracle/PatternGuardTimerWithinForms595ScenarioOracle.java`
 + run script); Java trace 13 records (java 17.0.20), Go trace 13,
 `-mode pattern-guard-timerwithin-forms-595-diff` -> status `passing`,
 differences []; evidence
 `testdata/parity/pattern-guard-timerwithin-forms-595.evidence.json`.
- [x] Manifest/roadmap/CHANGELOG: `case.pattern-every` +6 DV runtime IDs
 (ord1-6), +6 goTests, +1 evidence path, Draft 4.595 note appended;
 summary 801/430/1753/386 consistent; roadmap + CHANGELOG entries.
- [x] Gates + review: `make check` exit 0. Parity review PASS
 (agent ParityReview595): 5 P3s — P3.1 variables now registered as
 float64 to mirror Java double.class; P3.2 observation/oracle/scenario
 text corrected to DeployWithPositionalParameters(1..5) (no
 BindPositionalParameters call); P3.3 test comment mechanism fixed;
 P3.4 left (non-string-field coverage belongs to the differential
 compare, not the row-type pin); P3.5 HTML-escape artifacts removed
- Shipped; Git owns identity. Draft 4.595 committed and pushed as `72d0eb91e`;
 parity review PASS (5 P3s, four fixed; P3.4 intentionally left).

## Current work unit
Active: Draft 4.596 ('infra-nwtable-join-and-select-delete').

- Unit: 4 executions across two sibling oracle files, same
  `infra.nwtable` store surface (namedWindow={true,false} pairs):
  - `InfraNWTableJoin` ords 0/1 `InfraNWTableJoinSimple`
    (`java-runtime-9aad0c9a0b81e251f6d4` /
    `java-runtime-333f1a440da03d4b266a`, static
    `java-675ca69dbc976b4c4448`): continuous join `from MyInfra as ce,
    SupportBean#keepall() as sb where sb.theString = ce.cid` over a
    keepall named window / a keyed table, listener rows only.
  - `InfraNWTableOnSelectWDelete` ords 0/1
    `InfraNWTableOnSelectWDeleteAssertion`
    (`java-runtime-27d8980edc91c4593d34` /
    `java-runtime-60c74e5e717dc6d9330e`, static
    `java-5a29ff903cb7fd7a100d`): `on SupportBean_S0 select and delete
    window(win.*).aggregate(0,(result,value)=>result+value.intPrimitive)
    as c0 from MyInfra as win where s0.p00=win.theString` — correlated
    select-AND-delete with an inline fold over matched store rows +
    iterator probes on the 'create' statement.
  - Java source read by primary agent (Java-contract scout aborted 3x
    on infra cancel → serial fallback recorded); contract pins: schema
    DDL `create schema MyEvent(cid string)` + `create window
    MyInfra.win:keepall() as MyEvent` OR `create table
    MyInfra(cid string primary key)` + `insert into MyInfra select *
    from MyEvent`; join asserts 4 rows for E1-E4 sends; select-delete
    asserts `assertPropsPerRowIteratorAnyOrder` for table /
    ordered for window; S0 insert into statement created/undeployed
    inside exec; supportbean/S0 fixtures registered in path.
- [x] Go-surface contract: scout `GoSurfaceJoinSel596` (read-only)
  mapped every surface: join infra×stream SUPPORTED via
  `JoinMany(JoinRecordSource(FromNamedWindow|FromTable), JoinRecordSource(
  From[bean].Window(KeepAll()).AsRecord())).On(OnSourcesEqual(...))`;
  ungrouped trigger-select aggregate fold SUPPORTED
  (`Sum[int32](NamedWindowField|TableField)`); DDL + `Statement.Snapshot`
  probes SUPPORTED. Single GAP: select-AND-delete trigger action.
- [x] **Engine addition (shared core)**: `triggerDefinition.deleteAfterSelect`
  flag; new `TriggerStream.SelectDeleteFromNamedWindow` /
  `SelectDeleteFromTable` builders keep `triggerSelectTable` semantics and
  re-run the matched-set deletion inside the same candidate processing via
  `executeTriggerActionWithTags` (nil-predicate table form maps to
  delete-all). Engine tests `TestOnSelectDeleteNamedWindow|Table` cover the
  exact Java sequence (sum fold + iterator empties) and pass.
- [x] Scenario + oracle assets by `AssetJoinSel596` (parity-asset-worker):
  `testdata/parity/infra-nwtable-join-select-delete.json` (4 cases / 30 steps),
  `tools/java-oracle/InfraNWTableJoinSelectDeleteScenarioOracle.java` +
  `run-infra-nwtable-join-select-delete.sh`. Two review-fix rounds via DM:
  (1) run-script output jq asserted raw iterator order for `any`-mode
  probes — fixed to `canonSnap` canonical sort (Java table emits {2,4,3});
  (2) oracle now emits canonical-sorted `new` rows at emission time for
  `any`-mode snapshot probes, so the checked-in raw trace equals the
  comparator-canonicalized form (ordered probes keep engine iterator order).
- [x] Go runner `internal/app/parity/infra_nwtable_join_select_delete.go`
  + run.go wiring (mode `infra-nwtable-join-select-delete` / `-diff`).
  Strict payload decoder now enforces exact key sets per event type
  (payload-extra rejection). Root + step field sets reuse
  `requireInfraNWTableOnDeleteFields` incl. `javaSourceFiles`.
- [x] Java trace via run script (30 records, canonical any-mode order) →
  Go replay 30 records → `-mode ...-diff` status `passing` / 0 differences;
  checked-in `go.trace.json` + `evidence.json` written.
- [x] Six-test family in run_test.go green (direct replay, diff evidence,
  4 trace mutations, checked-in equality, 9 raw-scenario mutations,
  runtime-ID mapping): `go test -run TestRunInfraNWTableJoinSelectDelete`
  PASS.
- [x] Manifest/roadmap/CHANGELOG: new `case.infra-nwtable-join-select-delete`
  (born-DV, 4 runtime IDs), mapping → `trigger.table-named-window` + 2
  goRefs; summary 802 cases / 800 impl / 431 DV / 1757 DV IDs / 4221
  assoc / referenced 3754 / unreferenced 382.
- [x] `make check` GREEN (parity 156s, internal/esper 119s). Independent
  parity review (`ParityReview596`, read-only): OVERALL PASS, 3×P3 —
  (1) PLANS mutation-count prose fixed (4 trace + 9 raw); (2) manifest
  trailing newline restored; (3) oracle SODA-redeploy listener claim
  routed back to `AssetJoinSel596` (inert, comment+behavior fix).
- [x] Post-P3 re-validation: build + six-test family + `TestOnSelectDelete`
  re-run green; regenerated trace byte-identical. Shipped; Git owns identity.

## Previous work units (shipped)
- Shipped: Draft 4.591 ('timer-interval spec-resolution forms')
 committed and pushed as `f76502639`; Git owns identity.
 case.pattern-every +5 DV IDs; review PASS (1 P3).
- Shipped: Draft 4.590 ('guard-while cluster') committed and
 pushed as `f20b88aec`; Git owns identity. case 4/4 DV.
- Shipped: Draft 4.589 ('withinmax W-harness + every-within fix')
 committed and pushed as `b57bf6587`; Git owns identity.
- Shipped: Draft 4.588 ('matchuntil 52-case W-harness')
 committed and pushed as `8a7f0c2a3`; Git owns identity.
- Shipped: Draft 4.587 ('matchuntil repeat-use-tags trio')
 committed and pushed as `7702eaa32`; Git owns identity. case
 7/9 DV; review PASS after P2 whitespace fix.
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
