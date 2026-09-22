# Draft 4.506 'rowrecog-repetition' contract

## Selection
`RowRecogRepetition.java` — all 6 unreferenced executions (rowrecog subdomain,
same family as shipped RowRecogAfter). Java commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c`.

## Runtime IDs (verified unreferenced)
- ord0 `RowRecogRepetitionRepeats{soda=false}` `java-runtime-d6d3a12949ef3e346d80`
- ord1 `RowRecogRepetitionRepeats{soda=true}` `java-runtime-cdf672905648f281ad59`
- ord2 `RowRecogRepetitionPrev` `java-runtime-ad8cb0aeafdfcdced0f2`
- ord3 `RowRecogRepetitionInvalid` `java-runtime-941bbeda7e12013505b3` (flags=[INVALIDITY])
- ord4 `RowRecogRepetitionDocSamples` `java-runtime-aa81841e62a62098b578`
- ord5 `RowRecogRepetitionEquivalent` `java-runtime-a47314c7aeffaf6d632c`

## Observable contract (frozen by NextJavaContract506)
- ord0/ord1: identical runtime behavior; soda only switches compileDeploy path.
  Go replays one trace for both ords. 8 assertion families over SupportBean
  (partition by intPrimitive, measures A as a[,B as b,C as c], define X as
  X.theString like "X%"): single-bound (A{2}, (A{2}), A B{2} C, A (B{2}) C,
  A (B{2}|C{2}), A{2} B{2}), range (A A A? B ≡ A{2,3} B), up-to (A? A? B ≡
  A{,2} B), at-least (A A A* B ≡ A{2,} B ≡ A{2,4} B), nested equivalents
  ((A B){2}, A (B C){2}, (A B){1,2} C, (A B){,2} C, (A B){2,} C). Scalar
  measures hold the single SupportBean; array measures (repeated vars) hold
  SupportBean[]; non-match sequences assert listener not invoked.
- ord2: A{3} + prev(A.intPrimitive) define; sends A1..A9 intPrimitive
  1,4,2,6,5,6,7,8; milestone(0) after A3; one new row a=[b6,b7,b8].
- ord3 (INVALIDITY): 'create variable int myvariable = 0' + 9
  tryInvalidCompile probes with exact pinned messages (A{}, A{null},
  A{myvariable}, A{prev(A)}, A{-1}, A{,-1}, A{-1,10}, A{-1,}, A{5,3}).
- ord4: three doc-sample sub-scenarios on object-array TemperatureSensorEvent
  {id,device,temp} partitioned by device — (a) A{2} measures A[0].id,A[1].id;
  (b) A{2,} B and A{2,3} B measures A[0..2].id,B.id; (c) A{,2} B.
- ord5: ~60 runEquivalent(before,after) pairs asserting
  RowRecogPatternExpandUtil.expand() expansion text (compile-time surface, no
  events). Contract decision: record pinned expansion strings verified via
  Query.TypedDescription() if the Go surface exposes equivalent expansion
  text; otherwise exclude as compile-text surface (precedent: ords 15/16/25
  exclusions) with justification.
- No virtual time, no iterators, no old-stream; milestones are ordering
  no-ops; listeners only via assertPropsNew/assertListenerNotInvoked.

## Go surface (frozen by NextGoSurface506)
- Repeat(min,max)/Optional/ZeroOrMore/OneOrMore/Reluctant on variables AND
  nested groups/alternations; TagFieldAt/TagEvents/TagCount/TagFirst;
  Prev/PrevTag in DEFINE; Build-time bound validation. Ords 0-2, 4 map
  cleanly, zero predicted engine changes.
- ord3: partially unrepresentable (expression-form quantifiers); use
  build-error/pinned-prefix record convention (context_key_segmented_invalid
  precedent).
- ord5: expansion-text surface decision per above.

## File ownership
- Assets506 (parity-asset-worker): `tools/java-oracle/RowRecogRepetitionScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-repetition.sh`, `testdata/parity/rowrecog-repetition.json`.
  Must NOT hand-author traces/evidence.
- Primary agent: `internal/app/parity/rowrecog_repetition.go`, run.go dispatch,
  run_test.go tests, manifest/roadmap/CHANGELOG/PLANS, trace+evidence
  generation, gates, review, commit.

## Predicted shared-core impact
LOW — zero matcher/NFA changes predicted; possible error-message alignment
for invalid probes and the ord5 expansion-text decision.
