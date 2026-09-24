# Draft 4.533 contract — infra-named-window-subquery

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowSubquery.java (3 executions, flags=[], no virtual time, each ends undeployAll).

## Executions

### ord 0 InfraSubqueryTwoConsumerWindow — java-runtime-901e88676d86ad580af1 / static java-f8f4ec7164371451341d
EPL (verbatim, leading \n literal):
```
\n create window MyWindowTwo#length(1) as (mycount long);\n @Name('insert-count') insert into MyWindowTwo select 1L as mycount from SupportBean;\n create variable long myvar = 0;\n @Name('assign') on MyWindowTwo set myvar = (select mycount from MyWindowTwo);
```
One SupportBean("E1",1) send. No listeners. Assertion: variable myvar (deployment 'assign') == 1L. Semantics: on-window set trigger fires AFTER window insert; scalar subquery sees the just-inserted row. #length(1) bounds scalar subquery to <=1 row.

### ord 1 InfraSubqueryLateConsumerAggregation — java-runtime-4261803b5e9dcef8a867 / static java-ff6a2dd6de98a3f1bb81
THREE separate compileDeploy calls sharing one path:
1. `@public create window MyWindow#keepall as SupportBean`
2. `insert into MyWindow select * from SupportBean` (unnamed)
3. `@name('s0') select * from MyWindow where (select count(*) from MyWindow) > 0` + addListener("s0")
Events: E1,E2 sent BETWEEN deploys 2 and 3; E3 sent after listener attach. Assertion: listener invoked (E3). Preload rows E1,E2 unobserved (listener attaches post-deploy). Uncorrelated count(*) subquery evaluates live per event.

### ord 2 InfraSubqueryWithFilterInParens — java-runtime-95adfcdb21f327e1cc4e / static java-557b2408e6489254dd8e
EPL (verbatim, trailing \n literal):
```
create window MyWindow#keepall as SupportBean;\n@name('insert') insert into MyWindow select * from SupportBean;\n@name('s0') select exists (select * from MyWindow(theString='E1')) as c0 from SupportBean_S0;\n
```
Listener s0 post-deploy. Sequence: S0(0)→c0=false; send E2; S0(0)→c0=false; send E1; S0(0)→c0=true. assertEqualsNew shape: exactly one new event, c0 Boolean. Parens filter = named-window filter inside uncorrelated exists-subquery, re-evaluated live per S0.

## Go API mapping (from GoSurface533 scout)
- Window: `esper.CreateNamedWindow(env, "MyWindowTwo", schema, esper.NamedWindowRetention(esper.LengthWindow(1)))` / `KeepAll()`.
- Insert: `esper.OnEvent(esper.From[Bean](env,"SupportBean")).InsertIntoNamedWindow(name, esper.SetColumn("mycount", esper.Literal(int64(1))))` or `CopyMatchingFields()` for `select *`.
- Variable: `env.RegisterVariable("myvar", int64(0))`; read via `engine.GetVariable("myvar")`.
- On-window set: `esper.OnRecord(esper.FromNamedWindow(env,"MyWindowTwo")).SetVariable("myvar", esper.SubqueryValue[int64](esper.FromNamedWindow(env,"MyWindowTwo"), esper.Field[any,int64]("mycount")))`.
- Ord 1 where: `esper.Select(esper.FromNamedWindow(env,"MyWindow"), esper.Wildcard()).Filter(esper.Greater(esper.SubqueryCount(esper.FromNamedWindow(env,"MyWindow")), esper.Literal(int64(0))))` — or stream .Filter on the RecordStream. Deploy as 3 separate Build/Deploy calls.
- Ord 2 exists: `esper.Select(esper.From[S0](env,"SupportBean_S0"), esper.Alias("c0", esper.SubqueryExists(esper.FromNamedWindow(env,"MyWindow").Filter(esper.Equal[string](esper.Field[any,string]("theString"), esper.Literal("E1"))), nil)))` — check SubqueryExists signature (predicate may be optional/nil-able; if not, use SubqueryExistsWithOptions or a tautology predicate).

## Deliverables (asset worker)
- testdata/parity/infra-named-window-subquery.json (3 cases; pin javaRuntimes/javaNames/javaStaticIds/javaFlags; EPL verbatim in observation fields)
- internal/app/parity/infra_named_window_subquery.go (runner + bean types + Java-render helpers)
- tools/java-oracle/InfraNamedWindowSubqueryScenarioOracle.java + run-infra-named-window-subquery.sh
- run.go mode registration + run_test.go case wiring

## Forbidden
Subagent: no formatter/lint/tests, no manifest/roadmap/CHANGELOG/PLANS edits, no commit/push, no engine (internal/esper) edits, no hand-authored traces/evidence.
