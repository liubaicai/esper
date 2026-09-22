# Draft 4.505 contract: rowrecog-after

## Scope
`RowRecogAfter.java` ALL 6 unreferenced executions (AFTER MATCH SKIP family):
- ord0 `RowRecogAfterCurrentRow` `java-runtime-040bbf4e7c7c45f3cd76`
- ord1 `RowRecogAfterNextRow` `java-runtime-ad336d2b8e1195e4abf6`
- ord2 `RowRecogSkipToNextRow` `java-runtime-ebf5f4119f980f7d9258`
- ord3 `RowRecogVariableMoreThenOnce` `java-runtime-3ff12edd4f2bb4ecec9b`
- ord4 `RowRecogSkipToNextRowPartitioned` `java-runtime-21259f97372ba54fe074`
- ord5 `RowRecogAfterSkipPastLast` `java-runtime-a960711a30045575a880`
- static `java-41b5fd1eb615163a8c32` (collection file)
- Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; no flags; no virtual time.

## Observable contract (from NextJavaContract505)
Event type `SupportRecogBean(theString string, value int)`. Every statement:
`@name('s0') select * from SupportRecogBean#keepall match_recognize(...)`.
Assertions per send: listener lastNew AND statement iterator — scenario needs
`send` + `snapshot` ops (snapshot = iterator read).

- ord0: `measures A.theString as a, B[0].theString as b0, B[1].theString as b1
  after match skip to current row pattern (A B*) define A as A.theString like "A%",
  B as B.theString like "B%"`. A1(1)→[{A1,null,null}]; B1(2)→[{A1,B1,null}]
  (skip-to-current-row extends same match). Runs TWICE in Java (compileDeploy +
  eplToModelCompileDeploy) — mirror as undeploy+redeploy replay.
- ord1: same measures/pattern but `AFTER MATCH SKIP TO NEXT ROW`. A1(1)→
  [{A1,null,null}]; B1(2)→listener NOT invoked, iterator [{A1,B1,null}]
  (incremental mode suppresses callback; iterator reflects extended match).
- ord2: `measures A.theString as a_string, B.theString as b_string all matches
  after match skip to next row pattern (A B) define B as B.value > A.value
  order by a_string, b_string`. E1(5); E2(3)→none,iter empty; E3(6)→[{E2,E3}];
  E4(4)→none; E5(6)→[{E4,E5}],iter 2 rows; E6(10)→[{E5,E6}],iter 3 rows;
  E7(9); E8(4)→none, iter stays 3.
- ord3: `measures A[0].theString as a0, B.theString as b, A[1].theString as a1
  all matches after match skip to next row pattern ( A B A ) define A as
  (A.value = 1), B as (B.value = 2)`. E1(3),E2(1),E3(2),E4(5),E5(1),E6(2)→none,
  iter empty; E7(1)→[{E5,E6,E7}]; E8(2); E9(1)→[{E7,E8,E9}], iter 2 rows.
- ord4: `partition by theString measures A.theString as a_string, A.value as
  a_value, B.value as b_value all matches after match skip to next row
  pattern (A B) define B as (B.value > A.value) order by a_string`. 22-event
  interleaved S1..S4; expected lastNew singles {S1,4,6},{S4,-1,10},{S4,10,11},
  {S1,4,7},{S4,-1,12},{S2,4,5}; cumulative iterator ordered by a_string.
- ord5: `measures A.theString as a_string, B.theString as b_string all matches
  after match skip past last row pattern (A B) define B as B.value > A.value
  order by a_string, b_string`. Same sends as ord2; E6(10)→NONE (E5-E6 not a
  match: skip-past-last forbids overlap — contrast ord2). Ends with
  undeployModuleContaining('s0').

## File ownership
- Assets505 (parity-asset-worker): `tools/java-oracle/RowRecogAfterScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-after.sh`, `testdata/parity/rowrecog-after.json`.
- Primary agent: `internal/app/parity/rowrecog_after.go`, `run.go` wiring,
  `run_test.go` tests, manifest/roadmap/CHANGELOG/PLANS.md.

## Constraints
- Java oracle FIXED at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c.
- Scenario ops: `case`, `deploy` (with verbatim `epl`), `send`, `snapshot`,
  `undeploy-all`. ord0 replays deploy→sends→undeploy-all→deploy→sends.
- Listener record: {case,operation:"listener",statement:"s0",sequence,time,new:[rows]}.
- Snapshot record: {case,operation:"snapshot",statement:"s0",sequence:0,time,new:[rows]}.
- ord1 B1 send produces NO listener record but the snapshot still shows the
  extended match — the scenario must place the snapshot after the send.
- Predicted zero shared-core change (all skip strategies, tag measures,
  partition-by, all-matches, order-by, snapshot iterator exist).
