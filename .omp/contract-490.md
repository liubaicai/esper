# Contract - Draft 4.490 `infra-table-subquery`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableSubquery.java — ALL 4 executions (ords 0-3), one file, one surface:
correlated scalar subqueries and subquery-in-filter against tables.
Born-DV unit, fully asset-only.

Runtime IDs (java-execution-inventory.jsonl lines 3057-3060):
- ord 0 `java-runtime-7b449dd45dd6961c5d61` InfraTableSubqueryAgainstKeyed []
- ord 1 `java-runtime-9cde668ef5b4781b068a` InfraTableSubqueryAgainstUnkeyed []
- ord 2 `java-runtime-8ece65643b6ec15616f2` InfraTableSubquerySecondaryIndex []
- ord 3 `java-runtime-a839574d871f88c2fdbf` InfraTableSubqueryInFilter []

Static IDs: ord0 java-84c3e4b24f1621c4e20f, ord1 java-8a837e4f238a838a719c,
ord2 java-22c9a3e16812af54b583, ord3 java-05c24f1601cc75b1683e.

NOTE: InFilter's class is defined FIRST in the file but is ordinal 3 in
executions() — use ordinal order.

File: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSubquery.java (170 lines)

## Java contract (frozen by NextJavaContract490)

### ord 0 InfraTableSubqueryAgainstKeyed (lines 71-98)
Deploys:
1. `@public create table varagg as (key string primary key, total sum(int))`
2. `into table varagg select sum(intPrimitive) as total from SupportBean group by theString`
3. `@name('s0') select (select total from varagg where key = s0.p00) as value from SupportBean_S0 as s0` + listener
Sends SupportBean("G2",200); S0(0,"G1") -> value=null; S0(0,"G2") -> value=200;
milestone(0); SupportBean("G1",100); S0(0,"G1") -> 100; S0(0,"G2") -> 200.
undeployAll. Correlated PK lookup; null when no row.

### ord 1 InfraTableSubqueryAgainstUnkeyed (lines 100-117)
Deploys:
1. `@public create table InfraOne (string string, intPrimitive int)` (UNKEYED)
2. `@name('s0') select (select intPrimitive from InfraOne where string = s0.p00) as c0 from SupportBean_S0 as s0` + listener
3. `insert into InfraOne select theString as string, intPrimitive from SupportBean`
NOTE: subquery select deploys BEFORE the insert feed. Full-scan on unkeyed.
Sends SupportBean("E1",10); milestone(0); S0(0,"E1") -> c0=10. undeployAll.

### ord 2 InfraTableSubquerySecondaryIndex (lines 119-150)
Deploys:
1. `@public create table MyTable(k0 string primary key, k1 string primary key, p2 string, value int)` (composite PK)
2. `create index MyIndex on MyTable(p2)` (secondary index, before rows)
3. `on SupportBean_S0 merge MyTable where p00 = k0 and p01 = k1 when not matched then insert select p00 as k0, p01 as k1, p02 as p2, id as value when matched then update set p2 = p02, value = id ` (trailing space after `id `)
4. `@Name('s0') select (select value from MyTable as tbl where sb.theString = tbl.p2) as c0 from SupportBean as sb` + listener
Sends S0(id=10,p00=G1,p01=SG1,p02=P2_1) -> insert; SupportBean("P2_1",-1) -> c0=10;
milestone(0); S0(id=11,p00=G1,p01=SG1,p02=P2_2) -> update p2=P2_2,value=11;
milestone(1); SupportBean("P2_1",-1) -> c0=null; SupportBean("P2_2",-1) -> c0=11.
undeployAll. Secondary index maintained across merge update of indexed column.

### ord 3 InfraTableSubqueryInFilter (lines 37-69, ordinal 3)
Deploys ONE 3-statement module (byte-exact, embedded newlines):
`create table MyTable(tablecol string primary key);\ninsert into MyTable
select p00 as tablecol from SupportBean_S0;\n@name('s0') select * from
SupportBean(theString=(select tablecol from MyTable).orderBy().firstOf())`
+ listener on s0. UNCORRELATED subquery inside the stream filter predicate;
enum orderBy().firstOf() over the multirow subquery result; select * over
SupportBean (full listener rows).
Sequence (sendAssert = send SupportBean(theString,0) then
assertListenerInvokedFlag):
- bean E -> false; S0 E (inserts tablecol=E); bean E -> true
- S0 C; bean E -> false; bean C -> true
- milestone(0)
- bean A -> false; bean C -> true; S0 A; bean A -> true; bean C -> false
undeployAll. 4 listener events total (E,C,C,A).

## Go surface (frozen by NextGoSurface490) — fully asset-only
- SubqueryValue / SubqueryValueWithOptions + OuterField for correlated scalar
  subqueries; SubqueryValues + EnumOrderByNatural + EnumFirstOf for the
  orderBy().firstOf() chain; subqueries inside stream filters supported.
- CreateTable keyed/unkeyed; Table.CreateIndex for the late index;
  OnEvent.InsertIntoTable; MergeIntoTableWhen with positional keys.
- Runner to clone: infra_nwtable_subq_filtered_correl.go (correlated scalar
  subquery precedent) or infra_table_faf_execute_query.go skeleton.
- Ops needed: case/deploy/deployed/send/listener records/undeploy-all.
  Non-invocation = absent record (no explicit "no event" record).

## Files (asset worker)
- tools/java-oracle/InfraTableSubqueryScenarioOracle.java
- tools/java-oracle/run-infra-table-subquery.sh
- testdata/parity/infra-table-subquery.json
- internal/app/parity/infra_table_subquery.go
- internal/app/parity/run.go (loader + runner/diff dispatch only)

## Forbidden
No internal/esper, internal/compat, manifest, docs, or existing
testdata/parity changes. No tests/formatters/builds.

## Validation (primary agent)
Java trace via run script; `-mode infra-table-subquery-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
