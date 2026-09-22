# Draft 4.504 contract — event-objectarray-nested

## Scope
`EventObjectArrayEventNested` ords 0-4 + `EventObjectArrayEventNestedPojo` ord 0 (6 executions, all flags []).
Static IDs: `java-2b57d1da10ba0e8fcce5` (collection file), `java-a9545c224ccf899a4d45` (pojo file).
Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c`. Chain id `event-objectarray-nested`.

## Runtime IDs (case order)
- array-property: java-runtime-1704d5718125f4502592 (ord 0)
- mapped-property: java-runtime-072c48af61cdd8b27856 (ord 1)
- map-name-nested: java-runtime-5a08e705396c651e2159 (ord 2)
- map-name: java-runtime-7d6eb46f34c9d597b7d1 (ord 3)
- oa-nested: java-runtime-957493fa76a9e36a6091 (ord 4)
- pojo: java-runtime-57d2cf8ae645bedf06e6 (EventObjectArrayEventNestedPojo ord 0, variant 'direct')

## Java sources
- regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNested.java
- regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/event/objectarray/EventObjectArrayEventNestedPojo.java
- schema config: regression-run/.../TestSuiteEventObjectArray.java:60-145

## Per-case contract (verbatim EPLs in the Java scout report at agent://NextJavaContract504)

### array-property (ord 0) — TWO deploy cycles
Schemas: MyArrayOA = objectarray {p0:int[], p1:SupportBean[]}; MyArrayOAMapOuter = objectarray {outer:'MyArrayOA'}.
Cycle1 EPL: `@name('s0') select p0[0] as a, p0[1] as b, p1[0].intPrimitive as c, p1[1] as d, p0 as e from MyArrayOA`
Send OA [int[]{1,2,3}, SupportBean[]{('e1',5),('e2',6)}] -> a,b,c,d,e = {1,2,5,bean(e2,6),p0-array}.
undeployAll. Cycle2 EPL: `@name('s0') select outer.p0[0] as a, outer.p0[1] as b, outer.p1[0].intPrimitive as c, outer.p1[1] as d, outer.p0 as e from MyArrayOAMapOuter`
Send OA [[p0,beans]] -> same values (e asserted type-only in Java but trace records it).

### mapped-property (ord 1) — THREE cycles, sendEventMap payloads (OA types declared from Map defs)
Schemas: MyMappedPropertyMap={p0:Map}; MyMappedPropertyMapOuter={outer:<map{p0:Map}>}; MyMappedPropertyMapOuterTwo={outerTwo:SupportBeanComplexProps}.
Cycle1: `select p0('k1') as a from MyMappedPropertyMap`; send {p0:{k1:'v1'}} -> a='v1'.
Cycle2: `select outer.p0('k1') as a from MyMappedPropertyMapOuter`; send {outer:{p0:{k1:'v1'}}} -> a='v1'.
Cycle3: `select outerTwo.mapProperty('xOne') as a from MyMappedPropertyMapOuterTwo`; send {outerTwo: SupportBeanComplexProps.makeDefaultBean()} -> a='yOne'.

### map-name-nested (ord 2) — TWO cycles
Schemas: MyNamedMap=map{n0:int}; MyObjectArrayMapOuter=objectarray{outer:<map{p0:'MyNamedMap',p1:'MyNamedMap[]'}>}.
Cycle1: `select outer.p0.n0 as a, outer.p1[0].n0 as b, outer.p1[1].n0 as c, outer.p0 as d, outer.p1 as e from MyObjectArrayMapOuter`
Send OA [{p0:{n0:1},p1:[{n0:2},{n0:3}]}] -> {1,2,3,map{n0:1},[{n0:2},{n0:3}]}.
Cycle2 same with '?' dynamic syntax: `outer.p0.n0?`, `outer.p1[0].n0?`, `outer.p1[1]?.n0`, `outer.p0?`, `outer.p1?` — same values.

### map-name (ord 3) — ONE cycle
Schema: MyOAWithAMap=objectarray{p0:'MyNamedMap',p1:'MyNamedMap[]'}.
EPL: `select p0.n0 as a, p1[0].n0 as b, p1[1].n0 as c, p0 as d, p1 as e from MyOAWithAMap`
Send OA [{n0:1},[{n0:2},{n0:3}]] -> {1,2,3,map{n0:1},[{n0:2},{n0:3}]}.

### oa-nested (ord 4) — ITERATOR, not listener
Schemas: TypeLev1={p1id:int}; TypeLev0={p0id:int,p1:'TypeLev1'}; TypeRoot={rootId:int,p0:'TypeLev0'}.
EPL: `@name('s0') select * from TypeRoot#lastevent`
Send OA [10,[100,[1000]]]; assertIterator next() props rootId/p0.p0id/p0.p1.p1id = {10,100,1000}.
Trace: iterator record (operation "iterator") with the three resolved values.

### pojo (EventObjectArrayEventNestedPojo ord 0) — THREE cycles
Schema NestedObjectArr = objectarray {simple:String, object:SupportBean_A, nodefmap:Map, map:<nested def>};
nested def = {simpleOne:Integer, objectOne:SupportBeanComplexProps, nodefmapOne:Map, mapOne:{simpleTwo:Integer, objectTwo:SupportBeanCombinedProps, nodefmapTwo:Map, mapTwo:{simpleThree:Long, objectThree:SupportBean_B}}}.
Also MyNested = objectarray {bean: MyNested.class} (POJO with List<MyInside> insides; MyInside{id}).
Cycle1: 22-column projection (verbatim EPL in scout report) over NestedObjectArr; send testdata OA; expected values per scout report (a3/a4 null via '?' missing keys).
Cycle2: `select * from NestedObjectArr` — type assertions only (propertyNames {simple,object,nodefmap,map}); trace: send the same event, record listener row (all 4 columns) — Java asserts types not values, but the listener still fires; record the row.
Cycle3: `select * from MyNested(bean.insides.anyOf(i=>id = 'A'))`; send OA [MyNested([MyInside('A')])]; assertListenerInvoked -> one listener record.

## Oracle requirements (tools/java-oracle/EventObjectArrayNestedScenarioOracle.java + run script)
- Mirror sibling oracles: parse scenario, per-case Configuration + runtime URI `event-objectarray-nested-<case>`, advanceTime(0), deploy EPLs verbatim, listener records {case,operation:listener,statement,sequence,time,new/old rows via sorted-property rows()}, undeploy between cycles.
- Deploy cycles: use scenario ops `deploy` (statement s0) and `undeploy-all` between cycles, or per-case sequential deploys — pick the op set already used by sibling oracles (deploy/undeploy-all/case/send).
- ord 4 iterator: after sends, call statement.iterator() (or safeIterator), read next() props rootId/p0.p0id/p0.p1.p1id, emit record {case,operation:"iterator",statement:"s0",sequence,value:{rootId:10,"p0.p0id":100,"p0.p1.p1id":1000}} — check sibling oracles for the iterator record shape precedent (grep "iterator" tools/java-oracle/*.java).
- normalize(): beans must render deterministically — SupportBean -> {theString,intPrimitive} row; SupportBean_A -> {id}; SupportBean_B -> {id}; SupportBeanComplexProps -> {mapProperty:{xOne,xTwo}, indexed:[1,2], nested:{nestedValue,nestedNested:{nestedNestedValue}}, arrayProperty:[10,20,30], simpleProperty:"simple"}; SupportBeanCombinedProps -> {indexed:[NestedLevOne rows], array:[same]}; NestedLevOne -> {mapprop:{k:{value}}, nestLevOneVal:"abc"}; NestedLevTwo -> {value}; Map -> sorted-keys row; Object[]/arrays -> JSON array; null -> {state:null}.
- Send ops: `send` with eventType + payload. OA sends: payload is a JSON array (positional). Map sends (ord 1): payload is a JSON object -> sendEventMap. SupportBean/beans inside OA payloads: construct the Java beans in the oracle (payload describes them structurally: {"theString":"e1","intBoxed":5} -> new SupportBean("e1",5); {"_bean":"SupportBeanComplexProps"} -> makeDefaultBean(); {"_bean":"SupportBean_A","id":"A1"}; {"_bean":"SupportBean_B","id":"B1"}; {"_bean":"MyNested","insides":[{"id":"A"}]}).
- ord 1 sends use sendEventMap (payload object) — the scenario marks them with the same `send` op; the oracle switches on eventType.
- pojo cycle3: MyNested bean with insides list; send OA [bean].

## Go runner requirements (internal/app/parity/event_objectarray_nested.go) — PRIMARY AGENT OWNS
- RegisterObjectArray for all types; nested types via WithNestedPropertySchema / WithNestedPropertySchemaFrom; AllowDynamicFields() where Field path strings are used.
- Field[any,any]("p0.p0id")-style path strings resolve at runtime via Schema.get; build-time needs AllowDynamicFields on the OA schema.
- '?' optional access -> Java null: wrap in CoalesceOf(expr, NullLiteral[any]()) so Missing renders {state:null}.
- ord 4: LastEvent window + statement.Snapshot -> iterator record.
- pojo cycle3: From with filter predicate EnumAnyOf over bean.insides.
- Beans: Go mirror structs with esper tags; SupportBeanComplexProps/CombinedProps mirrors already exist in package (wcwNestedOne/wcwNestedTwo or patternCombined* — reuse or add local mirrors).
- Rendering: bean-valued columns need manual ResultRecord construction where NormalizeResults can't render Go structs (precedent: wcwCombinedRows).

## Files
- NEW tools/java-oracle/EventObjectArrayNestedScenarioOracle.java (asset worker)
- NEW tools/java-oracle/run-event-objectarray-nested.sh (asset worker, copy sibling)
- NEW testdata/parity/event-objectarray-nested.json (asset worker)
- NEW internal/app/parity/event_objectarray_nested.go (primary)
- EDIT internal/app/parity/run.go (primary)
- EDIT internal/app/parity/run_test.go (primary)
- Manifest/roadmap/PLANS (primary)
