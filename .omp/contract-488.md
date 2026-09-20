# Contract - Draft 4.488 `infra-table-update-and-index`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableUpdateAndIndex.java — ALL 5 executions (ords 0-4), one file, one
surface: table update + unique-index semantics. Born-DV unit.

Runtime IDs (java-execution-inventory.jsonl lines 3061-3065):
- ord 0 `java-runtime-878326b2aef272d9ef78` InfraEarlyUniqueIndexViolation [INVALIDITY]
- ord 1 `java-runtime-59f3f0884fc9ae9d749f` InfraLateUniqueIndexViolation [INVALIDITY]
- ord 2 `java-runtime-1118b36d6f38fa1c78a8` InfraFAFUpdate [FIREANDFORGET]
- ord 3 `java-runtime-16e0f7011601678fa5df` InfraTableKeyUpdateSingleKey []
- ord 4 `java-runtime-0b3580bcd42327b7d2bb` InfraTableKeyUpdateMultiKey []

Static IDs: ord0 java-c8309a6f377c34ffcdb4, ord1 java-8db9b9cd27d04bb86d69,
ord2 java-4a2462a159e9d648257a, ord3 java-ad9c04a3fc0d98d4e4dd,
ord4 java-97fc9971820f1e5b947d.

File: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableUpdateAndIndex.java

## Java contract (frozen by NextJavaContract488)

### ord 0 InfraEarlyUniqueIndexViolation (lines 44-93)
Deploys (shared RegressionPath):
1. `@name('create') @public create table MyTableEUIV as (pkey0 string primary key, pkey1 int primary key, thecnt count(*))`
2. `into table MyTableEUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive`
Sends SupportBean("E1",10), SupportBean("E1",20) → two rows, same pkey0.
Four failure phases, all asserted by exact message:
- DEPLOY fail: `create unique index SecIndex on MyTableEUIV(pkey0)` compiles;
  deploy throws EPDeployException "Failed to deploy: Unique index violation,
  index 'SecIndex' is a unique index and key 'E1' already exists".
- FAF-runtime fail: `update MyTableEUIV set pkey1 = 0` (no where) throws
  EPException "Unique index violation, index 'MyTableEUIV' is a unique index
  and key 'MultiKey[E1,0]' already exists"; table unchanged — iterator
  any-order on 'create' [pkey0,pkey1] = {{E1,10},{E1,20}}.
- ON-UPDATE runtime fail: deploy `@name('on-update') on SupportBean_S1 update
  MyTableEUIV set pkey1 = 0`; send SupportBean_S1(0) → EPException whose
  getCause() is "Unexpected exception in statement 'on-update': Unique index
  violation, index 'MyTableEUIV' is a unique index and key 'MultiKey[E1,0]'
  already exists"; table unchanged (same iterator assert).
- COMPILE fail: `@name('on-merge') on SupportBean_S1 merge MyTableEUIV when
  matched then update set pkey1 = 0` → EPCompileException, getCause()
  "Validation failed in when-matched (clause 1): On-merge statements may not
  update unique keys of tables".
undeployAll. No listeners; no milestones.

### ord 1 InfraLateUniqueIndexViolation (lines 95-141)
Deploys:
1. `@name('create') @public create table MyTableLUIV as (pkey0 string primary key, pkey1 int primary key, col0 int, thecnt count(*))`
2. `into table MyTableLUIV select count(*) as thecnt from SupportBean group by theString, intPrimitive`
Sends SupportBean("E1",10), SupportBean("E2",20) → distinct pkey0.
- Deploy `@name('on-merge') on SupportBean_S1 merge MyTableLUIV when matched
  then update set col0 = 0` (succeeds — col0 not yet unique).
- DEPLOY fail: `create unique index MyUniqueSecondary on MyTableLUIV (col0)`
  compiles; deploy throws EPDeployException "Failed to deploy: Create-index
  adds a unique key on columns that are updated by one or more on-merge
  statements".
- undeployModuleContaining("on-merge") — mid-test undeploy boundary.
- Deploy `@name('on-update') on SupportBean_S1 update MyTableLUIV set pkey1 =
  0`, then `create unique index MyUniqueSecondary on MyTableLUIV (pkey1)` —
  BOTH succeed (on-update of a unique-indexed column is allowed; only
  on-merge is blocked).
- Send SupportBean_S1(0) → EPException, getCause() "Unexpected exception in
  statement 'on-update': Unique index violation, index 'MyUniqueSecondary' is
  a unique index and key '0' already exists"; table unchanged: iterator
  any-order on 'create' [pkey0,pkey1] = {{E1,10},{E2,20}}.
undeployModuleContaining("on-update"); undeployAll.

### ord 2 InfraFAFUpdate (lines 143-168)
Deploys:
1. `@public create table MyTableFAFU as (pkey0 string primary key, col0 int, col1 int, thecnt count(*))`
2. `create index MyIndex on MyTableFAFU(col0)` — non-unique secondary hash index
3. `into table MyTableFAFU select count(*) as thecnt from SupportBean group by theString`
Sends SupportBean("E1",0), SupportBean("E2",0).
FAF sequence (compileExecuteFAF):
- `update MyTableFAFU set col0 = 1 where pkey0='E1'`
- `update MyTableFAFU set col0 = 2 where pkey0='E2'`
- `select pkey0 from MyTableFAFU where col0=1` → exactly 1 row {pkey0:E1}
  (exercises MyIndex lookup post-update)
- `update MyTableFAFU set col1 = 100 where pkey0='E1'`
- `select pkey0 from MyTableFAFU where col1=100` → 1 row {pkey0:E1}
Assertion shape: result.getArray().length==1 && assertProps(row0).
undeployAll. No listeners/milestones.

### ord 3 InfraTableKeyUpdateSingleKey (lines 213-251)
Deploys:
1. `@name('s0') @public create table MyTableSingleKey(pkey0 string primary key, c0 int)`
2. `insert into MyTableSingleKey select theString as pkey0, intPrimitive as c0 from SupportBean`
3. `on SupportBean_S0 update MyTableSingleKey set pkey0 = p01 where pkey0 = p00`
Sends SupportBean(E1,10),(E2,20),(E3,30); milestone(0);
SupportBean_S0(0,"E2","E20") → iterator-any-order 's0' [pkey0,c0] =
{{E1,10},{E20,20},{E3,30}}; milestone(1); SupportBean_S0(0,"E1","E10");
milestone(2); assert {{E10,10},{E20,20},{E3,30}}; SupportBean_S0(0,"E3","E30");
milestone(3); assert {{E10,10},{E20,20},{E30,30}}. undeployAll.
Primary-key RENAME via on-update: row located by old key, re-keyed, c0 preserved.

### ord 4 InfraTableKeyUpdateMultiKey (lines 170-211)
Identical shape with composite key:
1. `@name('s1') @public create table MyTableMultiKey(pkey0 string primary key, pkey1 int primary key, c0 long)`
2. `insert into MyTableMultiKey select theString as pkey0, intPrimitive as pkey1, longPrimitive as c0 from SupportBean`
3. `on SupportBean_S0 update MyTableMultiKey set pkey0 = p01 where pkey0 = p00`
Sends SupportBean(E1,10,long=100),(E2,20,200),(E3,30,300); milestone(0);
S0(0,"E2","E20") → [pkey0,pkey1,c0] = {{E1,10,100},{E20,20,200},{E3,30,300}};
milestone(1); S0(0,"E1","E10"); milestone(2) → {{E10,10,100},{E20,20,200},{E3,30,300}};
S0(0,"E3","E30"); milestone(3) → {{E10,10,100},{E20,20,200},{E30,30,300}}.
undeployAll. Only pkey0 of the composite key is updated; pkey1 stays.

## Error-record convention (frozen)
Following the undeploy-error/build-error precedent (context_lifecycle.go:852,
infra_table_reset.go:267): error steps emit the pinned `expectError` text as
the record Value while the runner MUST attempt the operation and require an
error of the right class. Go error wording does NOT need to match Java
verbatim — the pinned text is the contract. This removes the wording gap
from shared-core scope; the semantic checks below remain required because
the runner asserts the error actually fires.

## Shared-core work (primary agent, internal/esper only)
- A. `Table.CreateIndex` (state.go:604): when unique=true, validate existing
  rows — reject with a unique-violation error instead of silently keeping
  the first row per key (rebuildTableIndexesLocked:571). Applies to root and
  scoped states; on failure the index must not be registered.
- B. Send-driven trigger update atomicity: executeTableWhereAction
  (trigger.go:3490+) applies row updates one-by-one; a mid-batch unique
  violation must leave the table unchanged (snapshot/restore or pre-validate
  all matched rows). FAF path already rolls back (faf.go:634-679).
- C. Merge compile validation: env.Build of a merge whose when-matched
  update assigns a unique-key column (PK or unique-indexed) must fail with
  an invalid-rule error ("On-merge statements may not update unique keys of
  tables" semantics).
- D. Engine registry of merge-updated columns per table (set at merge
  deploy, removed at undeploy) + CreateIndex rejection when a unique index
  targets a merge-updated column ("Create-index adds a unique key on columns
  that are updated by one or more on-merge statements" semantics).

## Go surface (frozen by NextGoSurface488) — ords 2-4 asset-only
- CreateTable + PrimaryKeyColumn[T] (+ UniqueIndex/SecondaryIndex decl-time),
  Table.CreateIndex for late index.
- Into-table grouped feed: From.GroupBy(...).Select(Alias("thecnt",CountAll()),...).IntoTable.
- OnEvent(From[T]).InsertIntoTable / UpdateTableWhere for on-event DML;
  updateInScope already supports PK rekey.
- FAF: FromTable.OnDemand().UpdateWhere/UpdateAll + ExecuteFireAndForget;
  FAF select via FromTable.Filter(...).Select(...) + ExecuteFireAndForget.
- undeployModuleContaining → per-label Deployment.Undeploy bookkeeping
  (infra_table_join.go precedent).
- milestone → no-op step (pinned, skipped by runner).
- Iterator asserts → snapshot op (infra_table_reset.go:259 precedent).
- Error ops: build-error (compile-rejected), deploy-error (new op or reuse),
  send-error (event_map_core.go:234 precedent — but emit pinned expectError,
  not live text), faf-error (new op, same pinned convention).

## Files
- Asset worker: tools/java-oracle/InfraTableUpdateAndIndexScenarioOracle.java,
  tools/java-oracle/run-infra-table-update-and-index.sh,
  testdata/parity/infra-table-update-and-index.json,
  internal/app/parity/infra_table_update_and_index.go,
  internal/app/parity/run.go (mode + -diff entries only).
- Primary agent (shared core): internal/esper/state.go,
  internal/esper/trigger.go, internal/esper/runtime.go (registry),
  internal/esper/faf.go only if wording/atomicity gaps surface.

## Forbidden
Asset worker: do NOT modify internal/esper, internal/compat, manifest, docs,
or existing testdata/parity files. Do NOT run tests or formatters.
Primary agent owns shared-core writes exclusively (one writer rule).

## Validation (primary agent)
Java trace via run script; `-mode infra-table-update-and-index-diff`
zero-diff; run_test.go pins; make check; parity review; commit/push.
