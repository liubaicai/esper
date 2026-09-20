# Contract — Draft 4.484 `infra-nwtable-on-merge-flow-itv`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 32-39: InfraFlow{nw,table} + InfraInnerTypeAndVariable{nw,table}
x {OBJECTARRAY,MAP,DEFAULT}. Asset-only (Go scout confirmed full API coverage; unit tests
already mirror the semantics). New sibling runner `infra-nwtable-on-merge-flow-itv`.

- ord 32 `InfraFlow{namedWindow=true}` — `java-runtime-403bba8c6b29e32b1a8f`, flags []
- ord 33 `InfraFlow{namedWindow=false}` — `java-runtime-ae05c015767242106de7`, flags []
- ord 34 `InfraInnerTypeAndVariable{nw=true,OBJECTARRAY}` — `java-runtime-3303d0922bd2d722fa3c`, flags [OBSERVEROPS]
- ord 35 `InfraInnerTypeAndVariable{nw=false,OBJECTARRAY}` — `java-runtime-f20972a347aabfcc0260`, flags [OBSERVEROPS]
- ord 36 `InfraInnerTypeAndVariable{nw=true,MAP}` — `java-runtime-484a8e636d3734b87673`, flags [OBSERVEROPS]
- ord 37 `InfraInnerTypeAndVariable{nw=false,MAP}` — `java-runtime-76d16e1334c83e6d4002`, flags [OBSERVEROPS]
- ord 38 `InfraInnerTypeAndVariable{nw=true,DEFAULT}` — `java-runtime-462d190f20742a0c266d`, flags [OBSERVEROPS]
- ord 39 `InfraInnerTypeAndVariable{nw=false,DEFAULT}` — `java-runtime-8495f57749c2105b15a8`, flags [OBSERVEROPS]

### Ords 32-33 (InfraFlow)
Setup (RegressionPath shared): createEPL = nw: `@Name('Window') @public create window MyMergeInfra#unique(theString) as SupportBean` | table: `@Name('Window') @public create table MyMergeInfra (theString string primary key, intPrimitive int, intBoxed int)`; compileDeploy+addListener("Window"). Then `@Name('Insert') insert into MyMergeInfra select theString, intPrimitive, intBoxed from SupportBean(boolPrimitive)` and `@Name('Delete') on SupportBean_A delete from MyMergeInfra` (unconditional delete-all).
Merge EPL (verbatim): `@Name('Merge') on SupportBean(boolPrimitive=false) as up merge MyMergeInfra as mv where mv.theString=up.theString when matched and up.intPrimitive<0 then delete when matched and up.intPrimitive=0 then update set intPrimitive=0, intBoxed=0 when matched then update set intPrimitive=up.intPrimitive, intBoxed=up.intBoxed+mv.intBoxed when not matched then insert select ` + (nw: `*` | table: `theString, intPrimitive, intBoxed`). compileDeploy+addListener("Merge"). fields=[theString,intPrimitive,intBoxed].

runAssertionFlow (runs TWICE; milestoneInc after steps 3,5,7,9):
1. send SB(bool=T,E1,10,200) -> insert-into path; nw: Window new{E1,10,200}; table: Window not invoked; iterator {E1,10,200}; Merge not invoked.
2. SB(F,E1,11,201) -> matched update, intBoxed=201+200=401; nw Window IR{E1,11,401}/{E1,10,200}; iterator {E1,11,401}; Merge IR same.
3. SB(F,E2,13,300) -> not-matched insert; nw Window new{E2,13,300}; iterator {E1,11,401},{E2,13,300}; Merge new{E2,13,300}.
4. SB(F,E2,14,301) -> update 301+300=601; IR{E2,14,601}/{E2,13,300}; iterator {E1,11,401},{E2,14,601}.
5. SB(F,E2,15,302) -> update 302+601=903; IR{E2,15,903}/{E2,14,601}.
6. SB(F,E3,40,400) -> insert; new{E3,40,400}; iterator 3 rows.
7. SB(F,E3,0,1000) -> reset clause (intPrimitive=0): {E3,0,0} NOT 1400; IR{E3,0,0}/{E3,40,400}.
8. SB(F,E2,-1,1000) -> delete; Window(nw)+Merge old{E2,15,903}; iterator {E1,11,401},{E3,0,0}.
9. SB(F,E1,-1,1000) -> delete; old{E1,11,401}; iterator {E3,0,0}.
Between passes: undeployModuleContaining("Merge"); send SupportBean_A("A1") (clears infra);
listenerReset("Window"); eplToModelCompileDeploy(same epl,path)+addListener("Merge").
After pass 2: send SupportBean_A("A2") (clear); undeploy Merge; wildcard EPL (verbatim):
`@name('Merge') on SupportBean(boolPrimitive = false) as up merge MyMergeInfra as mv where mv.theString = up.theString when not matched then insert select ` + (nw: `up.*` | table: `theString, intPrimitive, intBoxed`); send SB(F,E99,2,3) -> iterator {E99,2,3}.
Ambiguous-columns module (compile+deploy only, NO sends, no path): `create schema TypeOne (id long, mylong long, mystring long);` + (nw: `@public create window MyInfraTwo#unique(id) as select * from TypeOne;` | table: `@public create table MyInfraTwo (id long, mylong long, mystring long);`) + `on TypeOne as t1 merge MyInfraTwo nm where nm.id = t1.id when not matched and mystring = 0 then insert select * when not matched then insert (id, mylong, mystring) select 0L, 0L, 0L`. Then undeployAll.
NOTE: the ambiguous-columns table merge is an UNKEYED table with a where clause — Go's
key-only MergeIntoTableWhen cannot express it; unobservable (Java never sends TypeOne
events), so the runner deploys the nil-keys equivalent.

### Ords 34-39 (InfraInnerTypeAndVariable)
Module 1 (path): `<repAnnot(MyLocalJsonProvidedMyInnerSchema)> @public create schema MyInnerSchema(in1 string, in2 int);` + `<repAnnot(MyLocalJsonProvidedMyEventSchema)> @public @buseventtype @public create schema MyEventSchema(col1 string, col2 MyInnerSchema)`. repAnnot: OBJECTARRAY->`@EventRepresentation('objectarray')`, MAP->`@EventRepresentation('map')`, DEFAULT->`` (empty).
Module 2 (path): nw: `<repAnnot(MyLocalJsonProvidedMyInfraITV)> @public create window MyInfraITV#keepall as (c1 string, c2 MyInnerSchema)` | table: `@public create table MyInfraITV as (c1 string primary key, c2 MyInnerSchema)`.
Module 3 (path): `@name('createvar') @public create variable boolean myvar` (Boolean, initial null).
Module 4 (path): Merge EPL verbatim (from Java source lines 916-925): `@name('Merge') on MyEventSchema me merge MyInfraITV mw where me.col1 = mw.c1 when not matched and myvar then insert select col1 as c1, col2 as c2 when not matched and myvar = false then insert select 'A' as c1, null as c2 when not matched and myvar is null then insert select 'B' as c1, me.col2 as c2 when matched then delete` — transcribe verbatim; the tri-state myvar (null/true/false) selects among three not-matched branches; matched deletes.
Steps: set-variable myvar (null/true/false) + send MyEventSchema + iterator snapshots on the infra. Full step sequence to be transcribed verbatim by the asset worker from the Java source (sendMyInnerSchemaEvent/sendSupportBeanEvent helpers, lines 1386-1421).

## Harness
One scenario `infra-nwtable-on-merge-flow-itv`, eight cases
(flow-{nw,table}, itv-{nw,table}-{objectarray,map,default}), fresh engine per case with
WithRuntimeURI(runtimeID). Ops: deploy/deployed/send/snapshot/set-variable/undeploy/
undeploy-all. Listeners on 'Window' and 'Merge' for flow cases; none for itv cases
(iterator-only).

## Go surface (asset-only)
Filtered SupportBean sources (boolPrimitive true/false), InsertIntoTable/InsertIntoNamedWindow
feeder, DeleteAllFrom*, 4-branch merge (matched-delete <0, matched-update =0, matched-update
fallback with TableField/NamedWindowField RHS, not-matched insert), wildcard
CopyMatchingFields merge, undeploy/redeploy, NW Subscribe + Table.Snapshot assertions;
RegisterVariable + SetVariable tri-state, WhenNotMatched(myvar)/Equal(myvar,false)/
IsNull(myvar) clauses, nested-fragment SetColumn('c2', col2), NullLiteral c2,
WhenMatchedDeleteAny, TableColumn.Nested + WithNestedPropertySchema. Existing mirror tests:
trigger_test.go infra-flow block (~L1439-1620), trigger_inner_type_variable_test.go.

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergeFlowITVScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-flow-itv.sh (new)
- testdata/parity/infra-nwtable-on-merge-flow-itv.json (new)
- internal/app/parity/infra_nwtable_on_merge_flow_itv.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/**, internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.
- Do NOT modify the checked-in infra-nwtable-on-merge* assets or runners.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-flow-itv-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
