# Contract — Draft 4.482 `infra-nwtable-on-merge-multiaction`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 20-25: three execution classes x {namedWindow=true,false}.
Asset-only (Go scout confirmed full API coverage; unit tests already mirror the semantics).
New sibling runner `infra-nwtable-on-merge-multiaction` — do NOT extend checked-in runners.

- ord 20 `InfraMultiactionDeleteUpdate{namedWindow=true}` — `java-runtime-dfa83c0593d19172be7d`
- ord 21 `InfraMultiactionDeleteUpdate{namedWindow=false}` — `java-runtime-ffbda0563d50878dafd8`
- ord 22 `InfraUpdateOrderOfFields{namedWindow=true}` — `java-runtime-3034e5da517c1235da22`
- ord 23 `InfraUpdateOrderOfFields{namedWindow=false}` — `java-runtime-cb5eefe84a486b090e53`
- ord 24 `InfraSubqueryNotMatched{namedWindow=true}` — `java-runtime-905164d98662721e5509`
- ord 25 `InfraSubqueryNotMatched{namedWindow=false}` — `java-runtime-d3e1f3aa50c4375f657f`
All flags []; static OBSERVEROPS heuristic only. No virtual time, no FAF, no compile errors.
env.milestone(0/1/2) are ordering markers only — implicit in step order, no scenario op.

### Ords 20-21 (InfraMultiactionDeleteUpdate)
Deploys (RegressionPath):
- nw: `@name('Create') @public create window WinMDU#keepall as SupportBean`
- table: `@name('Create') @public create table WinMDU (theString string primary key, intPrimitive int)`
- `insert into WinMDU select theString, intPrimitive from SupportBean`
- merge EPL verbatim: `@name('merge') on SupportBean_ST0 as st0 merge WinMDU as win where st0.key0=win.theString when matched then delete where intPrimitive<0 then update set intPrimitive=st0.p00 where intPrimitive=3000 or p00=3000 then update set intPrimitive=999 where intPrimitive=1000 then delete where intPrimitive=1000 then update set intPrimitive=1999 where intPrimitive=2000 then delete where intPrimitive=2000` (six ordered matched actions)

Steps (fields theString,intPrimitive; iterator on 'Create'):
1. SupportBean(E1,1); SupportBean_ST0("ST0","E1",0) -> rows {{E1,1}} ordered.
2. milestone. SupportBean(E2,-1); ST0("ST0","E2",0) -> {{E1,1}} (E2 deleted by action1 intPrimitive<0).
3. SupportBean(E3,3000); ST0("ST0","E3",3) -> anyOrder {{E1,1},{E3,3}} (action2: intPrimitive=3000 -> set to st0.p00=3).
4. milestone. SupportBean(E4,4); ST0("ST0","E4",3000) -> anyOrder {{E1,1},{E3,3},{E4,3000}} (action2 alt p00=3000 -> set 3000).
5. SupportBean(E5,1000); ST0("ST0","E5",0) -> anyOrder +{E5,999} (action3 sets 999 BEFORE action4 delete where=1000 -> survives).
6. milestone. SupportBean(E6,2000); ST0("ST0","E6",0) -> anyOrder +{E6,1999} (action5 sets 1999 before action6 delete where=2000 -> survives).
7. undeployModuleContaining("merge") -> undeploy op on 'merge'; redeploy identical merge statement (EPL->model->compile roundtrip; no sends/asserts after — observable only as successful redeploy); undeployAll.

Key semantics: action where-clauses evaluate against the row state produced by earlier
actions in the same match (delete-then-update ordering is the point). Unqualified
intPrimitive in action wheres = merge-target row; unqualified p00 = trigger stream.

### Ords 22-23 (InfraUpdateOrderOfFields)
ONE compileDeploy of a 3-statement module (no RegressionPath), verbatim:
- nw: `@public create window MyInfraUOF#keepall as SupportBean;`
- table: `@public create table MyInfraUOF(theString string primary key, intPrimitive int, intBoxed int, doublePrimitive double);`
- `insert into MyInfraUOF select theString, intPrimitive, intBoxed, doublePrimitive from SupportBean;`
- `@name('Merge') on SupportBean_S0 as sb merge MyInfraUOF as mywin where mywin.theString = sb.p00 when matched then update set intPrimitive=id, intBoxed=mywin.intPrimitive, doublePrimitive=initial.intPrimitive;`
Listener on 'Merge'; assertPropsPerRowLastNew('Merge', [intPrimitive,intBoxed,doublePrimitive]).

Steps:
1. makeSupportBean(E1,1,2) = {theString:E1,intPrimitive:1,doublePrimitive:2.0,intBoxed:null}; SupportBean_S0(5,"E1") -> Merge lastNew {{5,5,1.0}}.
2. milestone. makeSupportBean(E2,10,20); S0(6,"E2") -> {{6,6,10.0}}.
3. S0(7,"E1") -> {{7,7,5.0}} (E1 row now intPrimitive=5 -> doublePrimitive=initial=5.0).
4. undeployAll.

Key semantics: UPDATE SET assignments evaluate left-to-right — intBoxed=mywin.intPrimitive
reads the ALREADY-UPDATED intPrimitive (=id), while initial.intPrimitive reads the
pre-update value. Unqualified id = trigger SupportBean_S0.id.

### Ords 24-25 (InfraSubqueryNotMatched)
Deploys (RegressionPath):
- nw: `@name('Create') @public create window InfraOne#unique(string) (string string, intPrimitive int)`
- table: `@name('Create') @public create table InfraOne (string string primary key, intPrimitive int)`
- merge EPL verbatim (from Java source lines 1156-1200): correlated subquery in the
  not-matched insert assignment (SubqueryValue over a second infra, OuterField correlation
  to the trigger event). Full EPL text to be transcribed verbatim by the asset worker from
  the Java source — do not paraphrase.

## Harness
One scenario `infra-nwtable-on-merge-multiaction`, six cases
(multiaction-{nw,table}, orderoffields-{nw,table}, subquery-{nw,table}), fresh engine per
case with WithRuntimeURI(runtimeID). Ops: deploy/deployed/send/snapshot(mode ordered for
1 row, any for >1)/undeploy/undeploy-all. Listeners on 'Merge' for orderoffields cases;
snapshots on 'Create' for multiaction cases.

## Go surface (asset-only)
Multi-action matched chains (evaluateTableMergeActions ordered-chain + delete-termination);
ordered assignments with initial.* (InitialTableField/InitialNamedWindowField);
correlated SubqueryValue in not-matched inserts (OuterField correlation to trigger event).
Existing mirror tests: trigger_test.go TestTriggerOrderedScalarAssignmentsMatchInfraUpdateOrderOfFields,
TestTriggerMultiActionMergeMatchesInfraMultiactionDeleteUpdate,
TestTriggerMergeNotMatchedAssignmentUsesCorrelatedSubquery.

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergeMultiactionScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-multiaction.sh (new)
- testdata/parity/infra-nwtable-on-merge-multiaction.json (new)
- internal/app/parity/infra_nwtable_on_merge_multiaction.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/**, internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.
- Do NOT modify the checked-in infra-nwtable-on-merge{,-nested,-insertstream}.* assets.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-multiaction-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
