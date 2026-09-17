# FROZEN CONTRACT — InfraNWTableOnUpdate (Draft 4.448), 6 executions
Oracle commit: 9e1b9f1cc9117fea4bf33ab043762c045d73839c
Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnUpdate.java
No flags; no virtual time. Scenario id: infra-nwtable-on-update.

## IDs
- ord0 SceneOne{namedWindow=true}:  runtime java-runtime-f8090e148364d7b15116, static java-bc54a78188b2249a0a9f
- ord1 SceneOne{namedWindow=false}: runtime java-runtime-922adbe6628b17d5ec61, static java-bc54a78188b2249a0a9f
- ord4 SubquerySelf{namedWindow=true}:  runtime java-runtime-046926326cad5f0172cd, static java-c67318a7541b42eb7940
- ord5 SubquerySelf{namedWindow=false}: runtime java-runtime-702d919aaadcbb188467, static java-c67318a7541b42eb7940
- ord6 MultikeyWArray{namedWindow=true}:  runtime java-runtime-5cc56f78a52d6a452948, static java-9ab08590ab6533e21139
- ord7 MultikeyWArray{namedWindow=false}: runtime java-runtime-b23b2fbc3cc233237ac9, static java-9ab08590ab6533e21139
- javaNames: InfraNWTableOnUpdateSceneOne{namedWindow=true/false}, InfraSubquerySelf{namedWindow=true/false}, InfraSubqueryMultikeyWArray{namedWindow=true/false}
- flags: [] all six. Ords 2/3 (InfraUpdateOrderOfFields) already referenced — excluded.

## ords 0/1 InfraNWTableOnUpdateSceneOne (lines 96-161)
Deploy order:
1. nw:  `@name('create') @public create window MyInfra.win:keepall() as SupportBean`  [listener 'create']
   tbl: `@name('create') @public create table MyInfra(theString string, intPrimitive int primary key)`  [listener 'create']
2. `@name('insert') insert into MyInfra select theString, intPrimitive from SupportBean`  [no listener]
milestone(0); send SupportBean("A1",1); send SupportBean("B2",2)
3. `@name('update') on SupportBean_S0 update MyInfra set theString = p00 where intPrimitive = id`  [listener 'update']
milestone(1)
send SupportBean_S0(id=1, p00="X1") -> IRPair("update", [theString,intPrimitive], new={"X1",1}, old={"A1",1})
  nw: ordered iterator 'create' {{"B2",2},{"X1",1}} — updated row reinserted at tail
  tbl: iteratorAnyOrder same rows
milestone(2)
send SupportBean_S0(id=2, p00="X2") -> IRPair new={"X2",2} old={"B2",2}; iteratorAnyOrder {{"X1",1},{"X2",2}}
milestone(3); iteratorAnyOrder again; undeployModuleContaining("insert"),("update"),("create"); milestone(4); undeployAll.
Semantics: on-update listener new=post-update row, old=pre-update row. Unqualified names:
theString/intPrimitive=infra row, p00/id=trigger. nw 'create' listener receives the same
IR pair (window consumer) plus new-rows on inserts; table 'create' listener never delivers.

## ords 4/5 InfraSubquerySelf (lines 209-245, ESPER-507)
Deploy order:
1. nw:  `@name('create') @public create window MyInfraSS#keepall as SupportBean`  [NO listener]
   tbl: `@name('create') @public create table MyInfraSS(theString string primary key, intPrimitive int)`  [NO listener]
2. ANONYMOUS: `insert into MyInfraSS select theString, intPrimitive from SupportBean`
3. ANONYMOUS on-update, byte-exact (@Name double-quotes + embedded newline):
   `@Name("Self Update")\non SupportBean_A c\nupdate MyInfraSS s\nset intPrimitive = (select intPrimitive from MyInfraSS t where t.theString = c.id) + 1\nwhere s.theString = c.id`
   [NO listener]
Events: SupportBean("E1",1); SupportBean("E2",6); SupportBean_A("E1") -> E1: 1->2
milestone(0)
SupportBean_A("E1") -> E1: 2->3; SupportBean_A("E2") -> E2: 6->7
assertPropsPerRowIteratorAnyOrder("create", [theString,intPrimitive], {{"E1",3},{"E2",7}}); undeployAll.
Semantics: subquery correlated to TRIGGER (c.id), reads pre-update value of same infra.
Only observable output is the final 'create' iterator.

## ords 6/7 InfraSubqueryMultikeyWArray (lines 48-93)
Deploy order:
1. nw:  `@name('create') @public create window MyInfra#keepall() as (value int)`  [listener 'create']
   tbl: `@name('create') @public create table MyInfra(value int)`  [listener 'create']
2. FAF (compileExecuteFAFNoResult): `insert into MyInfra select 0 as value` -> row value=0
3. ANONYMOUS on-update, NO where (updates ALL rows), no listener:
   `on SupportBean update MyInfra set value = (select sum(value) as c0 from SupportEventWithIntArray#keepall group by array)`
Events: SupportEventWithIntArray(id,array,value): E1{[1,2],10}; E2{[1,2],11}
milestone(0)
send SupportBean() [default] -> subquery single group [1,2] sum=21 -> value=21; iterator 'create' value==21
send E3{[1,2],12}; send SupportBean() -> sum=33 -> value=33
milestone(1)
send E4{[1],13}; send SupportBean() -> TWO groups -> scalar assignment NULL -> value=null
undeployAll.
Semantics: group by int[] = array CONTENT equality; sum(int) -> Integer; multi-row
subquery in scalar assignment -> null not error. SupportBean trigger does NOT insert.
nw 'create' listener: new={value:0} on FAF insert + IR pairs per update; table: nothing.

## Pitfalls
- Dispatch order (sibling fix): infra consumer deliveries precede trigger's own listener
  in one merged dispatch — nw SceneOne: 'create' IR pair then 'update' IR pair per event.
- keepall NW on-update = remove+reinsert: updated row moves to tail of iteration order.
- Table 'create' listeners never deliver; nw 'create' listeners deliver on insert AND update.
- Multi-row aggregated subquery in scalar assignment -> null, not error.
- Anonymous statements: SubquerySelf insert+on-update, MultikeyWArray on-update, FAF insert.
- assertPropsIRPair = exactly one new + one old row, then listener reset.
- Milestones HA-only, no observable output.
- SceneOne asserts StatementType.ON_UPDATE — no Go equivalent; record deployed marker.

## Go surface (scout verdict: all six READY, zero engine work expected)
- UpdateNamedWindow(window, predicate, assignments...) trigger.go:527; UpdateTableWhere :462.
- Subquery-in-assignment: visitQueryExpressions covers trigger.assignments (plan.go:3897);
  NW/table subquery sources take locked live snapshots (subquery.go:2290).
- Self-correlation: OuterField[V] inside SubqueryWhere resolves to trigger event.
- Group-by-array: SubqueryValueWithOptions + SubqueryGroupKey(Field[any,[]int]) +
  SubqueryCardinalityMode(SubqueryNullOnMultiple); EqualValues = slice content equality.
- FAF insert: FromNamedWindow/FromTable(env,"MyInfra").OnDemand().InsertRows(InsertValues(...)).
- Mutation deferral covers NW update (runtime.go:4085-4088, 5711-5714).
- Closest template: internal/app/parity/infra_nwtable_on_delete.go.
- Event types: SupportBean, SupportBean_S0{id,p00}, SupportBean_A{id},
  SupportEventWithIntArray{id,array []int,value int}.
