# Draft 4.534 contract — infra-nwtable-event-type

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableEventType.java (3 executions, flags=[], no virtual time).

## Executions

### ord 0 InfraNWTableEventTypeInvalid — java-runtime-d3c3a24f11288969df7e / static java-08b4a7da65c76bf5abdc
Two tryInvalidCompile probes (compile-only, no deploy/events):
- A: `create schema SchemaOne as (p0 string);\ncreate window SchemaOne#keepall as SchemaOne;\n` → prefix `Error starting statement: An event type or schema by name 'SchemaOne' already exists`
- B: `create schema SchemaTwo as (p0 string);\ncreate table SchemaTwo(c0 int);\n` → prefix `An event type by name 'SchemaTwo' has already been declared`
Go: `env.RegisterSchema`/`RegisterMap` for SchemaOne/SchemaTwo, then `CreateNamedWindow`/`CreateTable` with the colliding name → expect build error; record `compile-error` op with pinned Java prefix as value (convention: context_variables.go buildError).

### ord 1 InfraNWTableEventTypeDefineFields — java-runtime-8dc1233d90336dcdb929 / static java-50c0d9ea894da18ddb54
Two cycles in one execution: namedWindow=true then false.
- NW EPL: `@name('s0') @public create window MyInfra#keepall as (c0 int[], c1 int[primitive])`
- Table EPL: `@name('s0') @public create table MyInfra (c0 int[], c1 int[primitive])`
Assertion: positional property descriptors [0]={c0,Integer[].class}, [1]={c1,int[].class}.
Go: `int[]`→`[]*int32` (boxed Integer[]), `int[primitive]`→`[]int32` (primitive int[]). Record `types` op (convention: epl_other_select_expr.go) with entries {name,type} where type renders Java class names: `Integer[]` for []*int32, `int[]` for []int32. Two cases: define-fields-window, define-fields-table.

### ord 2 InfraNWTableEventTypeInsertIntoProtected — java-runtime-14412293edec92196bd9 / static java-c007ca393634b73ec8b5
Single-module EPL (verbatim, doubled @public literal):
`module test;\n@name('event') @public @buseventtype @public create map schema Fubar as (foo string, bar double);\n@name('window') @protected create window Snafu#keepall as Fubar;\n@name('insert') @private insert into Snafu select * from Fubar;\n`
sendEventMap {foo:"a",bar:1d},{foo:"b",bar:2d} to "Fubar"; assertPropsPerRowIterator("window",["foo","bar"],[["a",1d],["b",2d]]) exact order; undeployAll.
Go: env-level equivalents (no module needed): RegisterMap Fubar + BusEventType, CreateNamedWindow Snafu KeepAll, OnRecord(FromAny(Fubar)).InsertIntoNamedWindow(Snafu, CopyMatchingFields()); send via engine.Send("Fubar", map); snapshot/iterate window statement → record `snapshot` op rows.

## Deliverables (asset worker)
- testdata/parity/infra-nwtable-event-type.json (3 cases; metadata pins; verbatim EPL)
- internal/app/parity/infra_nwtable_event_type.go (runner + type-token renderer)
- tools/java-oracle/InfraNWTableEventTypeScenarioOracle.java + run-infra-nwtable-event-type.sh
- run.go mode registration + run_test.go wiring

## Forbidden
Subagent: no formatter/lint/tests, no manifest/roadmap/CHANGELOG/PLANS edits, no commit/push, no internal/esper edits, no hand-authored traces/evidence.
