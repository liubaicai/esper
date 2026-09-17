# FROZEN CONTRACT — InfraNWTableOnDelete (Draft 4.447), all 6 executions
Oracle commit: 9e1b9f1cc9117fea4bf33ab043762c045d73839c
Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnDelete.java
No flags; no virtual time. Scenario id: infra-nwtable-on-delete.

## IDs
- runtimeIds: ord0 java-runtime-6d190be4d9e8b76b11f3 (cond nw), ord1 java-runtime-992fcd3db7f15b8254cd (cond table), ord2 java-runtime-2ab09353c939b15ee52e (pattern nw), ord3 java-runtime-e795679c4403d309f805 (pattern table), ord4 java-runtime-11809b3f1d90b37d52b8 (deleteall nw), ord5 java-runtime-b7a9b1b53c0009e7d347 (deleteall table)
- staticIds (per class, shared by the nw/table pair): InfraDeleteCondition java-eb28fd3f17513a399ac9; InfraDeletePattern java-132a1f9e7f2c434a19e9; InfraDeleteAll java-e3aeca986a0cfee60dc7
- javaNames: InfraDeleteCondition{namedWindow=true}, InfraDeleteCondition{namedWindow=false}, InfraDeletePattern{namedWindow=true}, InfraDeletePattern{namedWindow=false}, InfraDeleteAll{namedWindow=true}, InfraDeleteAll{namedWindow=false}
- javaFlags: []

## Shared helpers
- sendSupportBean(theString, intPrimitive) → SupportBean{theString, intPrimitive}
- sendSupportBean_A(id)/_B(id) → SupportBean_A/B{id:String}
- getCount = FAF `select count(*) as c0 from MyInfra` → Long (window AND table)
- assertPropsNew/Old = exactly 1 invocation since reset, 1 row, other stream null; resets listener
- assertPropsPerRowIterator = ordered iterator compare; null expected ≡ empty
- assertListener(name, consumer) = raw listener, no reset, no count check
- milestone(n) = HA checkpoint boundary (no-op)

## ord 0/1 InfraDeleteCondition{namedWindow=true/false}
EPLs (deploy order):
 1) nw: `@name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean`
    table: `@name('CreateInfra') @public create table MyInfra (a string primary key, b int)`  [space before paren]
    → listener "CreateInfra"
 2) `on SupportBean_A delete from MyInfra where 'X' || a || 'X' = id`  (anonymous, no listener)
 3) `on SupportBean_B delete from MyInfra where b < 5`  (anonymous, no listener)
 4) `insert into MyInfra select theString as a, intPrimitive as b from SupportBean`  (anonymous)
Sends/assertions:
 - SB(E1,1), SB(E2,2); milestone(0)
 - SB(E3,3) → count==3; listenerReset(CreateInfra); CreateInfra iterator AnyOrder {E1,1},{E2,2},{E3,3}
 - A("XE2X") → deletes E2 only. nw: CreateInfra assertPropsOld {E2,2}. listenerReset; iterator AnyOrder {E1,1},{E3,3}; count==2
 - SB(E7,7) → iterator AnyOrder {E1,1},{E3,3},{E7,7}; count==3; milestone(1)
 - B("B1") → deletes b<5 → E1,E3 (E7 kept). nw: assertListener → lastOldData len 2, [0]={E1,1},[1]={E3,3} (single batch, insertion order). iterator AnyOrder {E7,7}; count==1; undeployAll
 - table: identical sends/iterators/counts; ZERO listener assertions.

## ord 2/3 InfraDeletePattern{namedWindow=true/false}
EPLs:
 1) nw: `@name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean`
    table: `@name('CreateInfra') @public create table MyInfra(a string primary key, b int)`  [NO space before paren]
    → listener "CreateInfra"
 2) `@name('OnDelete') on pattern [every ea=SupportBean_A or every eb=SupportBean_B] delete from MyInfra` → listener "OnDelete"
 3) `insert into MyInfra select theString as a, intPrimitive as b from SupportBean` (anonymous)
Sends/assertions:
 - SB(E1,1) → nw: CreateInfra assertPropsNew {E1,1}; OnDelete iterator empty. both: CreateInfra iterator ordered {E1,1}; count==1
 - A("A1") → delete-all. nw: CreateInfra assertPropsOld {E1,1}; OnDelete iterator empty. both: CreateInfra iterator empty; OnDelete assertPropsNew {E1,1} (deleted row as NEW); count==0; milestone(0)
 - SB(E2,2) → nw: CreateInfra assertPropsNew {E2,2}. both: iterator {E2,2}; count==1
 - B("B1") → delete-all. nw: CreateInfra assertPropsOld {E2,2}. both: iterator empty; OnDelete assertPropsNew {E2,2}; count==0; undeployAll

## ord 4/5 InfraDeleteAll{namedWindow=true/false}
EPLs — `@Name` capital N:
 1) nw: `@Name('CreateInfra') @public create window MyInfra#keepall as select theString as a, intPrimitive as b from SupportBean`
    table: `@Name('CreateInfra') @public create table MyInfra (a string primary key, b int)`  [space before paren]
    → listener "CreateInfra"
 2) `@Name('OnDelete') on SupportBean_A delete from MyInfra` → listener "OnDelete"; OnDelete eventType propertyNames anyOrder == {a,b}
 3) `@Name('Insert') insert into MyInfra select theString as a, intPrimitive as b from SupportBean` (no listener)
 4) `@Name('Select') select irstream MyInfra.a as a, b from MyInfra as s1` → listener "Select"
Sends/assertions:
 - A("A1") on EMPTY infra → CreateInfra/Select/OnDelete all NOT invoked; count==0
 - SB(E1,1) → nw: CreateInfra assertPropsNew {E1,1}, Select assertPropsNew {E1,1}. table: CreateInfra NOT invoked, Select NOT invoked. both: CreateInfra iterator ordered {E1,1}; OnDelete iterator empty; count==1; milestone(0)
 - A("A2") → delete-all (1 row). nw: CreateInfra assertPropsOld {E1,1}; Select assertPropsOld {E1,1}; OnDelete iterator empty. both: CreateInfra iterator empty; OnDelete assertPropsNew {E1,1}; count==0
 - SB(E2,2), SB(E3,3) → listenerReset(CreateInfra); iterator ordered {E2,2},{E3,3}; OnDelete NOT invoked; count==2; milestone(1)
 - A("A2") → delete-all (2 rows). nw: assertListener CreateInfra → lastOldData[0]={E2,2},[1]={E3,3}; OnDelete iterator empty. both: CreateInfra iterator empty; assertListener OnDelete → lastNewData[0]={E2,2},[1]={E3,3}; count==0; undeployAll

## Pitfalls
1. On-delete trigger output = DELETED ROWS as newData (event type {a,b}) — not trigger event, not oldData. Only listener that fires for tables.
2. Empty delete → trigger listener NOT invoked.
3. `create window ... from SupportBean` is schema source ONLY; population exclusively via insert-into.
4. NW fires CreateInfra (new on insert, old on delete) + irstream Select; table fires NEITHER.
5. Where-clause scoping: `id` = trigger event prop; `a`,`b` = infra row props.
6. OnDelete statement iterator always empty.
7. Pattern fires once per A or B event; unconditional delete-all.
8. Ordered iterator/batch in DeleteAll & Pattern; AnyOrder in DeleteCondition.
9. EPL byte-exactness: `@Name` vs `@name`; `MyInfra (a` vs `MyInfra(a` spacing differs per execution.
10. assertPropsNew/Old = exactly ONE invocation since reset.
11. milestone positions: Condition after E2 send + after E7 recount; Pattern after first delete; DeleteAll after first delete + after 2-row refill.
12. Deployment order: Condition deploys both deletes BEFORE insert-into; `@public` required.

## Go surface (from scout)
- OnEvent[T](stream) / OnRecord(stream) → TriggerStream; DeleteFromNamedWindow(window, pred), DeleteAllFromNamedWindow(window), DeleteFromTableWhere(table, pred), DeleteAllFromTable(table); .Query(StatementName(...)).
- NamedWindowField[V] reads candidate row; TableField[V] for table rows; Field reads trigger event.
- Concat covers `'X' || a || 'X' = id`; Less covers `b < 5`.
- Pattern-triggered delete: NO native OnPattern. Workaround: PatternFromRecord(FromAny(env,"SupportBean_A"),"ea",Literal(true)).Every().Or(PatternFromRecord(FromAny(env,"SupportBean_B"),"eb",Literal(true)).Every()).Select(Alias("id", TagField[string]("ea","id"))).InsertInto("PatternTrigger") then OnRecord(FromAny(env,"PatternTrigger")).DeleteAllFromNamedWindow/DeleteAllFromTable. Precedent: TestTriggerPatternMultimatchMatchesInfraPatternMultimatch.
- getCount → FAF count or a deployed count statement (named_window_mutation precedent).
- Closest runner template: internal/app/parity/infra_nwtable_subq_uncorrel.go (same MyInfra NW/table pair, isTable dispatch, strict-loader conventions).
- Owning capability: trigger.table-named-window.
