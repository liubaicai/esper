# Contract — Draft 4.491 `infra-table-context`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableContext.java — ALL 3 executions (ords 0-2), one file, one surface: tables declared under a context (partitioned, non-overlapping initiated/terminated, context-visibility compile errors).

ORDERING PITFALL: class definition order is Invalid(35) → NonOverlapping(52) → Partitioned(87), but executions() order is Partitioned(ord 0), NonOverlapping(ord 1), Invalid(ord 2). Use ordinal order.

Runtime IDs:
- ord 0 `java-runtime-8b5b2d92d108da7e8fb2` InfraPartitioned, flags []
- ord 1 `java-runtime-5c625828c160a26df78b` InfraNonOverlapping, flags []
- ord 2 `java-runtime-df03d93aca6b6b5ccd59` InfraTableContextInvalid, flags []

Static IDs:
- ord 0 `java-62ab3ac014745d5ab5c5`
- ord 1 `java-969b28d4058f010a19af`
- ord 2 `java-69b99206b5ca50456737`

## ord 0 InfraPartitioned (lines 87-109)
Deploys (each own module, shared RegressionPath):
1. `@public create context CtxPerString partition by theString from SupportBean, p00 from SupportBean_S0`
2. `@public context CtxPerString create table MyTable(thesum sum(int))` — UNKEYED table
3. `context CtxPerString into table MyTable select sum(intPrimitive) as thesum from SupportBean`
4. `@name('s0') context CtxPerString select MyTable.thesum as c0 from SupportBean_S0` + listener — dotted table-column access in select clause

Sequence: SupportBean("E1",50); SupportBean("E2",20); SupportBean("E1",60); SupportBean_S0(0,"E1") → s0 c0=110; milestone(0) no-op; SupportBean_S0(0,"E2") → s0 c0=20; undeployAll.

## ord 1 InfraNonOverlapping (lines 52-85)
Deploys:
1. `@public create context CtxNowTillS0 start @now end SupportBean_S0`
2. `@public context CtxNowTillS0 create table MyTable(pkey string primary key, thesum sum(int), col0 string)` — KEYED
3. `context CtxNowTillS0 into table MyTable select sum(intPrimitive) as thesum from SupportBean group by theString` — group-by key maps to pkey; col0 never populated
4. `@name('s0') context CtxNowTillS0 select pkey as c0, thesum as c1 from MyTable output snapshot when terminated` + listener — emits entire partition table as one new-data batch per partition end

Sequence: SupportBean("E1",50); SupportBean("E2",20); milestone(0); SupportBean("E1",60); SupportBean_S0(-1) → terminates partition 1, s0 last-new batch (any order): {E1,110},{E2,20}; `context CtxNowTillS0 create index MyIdx on MyTable(col0)` deployed mid-run; `context CtxNowTillS0 select * from MyTable, SupportBean_S1 where col0 = p11` deploy-only (no listener, no S1 sent); SupportBean("E3",90); SupportBean("E1",30); SupportBean("E3",10); milestone(1); SupportBean_S0(-1) → partition 2 batch {E1,30},{E3,100}; undeployAll.

## ord 2 InfraTableContextInvalid (lines 35-50)
Deploys:
1. `@public create context SimpleCtx start after 1 sec end after 1 sec`
2. `@public context SimpleCtx create table MyTable(pkey string primary key, thesum sum(int), col0 string)`

Three tryInvalidCompile probes (assertMessage = startsWith prefix):
- `select * from MyTable` → `Table by name 'MyTable' has been declared for context 'SimpleCtx' and can only be used within the same context [`
- `select (select * from MyTable) from SupportBean` → `Failed to plan subquery number 1 querying MyTable: Mismatch in context specification, the context for the table 'MyTable' is 'SimpleCtx' and the query specifies no context  [select (select * from MyTable) from SupportBean]`
- `insert into MyTable select theString as pkey from SupportBean` → `Table by name 'MyTable' has been declared for context 'SimpleCtx' and can only be used within the same context [`

undeployAll.

## Shared-core change (primary agent, DONE)
`internal/esper/state.go`: `TableContext(name)` TableOption + `TableDefinition.contextName` + `Context()` accessor.
`internal/esper/plan.go`: `validateTableContext` — checks streamTable sources (input/joins/aggregate joins), subquery sources, tableTarget, trigger targets against query.contextName. Java-style messages.

## Go mapping notes
- ord 0: CreateKeyContextByStreams + CreateTable(TableContext) + IntoTable(WithContext) + SelectFromTableWhere/TableField or dotted table-column read (see infra_table_access_parity_test.go:127 precedent).
- ord 1: CreateInitiatedTerminatedContext (Literal(true) start = @now) + keyed CreateTable(TableContext) + grouped IntoTable(WithContext) + FromTable().Select(...).Query(WithContext + OutputSnapshotWhenTerminated) + late Table.CreateIndex + JoinMany over FromTable.
- ord 2: build-error op precedent from context_key_segmented_invalid.go; Go rejection verified for code+substring before recording pinned Java prefix.
- Runner skeleton: clone infra_table_subquery.go.

## Allowed files (asset worker)
- tools/java-oracle/InfraTableContextScenarioOracle.java
- tools/java-oracle/run-infra-table-subquery.sh clone → run-infra-table-context.sh
- testdata/parity/infra-table-context.json
- internal/app/parity/infra_table_context.go
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/** (shared core — primary agent owns)
- testdata/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- generated traces/evidence (primary agent generates)
- commits/pushes

## Validation (primary agent)
Java trace via run script; `-mode infra-table-context-diff` zero-diff; run_test.go pins; make check; parity review; commit/push.
