# Contract - Draft 4.489 `infra-table-faf-execute-query`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableFAFExecuteQuery.java — ALL 4 executions (ords 0-3), one file, one
surface: fire-and-forget insert/delete/update/select against tables.
Born-DV unit, fully asset-only (no shared-core work).

Runtime IDs (java-execution-inventory.jsonl lines 2973-2976):
- ord 0 `java-runtime-a79e19dc5f135bb8e628` InfraFAFInsert [FIREANDFORGET]
- ord 1 `java-runtime-a68109b2bb91de4ce1cd` InfraFAFDelete [FIREANDFORGET]
- ord 2 `java-runtime-196ff792f0f739c8d97c` InfraFAFUpdate [FIREANDFORGET]
- ord 3 `java-runtime-b995c40f3c052bcc277e` InfraFAFSelect [FIREANDFORGET]

Static IDs: ord0 java-1899404b366f6f3c1a31, ord1 java-11e9c7fcb6e1617929a9,
ord2 java-db32f6072d9b36a84471, ord3 java-c87f698c077e7dd7ada7.

File: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableFAFExecuteQuery.java (127 lines)

## Java contract (frozen by NextJavaContract489)

### ord 0 InfraFAFInsert (lines 44-61)
Deploys `@name('create') @public create table MyTableINS as (p0 string, p1 int)`
— UNKEYED table, lowercase @name.
FAF `insert into MyTableINS (p0, p1) select 'a', 1` via compileExecuteFAF.
Assertions: result.getArray().length == 0 (empty array); result event type
identity with create statement's row type; assertPropsPerRowIterator
('create', [p0,p1], {{a,1}}) — ORDERED iterator assert. undeployAll.

### ord 1 InfraFAFDelete (lines 63-81)
Deploys `@name('create') @public create table MyTableDEL as (p0 string
primary key, thesum sum(int))` + `into table MyTableDEL select theString,
sum(intPrimitive) as thesum from SupportBean group by theString` (no @name;
group key maps positionally to PK p0).
Sends SupportBean("G0",0)..("G9",9) — 10 events.
assertEquals(10L, iteratorCount) → FAF `delete from MyTableDEL` (delete-all)
→ assertEquals(0L). undeployAll.

### ord 2 InfraFAFUpdate (lines 83-99)
Deploys `@Name('TheTable') @public create table MyTableUPD as (p0 string
primary key, p1 string, thesum sum(int))` — capital-N @Name; statement name
'TheTable' != table name. Plus the same into-table grouped feed.
Sends SupportBean("E1",1), ("E2",2).
FAF `update MyTableUPD set p1 = 'ABC'` (update-all, no where).
assertPropsPerRowAnyOrder(stmt('TheTable').iterator(), [p0,p1],
{{E1,ABC},{E2,ABC}}). undeployAll.

### ord 3 InfraFAFSelect (lines 101-117)
Deploys `@Name('TheTable') @public create table MyTableSEL as (p0 string
primary key, thesum sum(int))` + same into-table feed.
Sends SupportBean("E1",1), ("E2",2).
FAF `select * from MyTableSEL` → assertPropsPerRowAnyOrder(result.getArray(),
[p0], {{E1},{E2}}). undeployAll.

## Scenario-op mapping (frozen)
- compileDeploy -> `deploy` + `deployed` marker (488 convention).
- FAF writes -> `deploy` op with `Faf*` label + `epl`, NO trace record.
- FAF select -> `snapshot` op carrying `epl` + `fields` (488 convention).
- iterator asserts -> `snapshot` op on the create statement label.
- iteratorCount -> `snapshot` op with a count pin (or record count field).
- undeployAll -> `undeploy-all`.

## Go surface (frozen by NextGoSurface489) — fully asset-only
- CreateTable (incl. unkeyed), grouped IntoTable feeds (GroupBy+Select+IntoTable).
- FAF mutations: FromTable(env,"T").OnDemand().Insert/InsertRows/UpdateAll/
  DeleteAll + ExecuteFireAndForget. NOTE: assignment-form Insert on an
  UNKEYED table is untested — ord 0 may need InsertRows positional form.
- FAF select: FromTable(env,"T").Query() or .Select(...) + ExecuteFireAndForget.
- Snapshot op for iterator asserts (infra_table_update_and_index.go precedent).
- Only event type: SupportBean(theString, intPrimitive).
- Runner to clone: infra_table_update_and_index.go minus error ops.

## Files (asset worker)
- tools/java-oracle/InfraTableFAFExecuteQueryScenarioOracle.java
- tools/java-oracle/run-infra-table-faf-execute-query.sh
- testdata/parity/infra-table-faf-execute-query.json
- internal/app/parity/infra_table_faf_execute_query.go
- internal/app/parity/run.go (loader + runner/diff dispatch only)

## Forbidden
No internal/esper, internal/compat, manifest, docs, or existing
testdata/parity changes. No tests/formatters/builds.

## Validation (primary agent)
Java trace via run script; `-mode infra-table-faf-execute-query-diff`
zero-diff; run_test.go pins; make check; parity review; commit/push.
