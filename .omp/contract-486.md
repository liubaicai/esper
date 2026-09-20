# Contract - Draft 4.486 `infra-nwtable-on-merge-insertonly-deletethenupdate`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 46-53: the six remaining InfraInsertOnly
executions (ord 46 = namedWindow=true soda+useColumnNames; ords 47-51 = all
five namedWindow=false table variants) + InfraDeleteThenUpdate{nw,table}
(ords 52-53). A born-DV unit.

Runtime IDs (java-execution-inventory.jsonl lines 2819-2833, all flags []):
- ord 46 `java-runtime-5cdc46289e4fac78a0c5` InfraInsertOnly{namedWindow=true,useEquivalent=false,soda=true,useColumnNames=true}
- ord 47 `java-runtime-8e9616eb8385c473d45a` InfraInsertOnly{namedWindow=false,useEquivalent=true,soda=false,useColumnNames=false}
- ord 48 `java-runtime-af614186a63cbeb33ae5` InfraInsertOnly{namedWindow=false,useEquivalent=false,soda=false,useColumnNames=false}
- ord 49 `java-runtime-eb7754c9e46c8c465c14` InfraInsertOnly{namedWindow=false,useEquivalent=false,soda=false,useColumnNames=true}
- ord 50 `java-runtime-f21a6fc889f14608828f` InfraInsertOnly{namedWindow=false,useEquivalent=false,soda=true,useColumnNames=false}
- ord 51 `java-runtime-f7a73c74e857ffbdfd15` InfraInsertOnly{namedWindow=false,useEquivalent=false,soda=true,useColumnNames=true}
- ord 52 `java-runtime-5816ec0ef519ec8a48e1` InfraDeleteThenUpdate{namedWindow=true}
- ord 53 `java-runtime-3cca4ced23a6097b5023` InfraDeleteThenUpdate{namedWindow=false}

Static IDs: java-0b6e7bd4eb235001d140 (InfraInsertOnly) + the InfraDeleteThenUpdate
class static ID (from static-manifest.json).

## Java contract (frozen by NextJavaContract486 + NextGoSurface486)

### InfraInsertOnly (ords 46-51, InfraNWTableOnMerge.java:464-528)
Same step sequence as ords 42-45 (already verified in 4.485):
1. compileDeploy createEPL: nw → `@Name('Window') @public create window
   InsertOnlyInfra#unique(p0) as (p0 string, p1 int)`; table → `@Name('Window')
   @public create table InsertOnlyInfra (p0 string primary key, p1 int)`.
2. compileDeploy(soda, mergeEPL, path) — merge EPL by flags:
   - useEquivalent (ord 47): `on SupportBean merge InsertOnlyInfra where 1=2
     when not matched then insert select theString as p0, intPrimitive as p1`
   - useColumnNames (ords 46, 49, 51): `on SupportBean as provider merge
     InsertOnlyInfra insert(p0, p1) select provider.theString, intPrimitive`
   - plain (ords 48, 50): `on SupportBean merge InsertOnlyInfra insert select
     theString as p0, intPrimitive as p1`
   soda=true (ords 46, 50, 51) → eplToModel round-trip then compileDeploy(model);
   runtime semantics identical.
3. assertSame(windowType, onType) — Java-internal identity, not observable.
4. addListener("on"); send E1 → snapshot {E1,1} + listener {p0=E1,p1=1};
   milestone; send E2 → snapshot {E1,1},{E2,2} + listener {p0=E2,p1=2};
   undeployAll.

### InfraDeleteThenUpdate (ords 52-53, InfraNWTableOnMerge.java:304-345)
1. compileDeploy create: nw → `@name('create') @public create window
   MyInfra#keepall() as (p0 string, p1 int)`; table → `@name('create') @public
   create table MyInfra(p0 string primary key, p1 int)`.
2. compileDeploy merge + addListener("merge"): `on SupportBean sb merge MyInfra
   where theString = p0 when matched then delete then update set p1 =
   intPrimitive` — multi-action delete THEN update on same match.
3. compileExecuteFAFNoResult(`insert into MyInfra select 'A' as p0, 1 as p1`,
   path) — FAF seed.
4. assertPropsPerRowIterator("create", ORDERED, [[A,1]]).
5. send SupportBean("A", 10).
6. DIVERGENT: namedWindow → ordered iterator [[A,10]] (update wins); table →
   assertIterator hasNext()==false (delete wins, empty table). Java comment:
   "no guarantee whether the delete or the update wins" — but the oracle
   deterministically asserts update-wins for NW and delete-wins for table.
7. undeployAll. 'merge' listener attached but NEVER asserted.

## Go surface (frozen by NextGoSurface486)
- Insert-only merge over tables: MergeIntoTableWhen(keys=[Field(theString)],
  WhenNotMatchedAny(...)) — proven by the simple-table case in
  infra_nwtable_on_merge.go. For the useEquivalent `where 1=2` over a table:
  constant never-matching key expression (contract-freeze decision).
- Delete-then-update: WhenMatchedActions(ThenDelete(cond), ThenUpdate(cond,...))
  — proven by infra_nwtable_on_merge_multiaction.go. The nw-update-wins /
  table-delete-wins divergence is implemented and unit-tested in
  internal/esper/trigger_infra_property_eval_test.go.
- FAF seed: FromTable/FromNamedWindow(name).OnDemand().InsertRows(
  InsertValues(...)) + engine.ExecuteFireAndForget — precedent in
  infra_nwtable_on_update.go:486-494.
- Runner to clone: infra_nwtable_on_merge_invalid_insertonly.go (insert-only
  machinery) + infra_nwtable_on_merge.go (table fixture + faf op).
- Zero shared-core work expected.

## Files (asset worker)
- tools/java-oracle/InfraNWTableOnMergeInsertOnlyDeleteThenUpdateScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-insertonly-deletethenupdate.sh (new)
- testdata/parity/infra-nwtable-on-merge-insertonly-deletethenupdate.json (new)
- internal/app/parity/infra_nwtable_on_merge_insertonly_deletethenupdate.go (new)
- internal/app/parity/run.go (add mode + -diff only)

## Forbidden
Do NOT modify internal/esper, internal/compat, manifest, docs, or the
checked-in infra-nwtable-on-merge* files. Do NOT run tests or formatters.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-insertonly-deletethenupdate-diff`
zero-diff; run_test.go pins; make check; parity review; commit/push.
