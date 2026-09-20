# Contract - Draft 4.487 `infra-table-select-enum-multikey`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableSelect.java ordinals 1-4 — four executions sharing the
"select from table" surface (enum method + array/composite multikey joins).
Ord 0 `InfraTableSelectStarPublicTypeVisibility` is deferred to its own unit:
it is a 14-sub-assertion select-shape matrix (subqueries, join fragments,
subscribers, single-row funcs, FAF DML, output snapshot) — a different
semantics cluster. A born-DV unit.

Runtime IDs (java-execution-inventory.jsonl lines 3053-3056, all flags []):
- ord 1 `java-runtime-27e7ce929b90e4009b48` InfraTableSelectEnum
- ord 2 `java-runtime-d52d06b4618e30451543` InfraTableSelectMultikeyWArraySingleArray
- ord 3 `java-runtime-83451626aeee59b34ac2` InfraTableSelectMultikeyWArrayTwoArray
- ord 4 `java-runtime-b8b3e8c1f04c0c918ea2` InfraTableSelectMultikeyWArrayComposite

Static IDs: ord1 java-ece967984c50b6e02d47, ord2 java-2ae2dfa34214fa74ffc8,
ord3 java-79057cf89b8898584f05, ord4 java-2a4767401dba7efd8f97.

NOTE: file lives under suite/infra/tbl/ (NOT nwtable):
regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSelect.java

## Java contract (frozen by NextJavaContract487)

### ord 1 InfraTableSelectEnum (lines 143-161)
Single deployment:
`@public create table MyTable(p string);\n@name('s0') select t.firstOf() as c0 from MyTable as t;\n`
— NO listener attached. FAF seed `insert into MyTable select 'a' as p`.
Assertion: assertIterator('s0') → next().get('c0') = Object[]{'a'} —
firstOf() yields the first table row's UNDERLYING Object[] (not EventBean).
undeployAll.

### ord 2 InfraTableSelectMultikeyWArraySingleArray (lines 111-141)
Deploys:
- `@public create table MyTable(k int[primitive] primary key, value int);`
- `insert into MyTable select array as k, value from SupportEventWithIntArray;`
- `@name('s0') select t.value as c0 from SupportEventWithManyArray, MyTable as t where k = intOne;`
Listener on 's0'. Sends: SupportEventWithIntArray('E1',[1,2],10),
('E2',[1,3],20), ('E3',[2],30); milestone(0) (serde checkpoint, no-op);
then SupportEventWithManyArray queries: intOne=[2] → c0=30; [1,3] → 20;
[1,2] → 10. undeployAll. Array PK equality is by CONTENT (Arrays.equals);
bean property literally named 'array'.

### ord 3 InfraTableSelectMultikeyWArrayTwoArray (lines 79-109)
Deploys:
- `@public create table MyTable(k1 int[primitive] primary key, k2 int[primitive] primary key, value int);`
- `insert into MyTable select intOne as k1, intTwo as k2, value from SupportEventWithManyArray(id = 'I');`
- `@name('s0') select t.value as c0 from SupportEventWithManyArray(id='Q'), MyTable as t where k1 = intOne and k2 = intTwo;`
Listener on 's0'. Inserts (id='I'): ([1,2],[3,4],10), ([1,3],[1],20),
([2],[],30); milestone(0); queries (id='Q'): ([2],[]) → 30;
([1,2],[3,4]) → 10; ([1,3],[1]) → 20. undeployAll. EMPTY array [] is a
valid key component matching []. Same event type serves insert (id='I')
and query (id='Q') streams via filters.

### ord 4 InfraTableSelectMultikeyWArrayComposite (lines 43-77)
Deploys:
- `@public create table MyTable(k0 string primary key, k1 string primary key, k2 string primary key, v string);`
- `create index MyIndex on MyTable(k0, k1, v btree);` (planning-only, non-observable)
- `insert into MyTable select p00 as k0, p01 as k1, p02 as k2, p03 as v from SupportBean_S0;`
- `@name('s0') select t.v as v from SupportBean_S1, MyTable as t where k0 = p10 and k1 = p11 and v > p12;`
Listener on 's0'. Sends S0: ('A','BB','CCC','X1'), ('A','BB','DDDD','X4'),
('A','CC','CCC','X3'), ('C','CC','CCC','X4'); milestone(0); S1 asserts:
('A','CC','') → 'X3'; ('C','CC','') → 'X4'; ('A','BB','X3') → 'X4'
('X1' not > 'X3'); ('A','BB','Z') → assertListenerNotInvoked. undeployAll.
String '>' is lexicographic.

## Go surface (frozen by NextGoSurface487) — ZERO shared-core changes
- Tables: esper.CreateTable(env, name, []TableColumn{PrimaryKeyColumn[T]...},
  SecondaryBTreeIndex("MyIndex","k0","k1","v") for ord4).
- Array PKs: PrimaryKeyColumn[[]int] — proven infra_table_into_table.go:819-838,
  infra_table_access_parity_test.go:2350+; content equality via value.go:306-340.
- Continuous insert: OnEvent(From[T]).InsertIntoTable(name, SetColumn...).
- Join select: JoinMany(JoinSource(From[T]), JoinRecordSource(FromTable)) with
  OnSourcesEqual / OnSourcesCompare(JoinGreater) — infra_table_join.go:343-356.
- ord1 firstOf(): Func1[Event,any]("firstOf", e.Underlying(), FirstEventValue())
  inside a FromTable select; iterator via statement.Snapshot.
- milestone(0) → documented no-op step (pinned in scenario, skipped by runner).
- assertListenerNotInvoked → absence of listener record (pinned step shape).
- Runner to clone: infra_table_join.go skeleton + infra_table_into_table.go
  bean/array-key machinery + infra_nwtable_on_merge.go snapshot protocol.

## Files (asset worker)
- tools/java-oracle/InfraTableSelectEnumMultikeyScenarioOracle.java (new)
- tools/java-oracle/run-infra-table-select-enum-multikey.sh (new)
- testdata/parity/infra-table-select-enum-multikey.json (new)
- internal/app/parity/infra_table_select_enum_multikey.go (new)
- internal/app/parity/run.go (add mode + -diff only)

## Forbidden
Do NOT modify internal/esper, internal/compat, manifest, docs, or checked-in
infra-table* files. Do NOT run tests or formatters.

## Validation (primary agent)
Java trace via run script; `-mode infra-table-select-enum-multikey-diff`
zero-diff; run_test.go pins; make check; parity review; commit/push.
