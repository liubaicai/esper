# Draft 4.471 — context-key-segmented-remainder

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (verified).
Source: regression-lib/.../suite/context/ContextKeySegmented.java. All five
runtime IDs carry flags [] (no INVALIDITY flag despite tryInvalidCompile).

## Scope (3 executions)

### Ord 8 ContextKeySegmentedSubselectPrevPrior — DV
- runtime `java-runtime-2afbd86618b752a6dd5b`, static `java-1dbfa1926e47afc1a7b4`.
- Context: `@public create context SegmentedByString partition by theString from SupportBean`.
- Deploy 2: `select theString, (select prev(0, id) from SupportBean_S0#keepall) as col1 from SupportBean`.
  Sends: SB(G1,10)->{G1,null}; S0(1,E1); SB(G1,11)->{G1,1}; SB(G2,20)->{G2,null};
  S0(2,E2); SB(G2,21)->{G2,2}; SB(G1,12)->{G1,null} (multi-row subselect -> null).
- undeploy s0, Deploy 3 same shape with `prior(0, id)`; same sequence, same values.
- Semantics: S0 not in partition spec -> broadcasts into every EXISTING
  partition's subquery keepall, never allocates; new partitions start empty;
  undeploy+redeploy resets partition state.
- Go surface (scout-verified): SubqueryValueWithOptions + Prev(0, Field)/Prior(0, Field)
  + SubqueryNullOnMultiple over KeepAll AsRecord source; per-partition subquery
  registries already broadcast fan-out events to existing partitions only.

### Ord 19 ContextKeySegmentedInvalid — intentionally-different (compile probes)
- runtime `java-runtime-5e338406dcc2ca1aaf6a`; NO per-execution static ID
  (analyzer skipped); suite static `java-11c10ab303bcf5d97107`.
- 9 probes, Java prefixes pinned byte-exact (see scout report). Go verdicts:
  - probe 1 (per-stream filter `SupportBean(dummy=1)` in partition spec):
    unrepresentable — KeyContextStream has no Filter field -> intentionally-different.
  - probe 2 (`partition by dummy`): Go rejects at Build (validateExprFields) -> pin.
  - probe 3 (mismatched key counts): Go rejects in NewKeyContextByStreams -> pin.
  - probe 4 (cross-stream key-type mismatch String vs Integer): ADD validation
    in NewKeyContextByStreams comparing key expr types -> pin.
  - probe 5 (same type twice): Go rejects -> pin.
  - probe 6 (subtype/supertype duplicate ISupportBaseAB/ISupportA): Go has no
    type inheritance -> intentionally-different.
  - probe 7 (statement on unlisted type under single-type context): ADD —
    record single-type streamKeys so validateSegmentedContextEventType fires.
    Must not break fan-out of unlisted types into existing partitions
    (runtime fan-out is event-delivery, not statement-build).
  - probe 8 (named window in partition criteria): verify Go rejection path
    (unknown type at context create vs statement build) -> pin whichever.
  - probe 9 (named window under segmented context, schema type unlisted):
    ADD validation in RegisterNamedWindowInModule/NamedWindowContext.
- Scenario records pinned Java prefixes; runner verifies each expressible Go
  rejection boundary before recording the pinned value (established pattern).

### Ord 20 ContextKeySegmentedTermByFilter — re-association fix only
- Existing case.context-key-segmented-term-by-filter is DV'd but carries the
  WRONG runtime ID java-runtime-820bb6f72b84ad070ce4 (ord 6 Subtype);
  scenario content matches ord 20 exactly (incl. `intPrimitive>= 0` filter).
- Fix: javaRuntimeIds/differentialVerifiedRuntimeIds ->
  java-runtime-e64c1b8b8cd2dcd39154; static java-a2d863c8efbad9870043.
  Ord 6 Subtype becomes unreferenced backlog (note in roadmap).
- Go scout's "Java counts terminating-filter event for absent key" claim does
  NOT produce an observable difference in this sequence (SB(C,-1)->none in
  both); no engine change.

## Deferred (different semantics cluster — next unit candidates)
- Ord 25 WPatternFireWhenAllocated: needs OnPattern trigger + fire-on-allocation
  timer evaluation (timer:interval(0) fires at partition allocation, not via
  AdvanceTime) + context variable readback.
- Ord 28 RegExFilter: needs `terminated after <duration>` on segmented contexts
  (declared but unexercised — no time advance; `like` filter exists).

## Files
- Shared core (primary agent only): internal/esper/context.go (key-type
  mismatch, single-type streamKeys), plan.go (probe 7/8 paths if needed),
  state.go (probe 9 named-window check).
- Parity assets: testdata/parity/context-key-segmented-subselect-prev-prior.*,
  context-key-segmented-invalid.*, runner(s) internal/app/parity/,
  oracle(s) tools/java-oracle/, run.go/run_test.go wiring.
- Manifest: new DV case + intentionally-different case; ord-20 re-association;
  capability context.key-segmented (or nearest) updated.

## Validation
- diff modes passing/0 differences; mutation families; make check.
