# Contract — Draft 4.483 `infra-nwtable-on-merge-pattern-nowhere`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 26-31: three execution classes x {namedWindow=true,false}.
Asset-only (Go scout confirmed full API coverage; ords 26-27 use the established
pattern→route-stream→OnRecord merge workaround since direct OnPattern merge is blocked).
New sibling runner `infra-nwtable-on-merge-pattern-nowhere`.

- ord 26 `InfraPatternMultimatch{namedWindow=true}` — runtime ID from inventory
- ord 27 `InfraPatternMultimatch{namedWindow=false}` — runtime ID from inventory
- ord 28 `InfraNoWhereClause{namedWindow=true}` — runtime ID from inventory
- ord 29 `InfraNoWhereClause{namedWindow=false}` — runtime ID from inventory
- ord 30 `InfraMultipleInsert{namedWindow=true}` — runtime ID from inventory
- ord 31 `InfraMultipleInsert{namedWindow=false}` — runtime ID from inventory
All flags [] (the OBSERVEROPS static-manifest stamp on these classes is a heuristic —
the actual flags() at L986-988 belongs to InfraInnerTypeAndVariable ords 34-39).
No virtual time, no FAF, no listeners on 'create'; milestones are Java-only recovery
checkpoints — drop them (no scenario op). Each execution ends with undeployAll.

### Ords 26-27 (InfraPatternMultimatch)
Deploy 1 (path): nw: `@name('create') @public create window MyInfraPM#keepall as (c1 string, c2 string)` | table: `@name('create') @public create table MyInfraPM as (c1 string primary key, c2 string primary key)` (composite PK c1+c2).
Deploy 2 (same path): `@name('Merge') on pattern[every a=SupportBean(theString like 'A%') -> b=SupportBean(theString like 'B%', intPrimitive = a.intPrimitive)] me merge MyInfraPM mw where me.a.theString = mw.c1 and me.b.theString = mw.c2 when not matched then insert select me.a.theString as c1, me.b.theString as c2 ` — note trailing space, no semicolon. No listener attached.
Steps: send SupportBean(A1,1); send SupportBean(A2,1); milestone(0); send SupportBean(B1,1) -> iterator 'create' anyOrder {A1,B1},{A2,B1}; send SupportBean(A3,2); milestone(1); send SupportBean(A4,2); send SupportBean(B2,2) -> iterator {A1,B1},{A2,B1},{A3,B2},{A4,B2}; undeployAll.
Semantics: `every a` spawns one waiting branch per A-event; one B completes every pending
branch with matching intPrimitive -> one merge trigger per match (B1 fires twice).
Composite-key where makes re-matches no-ops.
Go side: pattern→route-stream→OnRecord merge workaround (trigger_pattern_multimatch_test.go
precedent; infra_nwtable_on_delete.go infraNWTableOnDeleteBuildPattern two-plan deploy).

### Ords 28-29 (InfraNoWhereClause)
Single module deploy: `@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n` + nw: `@name('create') @public create window MyInfraNWC#keepall as MySchema;\n` | table: `@name('create') @public create table MyInfraNWC (col1 string, col2 int);\n` (UNKEYED table — no primary key) + `on SupportBean_A delete from MyInfraNWC;\n` (unnamed) + `on MyEvent me merge MyInfraNWC mw when not matched and me.in1 like "A%" then insert(col1, col2) select me.in1, me.in2 when not matched and me.in1 like "B%" then insert select me.in1 as col1, me.in2 as col2 when matched and me.in1 like "C%" then update set col1='Z', col2=-1 when not matched then insert select "x" || me.in1 || "x" as col1, me.in2 * -1 as col2;\n` (unnamed merge, NO where clause).
Steps (assertPropsPerRowIteratorAnyOrder('create', col1,col2)): send MyEvent(E1,2) -> {xE1x,-2}; send MyEvent(A1,3) -> still {xE1x,-2} (matched, no matched clause fires); send SupportBean_A(Ax1) -> empty; milestone(0); send MyEvent(A1,4) -> {A1,4}; send MyEvent(B1,5) -> still {A1,4}; milestone(1); send SupportBean_A(Ax1) -> empty; milestone(2); send MyEvent(B1,5) -> {B1,5}; send MyEvent(C,6) -> {Z,-1}; undeployAll.
Semantics: merge without where = every existing target row is 'matched'; 'when not
matched' fires only when target is EMPTY. 'when matched and C%' updates ALL existing rows.
MyEvent sent as Map (default representation).

### Ords 30-31 (InfraMultipleInsert)
Single module deploy + addListener("Merge"): `@public @buseventtype create schema MyEvent as (in1 string, in2 int);\ncreate schema MySchema as (col1 string, col2 int);\n` + nw: `@public create window MyInfraMI#keepall as MySchema;\n` (unnamed) | table: `@public create table MyInfraMI (col1 string primary key, col2 int);\n` + `@name('Merge') on MyEvent merge MyInfraMI where col1=in1 when not matched and in1 like "A%" then insert(col1, col2) select in1, in2 when not matched and in1 like "B%" then insert select in1 as col1, in2 as col2 when not matched and in1 like "C%" then insert select "Z" as col1, -1 as col2 when not matched and in1 like "D%" then insert select "x"||in1||"x" as col1, in2*-1 as col2;\n`.
Steps (assertPropsNew('Merge', col1,col2) = listener insert-stream record; assertListenerNotInvoked = no record): send MyEvent(E1,0) -> not invoked; milestone(0); send MyEvent(A1,1) -> new {A1,1}; send MyEvent(B1,2) -> new {B1,2}; send MyEvent(C1,3) -> new {Z,-1}; milestone(1); send MyEvent(D1,4) -> new {xD1x,-4}; send MyEvent(B1,2) -> not invoked (matched; no matched clauses); undeployAll.
Semantics: ordered when-not-matched clauses, first matching condition wins; insert(col1,col2)
column-list form and select-alias form are equivalent; on-merge insert actions deliver rows
to the merge statement's own listener.

## Harness
One scenario `infra-nwtable-on-merge-pattern-nowhere`, six cases
(patternmultimatch-{nw,table}, nowhere-{nw,table}, multipleinsert-{nw,table}), fresh
engine per case with WithRuntimeURI(runtimeID). Ops: deploy/deployed/send/snapshot(mode
any)/undeploy-all. Listener on 'Merge' for multipleinsert cases only.

## Go surface (asset-only)
Pattern→route-stream→OnRecord merge workaround (ords 26-27); nil-match no-where merge
(ords 28-29); ordered WhenNotMatched clauses + merge listener insert-stream records
(ords 30-31). Existing mirror tests: trigger_pattern_multimatch_test.go,
TestTableMergeWithoutPrimaryKeyUsesExistingRowAsMatch,
TestTriggerMultipleInsertBranchesMatchInfraMultipleInsert.

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergePatternNoWhereScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-pattern-nowhere.sh (new)
- testdata/parity/infra-nwtable-on-merge-pattern-nowhere.json (new)
- internal/app/parity/infra_nwtable_on_merge_pattern_nowhere.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/**, internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.
- Do NOT modify the checked-in infra-nwtable-on-merge{,-nested,-insertstream,-multiaction}.* assets.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-pattern-nowhere-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
