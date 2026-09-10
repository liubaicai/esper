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

- Shipped: Draft 4.328 ('infra-named-window-on-update') committed at `f7c9dc175`; Git owns identity.
- Shipped: Draft 4.329 ('infra-named-window-on-update-misc') committed; Git owns identity. InfraNamedWindowOnUpdate.java is now 8/8 executions differential-verified.
- Shipped: Draft 4.330 ('infra-named-window-insert-from') committed; Git owns identity.
- Shipped: Draft 4.332 ('resultset-aggregate-filter-named-parameter-sorted-join') committed; Git owns identity. ResultSetAggregateFilterNamedParameter.java is now covered at ordinals 0-18 minus 19/20 (19 audit/reuse and 20 invalid remain).
- Shipped: Draft 4.333 ('infra-nwtable-subq-correl-coerce') committed; Git owns identity. The InfraNWTableSubq* file family is nearly closed: uncorrel, correl-join, filtered-correl, correl-coerce, at-eventbean, subquery, delete-aggregate are all differential-verified; only correl-index-sharing (case.query-subquery-index-sharing) remains implemented-not-DV.
- Shipped: Draft 4.334 ('epl-other-stream-expr') committed at `b793bbd13`; Git owns identity.
- Shipped: Draft 4.335 ('epl-other-select-expr-stream-selector') committed as 21c6f7151; Git owns identity. EPLOtherSelectExprStreamSelector.java now 12/17 executions DV (ords 4-15; 0/16 invalid-compile, 1/2/3 deferred with rationale in the manifest).
- Shipped: Draft 4.336 ('filter-rebool-context-pattern-value') committed as 4115ff7e3; Git owns identity. ExprFilterOptimizableBooleanLimitedExpr.java is now 10/14 executions DV (ords 1/4 land in 4.337).
- Shipped: Draft 4.337 ('filter-rebool-mixed-value-self') committed as 492e0f67b; Git owns identity. ExprFilterOptimizableBooleanLimitedExpr.java is fully dispositioned: 12/14 DV, ord 2 excluded (wall-clock), ord 11 intentionally-different.
- Shipped: Draft 4.338 ('insert-into-populate-ctor') committed as 34edca018; Git owns identity. EPLInsertIntoPopulateUnderlying.java now 8/13 executions DV.
- Shipped: Draft 4.339 ('insert-into-charsequence-factory') committed as f08ecea99; Git owns identity. EPLInsertIntoPopulateUnderlying.java now 10/13 executions DV.
- Shipped: Draft 4.340 ('insert-into-until-pattern-array') committed as 6addb61ea; Git owns identity. EPLInsertIntoPopulateUnderlying.java now 12/13 executions DV.
- Shipped: Draft 4.341 ('insert-into-invalidity') committed as 121fdf06a; Git owns identity. EPLInsertIntoPopulateUnderlying.java fully dispositioned (12/13 DV + 1 implemented-not-DV).
- Shipped: Draft 4.342 ('event-precedence-contained-outputrate') committed as 888916ba3; Git owns identity. EPLInsertIntoEventPrecedence.java now 6/11 executions DV.
- Shipped: Draft 4.343 ('event-precedence-subquery-engine') committed as 7b26930a1; Git owns identity. EPLInsertIntoEventPrecedence.java now 10/11 executions DV.
- Shipped: Draft 4.344 ('event-precedence-validation') committed as f68ec45b4; Git owns identity. EPLInsertIntoEventPrecedence.java fully dispositioned (10/11 DV + 1 implemented-not-DV).
- Shipped: Draft 4.345 ('viewgroup-merge-view') committed as 41a3d402d; Git owns identity. ViewGroup.java now 2/20 executions DV.
- Shipped: Draft 4.346 ('viewgroup-derived-value-views') committed as 1225a63e5; Git owns identity. ViewGroup.java now 6/20 executions DV.
- Shipped: Draft 4.347 ('viewgroup-time-windows') committed as 51e284b7e; Git owns identity. ViewGroup.java now 11/20 executions DV.
- Shipped: Draft 4.348 ('viewgroup-reclaim') committed as 9605dbf76; Git owns identity. ViewGroup.java now 14/20 executions DV.
- Shipped: Draft 4.349 ('viewgroup-expression-groupwin') committed as 5c2bede02; Git owns identity. ViewGroup.java now 15/20 executions DV.
- Shipped: Draft 4.354 ('subselect-preeval-off') committed and pushed; Git owns identity. EPLSubselectOrderOfEvalNoPreeval.java now 1/1 executions DV — the subselect evaluation-order/preeval-configuration domain is fully closed for all non-policy-gated suites.
- Shipped: Draft 4.369 ('dataflow-eventbus-source') committed as 063cc4a76 and pushed; Git owns identity. case.dataflow-eventbus now 4/4 executions DV (253 DV cases, 930 DV runtime IDs).
- Shipped: Draft 4.370 ('dataflow-eventbus-sink') committed as 801aeaffe and pushed; Git owns identity. case.dataflow-eventbus-collector now 5/5 executions DV across three chains (254 DV cases, 932 DV runtime IDs).
- Current target: Draft 4.374-a 'dataflow-lifecycle-core' — first of the lifecycle units: 6 GREEN executions (ConfigAndInstance 67b54e408ca6f86b4ca5, APIStatistics c8cbd0bc1aeb3eba6006, ParameterInjectionCallback 1dd9223830c5409fc252, OperatorInjectionCallback 1092d7e9660b84bc6e6d, InvalidJoinRun 7e40b511fb6b6db584fd, BlockingException 3bdb22d6cdef39f0336c) upgrading case.dataflow-lifecycle (18 rts) with per-case dv list = those 6. Follow-up: 4.375 lifecycle cancel/join YELLOW group (7) + BlockingMultipleRunnable guard disposition; BlockingRunJoin RED (wall-clock); remaining dataflow dispositions; DV-list consolidation; database slices; perf §4.9 (incremental join first).

Active: Draft 4.353 ('subselect-tail-wildcard-no-preeval').

- [x] Parallel read-only scouts dispatched and complete (Java contract agent_713fd793: wildcard statement byte-exact, 4 sends/2 records, equals-dispatched-on-left semantics, chain-A oracle audit — bean-class registration plus local ArrayCollMap mirror and an instance registry needed since regression-lib is off the script classpath; Go surface agent_20b5abba: wildcard-in GREEN, NoPreeval RED-for-runner-only — selfSubselectPreeval is a Java runtime configuration the Go engine lacks).
- [x] Freeze contract; asset + runner work. (Rescoped to in-wildcard only; NoPreeval deferred to a dedicated engine unit. Writer agent_0f430414 delivered oracle + scenario (append-only) + regenerated Java trace (52→54 records, prefix byte-identical, contract exact); primary built the Go case: SubqueryIn[any] + EventValue[any] over the S1 length(1000) window, anyObject as an `any` interface field, S2/ArrayCollMap registrations, type-discriminating nested decode; probe verified true/false before the scenario landed; +1 runtime ID/execution at source-order index 6; mutation family +2 (same-instance match, class mismatch).)
- [x] Manifest, evidence, roadmap, CHANGELOG. (Evidence passing 0 differences, 15 IDs/15 executions, 54 records per side; manifest case.subselect-in 15/15 at index 6; summary assoc 3528 / referenced 3275 / unreferenced 861, dvRuntimeIds 898 unchanged; roadmap epl 262→261 + 4.353 supplement; CHANGELOG 4.353 entry.)
- [x] Gates: affected packages (parity 176s / compat / esper), make check, git diff --check all green; wildcard-type registrations then gated into the in-wildcard case branch per review and the family re-run green.
- [x] Independent parity review. Reviewer agent_65d24f72-efb3-463a-aef6-aa36e8c6fd3a OVERALL PASS with independently recomputed manifest arithmetic, canonical-bytes prefix checks, per-index mutation verification, and three derivational probes (silent S1 send, S2 non-match under three independent reasons, decode/equality path). Two P3s: (1) wildcard-type registrations gated into the case branch (fixed in-unit, family green); (2) convention tension — this older chain entry carries DV at case level with no per-case differentialVerifiedRuntimeIds list, so summary dvRuntimeIds stays 898 despite new evidenced runtimes (consistent with reviewed 4.352 convention; quality-strategy line 40 tension recorded) — DEFERRED as its own consolidation unit: backfill per-case DV lists across old chain entries or amend quality-strategy line 40 to codify case-level DV.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in).

Reconciliation for 4.352: subselect domain has 25 open executions of which 16 are policy-gated performance suites (CorrelatedAggregation/Filtered/InKeyword/NamedWindow Performance), 4 are the exists-suite SODA/OM variants already dispositioned intentionally-different in the Go test file (manifest case.subselect-exists should surface that disposition in a future unit), 3 are EPLSubselectAggregatedInvalid (invalidity policy) plus the two range-coercion join executions selected here, 1 is EPLSubselectInWildcard, and 1 is EPLSubselectOrderOfEvalNoPreeval. The selected pair is the tightest same-suite closed loop: both are multi-outer-stream correlated aggregated subselects pinning between range reversal, >=/<= non-reversal, single-sided >/<, and null endpoint semantics.

Active: Draft 4.352 ('subselect-range-coercion-joins').

- [x] Parallel read-only scouts dispatched and complete (Java contract agent_a2ba0fa0: 7 statements byte-exact, 17/5/17-send sequences, 55-record structure; Go surface agent_cd0357b6: all GREEN — JoinField multi-stream correlation, BetweenOf reversal, Of-form comparisons, SubquerySum empty-null; primary spike-validated SubquerySum-inside-JoinMany.Select green before freezing).
- [x] Freeze contract; asset work; Go runner cases + tests. (Serial exception recorded: writer dispatch agent_5e19645d failed at spawn on the platform 5-hour usage cap until 18:26:26, so the primary performed the oracle/scenario/Java-trace scope itself. Scenario 14→21 cases (+95 sends, 7 markers); oracle +3 ST types, +7 EPL branches; Java trace regenerated via the unchanged run script — 112 records, existing 57 byte-identical, all 7 sumi sequences matching the frozen contract including the 8/27 reversal discriminators. Go runner: +3 ST struct registrations, +7 case builders (JoinField positional sources; two between orientations), decode branches, caseOrder, +2 runtime IDs/executions; evidence-driven test family extended with 7 new mutations — 16/16 pass.)
- [x] Manifest (+2 runtime IDs — suite 20/21 DV, EPLSubselectAggregatedInvalid remains invalidity-policy), evidence, roadmap, CHANGELOG. (Evidence passing 0 differences, 15 IDs/15 executions; manifest assoc 3527 / referenced 3274 / unreferenced 862, dvRuntimeIds 898 unchanged — this older case entry carries DV at case level; roadmap epl 264→262; 4.352 supplements newest-first.)
- [x] Gates: affected packages, make check, git diff --check all green (202s parity, 93s esper, compat clean). Pre-review self-checks: evidence 57-record prefixes byte-identical both sides; oracle EPL transcription byte-exact against the joined Java literals for 6/7 with the 7th the documented single-@name adaptation of the suite's duplicated @name typo.
- [x] Independent parity review (dispatched on user instruction ahead of the cap window; agent spawn succeeded). Reviewer agent_5b3fb6ec-3ea3-4cd8-86c8-b5cd4d17824b OVERALL PASS across contract fidelity, Go builder fidelity, trace/evidence integrity (canonical-bytes prefix check), independently recomputed manifest arithmetic, mutation adequacy (each index/record verified), prose accuracy, and ownership; four derivational probes (reversal sums 8 and 27, unsatisfiable no-reversal bounds, null-key2 deref) all consistent. Two P3 nits fixed in-unit: stale "13 executions" runner comment → 15; scenario JSON re-emitted at the established 2-space indent with content verified identical.
- [x] Commit and push (fold ALL checkpoint closure edits into the semantic commit; no post-commit checkpoint-only push).

- Deferred: epl-other-as-keyword-backtick (FAF/on-trigger/merge integration complex — Go runner's SelectFromNamedWindow subscribes to both trigger and NW changes producing extra records; Java oracle had 37 compilation errors; both need engine-level investigation before retry). Next candidates after 4.336: ExprFilterOptimizableBooleanLimitedExpr ords 1+4 (N+2), EPLOtherPlanInKeywordQuery (9), EPLInsertIntoPopulateUnderlying (9), EPLInsertIntoEventPrecedence (7).

## Current work unit
Active: Draft 4.374-a ('dataflow-lifecycle-core').

- [x] Parallel read-only scouts dispatched and complete (prefetched during 4.373 review). Java contract scout agent_38f3e7e9: full 18-ID inventory verified against the execution inventory; strict triage — 6 GREEN (ConfigAndInstance CRUD + 5 error paths; APIStatistics 2-operator shape with time magnitudes excluded; both InstantiationOptions callback executions; InvalidJoinRun pure state machine; BlockingException synchronous throw), 7 YELLOW cancel/join executions (bounded-wait adaptable, deferred to 4.375), BlockingRunJoin RED (deltaJoin>=500ms wall-clock assert, engine-covered join-blocking semantics), BlockingMultipleRunnable pending the Go multi-source run() guard check. Go surface scout agent_222742bb: full lifecycle surface mapped (Stats{Processed,Emitted,Errors,Dropped} increment points; OperatorStats{Name,Number,PrettyPrint,Submitted,SubmittedByPort,Elapsed}; Join/Cancel matrix: Join-before-start ErrorState, Join-after-cancel ErrorCanceled, Cancel-on-Complete ErrorState vs Java silent — record states, never Go error text; InstanceID defaults to definition name vs Java null — record only explicitly-set ids; Elapsed is wall-clock — '>0' tokens only; Run has no multi-source guard — BlockingMultipleRunnable deferred pending disposition).
- [x] Contract frozen. Scenario dataflow-lifecycle-core, 6 cases, per-case record protocols (count/value/state/lifecycle + error-class tokens per the create-start-stop-destroy and exceptions conventions; message text never recorded; normalized bare-port pretty-prints; epoch time; per-case fresh runtime): (0) config-and-instance — saved-config CRUD cycle with error tokens (instantiation-not-found, save-not-found, already-exists x2) + saved-config/instance counts + saved-config run() listener-invoked state; deployment-id never recorded; ~14-18 records. (1) statistics — statement-type/object-name values, stats-size 2, per-operator name/number/normalized pretty-print, submitted counts 2/[2] and 0/len 0, elapsed '>0' tokens only; ~10 records. (2) parameter-injection-callback — provider contexts count 3, sorted parameter names, per-context operator/dataflow/factory-identity values, resolved params abc/def/xyz; ~12 records. (3) operator-injection-callback — contexts count 1, MyOp/MyDataFlowOne values; ~4 records. (4) invalid-join-run — join-before-exec/run-after-cancel/start-after-cancel error-class tokens, INSTANTIATED/CANCELLED states, cancel-twice idempotent count; ~7 records. (5) blocking-exception — run-error class token, COMPLETE state, capture count 0; ~4 records. Total ~51 records (writer's per-record breakdown is the cross-check baseline). Manifest: case.dataflow-lifecycle → differential-verified with dv list = the 6 GREEN IDs (12 remain implemented-not-DV pending 4.375/4.376); summary dvCases 258, dvIds 943, assoc/referenced/unreferenced unchanged 3558/3293/843. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_lifecycle_core.go (6 case builders: saved-config/instance CRUD cycle with error-class tokens and a saved-config run through EventBusSink; the two-operator statistics shape via a two-bean source (both sides count exactly the two bean submissions (neither counts the final marker)); parameter provider with three declared properties (contexts 3, factory-kind identity, resolved overrides); operator provider substitution; the InvalidJoinRun synchronous state machine; the blocking-exception error surfacing). Integration alignments: MyEvent empty-schema registration; SubmitPort on the declared port; capture declared with input 'outstream' for pretty-print parity; provider requires declared Properties; label fixes (flow/count ops) per the writer's baseline.
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Writer agent_482a765f delivered oracle + scenario + script + Java trace 49 records across three amendments (statement-property records → oracle-internal; capture per-port-length dropped as structurally unmirrorable; null probe → 'absent' token), deterministic rerun. Evidence passing, 0 differences. Manifest case.dataflow-lifecycle → differential-verified with per-case dv list = the 6 GREEN IDs (12 remain implemented-not-differential; BlockingRunJoin wall-clock permanently deferred); summary dvCases 258, dvIds 943, assoc/referenced/unreferenced unchanged 3558/3293/843; roadmap + CHANGELOG 4.374-a supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 180s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_c979195a OVERALL PASS, no P1: five EPL literals byte-exact, assertion-to-record mapping verified assertion-by-assertion across all six executions, 49-record recount both sides zero semantic differences, manifest dv-union recount 943 with exactly the +6 GREEN delta, no internal/esper changes, no skips. Two P2s fixed pre-commit: the composition-difference comment was factually wrong about Java (its statistics-wrapped emitter counts only submit/submitPort — both sides count exactly the two bean submissions, neither counts the marker; comment + PLANS phrase corrected); four state records now read instance.State() back before recording (COMPLETE x2, INSTANTIATED, CANCELLED — case 5's COMPLETE was the value not implied by the call outcome). Three P3s fixed/noted: mutation retargeted to the join token record; CHANGELOG/roadmap token count corrected to four error-path records; advisory read-back strengthening noted for a future pass.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

Active: Draft 4.373 ('dataflow-exceptions').

- [x] Parallel read-only scouts dispatched and complete (prefetched during 4.372 review). Java contract scout agent_746e9667: EPLDataflowAPIExceptions (java-runtime-c1d106a7e1cc70655006, direct variant) has two synchronous-throw parts — Part 1 source-throw (DefaultSupportSourceOp wraps as 'Support-graph-source generated exception: My-Exception-Is-Here'; handler invoked once on the source thread; state COMPLETE) and Part 2 operator-throw (MyExceptionOp.onInput throws 'Operator-thrown-exception'; handler receives the RAW getTargetException and swallows — submit returns normally, exactly one context; state COMPLETE); byte-exact EPL quirk: part 2's two graph clauses concatenated with NO separator; cancel-after-COMPLETE is a silent no-op; static handler list must be cleared between parts. Dispositions confirmed: invalid-graph (19 compile-prefix probes + 1 instantiate probe) and custom-properties (compile-only + Java forge-reflection internals) stay implemented-not-differential — existing manifest difference text already correct. Go surface scout agent_b60d848c: DataflowError{DataflowName, InstanceID, OperatorName, OperatorNum, OperatorPrettyPrint, Err} byte-exact representable; Fail is the zero-value default; Complete-with-runErr under start (never CANCELLED); Cancel-on-Complete returns ErrorState (Go) vs Java silent no-op — do not record as success; Continue row-drop has no Java counterpart in this file (unit-evidence only); pretty-print for typed ports renders package-qualified — freeze bare-port normalization.
- [x] Contract frozen. Scenario dataflow-exceptions, ONE case 'exceptions' (rt c1d106a7e1cc70655006), two flows in source order in one runtime (fresh handler state between, mirroring undeployAll), NO data rows — state/count/value records only, 12 records total, 6 per flow: (a) count handler-contexts=1; (b) lifecycle value record handler.error-class = 'source-error' (flow A) / 'operator-throw' (flow B); (c) handler.operator-name = 'DefaultSupportSourceOp' / 'MyExceptionOp'; (d) handler.operator-number = 0 / 1; (e) handler.operator-pretty-print NORMALIZED bare-port form 'DefaultSupportSourceOp#0() -> outstream' / 'MyExceptionOp#1(outstream)' (Java asserts its full '<SupportBean>' string in-process; Go strips the package-qualified type label); (f) state instance.state=COMPLETE. Message text never recorded (invalidity policy). Go runner mirrors: flow A CustomTypedSource failing Run; flow B source emits one bean then a CustomPorts erroring Process; ErrorPolicy Fail default; ends Complete-with-runErr, no cancel records. Manifest: case.dataflow-exceptions → differential-verified, dv list=[c1d106a7e1cc70655006]; summary dvCases 257, dvIds 937 (c1d1 new to union), assoc 3558 / referenced 3293 / unreferenced 843 unchanged. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_exceptions.go (two flows: CustomTypedSource failing source + CustomPorts failing operator with handler-recorder; normalized bare-port pretty-print incl. stripping Go's implicit '-> out' suffix; error attribution verified — downstream operator failures attribute to the throwing operator). Cross-check fixes: SubmitPort with the declared port (Submit hardcodes 'out' on the raw source path). run.go 'dataflow-exceptions'/-diff modes; run_test.go family (evidence, 12-record direct replay with attribution/normalization/COMPLETE pins, 6 mutations incl. unnormalized pretty-print and CANCELLED flip).
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Writer agent_dda7d453 delivered oracle + scenario + script + Java trace 12 records, deterministic rerun. Evidence passing, 0 differences. Manifest case.dataflow-exceptions → differential-verified, dv list=[c1d1]; summary dvCases 257, dvIds 937, assoc/referenced/unreferenced unchanged 3558/3293/843; roadmap + CHANGELOG 4.373 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 182s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_db3fbd4d OVERALL PASS: both EPL literals byte-exact (flow B no-separator join), 1:1 assertion-to-record mapping across run() lines 38-87, 12-record recount both sides zero semantic differences, manifest dv-union recount 937 with sole +1 c1d1, zero internal/esper changes, no skips. One P3 fixed pre-commit: invalid-graph probe count corrected 21→19 (grep-verified 19 tryInvalidCompile + 1 tryInvalidInstantiate) across CHANGELOG/roadmap/PLANS.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

Active: Draft 4.372 ('dataflow-ports-feedback').

- [x] Parallel read-only scouts dispatched and complete (prefetched during 4.371 review). Java contract scout agent_15d3ff42: byte-exact graphs for all three executions of EPLDataflowInputOutputVariations (FanInOut MultiInMultiOutGraph with trailing space + blank lines + no space before {}; Factorial FactorialGraph; LargeNumOps 17-stage Select chain); MyCustomOp routing onS0→submitPort(1)=OutTwo (S0-strings) / onS1→submitPort(0)=OutOne (S1-ints, deliberate crossover, lax typing — String into SchemaTwo-typed port); MyFactorialOp onInput→TempResult feedback self-edge, onTemp recursion terminating submitPort(1)=[120L]; captures receive raw Object[] via the onInput(Object) PassAlong branch (positional p0 projection); LOAD-BEARING no-flush fact: BeaconSource lacks @DataFlowOpProvideSignal so FinalMarkers die at the source channel — capture reads are current-batch semantics, never signal-flushed (contrast 4.371). Go surface scout agent_3fcaee4f: untyped CustomPorts (CustomTypedPorts would reject Java's lax cross-type emission), ConnectFeedbackPorts self-edge precedent (dataflow_feedback_test.go factorial 120 → DataflowComplete), no output port named "out" on factorial (signal mis-forwarding hazard), capture records bypass canonicalizeAnyModeRows so BOTH sides must pre-sort rows per record (sortResultRecordsByCanonicalFields JSON comparator, infra_named_window_insert_from.go:561-583 precedent) — mandatory because Go's configured beacons are goroutines with racy arrival.
- [x] Contract frozen. Scenario dataflow-ports-feedback, 3 cases, 6 records/side, capture records with per-record canonical row sort: (0) fan-in-out 6309a6b7e0f0ba981a97 — 4 records in suite get() order, labels flow:SupportOpCountFutureOneA/OneB/TwoA/TwoB (the distinct name params), rows OneA/OneB=[{p0:S1-10},{p0:S1-20}] sorted, TwoA/TwoB=[{p0:S0-A1},{p0:S0-A2}] sorted; graph byte-exact incl. trailing space after 'MultiInMultiOutGraph ' and no space before 'OutTwo<SchemaOne>{}'; (1) factorial 98c5bdc6afa84705b9db — 1 record flow:DefaultSupportCaptureOp [{p0:120}], feedback self-edge, instance reaches COMPLETE on its own (no cancel, minimal capture-only protocol); (2) large-num-ops bddc8c72bd9d2a2c8869 — 1 record flow:DefaultSupportCaptureOp [{p0:A1}], the 17 identity-Select stages modeled as identity customs (documented difference: Java's select: subquery parameter has no typed-API analogue; deep-linear shape already engine-pinned). Raw Object[] passthrough = positional p0 projection on both sides. Manifest: case.dataflow-captive-emitter-ports dv list += 6309 (→ full DV); case.dataflow-feedback → differential-verified with dv list = both IDs; summary dvCases 256, dvIds 936 (6309/98c5bdc6/bddc8c72 all new to union), assoc/referenced/unreferenced unchanged 3558/3293/843. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_ports_feedback.go (3 case builders: fan-in-out with CustomPorts 2-in/2-out crossover routing + four named captures; factorial with ConnectFeedbackPorts self-edge and natural completion; large-num-ops 17-stage identity chain); dataflowPortsFeedbackRuntime adapter implements OnSignal swallow (BeaconSource markers die at the custom operator — mirrors Java ops without onSignal and fixes the engine's default out-port forwarding rejection); canonical per-record row sort both sides; run.go 'dataflow-ports-feedback'/-diff modes; run_test.go family (evidence, 6-record direct replay with crossover/factorial/A1 pins, 5 mutations).
- [x] Evidence pipeline, manifest DV upgrades, roadmap, CHANGELOG. (Writer agent_24875e4f delivered oracle + scenario + script + Java trace 6 records, deterministic across runs, no deviations. Evidence passing, 0 differences. Manifest: case.dataflow-captive-emitter-ports dv list += 6309 (full DV, difference appended preserve-then-append); case.dataflow-feedback → differential-verified with dv list = both IDs; summary dvCases 256, dvIds 936 (6309/98c5bdc6/bddc8c72 all new to union), assoc/referenced/unreferenced unchanged 3558/3293/843; roadmap + CHANGELOG 4.372 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 187s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_daa5b935 OVERALL PASS with zero P1/P2: all three graph literals byte-exact (757/429/1199 chars), routing transcription verbatim (onS0→port1 OutTwo crossover, factorial recursion line-for-line), 6-record recount both sides with every row canonically sorted, manifest dv-union recount 936 with exactly the +3 delta and pre-association verified, feedback self-edge/OnSignal-swallow engine semantics confirmed, no skips, no internal/esper changes. Three P3 observations recorded without action (java-baseline evidence-drop follows the established upgrade convention; scenario description abbreviates the 17-stage chain in prose while the oracle carries the byte-exact text; the differ-bypass premise was engine-confirmed).
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

Active: Draft 4.371 ('dataflow-captive-lifecycle').

- [x] Parallel read-only scouts dispatched and complete (prefetched during 4.370 review). Java contract scout agent_819da682: single execution EPLDataflowAPIStartCaptive (java-runtime-407e42478546e709d774; EPDataFlowEmitterOperator is a pure interface, nothing to replay); Flow A fully synchronous zero-latch (captive start → runnables 0 / emitters {'src1'} keyed by the Emitter NAME parameter — the manifest's named-routing difference; E1/E2 accumulating reads; FinalMarker signal empties current (signal ≠ row) then getAndReset batch; E3 fresh current; STILL RUNNING after signal (captive has no completion path); cancel → CANCELLED with no thread interruption); Flow B doc sample instantiate-only; FanInOut (submitPort, capture-local any-order) + Factorial (feedback loop) assessed and deferred to a follow-up unit; LargeNumOps belongs with the select chains. Go surface scout agent_d2544997: StartCaptive/CaptiveEmitter/SubmitSignal surface fully mapped (dataflow.go:4787/:3482/:540); captive never completes by itself — only Cancel (or a Fail-policy operator error, which moves to Complete not Canceled); emitters map omits Emitters without outgoing edges; raw []any submits fail object-array materialization so the faithful replay registers a struct; second StartCaptive is an ErrorState internal assert; capture operator mirrors DefaultSupportCaptureOp via Process + OnSignal (current/received split).
- [x] Contract frozen. Scenario dataflow-captive-lifecycle, ONE case 'captive-emitter' (rt 407e42478546e709d774), record protocol = create-start-stop-destroy conventions (state records {operation:"state", name:"instance.state", value:...}, count records, capture records over flow:DefaultSupportCaptureOp with p0/p1 row fields, case-local sequences, epoch time), ~10 records: (1) count runnables=0; (2) count emitters src1=1; (3) state RUNNING; (4) capture [E1{p0:E1,p1:10}]; (5) capture [E1,E2]; (6) capture [] post-signal; (7) capture [E1,E2] getAndReset batch; (8) capture [E3{p0:E3,p1:30}]; (9) state RUNNING (the stays-running contract); (10) state CANCELLED; plus state INSTANTIATED for the doc-sample flow B (instantiate-only, LogSink real operator, no options). Graph A byte-exact `@name('flow') create dataflow MyDataFlow Emitter -> outstream<MyOAEventType> {name:'src1'}DefaultSupportCaptureOp(outstream) {}` (no space before DefaultSupportCaptureOp); capture injected via OperatorProvider by class simple-name; MyOAEventType as registered struct {P0 string; P1 int} (raw []any would fail Go materialization — documented adaptation, rows normalize identically); signal = esper.FinalMarker{}; second StartCaptive/submit-after-cancel are internal ErrorState asserts, not records. Manifest: case.dataflow-captive-emitter-ports gains java-runtime-407e42478546e709d774 as 2nd javaRuntimeId (FanInOut stays out — not yet DV), dv list = [407e]; summary dvCases 255, dvIds 933, assoc 3558, referenced 3293, unreferenced 843. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_captive_lifecycle.go (capture operator mirroring DefaultSupportCaptureOp via Process + OnSignal current/received; count/state/capture records; internal asserts for emitter key 'src1', still-RUNNING after signal, Runnables/Emitters counts); run.go 'dataflow-captive-lifecycle'/-diff modes; run_test.go family (evidence, 11-record direct replay with signal-semantics and tail-state pins, 6 mutations incl. post-signal-fill/flush-swap/state-flip/count-drift). Cross-check fixes during integration: state values aligned to the oracle's uppercase enum names; count operation aligned to 'count'.
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Writer agent_af8dab1b delivered oracle + scenario + script + Java trace 11 records, deterministic. Evidence passing, 0 differences; records semantically equal modulo the established empty-new omitempty convention. Manifest case.dataflow-captive-emitter-ports → differential-verified, +407e as 2nd javaRuntimeId, dv list=[407e], FanInOut documented implemented-not-differential; summary dvCases 255, dvIds 933, assoc 3558, referenced 3293, unreferenced 843; roadmap + CHANGELOG 4.371 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 184s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_1cd1c15e OVERALL PASS: both graph literals byte-equal via python reconstruction, 1:1 assertion-to-record mapping across the full run() body (lines 42-78 → records 1-11), 11-record recount both sides zero semantic differences, manifest dv-union recount 933 with sole +1 407e, no internal/esper changes, no skips. Two P2s fixed pre-commit: stale Current-target tail rewritten for 4.371; the case's manifest difference restored (had replaced the prior intentional-difference prose) and appended per the preserve-then-append convention with the stray leading space dropped. Two P3s fixed pre-commit: State() observations added before the first RUNNING record and after Cancel (records transitively engine-guaranteed → now directly observed); takeReceived comment corrected to single-batch semantics.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

Active: Draft 4.370 ('dataflow-eventbus-sink').

- [x] Parallel read-only scouts dispatched and complete (prefetched during 4.369 review). Java contract scout agent_3bfc8a24: sink ordinals 0/1/2 = AllTypes e6b4bb618f70b384cf03 / Beacon 1ecb769e10b4a818772c / SendEventDynamicType 11a7b321e6901dbad740; sink ord-0 AllTypes shares its name with the DV'd source execution — runtime ID discriminates; listener record protocol (operation "listener", labels s0/s1); byte-exact traps pinned (class: vs 'class : ', beacon trailing comma, OutStream<?> untyped port, doc-sample dataflow statement itself named s0, p1 constant 1 — the <10 bound is not randomness). Go surface scout agent_13bbd986: EventBusSink/EventBusSinkWithCollector already implemented (dataflow.go:1654-172, emissions DataflowEventBusSinkEmission); sends fully synchronous (Run return = all listener batches delivered); consumer = DeployPlans FromAny select + statement.Subscribe with NormalizeResults, deployed+subscribed BEFORE instantiate; canned BeaconSource drains synchronously inside Start; raw []any passes untouched without EventType; the sink output-stream Build rejection message matches Java verbatim; EPLDataflowBeacon already replayed by case.dataflow-connector-output (dataflow_connector.go) — 3 GREEN verdicts.
- [x] Contract frozen. Scenario dataflow-eventbus-sink, 3 cases, 13 records/side, listener protocol: (0) eventbus-sink-all-types e6b4bb618f70b384cf03 — 4 representation sub-runs in suite order MyXMLEvent/MyOAEvent/MyMapEvent/MyDefaultSupportGraphEvent, graph 'MyGraph DefaultSupportSourceOp -> instream<T>{}EventBusSink(instream) {}', s0 select * from <T>, 2 listener records per sub-run ({1.1,1,one},{2.2,2,two}), then the EventBusSink -> s1 invalid compile (in-process prefix) and the SampleSchema two-sink doc flow (instantiate-only, dataflow statement named s0) internal; 8 records. (1) eventbus-sink-beacon 1ecb769e10b4a818772c — @public MyEventBeacon(p0 string, p1 long) path schema, BeaconSource iterations:3/p0:'abc'/p1:1, 3 listener records {p0:abc,p1:1}; bounded-poll adaptation; 3 records. (2) eventbus-sink-dynamic-type 11a7b321e6901dbad740 — @buseventtype @public MyEventOne(type,p0,p1)/MyEventTwo(type,f0,f1) path schemas, MyObjectArrayGraphSource over {"type1",100,"abc"},{"type2","GE",-1}, collector routes on row[0], s0 then s1 with full projections incl. type; 2 records. Go adaptations frozen: canned BeaconSource + parsed Event envelopes (XML enters as envelope), listener rows via NormalizeResults, collector on raw []any, deployment-keyed consumers subscribed pre-instantiate. Manifest: case.dataflow-eventbus-collector → differential-verified, dv list = all 5 IDs (dfb59d/37aed via case.dataflow-eventbus chain, 1ecb769 via case.dataflow-connector-output, e6b4bb/11a7b3 via this chain — cross-chain evidence documented in difference); summary dvCases 254, dvIds 932 (e6b4bb/11a7b3 new to union), assoc 3557, referenced 3292, unreferenced 844 unchanged. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_eventbus_sink.go (3 case builders: all-types representation loop with deployed select consumers subscribed pre-instantiate; beacon mirroring the connector chain with channel+bounded-wait and fourth-delivery check; dynamic-type raw []any BeaconSource + EventBusSinkWithCollector routing on row[0], shared delivery slice asserting s0-then-s1); run.go 'dataflow-eventbus-sink'/-diff modes; run_test.go family (DiffWritesPassingEvidence / DirectReplay 13-record listener shape with dynamic full-projection pins / 6 trace mutations incl. statement-swap / sink output-stream Build rejection). Probe verified: 13 records, exact listener protocol.
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Writer agent_6b2f1167 delivered oracle + scenario + script + Java trace 13 records, deterministic re-run verified; documented correction — the doc graph has NO trailing newline after the final '}' per the Java literal. Evidence passing, 0 differences; java/go records semantically equal. Manifest case.dataflow-eventbus-collector → differential-verified, per-case dv list = all 5 IDs with cross-chain evidence split documented; summary dvCases 254, dvIds 932, assoc 3557 / referenced 3292 / unreferenced 844 unchanged; roadmap + CHANGELOG 4.370 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 181s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_0eca3a00 OVERALL PASS: 9/9 literals byte-exact vs the Java file, 13-record 8/3/2 recount both sides with every row exact, manifest dv-union recount 932 with the +2 delta identified (e6b4bb/11a7b3; 1ecb769 pre-unioned via connector-output), self-verifying evidence via DifferentialEvidence.Validate, no internal/esper changes, no skips. Three P3s fixed pre-commit: runner header comment reworded (doc flow is oracle-only, not replayed Go-side); stale Current-target tail rewritten for 4.370; scenario settle clause scoped to the Java oracle with the Go non-blocking fourth-delivery check documented.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

Active: Draft 4.369 ('dataflow-eventbus-source').

- [x] Parallel read-only scouts dispatched and complete. Go surface scout agent_cec3c3ce: EventBusSource 8 builder variants + Filter/FilterWithPorts exact signatures (dataflow.go:1562-1675); ingress dispatch synchronous through e.send→dispatchDataflowEvent (runtime.go:4791), instance must be Running, pre-start/post-cancel sends dropped; all four executions representable — AllTypes GREEN (4 representations, envelope via NormalizeEvents / WithUnderlying raw), SchemaObjectArray GREEN (RegisterObjectArray == path-deployed @public schema; collector param-provider identity has no Go surface — manifest already lists it open), Filter AllTypes GREEN (identity assertSame = documented difference; rows pin normalized fields), Filter Invalid YELLOW (nil/non-bool predicate → Build ErrorTypeMismatch representable; 3-output-streams/0-output-streams/implicit-conversion/prev() = case differences).
- [x] Contract frozen. Java contract scout agent_153445d4: runtime-ID mapping verified (dfb59d3bd4798d57cc0c=EventBusSource ord0 AllTypes, 37aed9aedcbbb25c4826=EventBusSource ord1 SchemaObjectArray, 0bc68f1db4dd07d89a66=Filter ord0 Invalid, 73c4f6808b38087c5a59=Filter ord1 AllTypes); delivery is sender-thread synchronous into a FIFO deque drained by one source thread (latch adaptation only); epstatement-source oracle conventions reused (explicit new:[] empty records, typed 3-field projection, fresh runtime per case, in-process invalidity prefixes). Frozen scope — 4 cases, 22 records/side: (0) eventbus-all-types 4 representation sub-runs (POJO/Map/XML/OA byte-exact graph), per sub-run 3 records (pre-start empty, 2-row fill {1.1,1,one}/{2.2,2,two} in send order, post-cancel empty), then 2 invalid compiles + doc-sample instantiate-only internal; (1) eventbus-schema-objectarray 3 sub-runs: envelope 1 row {p0:abc,p1:100}, underlying 1 row positional {"0":"abc","1":100} (NEW normalization pinned in scenario description), filter+collector with B-filtered-out empty record then {p0:A,p1:101} + collector-context asserts internal; (2) filter-all-types 4 representation sub-runs (POJO/XML/OA/Map order, sync run(), instance user-object/id pinned internal) + doc-sample instantiate-only + two-streams captive ({x,10} pass / {y,11} fail); (3) filter-invalid 0 records (5 message prefixes in-process; invalidity policy). Manifest: case.dataflow-eventbus gains 0bc68f1db4dd07d89a66 as 4th javaRuntimeId (epstatement-source precedent: invalidity ID inside DV case) and dv list = all 4 IDs; summary dvCases 253, dvIds 930, assoc 3557, referenced 3292, unreferenced 844. Writer dispatched for oracle + scenario + Java trace.
- [x] Go runner + mode + tests built (primary): dataflow_eventbus_source.go (4 case builders: all-types representation loop with fresh sub-env per sub-run, 3 schema-objectarray sub-run helpers, filter representation loop with DataflowOptions instance identity + two-streams StartCaptive shape, invalid zero-records); run.go 'dataflow-eventbus-source'/-diff modes; run_test.go family (DiffWritesPassingEvidence / DirectReplay 22-record shape incl. positional and collector-row pins / 6 trace mutations / Filter Invalid Build rejections). Probe verified: pre-start/post-cancel empty reads, send-order fill, positional {"0":"abc","1":100}, collector {p0:A,p1:101}.
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Writer agent_418181c1 delivered oracle + scenario + script + Java trace 22 records, deterministic re-run verified. Evidence passing, 0 differences; java/go records semantically equal (Go omits empty new keys via the established omitempty convention — same as epstatement chain). Manifest case.dataflow-eventbus → differential-verified, javaRuntimeIds +Invalid ID (4), per-case dv list 4 IDs; summary dvCases 253, dvIds 930, assoc 3557, referenced 3292, unreferenced 844; roadmap + CHANGELOG 4.367 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green (parity 177s / compat / esper, make check exit 0, git diff --check clean). Reviewer agent_992e1130-19c3-437b-8890-9a17654f21b4 OVERALL PASS: 6/6 graph strings + 19/19 literal comparisons byte-exact vs both Java files, 22-record/12+4+6+0 row-by-row trace recount on both sides with 0 semantic differences, full manifest dv-union recount 930 and all summary deltas verified, run_test family adequacy and no-skip confirmed, scope exactly the 13 contract files with zero internal/esper changes. Two P3s fixed pre-commit: draft-number collision (perf rounds owned 4.366-4.368 — parity unit renumbered 4.369, sink follow-up 4.370) and an informational date note matching existing convention.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

### Previous work unit (prior)

perf-compile-prune-round3 (2026-09-10, 两单元连续实施).

- Baseline：clean `master` `e77d1880e`；纯性能、零语义变化，Java oracle 无涉入。
- Unit A §4.7 编译谓词：`stateless_compile.go` 镜像泛型语义（同一操作函数、同一字段候选路径）编译纯谓词链；`matchesEventFilter`/`processStatelessEvent` 在 appliesTo 下使用编译链。等价性：矩阵差分测试（20 形态 × 6 事件 + 大小写/嵌入回退）。基准：rejected -32%、accepted 约 -50%。
- Unit B §4.1 第一阶段：`accept_index.go` 事件类型级候选裁剪（语句描述符 + 事件侧接受名集合 + 引擎缓存 + >1 语句门 + outputState 活检）；守护：context/update-istream/子查询/输出策略/variant/contained/historical/method 不可裁剪。中途修复 `addName` 翻回 prunable 的单向置位 bug（contained/unnest/variant 经 join/pattern 三处失败驱动）。基准：64 语句/2 类型 84→20 µs、68→4 allocs（对 HEAD worktree 同基准）。
- 应用侧接入指引：docs §7（合并引擎、快速路径资格、发送/监听器建议、验证方法）。
- Verification：全量 test、定向 -race（含 accept-index/contained/unnest/variant/join/pattern/insert-into/named-window）、vet、gofmt、三条差分链 passing。
- Delegation：ExprSemanticsScout（表达式语义全谱，识别三条陷阱）、DispatchAcceptanceScout（acceptsEventType/指标审计/子查询/上下文/输出策略可观测面）双只读 scout；PerfParityReviewer 终审。
- [x] Unit A 实现/测试/门禁/文档
- [x] Unit B 实现/守护测试/门禁/文档
- [x] 应用侧指引 + 独立复审 + 提交推送两远端

### Previous work unit (prior)

Draft 4.366 ('performance-property-access-stateless').

- P0 属性访问元数据缓存 + P1 stateless 过滤特化；详见 CHANGELOG 4.366 与 `docs/esper-go-performance.md` §2.1/§2.2。`PerfParityReview` 两轮（round 1 FAIL：getter/method 背书属性可复用单次求值改变用户代码调用次数 → 计划增加 schema 声明/getter 排除/plain-name/appliesTo 身份校验；round 2 PASS）。提交 `cc93ba276`。

### Previous work unit (oldest)

Draft 4.365 ('dataflow-beacon-source').

- [x] Parallel read-only scouts dispatched. Go surface scout agent_b00d7a45 COMPLETE: (a) WithBeans GREEN via BeaconEventSourceWithUnderlying + Iterations:1 + Alias myfield=Literal("abc"); (b) Variable GREEN via RegisterVariable + IterationsExpression: VariableRef (precedent dataflow_beacon_event_test.go:182-214); (c) Fields YELLOW — Avro/EventBean/underlying/param-URI all GREEN but Math.random() must be pinned deterministically and the tryInvalidCompile "requires one output stream" has no Go Build validation (documented difference or new check); (d) NoType GREEN — untyped empty []any per iteration, persistent mode until cancel. Timing: InitialDelay/Interval are real-time (not virtual clock), so wall-clock assertions stay Go-test-side. The constructor-with-args MyEventNoDefaultCtor maps to zero-value allocation (already a documented intentional difference in case.dataflow-beacon's difference field). Java contract scout agent_2490b1af in flight.
- [x] Contract frozen; writer agent_12d52391 delivered the Java assets (oracle + scenario + Java trace, 4 records). Delivered deterministic scope: (0) WithBeans 2 standalone sub-runs × 1 capture {myfield:"abc"} each (MyLegacyEvent bean + no-default-ctor bean via setter); (1) Variable 1 latch-3 capture delivering 3 empty-Map rows via var_iterations=3 resolved at instantiation; (2) NoType sub-run 2 iterations:5 × 5 empty Object[0] rows (undeclared type forges a transient object-array type); (3) NoType sub-run 5 instantiate-only (interval-0.5 p1-'abc' MyTestOAType graph, never started) × 0 rows — instantiation succeeding is the observable. Timing/initialDelay/unbounded/cancel sub-runs are real-time-only (not virtual-clock replayable) — dropped per the documented adaptation pattern. Fields execution (Math.random + EventBusSink polling, non-deterministic) stays out of the chain, covered by the existing go-unit family. Empty Map and empty Object[0] underlyings both normalize to {'kind':'row','fields':{}}; each capture read is one record over flow:DefaultSupportCaptureOp with case-local sequences (4 records: 2+1+0+1).
- [x] Go runner + mode + tests built (primary): dataflow_beacon_source.go four case builders; run.go 'dataflow-beacon-source' mode; run_test.go family (DiffWritesPassingEvidence / DirectReplay / DiffRejectsTraceMutations) all PASS.
- [x] Evidence pipeline, manifest DV upgrade, roadmap, CHANGELOG. (Evidence passing, 0 differences, 4 records per side; manifest case.dataflow-beacon → differential-verified with per-case dv list [WithBeans, Variable, NoType], Fields documented in difference; summary dvCases 251→252, dvRuntimeIds 923→926, assoc 3556 / referenced 3291 / unreferenced 845 unchanged (4 IDs pre-associated by the case); removed the summary's dead differentialVerifiedRuntimeIDs key that no Go struct field maps to; roadmap + CHANGELOG 4.365 supplements newest-first.)
- [x] Gates + independent parity review. Gates all green post-fix (parity 179s / compat / esper, make check exit 0, git diff --check clean). Review round 1 (reviewer agent_e84296d5-92fd-4e57-9b6a-e413b8c2c3d9) OVERALL FAIL: P1 — Go with-beans ran MyLegacyEvent in BOTH sub-runs, never exercising the MyEventNoDefaultCtor sibling; P2 — beacon-variable used Start + time.Sleep(500ms) instead of deterministic blocking completion; P3 — sub-run flow renames MyDataFlowOne-0/-1 without teardown; P3 — instantiate-only graph carried an extra capture operator absent from the Java sink-less graph. All four fixed in-unit: second struct registered as "MyEventNoDefaultCtor" used in sub-run 1 (rows unchanged); variable case drains via blocking Run; flow name "MyDataFlowOne" reused with a fresh Environment+Engine per sub-run (same runtime URI, explicit Close — Build rejects duplicate definition names with no removal API and Cancel errors on completed instances, so fresh-slate is the faithful mirror of Java undeployAll + recompileDeploy); capture operator dropped from the instantiate-only graph. Go trace + evidence regenerated from the fixed runner (evidence passing, 0 differences, java==go records equal, 4 records). Review round 2 (same reviewer) OVERALL PASS: fixes verified with engine-source-backed justification of the fresh-engine adaptation, programmatic trace/evidence equality, test-family adequacy confirmed, scope clean (zero internal/esper changes). Residual non-blocking nit: case 0's outer env/engine goes unused by its sub-runs.
- [x] Commit and push (single semantic commit with all checkpoint edits folded in; no post-commit checkpoint-only push).

## Delegation checkpoint
Draft 4.369 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'Java contract scout agent_153445d4' (byte-exact EPL/graph strings, send sequences, assertion observables, determinism verdicts and invalidity disposition for EventBusSource{AllTypes,SchemaObjectArray} + Filter{AllTypes,Invalid}; runtime-ID mapping against java-execution-inventory.jsonl) and 'Go surface scout agent_cec3c3ce' (EventBusSource/Filter builder surface, eventbus ingress dispatch semantics, representation normalization, per-execution GREEN/YELLOW/RED with case-builder sketches) over the 4.356-4.365 precedents.
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

perf-compile-prune-round3 unit:
- Delegation gate: two read-only scouts in parallel — 'ExprSemanticsScout' (exact eval semantics of all stateless-eligible kinds incl. the As zero-value contract for string predicates, eq-vs-ordering numeric policy split, pureBuiltin construction sites) and 'DispatchAcceptanceScout' (acceptsEventType name/variant/supertype semantics, sourceNodeAcceptsEvent per-kind routing incl. routed StreamType early-exit, metrics/audit gating, subquery/context/output-policy observability for type-mismatched statements). Independent 'PerfParityReviewer' reviewed both units' final diff.
- Primary owns all writes (shared surface single-writer), tests, benchmarks, docs, gates, commit, push.


perf-dispatch-allocation-round2 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'ListenerSnapshotScout' (complete listeners mutation census: 6 write sites incl. cleanupPreparedStatementLocked + markClosedLocked, dispatchSync the only structural reader, metrics.go len() reader, nextSubID monotonic ⇒ ascending ID = subscription order) and 'SendPathLifetimeScout' (send-local slice lifetimes incl. &-pointer gating, finishExternalRoutes control flow, typeNames cache replace-only semantics; delivered post-edit and doubled as implementation review). Independent 'PerfParityReviewer' reviewed the final diff.
- Primary owns all writes (shared semantic surface single-writer), docs, evidence, gates, commit, push.

Draft 4.365 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowBeaconJavaContract' (byte-exact graphs, BeaconSource configurations, expected outputs, determinism for all four executions) and 'DataflowBeaconGoSurface' (BeaconSourceWithOptions parameter/config surface, capture patterns) over the 4.356-4.364 precedents.
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

Draft 4.364 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowDocSamplesJavaContract' (byte-exact graphs/flows, expected outputs, determinism for both executions) and 'DataflowDocSamplesGoSurface' (Go graph API mapping over the 4.356-4.360 precedents).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

Draft 4.363 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'SubselectInvalidJavaContract' (byte-exact invalid/SODA expectations) and 'SubselectInvalidGoCoverage' (existing Go pins, representable Build rejections, difference dispositions). SERIAL SCOPE NOTE: the unit adds no new trace artifacts — manifest dispositions plus small Go Build-rejection tests are primary-owned end to end; the parallel scouts cover the investigation.
- Primary owns the manifest dispositions, Go tests, facts, gates, review, commit, and push.

Draft 4.362 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowEPStatementSourceJavaContract' (byte-exact graphs/statement wiring, lifecycle, expected outputs, determinism for all four executions) and 'DataflowEPStatementSourceGoSurface' (the Go EPStatementSource operator family contract: statement subscription, output capture, filtering, dynamic statement names) over the dataflow_types/dataflow_select_flows precedents.
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

Draft 4.361 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowSelectRepresentationJavaContract' (byte-exact graphs, wrapper representation semantics with/without additional properties, expected outputs, determinism) and 'DataflowSelectRepresentationGoSurface' (the Go Select operator's representation options over the 4.356-4.360 precedents).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

Draft 4.360 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowSelectStateJavaContract' (byte-exact graphs, output-rate-limit and time-window-triggered semantics, virtual-time requirements, expected outputs, determinism) and 'DataflowSelectStateGoSurface' (SelectTimeWindow/SelectWithOptions virtual-clock and snapshot contracts, rate-limit analogs, determinism guarantees).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the oracle/scenario/Java-trace assets after contract freeze.

Draft 4.359 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowSelectFlowsJavaContract' (byte-exact graphs/flows, iterate/final-marker semantics, join ordering, expected outputs, determinism) and 'DataflowSelectFlowsGoSurface' (Select/SelectIterate/SelectIterateOnFinalMarker/SelectJoin operator contracts, output capture, runner conventions).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the new oracle/scenario/Java-trace assets after contract freeze.

Draft 4.358 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowCSSDJavaContract' (byte-exact graph/lifecycle sequencing, state transitions, determinism for both executions) and 'DataflowCSSDGoSurface' (Start/Stop/Cancel/Join semantics, state observability, run/captive modes over the 4.356/4.357 precedent).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the new oracle/scenario/Java-trace assets after contract freeze.

Draft 4.357 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowOpLifecycleJavaContract' (byte-exact graph construction, op factories, lifecycle sequencing, expected outputs, determinism for all three executions) and 'DataflowOpLifecycleGoSurface' (typed events, CustomSource/SourceProvider, OperatorProvider keyed injection, Start/Join/Cancel semantics over the 4.356 precedent).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the new oracle/scenario/Java-trace assets after contract freeze.

Draft 4.356 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'DataflowTypesJavaContract' (byte-exact graphs/declarations for EPLDataflowBeanType and EPLDataflowMapType, op wiring, expected outputs, determinism) and 'DataflowTypesGoSurface' (the Go dataflow runtime API over the dataflow_connector.go differential precedent: graph construction, type declarations, source/sink, output capture, runner conventions).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the new oracle/scenario/Java-trace assets after contract freeze.

Draft 4.355 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'FromClauseMethodVariableJavaContract' (byte-exact statements, the Java method-source class shape and signatures, variable/constant/context-variable binding semantics, Map-and-OA projection shapes, invalid expectations, runtime IDs) and 'FromClauseMethodVariableGoSurface' (FromMethod/FromMethodOn/MethodProvider binding semantics, variable re-evaluation per trigger event, invalid validation errors, NEW-chain mode/run_test wiring).
- Primary owns the Go runner, mode wiring, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; the writer owns the new oracle/scenario/Java-trace assets after contract freeze.

Draft 4.354 unit:
- Delegation gate: two read-only scouts dispatched in parallel — 'PreevalOffGoEngineDesign' (option surface, acceptance-deferral semantics, exact blast radius over Statement.process runtime.go:6878 and the subquery registry, self-subselect meaning in the Go engine, regression set pinning preeval-on) and 'PreevalOffJavaOracleWiring' (pinned configuration API, MultiMatchHandler posteval semantics, byte-exact select-* rendering for the two new records, full oracle+script change list).
- Single-writer rule: the engine change touches internal/esper — the primary is the only implementation writer; the asset writer owns only chain-B oracle/scenario/trace files after contract freeze.
- Primary owns the engine option, runner case, tests, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.353 unit:
- Scouts (parallel, complete): Java contract agent_713fd793 (wildcard byte-exact, 4 sends/2 records, equals-on-left semantics, chain-A oracle audit incl. mirror-class + instance-registry requirement); Go surface agent_20b5abba (wildcard-in GREEN; NoPreeval RED-for-runner-only — selfSubselectPreeval is a Java runtime configuration the Go engine lacks → rescope + engine-unit deferral).
- Asset writer agent_0f430414: oracle + scenario (append-only) + Java trace regeneration (52→54, prefix byte-identical). Primary: Go runner case, decode, metadata, mutations, evidence, manifest, facts, gates, reviewer dispatch, commit.
- Reviewer agent_65d24f72: OVERALL PASS; P3 registration gating folded in; P3 DV-list convention tension → deferred consolidation unit.

Draft 4.352 unit:
- Scouts (parallel, complete): Java contract agent_a2ba0fa0 (7 statements byte-exact, per-case record structures); Go surface agent_cd0357b6 (all GREEN; SubquerySum-inside-JoinMany.Select spike-validated by primary before freeze).
- SERIAL EXCEPTION: asset-writer dispatch failed at spawn on the platform 5-hour usage cap; primary absorbed oracle/scenario/Java-trace scope.
- Primary: Go runner cases, oracle/scenario/Java-trace assets (serial exception), evidence, manifest, facts, gates, commit. Reviewer agent_5b3fb6ec: OVERALL PASS; two P3s fixed in-unit.

Draft 4.351 unit:
- Delegation gate: the ords 0/2/4 contracts and Go surfaces were pinned by the 4.350 re-audit scouts. Asset writer dispatched for the Java-side assets; primary owns the Go runner and tests.

Draft 4.350 unit:
- Delegation gate: the ords 3/5/1 contracts and Go surfaces were pinned by the 4.349 re-audit scouts (agent_9dcc7ff0 / agent_c0734ec4). Asset writer agent_e4551389 authored oracle/script/scenario/trace for 3 cases.
- Primary owns the runner, tests, evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.349 unit:
- Prefetch scouts (read-only, concurrent, dispatched): 'ViewGroupExpressionGroupwinJavaContract' pins ord 17 byte-exact (datetime-method key chain, send/assert sequence, exact rows) and re-audits epl-other-as-keyword-backtick (EPLOtherAsKeywordBacktick.java: the 4.335-era deferral cited 37 oracle compile errors — the multi-module oracle harness now deployed in 4.346-4.348 may resolve them); 'ViewGroupExpressionGroupwinGoSurface' adjudicates the Go expression-key groupwin surface (GroupWindow with a non-field Expr key) and the as-keyword/backtick surface with file:line evidence.

Draft 4.348 unit:
- Prefetch scouts (read-only, concurrent, dispatched): 'ViewGroupReclaimJavaContract' pins ords 2/3/9 byte-exact (hint texts, advance-time timelines, schedule-count assertions, flipTime semantics) and 'ViewGroupReclaimGoEngineDesign' designs the view-level reclaim + schedule-count introspection with blast radius over shipped grouped-view tests.

Draft 4.349 unit:
- Prefetch scouts (read-only, concurrent, complete): 'ViewGroupExpressionGroupwinJavaContract' (agent_9dcc7ff0-0fbd-4115-a3ee-932827e1c1cf) pinned ord 17 byte-exact and re-audited as-keyword-backtick (7/7 representable, 4.335 blockers stale); 'ViewGroupExpressionGroupwinGoSurface' (agent_c0734ec4-f5bc-4357-838f-860cae83fddc) adjudicated GroupWindow arbitrary-Expr keys as GREEN (keyed on expression VALUE at runtime.go:14749-14763) and found all as-keyword surfaces represented.
- Asset writer (isolated, parallel, same writer continuity): owns viewgroup-merge-view scenario/oracle/trace extension for ord 17.
- Primary owns the runner case builder, run_test extension, evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.347 unit:
- Prefetch scouts (read-only, concurrent, dispatched): 'ViewGroupTimeWindowsJavaContract' pins ords 10-13/16 byte-exact contracts (advance-time timelines, IR pairs, group-key split) and 'ViewGroupTimeWindowsGoSurface' adjudicates the Go time-window/virtual-time surface (TimeBatch/TimeAccum/TimeOrder/TimeLengthBatch/TimeWindow specs, AdvanceTime) with file:line evidence.
- Asset writer (isolated, parallel): owns viewgroup-merge-view scenario/oracle/trace extension for the 5 cases.
- Primary owns runner time-window builders, run_test family extension, evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.346 unit:
- Prefetch scouts (read-only, concurrent, complete): 'ViewGroupDerivedValueJavaContract' (agent_05f00b11-bb25-450c-8910-145715962a68) pinned the full byte-exact contracts of ords 1/4/5/6 (all 16 ord-1 assertLastNewRow pins with exact double arithmetic, ord-4/5 progressions, ord-6 IR pairs + iterator, per-statement listenerReset semantics, bean shapes); 'ViewGroupDerivedValueGoDesign' (agent_74507338-028e-4831-87d3-7515a757d3b3) adjudicated the ord-6 shape as already-covered by the injected grouped path and designed the validation-first plan.
- Asset writer (isolated parity-asset writer, two rounds with continuity): agent_d10fafe3-8fb9-4f33-8f4d-b2f4ecb6e72d authored the initial oracle/script/scenario/trace trio (ords 0/6/14), trimmed ord 6 on contract change, then extended with ords 1/4/5/6 (85 records, multi-module stats deploys, NaN {"state":"nan"} convention) and canonicalized per-send record order to deployment order (Java dispatches same-event statements in reverse deployment order; the suite only asserts per-listener).
- Primary owns internal/esper engine/accessor changes (this unit: additive LinearRegression accessors, UnivariateStatistics one-pass formula switch, compat numeric canonicalization, deploy-time persistent grouping-injection relocation, runner snapshot op + 6-case builders), run.go wiring, run_test family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push. Reviewer agent_21bfba34-124b-47aa-bf38-838eeac168bc findings (2 P2 prose/formula, P3 hygiene) addressed in-unit.

Draft 4.345 unit:
- Prefetch scouts (read-only, concurrent, complete): 'ViewGroupJavaContract2' (agent_ac68b9d7-6306-4012-9439-8ff8b1030b99) pinned all 20 executions with the ord-0 merge-view mechanism verified in Java source (GroupByViewImpl/MergeView/GroupByViewUtil; ResultSetProcessorRowPerEventImpl AGGREGATED_UNGROUPED; applyAggViewResult new-then-old), classified virtual-time/compile-only/performance/serder executions, and recommended ords 0/6/14; 'ViewGroupGoEngineScout' (agent_153be8e5) located the Go groupwin architecture (windowRuntimeState.groups/groupOrder; delta path already cross-group) and the exact root cause (implicit aggregate group-by injection gated on aggregateDefinitionReadsNonKeyEvent at runtime.go:18098), the minimal fix design with blast radius, and the regression test plan.
- Asset writer (isolated parity-asset writer, concurrent with primary engine work): owns tools/java-oracle/ViewGroupMergeViewScenarioOracle.java, tools/java-oracle/run-viewgroup-merge-view.sh, testdata/parity/viewgroup-merge-view.json, testdata/parity/viewgroup-merge-view.trace.json.
- Primary owns internal/esper engine fixes, the view_group_intersect re-model, the app/parity runner (view_group_merge_view.go), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.344 unit:
- Delegation gate: the ord-10 sub-case list and dispositions were pinned by the 4.342 scouts (agent_53742bea / agent_5ec65a8f). The unit is implemented-only (invalidity policy: no oracle/scenario/trace assets) and every remaining file is primary-owned; no agent task has an independent scope. Serial exception recorded here.

Draft 4.343 unit:
- Delegation gate: the ords 5-8 contracts, Go shapes and the three engine gaps were pinned by the 4.342 scouts (agent_53742bea / agent_5ec65a8f) covering all 7 open executions of the class. The engine implementation is primary-only (single writer on shared internal/esper semantics per AGENTS.md); the disjoint scenario/oracle/trace asset extension is dispatched to an isolated asset writer in parallel.

Draft 4.342 unit:
- Prefetch scouts (read-only, concurrent, complete): 'EventPrecedenceJavaContract' (agent_53742bea-99c2-4b05-bdff-e65a0aa5f428) pinned all 7 open executions (SODA is compile-path only; exact flattened id orders for ord 4; ord 9 batch interleave; ord 10's 7 invalid sub-cases with message sources) and recommended ords 5-8 as the subquery-precedence unit; 'EventPrecedenceGoSurface' (agent_5ec65a8f-17a8-48df-bbe1-9753bd9e5820) adjudicated ord 4 + ord 9 READY with file:line precedents and identified the three concrete engine gaps for ords 5-8 plus the ord 10 dispositions.
- Asset writer (isolated parity-asset writer, concurrent with primary test work): owns tools/java-oracle/EPLInsertIntoEventPrecedenceScenarioOracle.java, testdata/parity/insertinto-event-precedence.json, testdata/parity/insertinto-event-precedence.trace.json (regenerated, validated end-to-end ×2).
- Primary owns internal/esper/epl_insert_into_event_precedence_parity_test.go (2 new tests), evidence JSON extension, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.341 unit:
- Delegation gate: the ord-12 case list and Go surface dispositions were pinned by the 4.338 scouts (agent_fae76b58 / agent_8cd5b2e7) covering all 9 open executions of the class; the unit changes no oracle/scenario/trace asset (invalidity policy: implemented-only Go unit tests) and every remaining file is primary-owned, so no agent task — scout or writer — has an independent scope. Serial exception recorded here.

Draft 4.340 unit:
- Delegation gate: the two executions' contracts and Go surfaces were pinned by the concurrent 4.338 scouts (agent_fae76b58 / agent_8cd5b2e7) covering all 9 open executions of the class; no new independent read-only scout task exists for ords 9/10, and the only disjoint write task is the scenario/oracle asset extension, dispatched to an isolated asset writer while the primary owns the Go tests and central facts.

Draft 4.339 unit:
- Delegation gate: the two executions' contracts and Go surfaces were pinned by the concurrent 4.338 scouts (agent_fae76b58 / agent_8cd5b2e7) covering all 9 open executions of the class; no new independent read-only scout task exists for ords 7/8, and the only disjoint write task is the scenario/oracle asset extension, dispatched to an isolated asset writer while the primary owns the Go tests and central facts.

Draft 4.338 unit:
- Prefetch scouts (read-only, concurrent, complete): 'PopulateUnderlyingJavaContract' (agent_fae76b58-ea6e-4c11-bb16-fea6007cd227) pinned all 9 open executions with stages/IDs/bean constructors and recommended ords 0/1/2/11 first; 'PopulateUnderlyingGoSurface' (agent_8cd5b2e7-49b6-4bea-a62f-ba76ee7fa6c4) adjudicated all four READY with file:line precedents (route validation plan.go:1428-1646, mergeSchemaUnderlying/setStructField schema.go:3027/3201, MatchUntil/TagEvents, AggregateStream.InsertInto stream.go:1718, WithJSONDefaults schema.go:331) and the approved-difference set (column-list-ignored, single-into-array wrap, numeric widening).
- Asset writer (isolated parity-asset writer, concurrent with primary test work): owns tools/java-oracle/EPLInsertIntoPopulateUnderlyingScenarioOracle.java (4 new case registrations), testdata/parity/epl-insert-into-populate-underlying.json (4 new cases), testdata/parity/epl-insert-into-populate-underlying.trace.json (regenerated, validated end-to-end ×2).
- Primary owns internal/esper/epl_insert_into_populate_underlying_parity_test.go (4 new tests), evidence JSON extension, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.337 unit:
- Delegation gate: the two executions' contracts and Go surfaces were pinned by the concurrent 4.336 scouts (agent_1a89be3a / agent_f2bc15ed) covering all 9 open executions of the class; no new independent read-only scout task exists for ords 1/4, and the only disjoint write task is the scenario/oracle asset extension, dispatched to an isolated asset writer while the primary owns the Go tests and central facts.

Draft 4.336 unit:
- Prefetch scouts (read-only, concurrent, complete): 'ReboolJavaContract' (agent_1a89be3a-5ef4-4052-be50-25c263461987) pinned all 9 open executions with exact EPL/timelines/IDs and recommended the 6/7/8/12/13 slicing; 'ReboolGoSurface' (agent_f2bc15ed-4620-47f2-8fe7-7462016bf308) adjudicated per-execution READY with API precedents (RegexpMatch expr.go:2366, Like expr.go:2370, ContextPatternField expr.go:207, CreatePatternInitiatedContext context.go:1614, TagField, VariableRef/ConstantVariable), the filter-start→pattern-start adaptation, and the ord 11/2 dispositions.
- Asset writer (isolated parity-asset writer, concurrent with primary test work): owns tools/java-oracle/FilterReboolScenarioOracle.java (SupportBean_S1 registration), testdata/parity/filter-rebool-optimizable.json (5 new cases), testdata/parity/filter-rebool-optimizable.trace.json (regenerated, validated end-to-end ×2).
- Primary owns internal/esper/expr_filter_optimizable_boolean_limited_parity_test.go (5 new tests), evidence JSON extension, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push.

Draft 4.335 unit (recorded pre-implementation): prefetch scouts for this unit ran in the prior session ('SelectExprScout' agent_f18af34d Java contract; 'SelectExprGoSurface' agent_710001ea Go surface, adjudicated zero engine work for ords 8/9). The isolated asset writer's deliverable (oracle + run script + validated Java trace) was already committed at `b110f3f5d`. Remaining tasks that session (scenario, Go runner, run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, push) were all primary-owned shared-surface or central-fact files with no file-disjoint independent task, so no second writer was dispatched; independent parity review agent_98d542c4 returned OVERALL PASS on all five dimensions with fresh end-to-end reproduction, and its single P2 (manifest note ordinal typo) was fixed and re-confirmed by the same reviewer.


Draft 4.329 unit:
- Prefetch scouts: 'OnUpdateJavaContract' (agent_480e7832, all 8 executions' contracts, complete before 4.328) + 'OnUpdateVariantsGoSurface' (`agent_8dd095cb-8ee4-484c-80f1-406e44f5ec68`, adjudicated ordinals 0/3/4/5 READY, zero engine work, with the approved-difference precedents and the listenerReset two-record convention).
- Asset writer (isolated parity-asset writer, concurrent with primary runner work, `agent_03d45f5c-0785-481b-ac5e-53a4d9ff1af5`): authored the oracle/script/scenario trio and validated the oracle end-to-end against the fixed tree; its probe-proven wrapper merge deviation (create+insert single deploy) was adopted by the primary into the runner pins.
- Primary owns the Go runner ('internal/app/parity/infra_named_window_on_update_misc.go'), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; primary fixed the doubled-@name constant bug and the missing WithOldStream on the subclass create consumer exposed by the first replay, and added per-case payload value pinning after the payload-value-drift mutation initially replayed.
Draft 4.333 delegation addendum (recorded post-push per the 4.331→4.332 precedent): independent parity reviewer 'CorrelCoerceParityReview' (`agent_4345de57-daf8-4e11-a510-f022b3ff98da`) initially FAIL on two P2 prose defects — the manifest/CHANGELOG/roadmap CreateIndex claim did not match the runner (index deploy was an empty marker) and the PLANS "InfraNWTableSubq* family fully closed" overstatement — resolved by actually modeling the index (table/window CreateIndex("MyIndex", []string{"col2","col1"}, IndexHash, false) on the live infra, empty-deployment marker kept for undeploys) and correcting PLANS to "nearly closed ... correl-index-sharing remains implemented-not-DV"; P3s (wrong error-message ID, dead statements map, doubled heading) fixed; re-confirmed OVERALL PASS. Go trace regenerated post-fix, still 56/56 zero differences.

ViewGroup spike record (4.334 candidate re-scoped): both scouts complete ('ViewGroupJavaContract' read-only; Go surface via repo evidence). Of the 8 unreferenced executions, ord 6 (MultiProperty multi-key groupwin IR delivery) is parity-ready, ord 7 (Invalid compile messages) has no Go rejection surface, ord 8 is PERFORMANCE/wall-clock (excluded), ord 19 is SERDEREQUIRED compile-only, and the reclaim trio (ord 2/3/9) needs engine work (sweepReclaimGroups at runtime.go:18578 only covers aggregate group-by, not grouped VIEW retention, plus a schedule-count introspection surface). CRITICAL divergence found by spike: ord 0's `#groupwin(p1)#length(2)` + `sum(p2)` produces Java cross-group union sums (10/21/33/36 — upstream merge esper-8 semantics) but the Go engine computes per-group sums (10/11/22/25); closing this needs a dedicated engine unit re-modeling aggregate-over-groupwin scoping, with care for the shipped view-group matrix parity tests. ViewGroup deferred as a family until the engine scoping unit lands; candidates for the next zero-engine slice: EPLOtherStreamExpr (9), ExprFilterOptimizableBooleanLimitedExpr (9), EPLOtherPlanInKeywordQuery (9).

Draft 4.330 unit:
- Prefetch scout 'InsertFromScout' (`agent_fdcfb9e3-d4c8-47ac-942d-05016acdee45`, read-only, ran during the 4.329 review): pinned all 7 executions of InfraNamedWindowInsertFrom.java, the seed/keyword/where semantics, Go surface precedents (CreateNamedWindowQuery, Filter-preserving namedWindowDirect, Like, InsertNamedWindow), and recommended the 0/1/5/6 slicing with 2/3/4 deferrals.
- Asset writer (isolated parity-asset writer, concurrent with primary runner work, `agent_12f75aa2-8576-4285-b4cc-b16c097a8310`): authored the oracle/script/scenario trio, validated the oracle end-to-end, and corrected the primary's step totals (50 not 47) and the B9 route payload.
- Primary owns the Go runner ('internal/app/parity/infra_named_window_insert_from.go'), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; primary fixed the lenient listener leak (per-case subscribe sets) and the ord-1 span recount exposed by the first replay.
Draft 4.331 unit:
- Prefetch scout 'AggregateFilterNamedParameterScout' (agent_a78845dd-992b-4741-9b91-eb88622ac976, read-only, ran during the 4.330 review; COMPLETE): real gap = 8 unreferenced executions (ord 9/11/13/15/17/18/19/20), slicing + spike list delivered.
- Spikes resolved by the primary before implementation (zero engine work): join aggregate group rows are joinTuples and FilterAggregate predicates bind via JoinField (ctx.Event fallback, expr.go:490-517 + expr.go:3346-3360); first(*,filter).theString navigation not needed for this slicing.
- Asset writer (isolated parity-asset writer, concurrent with primary runner work, `agent_34f6437a-561c-4332-bdc6-c5a3db218c67`): authored the oracle/script/scenario trio and validated end-to-end; corrected the step enumeration, confirmed the no-space EPL concatenation and Integer[]/Map[] rendering conventions.
- Primary owns the Go runner ('internal/app/parity/resultset_aggregate_filter_named_parameter_linear_join.go'), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; primary fixed CountEver non-generic form, the lenient... (n/a here) mixed-event expectation shape (post-LoadTrace map form), and the ord-1 span/index recount.
Draft 4.332 unit:
- Prefetch scout 'SortedJoinScout' (`agent_4ab62aa5-5513-449a-b98c-d7d0a53070ec`, read-only, ran during the 4.331 review; COMPLETE): pinned ord 15/17/18 contracts, adjudicated zero engine work, and delivered the JoinEventValue/SortedEventsBy compositional requirement for join sorted event-array columns.
- Asset writer (isolated parity-asset writer, concurrent with primary runner work, `agent_dc05d3dc-2ad0-4ffc-b331-c13c1a6d1b54`): authored the oracle/script/scenario trio, validated end-to-end twice (byte-identical traces), and caught the primary's rotated bound-case row table.
- Primary owns the Go runner ('internal/app/parity/resultset_aggregate_filter_named_parameter_sorted_join.go'), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; primary fixed the ord-18 stream-bound filter bug exposed by the first replay and the multicriteria-order-swap mutation that initially encoded the original order.
- Post-validation: independent parity reviewer 'SortedJoinParityReview' (`agent_8e20aaed-aeda-4492-af7f-8aa3ed85b229`) returned OVERALL PASS on all five dimensions with two P3 documentation nits (doubled heading fixed here; this reviewer line added here per unit convention).

Draft 4.328 unit:
- Prefetch scouts (read-only, concurrent, ran during the 4.327 review): 'OnUpdateJavaContract' (`agent_480e7832-b141-4b55-a5d2-f16eebcc1493`) pinned all 8 executions, per-execution EPL/timelines/assertions, update old/new dispatch semantics (OnExprViewNamedWindowUpdate single dispatch, zero-match no-invocation), and the static/inventory id-namespace quirk (inventory file-level id java-0b7af7284d24833e1753 coincides with PrimitiveArray's class id; per-class static ids come from static-manifest.json).
- 'OnUpdateGoSurface' (`agent_79ed2dad-cd95-4dd8-b0b6-c1a7ecc4d486`) adjudicated all items READY, zero engine work: TriggerStream.UpdateNamedWindow (trigger.go:522), SetColumn/CopyMatchingFields, EqualOf array branch, IntersectWindows/UnionWindows retention precedents, update old/new engine dispatch (trigger.go:2535-2568), snapshot op, and the union-update branch flagged as the only untested runtime path (this unit's union case exercises it for the first time).
- Asset writer (isolated parity-asset writer, concurrent with primary runner work, `agent_c0087c09-7131-41ff-925c-0d0608c04992`): authored 'tools/java-oracle/InfraNamedWindowOnUpdateScenarioOracle.java', 'tools/java-oracle/run-infra-named-window-on-update.sh', 'testdata/parity/infra-named-window-on-update.json'; ran the oracle end-to-end against the real Java tree and validated the trace against every Java assertion.
- Primary owns the Go runner ('internal/app/parity/infra_named_window_on_update.go'), run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push; primary synced the runner pins after the writer's @public/trigger-sends corrections and made both sides emit mode-any snapshots in canonical row order.
- Post-validation concurrent batch: independent parity reviewer 'OnUpdateParityReview' (`agent_c6e06cce-1f2d-4d22-a7e1-97ff302f9d45`) returned OVERALL PASS on all five dimensions; the single P2 (stale totals line in PLANS.md, 42→46 steps) was fixed and re-confirmed by the same reviewer; one P3 (milestone steps omitted per family convention, no observable effect). N+1 prefetch scout 'OnUpdateVariantsGoSurface' (`agent_8dd095cb-8ee4-484c-80f1-406e44f5ec68`, read-only) dispatched concurrently for the deferred executions 0/3/4/5 unit. Serial-exception note: no separate Java-contract scout is dispatched for N+1 because the completed OnUpdateJavaContract already pinned all 8 executions' contracts including 0/3/4/5; the only remaining independent read-only task is the Go-surface adjudication of the four variant adaptations.

Prior 4.327 unit (shipped as 1d21f99ba): prefetch scouts 'FilteredCorrelJavaContract' (`agent_9cd7d0cb-d1c5-4f9f-8110-d3df3f13b291`) + 'FilteredCorrelGoSurface' (`agent_57aa3160-d458-440e-ad67-1e225252dd04`) ran concurrently before implementation; asset writer `agent_3667f273-434c-4eaf-b4aa-db55769db5a1` authored the oracle/script/scenario trio on disjoint files; primary owned the runner, run.go wiring, run_test.go family, traces/evidence, manifest, roadmap, CHANGELOG, PLANS.md, gates, review, commit, and push, and fixed the oracle S0 send-type mapping during bring-up. Independent parity review `FilteredCorrelReview` (`agent_6761c0e0-3c0e-4c49-9eab-69cd92229352`) returned OVERALL PASS on all five dimensions; the single P2 (stale "80 steps") was fixed and re-confirmed by the same reviewer.

Prior 4.326 unit (shipped as 664e19b63): prefetch scouts 'JavaContract4326' (agent_31eba2d1-2f6b-4257-83a4-71ae727035cf) + 'GoSurface4326' (agent_d0b156be-478b-4e5c-86a5-694cee174254) ran concurrently before implementation; isolated asset writer owned the oracle/script/scenario trio; primary owned runner/wiring/tests/traces/evidence/manifest/roadmap/CHANGELOG/gates. Independent parity review `StartStopParityReview` (agent_d2b3179a-3a0c-46e1-930d-417ec16e963f) returned PASS with no P1/P2 findings; three P3 notes recorded (validator does not pin positional interleaving of undeploy/snapshot steps; create-table constant rename; latent TraceRecord.New omitempty vs Java always emitting "new" for snapshots).

## Prior outcomes
Draft 4.301 is frozen as all 7 executions of `ResultSetQueryTypeRowPerEvent.java` (ordinals 0-6; runtimes `java-runtime-5111b05c6bc620b88e15`, `java-runtime-50601d6f0cc0411a9f90`, `java-runtime-06c962063c57e3a3adee`, `java-runtime-14d3e2b22e8c3ef00657`, `java-runtime-1f1dae3e5953610a77e3`, `java-runtime-9160c96fccf23486d782`, `java-runtime-cce782d69a46b20b8609`; flags empty). Scenario, Java oracle, Go runner, manifest/evidence, full gates, review and commit are complete; continue from remaining resultset order-by/output/querytype executions.

- Previous unit (Draft 4.261, implemented, review PASS, pending commit):
  case.variables-use closed EPLVariablesUse at 9/11 executions (101/101
  records, 0 differences; legacy 11 records byte-identical). Batch:
  `VURJavaContract`+`VURGoSurface` read-only scouts; frozen contract in
  local vur-contract.md; `VURCoreWriter` (internal/esper) and `VURAssets`
  (parity-asset-worker, isolated: oracle/scenario/runner/run_test) wrote
  concurrently on disjoint files. Engine changes: Java-widening coercion
  matrix (Byte<Short<Integer<Long<Float<Double), verbatim runtime set-path
  diagnostics with javaTypeName rendering, compile-time const inner
  sentences, equalValuesUnwrapped membership matching, map-payload
  materialization + string->named-kind coercion, per-slot multi-match IN
  delivery (Java FilterParamIndexIn flattened slots [V2,V1,V2] double-
  deliver ENUM_VALUE_2). Runner/oracle: label-keyed deployments, targeted
  undeploy op mirroring undeployModuleContaining, persistent compiled-
  module path with compileWithoutPath SODA re-create bypass, nullable
  FullBean rendering, bare-message error-chain walk. `VURReview` verdict
  PASS (1xP2 scratch-file hygiene — fixed; 4xP3 documentation notes —
  comments added at both Or-shape pins and the runtime.go per-slot
  boundary; plan.go compile-time unknown-variable sentence registered as
  future parity item; evidence scenario normalization is convention-
  consistent). Registered unrepresented: DotSeparateThread, WVarargs,
  byte[] boxed/primitive distinction, SupportBean[] declaration rejection.
- Closed prefetch (Draft 4.265): `ResultSetAggregateCountSum.java` ordinals 1–5 were subsequently shipped with the complete 9-execution differential case in commit `a439cff87`; the earlier `CountSumJavaContract` and `CountSumGoSurface` scouting notes remain historical context only. No CountSum writes are pending.
- Current N+1 scouts are `MinMaxJavaContract` (java-oracle-scout) and `MinMaxGoSurface` (scout), both read-only and complete. Their exact runtime IDs, source contract, API surface, allowed files and forbidden actions are recorded in the Draft 4.276 checkpoint above.
- Primary owns shared semantics, parity runner integration, generated trace/evidence, manifest, roadmap, CHANGELOG, validation, commit and push; no subagent may modify those central facts.
- Previous work unit (Draft 4.261, implemented, review PASS, pending commit):
  case.variables-use closed EPLVariablesUse at 9/11 executions (101/101
  records, 0 differences; legacy 11 records byte-identical). Batch:
  `VURJavaContract`+`VURGoSurface` read-only scouts; frozen contract in
  local vur-contract.md; `VURCoreWriter` (internal/esper) and `VURAssets`
  (parity-asset-worker, isolated: oracle/scenario/runner/run_test) wrote
  concurrently on disjoint files. Engine changes: Java-widening coercion
  matrix (Byte<Short<Integer<Long<Float<Double), verbatim runtime set-path
  diagnostics with javaTypeName rendering, compile-time const inner
  sentences, equalValuesUnwrapped membership matching, map-payload
  materialization + string->named-kind coercion, per-slot multi-match IN
  delivery (Java FilterParamIndexIn flattened slots [V2,V1,V2] double-
  deliver ENUM_VALUE_2). Runner/oracle: label-keyed deployments, targeted
  undeploy op mirroring undeployModuleContaining, persistent compiled-
  module path with compileWithoutPath SODA re-create bypass, nullable
  FullBean rendering, bare-message error-chain walk. `VURReview` verdict
  PASS (1xP2 scratch-file hygiene — fixed; 4xP3 documentation notes —
  comments added at both Or-shape pins and the runtime.go per-slot
  boundary; plan.go compile-time unknown-variable sentence registered as
  future parity item; evidence scenario normalization is convention-
  consistent). Registered unrepresented: DotSeparateThread, WVarargs,
  byte[] boxed/primitive distinction, SupportBean[] declaration rejection.
- Closed prefetch (Draft 4.260): epl/variable/EPLVariablesUse.java 10
  executions were prefetched by `VARJavaContract`/`VARGoSurface`; four
  shipped as Draft 4.260, the final two in-scope (EPRuntime API
  java-runtime-826b551e883c9398df67, ConstantVariable
  java-runtime-d273a38f6415e6c3ee62) shipped as Draft 4.261 above;
  DotSeparateThread (e1511c9dbe279de2adfe) and WVarargs
  (dd6fdf5b57faa2ba2600) remain permanently deferred with rationale in
  capability remaining.
- Closed work unit (Draft 4.262, shipped):
  EPLInsertIntoPopulateUndStreamSelect 3/4 executions differential-verified
  (47/47 records, 0 differences). Batch: `IUPJavaContract`+`IUPGoSurface`
  scouts; `IUPCoreWriter` (engine parity tests; zero engine changes needed —
  merge conditional insert, subtype→supertype column, cast chain all confirmed
  working) and `IUPAssets` (parity-asset-worker, isolated: oracle/script/
  scenario/runner/wiring). Key semantic pinned: non-map reps use explicit-
  Alias equivalents for transpose+extra columns (Build gate freeze stands);
  exec1 phase split runs JVM-per-invocation (undeployAll does not release
  @public path types). Exec3 Invalid registered implemented-not-DV with
  go-unit pins + verbatim Java texts preserved in capability remaining.
  `IUPReview` verdict PASS (2xP3: script isolation comment reworded to the
  accurate path-type mechanism; TraceWriter double-bump noted — final
  numbering assigned by merge-step renumber).
- Prefetched unit (none): next selection returns to roadmap-driven pick.
- Current work unit (Draft 4.259, implemented pending review):
  expr/datetime/ExprDTRound.java all 4 executions differential-verified at
  7/7 records, 0 differences. Engine addition:
  internal/esper/expr_dt_round.go (DateTimeRoundCeiling/Floor/Half with
  representation preservation, value-zone calendar rounding, Apache Commons
  month-length carry) + facade re-exports; runner internal/app/parity/
  expr_dt_round.go with mid-case redeploy. Assets from `DTRAssets`
  (parity-asset-worker, isolated) on the `DTRJavaContract`/`DTRGoSurface`
  frozen contract. `DTRReview` verdict FAIL (1xP1 2xP2 2xP3), all fixed
  and re-verified: Commons MODIFY_CEILING adds one target unit
  unconditionally (on-boundary inputs advance; P1 repro vectors added),
  the FIELDS walk includes the MILLISECOND row so roundHalf sec/min
  carries reproduce, week errors at evaluation like Java's unsupported
  field path, unit normalization edge-trims only, caldate forces
  sub-second zero. The reviewer's midnight-crossing repro arithmetic was
  itself incorrect (00:00:05 - 5s stays on the same day, day offset 16
  still carries); corrected vector pins the Java-faithful June carry.
  Shipped as `657d33d2c`.
- Closed prefetch (Draft 4.259, superseded by the implemented unit above):
  expr/datetime/ExprDTRound.java 4 executions: Input
  (9838679b35a6a3507332), Ceil (8a02976a0c5c4eb03030), Floor
  (95d240ad18c89abe6e99), Half (be79bf345b96e2ccc206). `DTRJavaContract` adjudicated
  the earlier roundHalf('month') "oracle self-inconsistency" as WRONG: the
  2002-05-30→2002-06-01 assertion follows Apache Commons
  DateUtils.modify(MODIFY_ROUND) with month-length-dependent carry
  (31d→day≥17, 30d→day≥16, Feb28→≥15, Feb29→≥16); msec roundHalf is
  identity; exact ties round up; roundHalf supports Date/Long/Calendar only.
  `DTRGoSurface`: minimal surface = 3 builders (RoundCeiling/RoundFloor/
  RoundHalf) + shared kernel in internal/esper/expr_dt_round.go, runner
  internal/app/parity/expr_dt_round.go reusing expr_dt_between templates;
  representation preserved per input property (Date/Long/Calendar/LDT/ZDT).
- Current work unit (Draft 4.258, implemented pending review):
  event/map/EventMapCore.java 4 of 5 executions differential-verified at
  6/6 records, 0 differences (nested three-level MyMap with verbatim
  sender-rejection text, metadata introspection marker, beanA fragment
  navigation, raw HashMap re-send). InvalidStatement execution
  (da04541dce5715cf9129) unrepresented: the Go type-safe chained API
  rejects its three shapes at language compile time; documented under
  capability remaining. Engine fix: SendObjectArray wrong-kind message.
  Assets from the earlier prefetch worker, re-scoped by the primary after
  `EMCJavaContract`/`EMCGoSurface` scouts. Shipped as `e5f07221d`.
  `EMCReview`
  verdict PASS-with-findings (3xP2 3xP3, all fixed): association count
  corrected to 3267, scenario/evidence stale invalid-statement metadata
  purged, SendObjectArray unregistered-name clause completed per
  EventTypeUtility.getMessageExpecting, metadata SchemaMap-kind check
  added, remaining-rationale wording made accurate.
- Closed unit (Draft 4.257, commit `706d5bd76`):
  resultset.querytype-aggregate-grouped all 9 executions.
- Closed deferral: ResultSetOrderByRowForAll implemented in Draft 4.256
  after fixing the engine divergence (order-by alias-column snapshot
  resolution in batched deliveries). All 3 executions differential-verified:
  NoOutputRateJoin (e6f5c075be4979efc531, iterator-only snapshots),
  OutputDefault{join=false} (7642ad83057714f39501) and {join=true}
  (046ed3b9a90cf000c5c7); Java/Go 4/4 records, 0 differences. Runtime-ID
  mapping follows java-execution-inventory ordinals (join=false precedes
  join=true in executions()).
- Deferred unit (AggregateGrouped, NOT implemented):
  resultset/querytype/ResultSetQueryTypeAggregateGrouped.java (9 executions,
  all unreferenced). Three grouped-emit divergences documented with repros.
- Deferred unit (EventMapCore, NOT implemented):
  event/map/EventMapCore.java 5 executions. Probe implementation exposed
  Go-side complexity in nested Map event type registration (3-level
  nesting), cross-type sender rejection (ObjectArray sender to Map type),
  and Java-bean property navigation inside Map-typed values. These need a
  dedicated unit focused on Map event representation parity.
- Deferred unit (ExprDTRound, NOT implemented):
  expr/datetime/ExprDTRound.java 4 executions (all unreferenced). Go
  lacks DateTimeRound/Ceil/Floor builders entirely. Requires new datetime
  expression builders for three rounding modes across five representations
  (Date/Long/Calendar/LDT/ZDT), month-length-dependent half-carry
  thresholds, an apparent oracle self-inconsistency in roundHalf(month),
  and LDT/ZDT roundHalf runtime error paths.
  epl/variable/EPLVariablesUse.java 8 unreferenced executions. Scout
  investigation revealed extensive API contract surfaces: EPRuntime alone
  has ~20 distinct assertion points (typed get/set, atomic rollback,
  numeric coercion, error messages), ConstantVariable covers a 17-operator
  truth table plus constant write protection across four channels.
  Deferred until dedicated units can verify each surface against Java
  truth without rushing.

## Delegation checkpoint
- Draft 4.264 unit agents: `JavaTB`/`GoTB` read-only scouts ran concurrently
  before implementation; `ParityAssetsTB` authored disjoint oracle/scenario/
  runner assets; `TimeBatchCoreRepair` and `JoinBatchLifecycle` investigated
  the shared runtime boundary; `AnyModeCompatTests` authored disjoint differ
  regression tests; `WTBParityReview-2` reviewed the integrated diff and
  returned strict PASS with no findings. All delegated tasks skipped build,
  formatter, linter, and tests as required.
- Draft 4.265 prefetch: `CountSumJavaContract` (java-oracle-scout) and
  `CountSumGoSurface` (scout) ran concurrently. Their frozen candidate is
  ResultSetAggregateCountSum ordinals 1–5; no writes started.
- Primary agent owns shared semantics, parity assets, generated trace/evidence,
  manifest, roadmap, CHANGELOG, PLANS, validation, commit, and push.
  (parity-asset-worker, isolated) authored the oracle/scenario/script trio on
  the frozen contract; its structured return failed schema validation and the
  isolated worktree was discarded, so the primary recovered all three files
  from the session transcript and verified fidelity by regenerating a
  byte-identical pinned trace (recovery recorded as the serial exception).
  Shared-core writer: primary agent. Review pending.

- Draft 4.252 unit agents: prefetch scouts `TIIJavaContract`
  (java-oracle-scout) and `TIIGoSurface` (scout) ran concurrently before
  implementation; no asset writer (prefetched assets verified drift-free —
  recorded serial exception); `TIIParityReview` verdict PASS with one P3
  PLANS arithmetic fix. Primary-owned: the unkeyed message parity fix and
  two oracle harness fixes surfaced by the first differential run.


- Coercion unit agents: prefetch scouts `CoercionJavaContract`
  (java-oracle-scout) and `CoercionGoSurface` (scout) ran concurrently with
  the Draft 4.219 review; asset writer `CoercionOracle`
  (parity-asset-worker, isolated) authored the oracle extension while the
  primary agent wrote the scenario extension, runner branch, and mutations.
  File ownership was disjoint.
- Filtered unit agents: prefetch scouts `FilteredJavaContract`
  (java-oracle-scout) and `FilteredGoSurface` (scout) ran concurrently with
  the Draft 4.217 review; asset writer `FilteredOracle` (parity-asset-worker,
  isolated) authored the new Java oracle and runner script on the frozen
  scenario contract while the primary agent wrote the Go runner, dispatch,
  scenario, and tests. File ownership was disjoint.
- Cube unit agents: prefetch scout `CubeJavaContract`
  (java-oracle-scout) ran concurrently with the Draft 4.226 review; no
  separate GoSurface scout because the cube surface is the same runner file
  already mapped by `RollupDimGoSurface`. Oracle case branches were authored
  by the primary agent directly (mechanical extension of a worker-authored
  file; recorded as the serial exception).
- Rollup-dimensionality unit agents: prefetch scouts
  `RollupDimJavaContract` (java-oracle-scout; its first delivery crashed
  after extraction and the frozen contract was redelivered on nudge) and
  `RollupDimGoSurface` (scout) ran concurrently with the Draft 4.225 review;
  asset writer `RollupDimOracle` (parity-asset-worker, isolated) authored
  the new oracle and runner script. File ownership was disjoint.
- Finale unit agents: prefetch scouts `FilterFinaleJavaContract`
  (java-oracle-scout) and `FilterFinaleGoSurface` (scout) ran concurrently
  with the Draft 4.224 review; asset writer `FinaleOracle`
  (parity-asset-worker, isolated) authored the oracle extension including
  multi-statement module compile and old-stream recording. File ownership
  was disjoint.
- Multi-stream unit agents: prefetch scouts `MultiStreamJavaContract`
  (java-oracle-scout) and `MultiStreamGoSurface` (scout) ran concurrently
  with the Draft 4.223 review; asset writer `MultiStreamOracle`
  (parity-asset-worker, isolated) authored the oracle extension via a
  Map-based S3 type; its patch was not auto-applied and dropped the
  SupportBean registration — the primary agent applied the patch and fixed
  the registration. File ownership disjoint except that one primary-owned
  fix.
- Wildcard unit agents: prefetch scouts `SameEventJavaContract`
  (java-oracle-scout) and `SameEventGoSurface` (scout) ran concurrently with
  the Draft 4.222 review; asset writer `SameEventOracle`
  (parity-asset-worker, isolated) authored the oracle extension (its om-case
  outer-stream defect was fixed by the primary agent after the worker
  parked). File ownership disjoint except that one primary-owned fix.
- Where-previous unit agents: prefetch scouts `WherePrevJavaContract`
  (java-oracle-scout) and `WherePrevGoSurface` (scout) ran concurrently with
  the Draft 4.221 review; asset writer `WherePrevOracle`
  (parity-asset-worker, isolated) authored the oracle case branches while
  the primary agent wrote the scenario extension, runner branch, and
  mutations. File ownership was disjoint.
- Join-gated unit agents: prefetch scouts `JoinFilteredJavaContract`
  (java-oracle-scout) and `JoinFilteredGoSurface` (scout) ran concurrently
  with the Draft 4.220 review; asset writer `JoinFilteredOracle`
  (parity-asset-worker, isolated) authored the oracle extension while the
  primary agent wrote the scenario extension, runner branch, and mutations.
  File ownership was disjoint.
- Multikey unit agents: prefetch scouts `MultikeyJavaContract`
  (java-oracle-scout) and `MultikeyGoSurface` (scout) ran concurrently with
  the Draft 4.218 review; asset writer `MultikeyOracle`
  (parity-asset-worker, isolated) authored the oracle extension (its compile
  failure against the regression-lib-only classpath was fixed by the primary
  agent adding the two event sources to the runner javac step after the
  parked worker could not be revived). File ownership disjoint except that
  one primary-owned script fix.
- Quantified-null unit agents: read-only scouts `JavaContract`
  (java-oracle-scout) and `GoSurface` (scout) ran concurrently before
  implementation; asset writer `OracleExtend` (parity-asset-worker,
  isolated) authored the Java oracle extension on the frozen scenario
  contract while the primary agent wrote the shared engine fix, Go runner,
  scenario, and tests. File ownership was disjoint.
- ExprClassStaticMethod unit agents: `ECSMJavaScout` (java-oracle-scout) and
  `ECSMGoScout` (scout) ran concurrently before implementation. They froze
  the 13-ordinal source contract and confirmed that 11 observable runtime IDs
  map to existing typed `Func0`/`Func1`/`Func2`, `DefineExpression`,
  `ExpressionRef`, named-window, and FAF surfaces without shared-runtime
  changes; ordinals 4 and 11 plus Janino error wording are real approved
  differences. `ECSMAssetWriter` (parity-asset-worker) authored only the
  oracle, runner script, and scenario on the frozen contract; primary owns Go
  runner, generated trace/evidence, tests, central facts, validation, review,
  commit, and push. File ownership is disjoint.
- Cast Dates repair review: `CastDatesReview-2`, APPROVED after canonical
  metadata/evidence alignment; repair commit `c29aab39f` pushed.
- Primary agent owns shared semantics, parity assets, generated trace/evidence,
  manifest, roadmap, CHANGELOG, PLANS, validation, review, commit, and push.

## Quantified null unit (closed; committed as `f8b6f203f`)

- [x] Concurrent read-only scouts (`JavaContract`, `GoSurface`) froze the two
  null/empty-set executions, the exact event vectors, and the reusable Go
  surface (no new builders; `Field[any, any]` + existing quantified builders).
- [x] Extend `subselect-quantified` scenario with `relational-null-no-rows`
  and `equals-in-null-no-rows` (boxed nullable payloads); extend the Java
  oracle via the `OracleExtend` asset worker; extend the Go runner metadata,
  case order, and query branches.
- [x] Fix `evaluateQuantifiedSubquery` relational ANY to keep a decisive false
  when a non-null row exists; regenerate the Java trace (40 records) and the
  passing evidence with zero differences.
- [x] Add four trace mutations for the new cases including the
  false-dominates guard; all reject.
- [x] Promote `case.subquery-empty-quantifiers` to differential-verified;
  extend `case.subquery-quantified-comparisons` to six runtime IDs; manifest
  summary is 136 differential cases, 432 differential runtime IDs, 3071
  associations, 2901 unique referenced runtimes, and 1235 unreferenced.
- [x] Complete final full gates and independent parity review
  (`QuantifiedNullReview` verdict: pass, zero findings); delivery is ready for
  the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `a5d4703760c4c5c166e9fef357422eb50370780a57781d7ef93fa56fcdf6af2d`;
the extended differential evidence is passing with 40 Java records, 40 Go
records, 0 differences, six frozen runtime IDs, and six execution names.
Focused quantified diff/mutation tests (14 mutations all reject), full
`internal/esper` regression after the engine fix, `go vet ./...`, `go test
./... -count=1 -timeout 240s`, `make check`, and `git diff --check` pass.

## Filtered scalar unit (closed; committed as `a5cff7b23`)

- [x] Prefetch scouts (`FilteredJavaContract`, `FilteredGoSurface`) froze the
  five-execution slice, the exact event vectors, and the reusable Go surface
  (existing `SubqueryValue[WithOptions]` builders; zero engine changes).
- [x] Create the `subselect-filtered` scenario (7 cases, 3 event types), the
  Java oracle and runner script via the `FilteredOracle` asset worker, and
  the Go runner, dispatch modes, and diff/mutation tests (9 mutations all
  reject).
- [x] Regenerate the pinned-commit Java trace (24 records) and the passing
  evidence with zero differences; register `case.subselect-filtered-scalar-filter`
  and promote `epl.subselect.filtered` to differential-verified (5/27
  runtimes); manifest summary is 137 differential cases, 437 differential
  runtime IDs, 3076 associations, 2901 unique referenced runtimes, and 1235
- [x] Complete final full gates and independent parity review
  (`FilteredScalarReview` verdict: pass; the single P3 formatting finding in
  the manifest mappings tail was fixed and re-validated); delivery is ready
  for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `ffd54beb15c38c3884cb777b48afdde6c031dc47c6b8168d2dd62b48e6be0656`;
the differential evidence is passing with 24 Java records, 24 Go records,
0 differences, five frozen runtime IDs, and five execution names. Focused
filtered diff/mutation tests (9 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Multikey wArray unit (closed; committed as `4d931c979`)

- [x] Prefetch scouts (`MultikeyJavaContract`, `MultikeyGoSurface`) froze the
  three-execution slice, the exact event vectors, and the reusable Go surface
  (existing `Is`/`And`/`OuterField` builders; zero engine changes).
- [x] Extend `subselect-filtered` scenario to ten cases with array event
  payloads; extend the Java oracle via the `MultikeyOracle` asset worker
  (classpath fix applied by the primary agent); extend the Go runner with
  two array structs, three branches, and four new trace mutations.
- [x] Regenerate the pinned-commit Java trace (38 records) and the passing
  evidence with zero differences; register `case.subselect-filtered-multikey-array`;
  manifest summary is 138 differential cases, 440 differential runtime IDs,
  3079 associations, 2901 unique referenced runtimes, and 1235 unreferenced.
- [x] Complete final full gates and independent parity review
  (`MultikeyReview` verdict: pass, zero findings); delivery is ready for the
  single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `2946762534ffedd7e1e42890abd71c7f132d5d12cdea9dbbd8f6c4be33525a75`;
the differential evidence is passing with 38 Java records, 38 Go records,
0 differences, eight frozen runtime IDs, and eight execution names. Focused
filtered diff/mutation tests (13 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Joined coercion unit (closed; committed as `dcec9e1e0`)

- [x] Prefetch scouts (`CoercionJavaContract`, `CoercionGoSurface`) froze the
  two-execution slice, the five-round vectors, exact rejection boundaries,
  and the reusable Go surface (`JoinMany`/`JoinField`; zero engine changes).
- [x] Extend `subselect-filtered` scenario to fifteen cases with boxed
  SupportBean payloads; extend the Java oracle via the `CoercionOracle`
  asset worker; extend the Go runner with the JoinMany branch, boxed bean
  fields, and four new trace mutations.
- [x] Regenerate the pinned-commit Java trace (63 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-joined-coercion`; manifest summary is 139
  differential cases, 442 differential runtime IDs, 3081 associations,
  2901 unique referenced runtimes, 1235 unreferenced; and
  `epl.subselect.filtered` at 10/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`CoercionReview` verdict: pass; the single P3 finding — the
  predicate-order mutation targeting a structural null record — was fixed by
  retargeting it to the p3 seq2 match record and re-validated); delivery is
  ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `05e3ea6dc541f87e84a233ecd4dec630573fb163437a2f97fd05b825b9452d56`;
the differential evidence is passing with 63 Java records, 63 Go records,
0 differences, ten frozen runtime IDs, and ten execution names. Focused
filtered diff/mutation tests (17 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Join-gated unit (closed; committed as `f138c56f1`)

- [x] Prefetch scouts (`JoinFilteredJavaContract`, `JoinFilteredGoSurface`)
  froze the two-execution slice, the fire/no-fire vectors, prior/prev
  unfiltered-window semantics, and the reusable Go surface (`Join`/`OnEqual`
  builders; zero engine changes).
- [x] Extend `subselect-filtered` scenario to seventeen cases; extend the
  Java oracle via the `JoinFilteredOracle` asset worker; extend the Go runner
  with the S2 struct, Join branch, and three new trace mutations.
- [x] Regenerate the pinned-commit Java trace (67 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-join-gated`; manifest summary is 140 differential
  cases, 444 differential runtime IDs, 3083 associations, 2901 unique
  referenced runtimes, 1235 unreferenced; `epl.subselect.filtered` reaches
  12/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`JoinGatedReview` verdict: pass, zero findings); delivery is ready for the
  single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `74cd9542e22d059247ae94f3e46bbbc7ede54491a4a3343fd271bbef05b64117`;
the differential evidence is passing with 67 Java records, 67 Go records,
0 differences, twelve frozen runtime IDs, and twelve execution names. Focused
filtered diff/mutation tests (20 mutations all reject), `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

## Where-previous unit (closed; committed as `f7c07c73f`)

- [x] Prefetch scouts (`WherePrevJavaContract`, `WherePrevGoSurface`) froze
  the three-execution slice, the three-round vectors, and the reusable Go
  surface (`Prev` inside `SubqueryValue`; zero engine changes).
- [x] Extend `subselect-filtered` scenario to twenty cases; extend the Java
  oracle via the `WherePrevOracle` asset worker; extend the Go runner with
  the Prev branch and two new trace mutations (om seq2 null-flip and seq3
  offset probe).
- [x] Regenerate the pinned-commit Java trace (76 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-where-previous`; manifest summary is 141
  differential cases, 447 differential runtime IDs, 3086 associations,
  2901 unique referenced runtimes, 1235 unreferenced;
  `epl.subselect.filtered` reaches 15/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`WherePrevReview` initial verdict flagged one P2 stale doc comment; the
  fix was re-checked by the same reviewer: pass, zero findings); delivery is
  ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `08ce7300b5fea5f0c896f70a779fb5c780b273b2fd12fc4d0f951c40b9091511`;
the differential evidence is passing with 76 Java records, 76 Go records,
0 differences, fifteen frozen runtime IDs, and fifteen execution names.
Focused filtered diff/mutation tests (22 mutations all reject), `go vet
./...`, `go test ./... -count=1 -timeout 240s`, `make check`, and
`git diff --check` pass.

## Wildcard events unit (closed; committed as `0870b2dd4`)

- [x] Prefetch scouts (`SameEventJavaContract`, `SameEventGoSurface`) froze
  the four-execution slice, the assertSame field-snapshot observation
  equivalent, and the reusable Go surface (`EventValue[esper.Event]`;
  zero engine changes).
- [x] Extend `subselect-filtered` scenario to twenty-four cases; extend the
  Java oracle via the `SameEventOracle` asset worker (om-case outer-stream
  fix applied by the primary agent); extend the Go runner with two
  EventValue branches and two new trace mutations.
- [x] Regenerate the pinned-commit Java trace (80 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-wildcard-events`; manifest summary is 142
  differential cases, 451 differential runtime IDs, 3090 associations,
  2901 unique referenced runtimes, 1235 unreferenced;
  `epl.subselect.filtered` reaches 19/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`WildcardReview` initial verdict flagged one P3 stale doc comment; the
  fix was re-checked by the same reviewer: pass, zero findings); delivery is
  ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `9a2162c75a92e8062f4df743914adff205155a845e0eddcf1f59805e6791181d`;
the differential evidence is passing with 80 Java records, 80 Go records,
0 differences, nineteen frozen runtime IDs, and nineteen execution names.
Focused filtered diff/mutation tests (24 mutations all reject), `go vet
./...`, `go test ./... -count=1 -timeout 240s`, `make check`, and
`git diff --check` pass.

## Multi-stream join unit (closed; committed as `afd03ee7d`)

- [x] Prefetch scouts (`MultiStreamJavaContract`, `MultiStreamGoSurface`)
  froze the three-execution slice, the five-round vectors, the R2 99-vs-null
  partial-correlation pair, and the reusable Go surface (`Join`/`JoinMany`;
  zero engine changes).
- [x] Extend `subselect-filtered` scenario to twenty-seven cases; extend the
  Java oracle via the `MultiStreamOracle` worker patch (manually applied with
  the SupportBean-registration fix); extend the Go runner with the S3 struct,
  three join branches, and three new trace mutations.
- [x] Regenerate the pinned-commit Java trace (92 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-multi-stream`; manifest summary is 143
  differential cases, 454 differential runtime IDs, 3093 associations,
  2901 unique referenced runtimes, 1235 unreferenced;
  `epl.subselect.filtered` reaches 22/27 DV runtimes.
- [x] Complete final full gates and independent parity review
  (`MultiStreamReview` initial verdict flagged one P1 — the scene-two R5 S2
  payload had been copied from joined-3-streams instead of the Java script's
  s0_2, leaving the three-way conjunct never positively hit — plus one P3
  doc-comment drift; both fixed, trace/evidence regenerated with scene-two
  R5=98, a positive-hit mutation added, and the fix re-checked scope-ready
  for commit).

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `4bdb5e62cfc6f5968f0d885d958f8cafc8f541706be644c8164bf2b9aaf3635d`; the differential evidence is passing with 92 Java
records, 92 Go records, 0 differences, twenty-two frozen runtime IDs, and
twenty-two execution names. Focused filtered diff/mutation tests (28
mutations all reject), `go vet ./...`, `go test ./... -count=1 -timeout
240s`, `make check`, and `git diff --check` pass.

## Suite finale unit (closed; committed as `45bac28c3`)

- [x] Prefetch scouts (`FilterFinaleJavaContract`, `FilterFinaleGoSurface`)
  froze the four-execution finale, the old-stream discriminator, and the
  reusable Go surface (`WithOldStream`/`Or`/`SortWindow`/multi-plan deploy;
  zero engine changes).
- [x] Extend `subselect-filtered` scenario to thirty-one cases; extend the
  Java oracle via the `FinaleOracle` asset worker (multi-statement module,
  old-stream recording, Map/EventBean normalize branches); extend the Go
  runner with four branches and four new trace mutations.
- [x] Regenerate the pinned-commit Java trace (103 records) and the passing
  evidence with zero differences; register
  `case.subselect-filtered-finale`; manifest summary is 144 differential
  cases, 458 differential runtime IDs, 3097 associations, 2901 unique referenced
  runtimes, 1235 unreferenced; `epl.subselect.filtered` reaches
  26/27 DV runtimes (only WildcardNoName approved difference remains).
- [x] Complete final full gates and independent parity review
  (`FinaleReview` initial verdict flagged one P1 — the Prior runtime-ID typo
  had persisted in the runner metadata, scenario, and evidence beyond the
  manifest fix — plus one P3 doc-comment drift; all four occurrences fixed,
  evidence regenerated, and the fix re-checked by the same reviewer: pass,
  zero findings); delivery is ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `89ce23db4f58d44acb3b17435969b7a6d7aa68ca8e83cbc23dcebec83f0aef85`;
the differential evidence is passing with 103 Java records, 103 Go records,
0 differences, twenty-six frozen runtime IDs, and twenty-six execution
names. Focused filtered diff/mutation tests (32 mutations all reject),
`go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`, and
`git diff --check` pass.

## Rollup dimensionality unit (closed; committed as `7fd4f42dd`)

- [x] Prefetch scouts (`RollupDimJavaContract`, `RollupDimGoSurface`) froze
  the four-execution unbound-rollup slice, per-round vectors, and the
  reusable Go surface (`GroupByRollup/Cube/GroupingSets`; zero engine
  changes).
- [x] Create the `rollup-dimensionality` scenario (ten cases), the new Java
  oracle and runner script via the `RollupDimOracle` asset worker, and the
  Go runner, dispatch modes, and diff/mutation tests (nine mutations all
  reject).
- [x] Generate the pinned-commit Java trace (50 records) and the passing
  evidence with zero differences; register
  `case.rollup-dimensionality-unbound-rollup`; promote
  `resultset.aggregate-dimensional` to differential-verified (4/24 DV
  runtimes); manifest summary is 145 differential cases, 462 differential
  runtime IDs, 3101 associations, 2905 unique referenced runtimes, 1231
  unreferenced.
- [x] Complete final full gates and independent parity review
  (`CubeReview` initial verdict flagged one P1 — two mutations targeted
  wrong-case indices — evidence-header runtime-ID omission, plus P2/P3 count-table drift; all
  fixed and re-checked by the same reviewer: pass); delivery is ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `cfa0669dc5e520ac26cb1abb176f933eef9aab5888815c942169626711230e72`;
the differential evidence is passing with 50 Java records, 50 Go records,
0 differences, four frozen runtime IDs, and four execution names. Focused
rollup-dimensionality diff/mutation tests (9 mutations all reject),
`go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`, and
`git diff --check` pass.

## Cube dimensionality unit (active)

- [x] Prefetch scout (`CubeJavaContract`) froze the two-execution cube
  slice with bitmask-ordering probes and exact per-round c-vectors.
- [x] Extend `rollup-dimensionality` scenario to fourteen cases; extend the
  oracle buildEPL and SupportBean decode (intBoxed) in-place; extend the Go
  runner with cube branches over the enriched bean.
- [x] Fix `cubeGroupingSets` to enumerate dim0-highest-bit descending;
  regenerate the pinned-commit Java trace (65 records) and the passing
  evidence with zero differences; register
  `case.rollup-dimensionality-unbound-cube`; manifest summary is 146
  differential cases, 464 differential runtime IDs, 3103 associations,
  2907 unique referenced runtimes, 1229 unreferenced.
- [x] Complete final full gates and independent parity review
  (initial review flagged one P2 — two mutations targeting wrong-case
  indices — plus P3 count/table drift; all fixed and re-checked: pass);
  delivery is ready for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `6c81e4a0eabfe4f06d7d874881f32dddb4e43800f7908c7ec4030ad318542c05`;
the differential evidence is passing with 64 Java records, 64 Go records,
0 differences, six frozen runtime IDs, and six execution names. Focused
rollup-dimensionality diff/mutation tests (12 mutations all reject), full
`internal/esper` regression after the cubeGroupingSets fix, `go vet ./...`,
`go test ./... -count=1 -timeout 240s`, `make check`, and `git diff --check`
pass.

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

## Historical work-unit contract (Generic Cast; closed)

- Capability/subdomain: `expr.core`, generic Cast extension of
  `case.expr-core-exists-cast`, one source-order `ExprCoreCast.java`
  execution (`ExprCoreCastGeneric`, ordinal 11) beside the fifteen
  already-verified executions.
- Java source and execution/runtime ID: fixed commit
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`,
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCast.java`
  lines 97-131 + `support/events/SupportGenericColUtil.java`,
  `ExprCoreCastGeneric` / `java-runtime-2fcd2094aaf18ca4ce03` (already in
  manifest case javaRuntimeIds; only DV status changes).
- Differential scope: extend `testdata/parity/expr-core-exists-cast.json`
  with one case `cast-generic`, one send of map event `MyEventGeneric` (8
  Object-typed props). Select = `select cast(listOfString,
  java.util.List<String>) as listOfString, cast(listOfOptionalInteger,
  java.util.List<java.util.Optional<Integer>>) as listOfOptionalInteger,
  cast(mapOfStringAndInteger, java.util.Map<String,Integer>) as
  mapOfStringAndInteger, cast(listArrayOfString, java.util.List<String>[]) as
  listArrayOfString, cast(listOfStringArray, java.util.List<String[]>) as
  listOfStringArray, cast(listArray2DimOfString, java.util.List<String>[][])
  as listArray2DimOfString, cast(listOfStringArray2Dim,
  java.util.List<String[][]>) as listOfStringArray2Dim, cast(listOfT,
  java.util.List<Object>) as listOfT from MyEventGeneric` (harness type
  renamed from MyEvent to avoid collision; same cast semantics).
- Observable contract: one row at virtual time zero; eight columns preserve
  the sent values elementwise (List/Map cast identity on erasure);
  Optional.get()=10 renders as "10". Deterministic rendering both sides:
  lists as JSON arrays of element tokens, maps as sorted-key JSON objects,
  Optional unwrapped.
- Allowed production files: none under `internal/esper` expected (List/Map
  assignable casts exist via castAnyToType); parity-layer only unless the
  Go surface scout flags a real gap.
- Allowed Go test/parity files: `internal/app/parity/expr_core_exists_cast.go`
  (case dispatch, MyEventGeneric decode branch, list/map/Optional
  normalizer cases), `internal/app/parity/run_test.go` (49->50 records,
  case count, mutations), scenario/trace/evidence fixtures, oracle +
  runner script guards.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreExistsCastScenarioOracle.java` (generic case,
  payload builders, List/Map/Optional token rendering in TraceWriter) and
  `testdata/parity/expr-core-exists-cast.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  `internal/esper` semantic surfaces; `goal.txt`; generated evidence before
  trace validation; central facts outside this unit's manifest/roadmap/CHANGELOG
  updates. `PLANS.md`, manifest, roadmap, CHANGELOG, traces, evidence remain
  primary-agent owned.
- Targeted validation: pinned Java oracle runner; focused exists-cast
  parity + mutation tests; scenario shape validator; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.


## Direct multirow unit (closed; committed as `b5599f1ba`)

- [x] Freeze the two `EPLSubselectMultirow.java` executions and exact runtime
  IDs with concurrent scouts `SubqueryMultirowJavaContract` and
  `SubqueryMultirowGoSurface`.
- [x] Implement the dedicated Go runner, dispatch modes, pinned Java oracle,
  scenario, and checked-in six-record trace/evidence.
- [x] Correct the staged single-column replay to accept eight sends with the
  Java 5-send/3-send redeploy split; focused value, window-retention, null,
  underlying-field, case, and record-count mutations reject.
- [x] Normalize only correlated `val` arrays in the direct multirow runner and
  its differential boundary. Stable JSON row keys mirror the Java oracle's
  `SupportBean[]`/`EventBean[]` sorting; shared `compat.normalizeValue` and
  global trace comparison remain unchanged. Swapped-row differential mutation
  passes while semantic mutations reject.
- [x] Register `case.subquery-multirow` and `query.subquery-basic` mapping;
  manifest summary is 523 cases, 135 differential cases, 430 differential
  runtime IDs, 3069 associations, 2901 unique referenced runtimes, and 1235
  unreferenced runtimes.
- [x] Complete final full gates and independent parity review; delivery is ready
  for the single semantic commit and push.

Final pre-commit verification: pinned Java oracle regenerated the trace with
checksum `0bffc4f680c631cdb5958e2a650a1d174f0cc00f3e2cf0581d20082cea50dcf1`;
the direct differential evidence is passing with 6 Java records, 6 Go records,
0 differences, both frozen runtime IDs, and both execution names. Focused
direct replay/mutation tests, `go test ./internal/compat ./internal/app/manifest
-count=1`, `go vet ./...`, `go test ./... -count=1 -timeout 240s`, `make check`,
and `git diff --check` pass. `DirectMultirowRunnerReview` approved the scoped
correlated-array normalization and its separate swapped-row acceptance test.

## Progress

- [x] Reconfirm baseline (`HEAD`/`origin/master` clean at `2a6c7f9a4`),
      roadmap, manifest.
- [x] Launch concurrent Java/Go read-only scouts for the remaining cast
      executions (Generic / WArray / Dates).
- [x] Java contract scout `DatesGenericArrayJavaContract` and Go surface scout
      `DatesGenericArrayGoSurface` both returned (6m13s / 20m25s); contract
      frozen: WArray{soda=false}+{soda=true} = one clean closed-loop unit;
      Generic and Dates deferred with recorded reasons.
- [x] Record the frozen WArray contract in this checkpoint and delegation
      records.
- [x] Implement the two `cast-warray` cases end-to-end: Java oracle (EPL
      insert-into, array-element token rendering, payload builders), scenario
      fixture (2 cases x 2 sends), Go runner (MyEvent map schema, MyArrayEvent
      target, decode branches, array/bean normalizer), run_test coverage
      (49 records, mutation indices).
- [x] Generate pinned Java trace; Go replay is zero-difference; add
      boundary/mutation coverage (4 warray mutations all reject).
- [x] Update manifest (case DV runtime IDs 13->15, summary 424->426),
      regenerate evidence (passing), roadmap, CHANGELOG.
- [x] Run full local gates and independent parity review; resolve findings
      (`WArrayReview`, parity-reviewer, APPROVED at 0.93 confidence with one
      P3 dead-code nit — unused `myEventWArrayMap()` — removed and the trace
      re-verified byte-identical afterwards).
- [x] Review final diff, record validation, create one semantic commit
      (`67514e7db` "expr: verify array cast parity"), push `master`, verify
      remote ref read-only (local == origin/master == `67514e7db`).

### Generic unit (closed)

- [x] Reconfirm baseline (`HEAD` == `origin/master` == `67514e7db`).
- [x] Launch concurrent read-only scouts `GenericCastJavaContract` +
      `GenericCastGoSurface`; primary agent independently read
      ExprCoreCastGeneric + SupportGenericColUtil.
- [x] Freeze the Generic contract in this checkpoint (1 case, 1 send,
      8 columns, runtime `java-runtime-2fcd2094aaf18ca4ce03`).
- [x] Confirm scout reports match the frozen contract; adjust if they
      surface gaps.
   "remaining": [
    "all aggregate/access-method families",
    "Java named filter-parameter EPL syntax/registration and exact compiler diagnostics, plus MathContext/BigDecimal scale and rounding policy",
    "aggregate state/index reuse and full Java trace parity"
   ],
      mutations added); evidence.
- [x] Full gates green (`make check`), `GenericReview` parity review APPROVED
      (one P2 fixed: evidence javaRuntimeIds/javaExecutions + Go fallback
      lists appended in lockstep; evidence regenerated, 16 IDs / 15
      executions); commit `f2e6c3771` pushed to `master`.

### Cast Dates unit (closed; metadata repair)

Baseline: implementation commit `500d8db84`; metadata repair follows on the
same committed behavior and does not modify the pinned trace.
Delegation checkpoint: `CastDatesJavaContract` (java-oracle-scout) and
`CastDatesGoSurface` (scout) were launched concurrently; primary agent read
`ExprCoreCast.java:376-820` independently.

Frozen work-unit contract (execution `ExprCoreCastDates`, runtime
`java-runtime-2ee2b8ab1bf9bb2c4e90`, inventory line 2157):
- 3 cases, 3 sends, 51 -> 54 records:
  - `cast-dates-base`: MyDateType map event `{yyyymmdd:"20100510"}`; 9
    columns (date/java.util.Date, long/java.lang.Long,
    calendar/java.util.Calendar targets, plus `.get("month")` chains);
    expected Date/Long epoch 1273449600000, month Integer 4 (zero-based).
  - `cast-dates-java8`: LocalDate, LocalDateTime, and LocalTime alias/FQCN
    targets render deterministic ISO values; zoneddatetime VV cells remain
    deferred because Go stdlib lacks zone-region parsing.
  - `cast-dates-constant`: SupportBean("E1",1); one constant-folded Date
    column with epoch 1044057600000.
- Go mapping uses `CastWithLayout[string,time.Time]`, `UnixMillis`, and
  `Month(...) - 1`; no `internal/esper` change was needed.
- Deferred follow-ups: ISO8601 `iso`, dynamic dateformat, non-string formatter
  parameters, invalid compile diagnostics, render-out-column metadata, and VV
  zoneddatetime cells.

Progress:
- [x] Oracle, Go runner, scenario, pinned trace, and zero-difference replay.
- [x] Four date mutations reject (epoch, month, Java 8 cell, constant cell).
- [x] Manifest and Draft 4.215 docs record 17 differential runtime IDs.
- [x] Post-commit repair aligns Go/evidence runtime IDs and execution names to
  Java inventory ordinal order; manifest prose now states four Exists plus
  thirteen typed Cast executions.
- [x] Regenerate evidence from the unchanged trace; targeted parity/compat
  tests, JSON checks, full gates, and `CastDatesReview-2` re-review all pass.
  One semantic repair commit and push remain.
- 2026-08-21: Repair validation completed against the unchanged pinned trace;
  `case.expr-core-exists-cast` now has 17 inventory-ordered runtime IDs, 17
  execution names, and 54 records with passing evidence and zero differences.
  The trace checksum is `f0efd3239e22fbb9533a648d7b2cee9ac321df6e45a94a77712418ffe8f7f40d`.
  `go vet ./...`, `go test ./... -count=1`, `make check`, targeted parity and
  compat tests, JSON validation, and `git diff --check` passed; the repair is
  ready for its semantic commit and push.

- 2026-08-20: GitLab DOES protect `master` (force-push rejected 2026-08-20,
  contradicting AGENTS.md/runbook claims). Amended commits cannot be repushed;
  finalize all file changes BEFORE the single unit commit/push. Post-commit
  `PLANS.md` closeout notes must ride with the next unit's commit.

## Discoveries and decisions

- 2026-08-20: Oracle harness fix: prepending `@name('s0')` to multi-statement
  EPL (cast-warray declares two schemas plus the insert) makes Esper name the
  first schema statement `s0` and the insert `s0-1`; the old first-`s0`
  statement finder attached the listener to the schema statement and the case
  produced zero records. The finder now selects the LAST `s0`-prefixed
  statement and the TraceWriter records a fixed `"s0"` statement name so Java
  and Go traces stay field-comparable.

- 2026-08-20: The `cast-bigdecimal-bigint` unit passed the independent
  parity review (`BigDecimalBigIntReview`, parity-reviewer, APPROVED, no
  findings). The reviewer independently re-verified every evidence,
  trace, mutation, oracle, and doc claim, including a Python bigint-pow
  exact-match of the 2^500500 / 2^500500+0.1 vectors and the
  `Double.toString` round-trip for 2.4.
- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The previous coalesce unit was delivered as `96a2aadf3`, with
  six zero-difference runtimes and 22 replay records; its invalid compile case
  remains implemented-only.
- 2026-08-20: The `ExprCoreCastInterface` contract is frozen. The Java
  regression asserts bean identity (`event.get("tX") == bean`), so the
  differential encoding is per-target bean presence. The `cast(item?, T)`
  semantics: marker/super-interface targets succeed via assignability or
  Implements; a class target (t3 ISupportBaseABImpl) is exact-class only and
  never satisfied through an interface; an abstract superclass target (t6)
  matches its concrete subclass. Send 1 must double-wrap
  (`item = SupportBeanDynRoot("abc")`), because a plain String satisfies no
  target interface. Both sides render matched beans as the Java simple class
  name token (deterministic, byte-exact) instead of `String.valueOf`'s
  address hash. Go requires zero `internal/esper` changes (castAnyToType
  already implements assignable/Implements/else-null).
- 2026-08-20: The `cast-interface` implementation needed one
  `internal/esper` shared semantic fix (correcting the scouts' "zero change"
  expectation): `typeOf` resolved a nil interface-typed `T` to the empty
  interface `any`, so every bean satisfied every target cell (all t0..t7
  matched on first probe). The fix recovers the static interface type via
  `reflect.TypeOf((*T)(nil)).Elem()`, preserving concrete/pointer resolution.
  Full `internal/esper` unit suite plus the complete parity suite pass with
  the fix. Differential encoding renders matched beans as Java simple-class
  name tokens on both sides, and the traces confirm the exact frozen
  bean-target matrix (S1->t0, S2->t5, S3->t2+t4, S4->t1/t2/t4/t6/t7,
  S5->t2+t3).
- 2026-08-20: `case.expr-core-relop` was delivered as `a448dbd4f` with two
  zero-difference runtime IDs and 30 replay records. Its scenario-shape review
  tightened exact case order and send counts.
- 2026-08-20: `case.expr-core-like-regexp` reached zero-difference replay and
  was delivered as `896ed1312`; the full-match fix and Java-LIKE test-modeling
  correction passed the complete local gates.
- 2026-08-20: Fresh concurrent IN/BETWEEN scouts Russell/Kierkegaard froze a
  safe five-execution scalar slice and identified the endpoint-policy Plan
  identity defect and nullable replay schema requirement.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records and exact `s0`/`s1`/`s2` range lifecycle; checked-in trace,
  evidence, mutation tests, manifest, roadmap, and CHANGELOG updates are in
  place. The endpoint-policy Plan identity regression is covered by a focused
  Go test.
- 2026-08-20: The first parity review's payload finding was resolved by
  symmetric frozen vectors in the Go runner and Java oracle. Validation now
  requires exact case/send order, event type, field set, and JSON value spelling
  for all 164 inputs, including the ten range pairs; a Go payload mutation test
  covers the rejection path.
- 2026-08-20: Shared differential provenance flags remain an intentional
  runner-level override used by existing modes and are out of scope for this
  unit; checked-in evidence still asserts the exact fixed commit, source,
  execution, and runtime metadata. Result fields remain maps because the
  protocol compares field identity/value structurally and JSON encoding gives
  deterministic lexicographic key order; field-map insertion order is likewise
  out of scope for this unit.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records, passed independent review and local gates, and was pushed as
  `033786cbd`; Git is authoritative for that commit identity.
- 2026-08-20: `case.expr-core-exists-cast` scalar-Cast extension reached
  zero-difference replay with 32 records (was 22) by reusing existing
  `castToString`/`castToBool`/`parseStringNumber` with no `internal/esper`
  production change. The independent parity review found one minor
  evidence-integrity defect: `javaExecutions[2]` relabeled the pre-existing
  `ExprCoreCastDoubleAndNullOM` runtime (`8fa7f5076dde9d791d08`) as
  `ExprCoreCastStringAndNullCompile`; the label was restored to match the
  execution inventory and evidence regenerated. Manifest case DV runtime IDs
  8→11, summary 419→422, cases stay 134.
- 2026-08-20: The `cast-bigdecimal-bigint` unit reached zero-difference
  replay with 40 records (was 32). The diff exposed a genuine
  `internal/esper` semantic defect: `castToBigRat`'s float branch used
  `big.Rat.SetFloat64` (binary-exact), but Java `BigDecimal.valueOf(double)`
  round-trips through `Double.toString` — fixed to
  `strconv.FormatFloat(v,'g',-1,bits)` + `big.Rat.SetString`, with
  exact-decimal trace rendering for non-integer big.Rat. Manifest case DV
  runtime IDs 11→12, summary 422→423, cases stay 134.
- 2026-08-20: The equality scouts confirmed the reusable typed
  `EqualOf`/`NotEqualOf`/`Is`/`IsNot` surface, deep slice equality, and typed-nil
  behavior. They also identified a Plan identity collision for interface-typed
  primitive literals and the missing direct differential replay.
- 2026-08-20: `case.expr-core-equals-is` implementation added a strict
  four-case scenario validator, fresh per-case Go deployments, and a pinned
  Esper Java oracle. The replay covers eight listener records in exact source
  order; `ExprCoreEqualsInvalid` remains a compile/build boundary.
- 2026-08-20: `Literal[T]` now includes the dynamic concrete type in Plan
  descriptions when `T` is an interface and the value is primitive. The
  focused regression keeps equivalent plans stable while distinguishing
  `Literal[any](1)` from `Literal[any]("1")`.
- 2026-08-20: Targeted validation passed: the pinned Java runner regenerated
  an eight-record trace byte-for-byte equal to the checked-in trace; Go
  differential replay/evidence reported zero differences; focused equality,
  parity, mutation, compat, and manifest tests passed. `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, focused expression and
  parity race tests, JSON/schema checks, shell syntax, layout, and
  `git diff --check` also passed.
- 2026-08-20: Independent parity review by Meitner returned no findings. The
  reviewer confirmed the Java source order, exact payload vectors, fresh case
  deployments, and the coercion coverage.
- 2026-08-20: `case.expr-core-relop` was delivered as `a448dbd4f` with two
  zero-difference runtime IDs and 30 replay records. Its scenario-shape review
  tightened exact case order and send counts.
- 2026-08-20: `case.expr-core-like-regexp` reached zero-difference replay and
  was delivered as `896ed1312`; the full-match fix and Java-LIKE test-modeling
  correction passed the complete local gates.
- 2026-08-20: Fresh concurrent IN/BETWEEN scouts Russell/Kierkegaard froze a
  safe five-execution scalar slice and identified the endpoint-policy Plan
  identity defect and nullable replay schema requirement.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records and exact `s0`/`s1`/`s2` range lifecycle; checked-in trace,
  evidence, mutation tests, manifest, roadmap, and CHANGELOG updates are in
  place. The endpoint-policy Plan identity regression is covered by a focused
  Go test.
- 2026-08-20: The first parity review's payload finding was resolved by
  symmetric frozen vectors in the Go runner and Java oracle. Validation now
  requires exact case/send order, event type, field set, and JSON value spelling
  for all 164 inputs, including the ten range pairs; a Go payload mutation test
  covers the rejection path.
- 2026-08-20: Shared differential provenance flags remain an intentional
  runner-level override used by existing modes and are out of scope for this
  unit; checked-in evidence still asserts the exact fixed commit, source,
  execution, and runtime metadata. Result fields remain maps because the
  protocol compares field identity/value structurally and JSON encoding gives
  deterministic lexicographic key order; field-map insertion order is likewise
  out of scope for this unit.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records, passed independent review and local gates, and was pushed as
  `033786cbd`; Git is authoritative for that commit identity.
- 2026-08-20: `case.expr-core-exists-cast` scalar-Cast extension reached
  zero-difference replay with 32 records (was 22) by reusing existing
  `castToString`/`castToBool`/`parseStringNumber` with no `internal/esper`
  production change. The independent parity review found one minor
  evidence-integrity defect: `javaExecutions[2]` relabeled the pre-existing
  `ExprCoreCastDoubleAndNullOM` runtime (`8fa7f5076dde9d791d08`) as
  `ExprCoreCastStringAndNullCompile`; the label was restored to match the
  execution inventory and evidence regenerated. Manifest case DV runtime IDs
  8→11, summary 419→422, cases stay 134.
- 2026-08-20: The `cast-bigdecimal-bigint` unit reached zero-difference
  replay with 40 records (was 32). The diff exposed a genuine
  `internal/esper` semantic defect: `castToBigRat`'s float branch used
  `big.Rat.SetFloat64` (binary-exact), but Java `BigDecimal.valueOf(double)`
  round-trips through `Double.toString` — fixed to
  `strconv.FormatFloat(v,'g',-1,bits)` + `big.Rat.SetString`, with
  exact-decimal trace rendering for non-integer big.Rat. Manifest case DV
  runtime IDs 11→12, summary 422→423, cases stay 134.
- 2026-08-20: The equality scouts confirmed the reusable typed
  `EqualOf`/`NotEqualOf`/`Is`/`IsNot` surface, deep slice equality, and typed-nil
  behavior. They also identified a Plan identity collision for interface-typed
  primitive literals and the missing direct differential replay.
- 2026-08-20: `case.expr-core-equals-is` implementation added a strict
  four-case scenario validator, fresh per-case Go deployments, and a pinned
  Esper Java oracle. The replay covers eight listener records in exact source
  order; `ExprCoreEqualsInvalid` remains a compile/build boundary.
- 2026-08-20: `Literal[T]` now includes the dynamic concrete type in Plan
  descriptions when `T` is an interface and the value is primitive. The
  focused regression keeps equivalent plans stable while distinguishing
  `Literal[any](1)` from `Literal[any]("1")`.
- 2026-08-20: Targeted validation passed: the pinned Java runner regenerated
  an eight-record trace byte-for-byte equal to the checked-in trace; Go
  differential replay/evidence reported zero differences; focused equality,
  parity, mutation, compat, and manifest tests passed. `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, focused expression and
  parity race tests, JSON/schema checks, shell syntax, layout, and
  `git diff --check` also passed.
- 2026-08-20: Independent parity review by Meitner returned no findings. The
  reviewer confirmed the Java source order, exact payload vectors, fresh case
  lifecycle, trace/evidence metadata, and central manifest references; residual
  risk is limited to semantic-vs-lexical payload validation and broader
  object-array coercion coverage.
- 2026-08-20: CASE scouts froze the three source-order executions and found no
  production semantic gap. The strict replay uses Java's DELL/MSFT/GE vector;
  the existing JOE unmatched assertion remains supplemental.
- 2026-08-20: CASE Java and Go traces are byte-identical for nine listener
  records. Evidence reports zero differences; focused CASE/parity, malformed
  scenario, trace mutation, compat, and manifest checks pass.
- 2026-08-20: Independent CASE review identified and the primary agent fixed
  clean-oracle provenance enforcement plus symmetric exact case/send metadata
  validation in the Java oracle and Go replay; a metadata mutation regression
  now covers the previously asymmetric path.
- 2026-08-20: CASE review follow-up by Confucius returned no findings. N+1
  read-only scouts Carson/Helmholtz identified the five-execution
  `case.expr-core-instanceof` candidate and its primitive-wrapper/interface
  risks; no N+1 writes started.
- 2026-08-20: The `instanceof` source review confirmed five source-order
  executions, 17 listener records, Boolean-only results, fresh lifecycle per
  execution, and no timer/old-stream behavior. The Go `InstanceOf[T]` surface
  is already present; replay modeling must preserve dynamic numeric types and
  custom interface hierarchy values.
- 2026-08-20: Sartre/Euler scouts completed concurrently. The strict scenario
  uses `SupportBean` payloads with explicit nullable fields and tagged
  `SupportBeanDynRoot` payloads (`itemType` plus optional `itemValue` or
  hierarchy constructor fields). Go decodes tags to `string`, `float32`,
  `int`, `int64`, or local pointer/interface values; Java constructs the
  corresponding fixed support objects. No production write is authorized by
  the frozen contract.
- 2026-08-20: The pinned InstanceOf oracle regenerated the checked-in trace
  byte-for-byte; Go replay and evidence reported 17 records and zero
  differences. The malformed-scenario test now mutates the first dynamic send
  at `Steps[10]`, rather than the preceding case marker.
- 2026-08-20: Independent review by Laplace returned no findings. Residual
  risk remains limited to the unexercised OM/SODA serialization path, unknown
  JSON keys ignored by the generic step decoder, and broader optional/missing
  and hierarchy matrices outside this five-execution slice.
- 2026-08-20: TypeName implementation now consumes declared nested fragment
  metadata without changing ordinary Go reflection names; non-Avro nil-like
  fragments remain null while Avro retains declared metadata.
- 2026-08-20: The pinned TypeName oracle regenerated successfully after passing
  the shared Avro configuration and Jackson classpath requirements. Temporary
  Java/Go replay produced 18 records and zero differences across all six
  representation branches.
- 2026-08-20: Independent parity review by Planck was attempted after targeted
  validation but the agent remained nonresponsive and was closed without a
  report. The primary agent performed a bounded local review of the complete
  TypeName diff and found no concrete issues; the serial-review exception is
  retained here rather than represented as an independent approval.
- 2026-08-20: Exists/Cast scouts Turing and Plato were started concurrently for
  the next unit but produced no deliverables after waits and interruption; they
  were closed. Local inspection froze the four `ExprCoreExists` executions,
  their exact runtime IDs, boolean vectors, and Null/Missing contract. The
  unit has no authorized production change unless replay proves a regression.
- 2026-08-20: The initial Go replay produced 12 records but differed from Java
  for null dynamic roots, missing indexed/mapped paths on incompatible values,
  and the OM/compile null vectors. Ordinary `Exists` and explicit nullable
  fields must retain Null-as-present semantics, so the authorized production
  fix is typed `OptionalProperty` plus `?` path-boundary propagation rather
  than a global Exists change.
- 2026-08-20: The pinned Java oracle regenerated a 12-record trace byte-identical
  to the checked-in trace (SHA-256
  `20c4ca6c1ff48bf2f12ec4a267003726535ace672d38291a522ec444b04e678a`). Go
  differential replay/evidence reports 12 records and zero differences.
- 2026-08-20: Focused Exists/Cast, malformed-scenario, checked-in-evidence,
  trace-mutation, compat/manifest, JSON/schema, shell syntax, changed-file
  formatting, `go vet ./...`, `go test ./... -count=1 -timeout 240s`, focused
  expression/parity race tests, `make check`, and `git diff --check` passed.
- 2026-08-20: Independent review by Kuhn was attempted after targeted
  validation but remained nonresponsive and was closed without a report. The
  primary agent's bounded local review found no concrete findings; the exact
  serial-review exception is retained here rather than represented as an
  independent approval. N+1 Java scout Boyle confirmed the remaining Cast
  executions belong to `ExprCoreCast.java`, so no future writes began.
- 2026-08-20: Final manifest audit retained both `ExprCoreExists.java` and
  `ExprCoreCast.java` as source references because the case preserves all 17
  inventoried runtime associations while the four Exists-source IDs and four
  Cast-source IDs are now differential-verified.
- 2026-08-21: Cast metadata repair validation passed the pinned Java trace
  replay, focused parity/mutation/manifest checks, JSON/schema checks, `gofmt`,
  `go vet ./...`, `go test ./... -count=1`, `make check`, and `git diff --check`.
  The regenerated evidence is passing with 17 inventory-ordered runtime IDs,
  17 execution names, 54 records, and zero differences; the checked-in trace
  remains byte-identical (SHA-256
  `f0efd3239e22fbb9533a648d7b2cee9ac321df6e45a94a77712418ffe8f7f40d`).
- Current unit result: `case.expr-core-exists-cast` is differential-verified
  across 17 exact inventory-ordered runtime IDs and 54 listener records. The
  Cast Dates metadata repair aligns Go/evidence runtime IDs and execution names
  with the manifest and Java inventory; the regenerated evidence is passing with
  zero differences. The pinned trace remains byte-identical. Full local gates
  and the independent `CastDatesReview-2` re-review pass; remote delivery is
  ready for the semantic repair commit and push.

## Validation evidence

- The pinned Java oracle launcher regenerated `testdata/parity/resultset-aggregate-sorted-first-last.trace.json` from `/home/baicai/app/esper` at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; Java 17 emitted two listener records. The regenerated Java trace and Go replay are byte-identical to their checked-in traces (SHA-256 `cdf1d0f16f5390c83aad9e382bc6ce3add799cf42df74a0ed7a4f045bbc154f1` and `2a3794da3340464f4b985b30483ba51dac3151924e55ff36f99b36827d020faf`).
- Differential evidence is passing with two Java records, two Go records, and zero differences; regenerated evidence is byte-identical to checked-in evidence (SHA-256 `5e4f215eae6669d68a44b08481f1347283a39e63d0f84d8d289933a3d566c6e4`). Focused first-last replay, checked-in evidence, trace-mutation, malformed-scenario, and runtime-mapping tests passed; `TestCapabilityManifestArtifactValidates` passed; shell/JSON/layout checks and `git diff --check` passed; full `make check` passed.
- Independent reviewer `FirstLastReview` returned PASS with one P3 documentation finding (the umbrella `case.aggregate-sorted-table-selector` difference note had silently dropped two remain-open items); the note was restored in the same unit and the manifest artifact test still passes. Full `make check` passed, and full `go test -race ./... -count=1 -timeout 600s` passed across every package, including `internal/app/parity` and `internal/esper`.
- Manifest totals are 589 cases, 587 implemented cases, 202 differential-verified cases, 738 differential-verified runtime IDs, and 3,359 runtime associations (3,133 referenced runtimes); the new first-last case is registered as `differential-verified`.

## Delivery

- Draft 4.298 is ready for one semantic commit/push once `FirstLastReview` returns PASS and the full race gate passes. Git will remain authoritative for the resulting commit identity.
- After the semantic commit, verify the pushed ref without editing tracked files or creating a checkpoint-only follow-up commit.

## Handoff

After delivery, verify the pushed ref read-only and select the next work unit from the roadmap and manifest; do not edit this file only to add the new commit hash.
