# Contract — Draft 4.479 `infra-nwtable-on-merge-basic`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 0-3 (the foundational on-merge surface), one runtime surface:
`on SupportBean sb merge MyInfra` over a keepall named window vs a primary-key table,
asserted via merge-statement listener new/old data plus infra iterator snapshots.
No virtual time, no FAF, no patterns, no secondary schemas — only SupportBean.

- ord 0 `InfraOnMergeSimpleInsert{namedWindow=true}` — `java-runtime-dbf13fb6d1ca3a37275a`, static `java-0fdb4e6490ae6d9e9103`, flags [] (static OBSERVEROPS).
  create: `@name('create') @public create window MyInfra#keepall() as (p0 string, p1 int)`
  merge: `@name('merge') on SupportBean sb merge MyInfra insert select theString as p0, intPrimitive as p1`
  Steps: deploy create; deploy merge + listener 'merge'; milestone(0); send (E1,1) -> merge new {p0=E1,p1=1}; iterator 'create' = [{E1,1}]; milestone(1); send (E2,2) -> merge last-new {E2,2}; iterator any-order = [{E1,1},{E2,2}]; undeployAll.
- ord 1 `InfraOnMergeSimpleInsert{namedWindow=false}` — `java-runtime-df3d7a21bdd769f1acce`, same static ID.
  create: `@name('create') @public create table MyInfra(p0 string primary key, p1 int)`; merge identical.
- ord 2 `InfraOnMergeMatchNoMatch{namedWindow=true}` — `java-runtime-240cb61eabb22eb2a215`, static `java-5f5d122bcbf66d59da6c`.
  create: `@name('create') @public create window MyInfra.win:keepall() as SupportBean` (listener 'create' attached)
  merge (VERBATIM incl. missing space `*when`): `@name('merge') on SupportBean sb merge MyInfra mw where sb.theString = mw.theString when matched and sb.intPrimitive < 0 then delete when not matched and intPrimitive > 0 then insert select *when matched and sb.intPrimitive > 0 then update set intPrimitive = sb.intPrimitive + mw.intPrimitive`
  Steps: milestone(0); send (E1,0) -> SILENT; send (E2,2) -> merge new {E2,2}; iterator = [{E2,2}]; milestone(1); send (E2,10) -> IR pair new {E2,12}/old {E2,2}; iterator = [{E2,12}]; milestone(2); send (E2,-1) -> old-only {E2,12}; iterator empty; milestone(3); send (E3,3) -> new {E3,3}; send (E3,4) -> IR pair new {E3,7}/old {E3,3}; iterator = [{E3,7}]; undeployAll; milestone(4).
- ord 3 `InfraOnMergeMatchNoMatch{namedWindow=false}` — `java-runtime-aaa45c95e95a919bd0c0`, same static ID.
  create: `@name('create') @public create table MyInfra(theString string primary key, intPrimitive int)` ('create' listener attached, never fires)
  merge: same as ord 2 except not-matched clause is `insert select theString, intPrimitive ` (explicit columns, trailing space).

## Harness
One scenario `infra-nwtable-on-merge`, four cases (simple-nw, simple-table, matchnomatch-nw,
matchnomatch-table), fresh engine per case with WithRuntimeURI(runtimeID). Ops:
deploy/deployed/send/snapshot(mode any for multi-row)/undeploy-all. Listeners on 'merge'
always and on 'create' for fidelity (fires only for NW). Milestones are HA checkpoint
no-ops emitting nothing. assertStatement(ON_MERGE) is oracle-internal and unrecorded.

## Go surface (asset-only; zero shared-core changes expected)
MergeIntoTableWhen/MergeIntoNamedWindowWhen with WhenMatchedDelete/WhenMatched/
WhenNotMatchedAny, TableField/NamedWindowField for matched-row reads
(internal/esper/trigger_test.go:1178,1299,3429). Clone runner
internal/app/parity/infra_nwtable_on_update.go + oracle
tools/java-oracle/InfraNWTableOnUpdateScenarioOracle.java + run-infra-nwtable-on-update.sh.

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergeScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge.sh (new)
- testdata/parity/infra-nwtable-on-merge.json (new)
- internal/app/parity/infra_nwtable_on_merge.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/** (shared core — primary agent owns; report gaps, do not patch)
- internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.

## Validation (primary agent)
Java trace via run-infra-nwtable-on-merge.sh; `-mode infra-nwtable-on-merge-diff` zero-diff;
run_test.go pin additions; make check; parity review; commit/push.
