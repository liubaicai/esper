# Draft 4.508 'rowrecog-prev' contract

## Selection
`RowRecogPrev.java` (5 unreferenced) + `RowRecogDataSet.java`
RowRecogExampleWithPREV (1) — 6 executions sharing prev() semantics inside
match_recognize DEFINE. Java commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c`. No flags. Event type
SupportRecogBean{theString:String, value:int, cat:String} (ctors
(theString), (theString,value), (theString,cat,value)).

## Runtime IDs (verified unreferenced)
- `RowRecogTimeWindowPartitionedSimple` `java-runtime-c096e55f89e15fcc54b7` — VIRTUAL TIME
- `RowRecogPartitionBy2FieldsKeepall` `java-runtime-9c42dd39d70d409a64d7`
- `RowRecogUnpartitionedKeepAll` `java-runtime-1200bc1d6899155bac4a`
- `RowRecogTimeWindowUnpartitioned` `java-runtime-241336e6d46c14fbcf06` — VIRTUAL TIME
- `RowRecogTimeWindowPartitioned` `java-runtime-59f7afa17be17f57fbd5` — VIRTUAL TIME
- `RowRecogExampleWithPREV` `java-runtime-41c5b0a59540aec34895`

## Observable contract (frozen by NextJavaContract508)
1. TimeWindowPartitionedSimple: `#time(5 sec)` partition by cat, measures
   A.cat cat, A.theString a_string, all matches, pattern (A), define A as
   PREV(A.value) = (A.value - 1), order by a_string. Timers
   0,1000,2000,2500,6200,6500,7000,10000,11199,11200,11600,16000 with the
   pinned send sequence; listener lastNew + iterator asserted; time-window
   expiry drops rows at 11200/11600/16000.
2. PartitionBy2FieldsKeepall: keepall, partition by theString,cat, measures
   a_string/a_cat/a_value/b_value, all matches, pattern (A B), defines
   A.value > PREV(A.value), B.value > PREV(B.value), order by
   a_string,a_cat. Null cat partition (S1/null/9) produces no output.
3. UnpartitionedKeepAll: TWO statements (undeployModuleContaining +
   redeploy). Stmt1 define A.value > PREV(A.value); stmt2 define
   PREV(A.value,2) = 5 (indexed prev). Order by a_string.
4. TimeWindowUnpartitioned: `#time(5)` measures a_string/b_string, all
   matches, pattern (A B), define A as PREV(A.theString,3)='P3' and
   PREV(A.theString,2)='P2' and PREV(A.theString,4)='P4' and
   Math.abs(prev(A.value,0))>=0, B as B.value in (PREV(B.value,4),
   PREV(B.value,2)). Multi-index prev on string+numeric, prev(x,0),
   Math.abs, IN over prev. Iterator expiry at 9500/11500.
5. TimeWindowPartitioned: same as 4 plus partition by cat, measures add
   A.cat cat, order by cat. Per-partition prev history + per-partition
   time expiry.
6. ExampleWithPREV: keepall, 20 measure columns (a/b scalar, c[0..2],
   d scalar, e[0..1], f[0..1] theString+value), ALL MATCHES, after match
   skip to current row, pattern (A B C* D E* F+), A undefined (matches
   any), defines B/C/D/E/F with prev() comparisons. E1/100..E9/84; E7→1
   row, E8→3 rows, E9→5 rows.

## Go surface (frozen by NextGoSurface508)
- Prev/PrevTag/Prior/PriorTag, In, Subtract, Abs, TagField/TagFieldAt —
  all exist; engine prev support (previousRolling + previousByEvent,
  retention sized by max offset, time-window eviction) is unit-tested
  (rowrecog_prev_test.go mirrors 4 of 5 executions; dataset test mirrors
  ExampleWithPREV).
- Runner needs `advance-time` step op (already in scenario schema;
  precedent context_nested_initterm.go:360) for the 3 virtual-time
  executions, plus `cat` field on the bean and OrderBy(ResultField).
- Predicted shared-core changes: ZERO.

## File ownership
- Assets508 (parity-asset-worker):
  `tools/java-oracle/RowRecogPrevScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-prev.sh`,
  `testdata/parity/rowrecog-prev.json`. No traces/evidence.
- Primary agent: `internal/app/parity/rowrecog_prev.go`, run.go dispatch,
  run_test.go tests, manifest/roadmap/CHANGELOG/PLANS, trace+evidence
  generation, gates, review, commit.
