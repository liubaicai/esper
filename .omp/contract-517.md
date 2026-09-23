# Contract — Draft 4.517 `infra-nwtable-context`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.
Source: `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableContext.java` — single execution class `InfraContext` parameterized by `namedWindow`; ord 0 = `InfraContext(true)`, ord 1 = `InfraContext(false)`. Identical flow; only the create-infra EPL differs.

## Unit

- ord 0 `InfraContext{namedWindow=true}` — runtime `java-runtime-dfaacac6bb82c21d6d47`
- ord 1 `InfraContext{namedWindow=false}` — runtime `java-runtime-913c09693fb262b75d75`
- static IDs: confirm from `testdata/compat/static-manifest.json` (executionClass `InfraNWTableContext$InfraContext`, two entries — one per ordinal; record both).
- flags: none.

## Byte-exact EPLs (verbatim from Java source)

```
ctx:     @public create context ContextOne start SupportBean_S0 end SupportBean_S1
create0: @public context ContextOne create window MyInfra#keepall as (pkey0 string, pkey1 int, c0 long)
create1: @public context ContextOne create table MyInfra as (pkey0 string primary key, pkey1 int primary key, c0 long)
insert:  context ContextOne insert into MyInfra select theString as pkey0, intPrimitive as pkey1, longPrimitive as c0 from SupportBean
s1: @name('s1')context ContextOne select * from MyInfra output snapshot when terminated
s2: @name('s2')context ContextOne select count(*) as thecnt from MyInfra output snapshot when terminated
s3: @name('s3')context ContextOne select pkey0, count(*) as thecnt from MyInfra output snapshot when terminated
s4: @name('s4')context ContextOne select pkey0, count(*) as thecnt from MyInfra group by pkey0 output snapshot when terminated
s5: @name('s5')context ContextOne select pkey0, pkey1, count(*) as thecnt from MyInfra group by pkey0 output snapshot when terminated
s6: @name('s6')context ContextOne select pkey0, pkey1, count(*) as thecnt from MyInfra group by rollup (pkey0, pkey1) output snapshot when terminated
```

NOTE: `@name('sN')` is concatenated directly onto `context` — no space. Each compileDeploy is a
separate module sharing one RegressionPath; `@public` on ctx+infra is required for cross-module
visibility. `start X end Y` (not `initiated`) = NON-overlapping init-term.

## Event sequence (Java order)

1. deploy ctx → deploy create → deploy insert
2. send SupportBean_S0(id=0) — partition starts
3. send SupportBean(theString=E1, intPrimitive=10, longPrimitive=100)
4. send SupportBean(E2, 20, 200)
5. deploy+addListener s1..s6 IN ORDER — LATE DEPLOY into the already-active partition
6. milestone(0) — no-op, no step
7. send SupportBean_S1(id=0) — partition ends; all six statements fire `output snapshot when terminated` once each
8. undeployAll

## Asserted output (identical both ords; new-data only, any-order within batch)

- s1: {pkey0=E1,pkey1=10,c0=100L},{pkey0=E2,pkey1=20,c0=200L}
- s2: one new event {thecnt=2L}, zero old
- s3: {E1,2L},{E2,2L} — ungrouped aggregate emits ONE ROW PER INFRA ROW, each carrying total count
- s4: {E1,1L},{E2,1L}
- s5: {E1,10,1L},{E2,20,1L} — pkey1 not in group-by; Esper returns the group member's value
- s6: {E1,10,1L},{E2,20,1L},{E1,null,1L},{E2,null,1L},{null,null,2L} — rollup grouping sets (pkey0,pkey1),(pkey0),(); grouped-away columns render null

Types: count(*)→long; c0 long; pkey1 int; pkey0 string. Cross-statement record order on the S1
event is engine dispatch order — pin from the generated Java trace.

## Go surface (verified by NextGoSurface517)

- ctx: `CreateInitiatedTerminatedContext(env, "ContextOne", Literal("global"), Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S0")), Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean_S1")))`
- ord 0 create: `CreateNamedWindow(env, "MyInfra", schema, NamedWindowContext("ContextOne"), NamedWindowRetention(KeepAll()))` — schema = map/struct with pkey0 string, pkey1 int, c0 int64
- ord 1 create: `CreateTable(env, "MyInfra", []TableColumn{PrimaryKeyColumn[string]("pkey0"), PrimaryKeyColumn[int]("pkey1"), TableColumnOf[int64]("c0")}, TableContext("ContextOne"))`
- insert: `From[Bean](env,"SupportBean").Select(Alias("pkey0",theString),Alias("pkey1",intPrimitive),Alias("c0",longPrimitive)).InsertInto("MyInfra", StatementName("insert"), WithContext("ContextOne"))`
- s1: `FromNamedWindow(env,"MyInfra").Query(StatementName("s1"), WithContext("ContextOne"), WithOutput(OutputSnapshotWhenTerminated()))` (ord 0) / `FromTable(env,"MyInfra").Query(...)` (ord 1)
- s2: `.Aggregate(Alias("thecnt", CountAll()))` + same query options
- s3: `.Aggregate(Alias("pkey0", <pkey0 field>), Alias("thecnt", CountAll()))` — ungrouped per-row
- s4: `.Aggregate(Alias("pkey0",...), Alias("thecnt",CountAll())).GroupBy(...)` — check exact GroupBy API on AggregateStream
- s5: same as s4 with extra pkey1 alias
- s6: `.GroupByRollup(...)` — check exact rollup API
- Late deploy: deploy s1..s6 AFTER the S0 partition is active and E1/E2 are inserted — Go must instantiate agent instances against existing infra state.

## Risk areas to probe before freezing implementation

- s3 ungrouped `select pkey0, count(*)` emitting one row per infra row (not one group row).
- s5 grouped query projecting a non-grouped column (pkey1) — group-member value semantics.
- s6 rollup null super-aggregate rows.
- Late-deployed statements into an already-active context partition reading existing window/table state.
- Table variant: `output snapshot when terminated` over a contexted table.

## Allowed files (asset lane)

- `tools/java-oracle/InfraNWTableContextScenarioOracle.java` (new)
- `tools/java-oracle/run-infra-nwtable-context.sh` (new)
- `testdata/parity/infra-nwtable-context.json` (new)
- `internal/app/parity/infra_nwtable_context.go` (new)
- `internal/app/parity/run.go` (mode wiring only: `infra-nwtable-context` + `-diff`)
- `internal/app/parity/run_test.go` (5-test family: passing-evidence, trace mutations, raw-scenario mutations, runtime-ID mapping, help listing)

## Forbidden

- `internal/esper/**` (shared core — primary agent owns; report gaps, do not patch)
- `internal/compat/**`, manifest, roadmap, CHANGELOG, PLANS.md
- No formatter, no commit/push, no hand-authored traces/evidence.

## Acceptance

- Oracle replays both cases, emits listener records in the TraceWriter shape.
- Runner maps every deploy statement to the Go surface; listeners on s1..s6.
- Scenario validates against `internal/compat/scenario.go` op rules.
