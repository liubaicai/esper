# Contract — Draft 4.494 `view-group-closure`

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c.
Source: regression-lib/.../suite/view/ViewGroup.java.

## Executions (last 3 unreferenced of ViewGroup.java; closes the file)

| ord | name | runtimeId | staticId | flags |
|-----|------|-----------|----------|-------|
| 7 | ViewGroupInvalid | java-runtime-a72aa6eebc4ce8d21115 | java-cd39f97bf5d0490c295a | [] |
| 8 | ViewGroupLengthWinWeightAvg | java-runtime-e718af611543b6d40436 | java-586becfef68e2842ef18 | EXCLUDEWHENINSTRUMENTED, PERFORMANCE |
| 19 | ViewGroupEscapedPropertyText | java-runtime-d9b2e762c318369875e5 | java-3426d2a7294c0f93c788 | SERDEREQUIRED |

## Ord 7 — ViewGroupInvalid (5 tryInvalidCompile probes, no deploy)

All prefixes start `Failed to validate data window declaration: `.
1. `select * from SupportBean#groupwin(theString)#length(1)#groupwin(theString)#uni(intPrimitive)` → `Multiple groupwin-declarations are not supported`
2. `select avg(price), symbol from SupportMarketDataBean#length(100)#groupwin(symbol)` → `Invalid use of the 'groupwin' view, the view requires one or more child views to group, or consider using the group-by clause`
3. `select * from SupportBean#keepall#groupwin(theString)#length(2)` → `The 'groupwin' declaration must occur in the first position`
4. `select * from SupportBean#groupwin(theString)#length(2)#merge(theString)#keepall` → `The 'merge' declaration cannot be used in conjunction with multiple data windows` — UNREPRESENTABLE in Go (no merge view): pinned-only record.
5. `create schema MyEvent(somefield null);\nselect * from MyEvent#groupwin(somefield)#length(2)` → `Group-window received a null-typed criteria expression`

## Ord 8 — ViewGroupLengthWinWeightAvg (performance smoke)

EPL: `@name('s0') select * from SupportSensorEvent#groupwin(type)#length(10000000)#weighted_avg(measurement, confidence)`
Steps: deploy+attach listener; send 100 SupportSensorEvent(0,"A","1",i,i); send 10000 SupportSensorEvent(0,"A1","1",i,i); wall-clock assert (not trace-visible); undeployAll.
SupportSensorEvent(int id, String type, String device, double measurement, double confidence).
Trace: deployed record + send markers only (listener never asserted; oracle records no per-event listener rows — keep the trace thin, e.g. deployed + a sent-count marker per batch).
Semantic pin: Java weighted_avg emits NaN when data exists but total weight is 0; Go currently emits Null — fix in shared core even though the thin trace does not observe it (engine unit test pins it).

## Ord 19 — ViewGroupEscapedPropertyText (compile+deploy+undeploy smoke)

Module EPL (3 statements):
```
create schema event as <EventWithTags>;
insert into stream1 select name, tags from event;
select name, tags('a\.b') from stream1.std:groupwin(name, tags('a\.b')).win:length(10) having count(1) >= 5;
```
EventWithTags: name String, tags Map (key "a.b"). Escaped mapped-property key `a\.b` → map key `a.b`.
Trace: deployed records only.

## Shared-core changes (primary agent owns)

- `internal/esper/expr.go`: WeightedAvg returns NaN (not Null) when count>0 and total weight==0.
- `internal/esper/runtime.go`: add weighted-avg to groupwin implicit-grouping detection.
- `internal/esper/plan.go` (or wherever window-chain validation lives): groupwin checks —
  multiple groupwin → "Multiple groupwin-declarations are not supported";
  groupwin last (no child) → "requires one or more child views";
  groupwin not first → "must occur in the first position";
  null-typed key → "Group-window received a null-typed criteria expression".

## Parity assets (asset worker owns — disjoint files)

- `internal/app/parity/view_group_closure.go` (runner, modes `view-group-closure` / `-diff`)
- `testdata/parity/view-group-closure.json` (scenario)
- `tools/java-oracle/ViewGroupClosureScenarioOracle.java` + `run-view-group-closure.sh`
- run.go dispatch + run_test.go pins (assigned to asset worker; primary reviews)

## Forbidden

No edits to /root/app/esper. No formatter/lint/test runs by subagents. No manifest/roadmap/CHANGELOG edits by subagents. No hand-authored traces.
