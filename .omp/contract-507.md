# Draft 4.507 'rowrecog-greedyness-ops' contract

## Selection
`RowRecogGreedyness.java` (3 unreferenced) + `RowRecogOps.java` (3
unreferenced) — 6 executions, same rowrecog pattern-semantics cluster.
Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`. No flags, no
virtual time, no build-error probes. Event type SupportRecogBean
(theString:String, value:int). Listener + iterator assertions.

## Runtime IDs (verified unreferenced)
- `RowRecogReluctantZeroToOne` `java-runtime-6db0d9ba9099e88cb87c`
- `RowRecogReluctantZeroToMany` `java-runtime-20afb4135dd06ce5fb8c`
- `RowRecogReluctantOneToMany` `java-runtime-470384aabd98754fe4f4`
- `RowRecogUnlimitedPartition` `java-runtime-ca4a83fa8b88dd4d2b32`
- `RowRecogAlterWithinConcat` `java-runtime-2e6e0566fe07043c23c7`
- `RowRecogRegex` `java-runtime-c489386411c57214eb75`

## Observable contract (frozen by NextJavaContract507)
1. ReluctantZeroToOne: `A?? B?` keepall, measures A.theString a_string,
   B.theString b_string, defines A v=1, B v=1. E1(v=1) → [{null,'E1'}]
   (reluctant A?? prefers skipping A; single event binds to B).
2. ReluctantZeroToMany: `A*? B? C`, measures A[0..2].theString a0/a1/a2,
   B b, C c; defines A v=1, B v in (1,2), C v=3. Sends E1,E2,E3(v=1),E4(v=3)
   → [{E1,E2,null,E3,E4}]; E11-E14(v=1),E15(v=3) → [{E11,E12,E13,E14,E15}];
   E16(v=1),E17(v=3) → [{null,null,null,E16,E17}]; E18(v=3) →
   [{null,null,null,null,E18}].
3. ReluctantOneToMany: `A+? B? C` same measures; E1-E3(v=1),E4(v=3) →
   [{E1,E2,null,E3,E4}]; E11-E14,E15 → [{E11,E12,E13,E14,E15}];
   E16(v=1),E17(v=3) → [{E16,null,null,null,E17}]; E18(v=3) → listener NOT
   invoked (A+ requires ≥1).
4. UnlimitedPartition: partition by value, `A B`, defines A theString='A',
   B theString='B'. 500 A(i),B(i) pairs → invoked each; 500 A-only
   (value i+100000) → not invoked; 500 B → invoked each. Stresses partition
   state repo beyond INITIAL_COLLECTION_MIN=100.
   NOTE: manifest `case.rowrecog-unlimited-partition` exists as
   'implemented' with NO runtime ID — this unit attaches the runtime ID
   via the new case; update the old case note to point at the DV case.
5. AlterWithinConcat: `(A|B)(C|D)` all matches, measures A..D theString;
   defines A v=1,B v=2,C v=3,D v=4. E1(3),E2(1),E3(2),E4(5),E5(1) → not
   invoked; E6(3) → [{a:'E5',b:null,c:'E6',d:null}]; E7(2),E8(3) →
   [{null,'E7','E8',null}].
6. Regex: pure JVM String.matches asserts, zero Esper runtime surface —
   pin via `unrepresentable` record or exclude with justification
   (JVM-only, precedent: ords 15/16/25 exclusions).

## Go surface (frozen by NextGoSurface507)
- Reluctant()/Optional()/ZeroOrMore()/OneOrMore(), RowAlternation inside
  RowSequence, PartitionBy, AllMatches/FirstMatch, TagFieldAt/TagField,
  In/Like defines, Statement.Snapshot — all exist and unit-tested
  (rowrecog_greedyness_test.go mirrors ords 0-2 event-for-event;
  rowrecog_regex_test.go covers (A|B)(C|D) shape; partition scale test
  covers 128 partitions).
- Predicted shared-core changes: ZERO.
- Runner MUST call .FirstMatch() for non-all-matches EPLs (Greedyness
  ords 0-2, Ops ord 5) and .AllMatches() for Ops ord 7.

## File ownership
- Assets507 (parity-asset-worker):
  `tools/java-oracle/RowRecogGreedynessOpsScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-greedyness-ops.sh`,
  `testdata/parity/rowrecog-greedyness-ops.json`. No traces/evidence.
- Primary agent: `internal/app/parity/rowrecog_greedyness_ops.go`, run.go
  dispatch, run_test.go tests, manifest/roadmap/CHANGELOG/PLANS,
  trace+evidence generation, gates, review, commit.
