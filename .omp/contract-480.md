# Contract — Draft 4.480 `infra-nwtable-on-merge-nested-insertstream`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 4-7: nested-event merge assignment + insert-stream merge.
Asset-only (Go scout confirmed full API coverage). New sibling runner
`infra-nwtable-on-merge-nested` — do NOT extend the checked-in ords 0-3 runner.

- ord 4 `InfraUpdateNestedEvent{namedWindow=true}` — `java-runtime-065003de88aca37795b8`
- ord 5 `InfraUpdateNestedEvent{namedWindow=false}` — `java-runtime-2e8d691b5e2c927038d7`
- ord 6 `InfraOnMergeInsertStream{namedWindow=true}` — `java-runtime-2c687a68317caea3c148`
- ord 7 `InfraOnMergeInsertStream{namedWindow=false}` — `java-runtime-081455731ccafbf6847c`
All flags []; static OBSERVEROPS heuristic only.

### Ords 4-5 (InfraUpdateNestedEvent)
Each execution runs TWO sub-scenarios in sequence — metaType "map" then "objectarray" —
each a full compileDeploy(path) -> send -> milestoneInc -> FAF assert -> undeployAll cycle.
Shared AtomicInteger milestone: map sub-run = milestone 0, objectarray = milestone 1.
4 case variants: nested-nw-map, nested-nw-oa, nested-table-map, nested-table-oa.

Verbatim EPL per sub-run (metaType in {map, objectarray}):
```
@public create <metaType> schema Composite as (c0 int);
@buseventtype @public create <metaType> schema AInfraType as (k string, cflat Composite, carr Composite[]);
namedWindow=true:  @public create window AInfra#lastevent as AInfraType;
namedWindow=false: @public create table AInfra (k string, cflat Composite, carr Composite[]);   — no primary key
insert into AInfra select theString as k, null as cflat, null as carr from SupportBean;
@public @buseventtype create <metaType> schema MyEvent as (cf Composite, ca Composite[]);
on MyEvent e merge AInfra when matched then update set cflat = e.cf, carr = e.ca   — NO where clause
```
Steps per sub-run: deploy module (6 statements, none @name'd) -> send SupportBean("E1",1)
-> send MyEvent (map payload {"cf":{"c0":1},"ca":[{"c0":1},{"c0":2}]}; objectarray payload
[[1],[[1],[2]]]) -> milestoneInc -> FAF "select cflat.c0 as cf0, carr[0].c0 as ca0,
carr[1].c0 as ca1 from AInfra" -> assert single row {cf0:1, ca0:1, ca1:2} -> undeployAll.
Observable surface: NO listeners, NO named statements — only the FAF result row.

### Ords 6-7 (InfraOnMergeInsertStream)
Single module via env.compileDeploy(epl) (no RegressionPath), listeners on s1-s4 only.
```
create schema WinOMISSchema as (v1 string, v2 int);
namedWindow=true:  @name('Create') create window WinOMIS#keepall as WinOMISSchema;
namedWindow=false: @name('Create') create table WinOMIS as (v1 string primary key, v2 int);
on SupportBean_ST0 as st0 merge WinOMIS as win where win.v1=st0.key0 when not matched then insert into StreamOne select * then insert into StreamTwo select st0.id as id, st0.key0 as key0 then insert into StreamThree(id, key0) select st0.id, st0.key0 then insert into StreamFour select id, key0 where key0="K2" then insert into WinOMIS select key0 as v1, p00 as v2;
@name('s1') select * from StreamOne;
@name('s2') select * from StreamTwo;
@name('s3') select * from StreamThree;
@name('s4') select * from StreamFour;
```
Steps: deploy 7-statement module (merge UNNAMED; StreamOne-Four implicit named streams) ->
send SupportBean_ST0("ID1","K1",1) [ctor (id,key0,p00)] -> s1/s2/s3 each new {id:ID1,key0:K1},
s4 silent -> milestone(0) -> send SupportBean_ST0("ID1","K2",2) -> s4 new {id:ID1,key0:K2};
'Create' iterator ORDERED {v1:K1,v2:1},{v1:K2,v2:2} -> undeployAll.
Both sends not-matched; all five not-matched actions fire in order.

## Harness
One scenario `infra-nwtable-on-merge-nested`, six cases (nested-nw-map, nested-nw-oa,
nested-table-map, nested-table-oa, insertstream-nw, insertstream-table), fresh engine per
case with WithRuntimeURI(runtimeID). Ops: deploy/deployed/send/snapshot/faf-select (for the
nested FAF assert — check existing faf ops in internal/compat/scenario.go)/undeploy-all.
Listeners on s1-s4 for insertstream cases; none for nested cases.

## Go surface (asset-only)
SetColumn composite values + nested schema metadata; WhenNotMatchedActions +
ThenInsertInto/ThenInsertIntoWhen/ThenInsertIntoTarget; RegisterMap/RegisterObjectArray
for the metaType schemas; Unnest not needed here. Existing mirror tests:
internal/esper/trigger_test.go ~L2427 (nested assignment) and ~L2940 (insert-stream).

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergeNestedScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-nested.sh (new)
- testdata/parity/infra-nwtable-on-merge-nested.json (new)
- internal/app/parity/infra_nwtable_on_merge_nested.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/**, internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.
- Do NOT modify the checked-in infra-nwtable-on-merge.* assets or runner.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-nested-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
