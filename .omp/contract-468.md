# Draft 4.468 contract — epl-insert-into-populate-single-col-by-method-call

Frozen by read-only scouts `NextJavaContract468` + `NextGoSurface468` (reports
recovered via hub replies during Draft 4.467 review) and re-verified line-by-line
by the primary agent against the pinned Java source and the Go surface.

## Java oracle (pinned commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c)

Single execution `EPLInsertIntoPopulateSingleColByMethodCall`
(`java-runtime-abe5e5cbda9667e7e112`, static `java-9db09f558176cc93b13e`,
variant "direct", flags []), source
`regression-lib/.../epl/insertinto/EPLInsertIntoPopulateSingleColByMethodCall.java`.
One `run()` = 9 rounds in one runtime: bean implicit only; map/OA/Avro/JSON each
implicit + configured. Each round delivers exactly 1 newData / 0 oldData on the
asserted listener.

### Implicit variant (file lines ~89-120)

```
s1: @name('s1') @public insert into {Prefix}_Stream select * from {origin}
s2: @name('s2') @public insert into {Prefix}_Stream select com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.{fn}(s0) from {eventType} as s0
```

s1 deploys first, listener attached, SILENT (sent event is the sibling type).
s2 deploys second, listener attached. Asserts: s1/s2 statement eventType
underlying is subclass of `underlyingType`; after send, s2 newData[0]:
eventType class is `eventTypeType` (BeanEventType for implicit-bean, WrapperEventType for the other implicit rounds),
underlying class is `underlyingType`, asserted props. Undeploy s2 then s1.

### Configured variant (file lines ~123-148)

```
insert: @name('insert') insert into {target} select com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.{fn}(s0) from {origin} as s0
s0:     @name('s0') select * from {target}
```

insert deploys first (no listener), s0 second (listener). Send ONE origin
event; assertEventNew("s0"): underlying class `underlyingType`, eventBean class
`eventBeanType`, asserted props. Undeploy s0 then insert.

### Rounds (prefix/origin/fn/eventType/payload/asserted props)

| case | round | origin | fn | event type sent | payload | asserted props |
|---|---|---|---|---|---|---|
| implicit-bean | implicit | SupportBean | convertEvent | SupportMarketDataBean | ("ACME",0,0L,null) | theString="ACME" |
| implicit-map | implicit | MapOne | convertEventMap | MapTwo | {one:"1",two:"2"} | one="1", two="|2|" |
| configured-map | configured | MapTwo→target MapOne | convertEventMap | MapTwo | {one:"3",two:"4"} | one="3", two="|4|" |
| implicit-oa | implicit | OAOne | convertEventObjectArray | OATwo | ["1","2"] | one="1", two="|2|" |
| configured-oa | configured | OATwo→target OAOne | convertEventObjectArray | OATwo | ["3","4"] | one="3", two="|4|" |
| implicit-avro | implicit | AvroOne | convertEventAvro | AvroTwo | {one:"1",two:"2"} | one="1", two="|2|" |
| configured-avro | configured | AvroTwo→target AvroOne | convertEventAvro | AvroTwo | {one:"3",two:"4"} | one="3", two="|4|" |
| implicit-json | implicit | JsonOne | convertEventJson | JsonTwo | {"one":"1","two":"2"} | one="1", two="|2|" |
| configured-json | configured | JsonTwo→target JsonOne | convertEventJson | JsonTwo | {"one":"3","two":"4"} | one="3", two="|4|" |

### UDFs (SupportStaticMethodLib.java:308-336)

- `convertEventMap(Map)`: `{one: one, two: "|"+two+"|"}`
- `convertEventObjectArray(Object[])`: `[values[0], "|"+values[1]+"|"]`
- `convertEventAvro(GenericData.Record)`: new record same schema, one=val1, two="|"+val2+"|"
- `convertEventJson(JsonEventObject)`: JSON string `{"one":v1,"two":"|v2|"}`
- `convertEvent(SupportMarketDataBean)`: `new SupportBean(symbol, volume.intValue())`

### Preconfigured types (TestSuiteEPLInsertInto.configure)

- beans: SupportBean, SupportMarketDataBean (addEventType class)
- maps: MapOne, MapTwo — {one:String, two:String}
- OA: OAOne, OATwo — props [one,two], types [String,String]
- Avro: AvroOne, AvroTwo — `record("name").fields().requiredString("one").requiredString("two").endRecord()`; `getEventMeta().getAvroSettings().setEnableAvro(true)`
- JSON: created mid-run — `@buseventtype @public create json schema JsonOne(one string, two string); @buseventtype @public create json schema JsonTwo(one string, two string);`

### Sends

FBEANWTYPE=sendEventBean; FMAPWTYPE=sendEventMap; FOAWTYPE=sendEventObjectArray;
FAVROWTYPE=sendEventAvro (GenericData.validate first); FJSONWTYPE=sendEventJson(string).

## Observable record protocol

- `listener` records: asserted listener only (s2 implicit / s0 configured),
  `new` = rendered rows, `old` = []. Bean rows render projected asserted fields
  {theString,intPrimitive} (typed-columns projected-render precedent; Java
  asserts theString only, intPrimitive=0 is deterministic from the source
  event). Map/OA/Avro/JSON rows render all schema fields sorted by name.
- `value` records: pin the type assertions as the schema kind name
  (Struct/Map/ObjectArray/Avro/JSON). Implicit: value(s1) then value(s2)
  emitted at deploy time, before the send's listener record. Configured:
  value(s0) emitted after the send's listener record (mirrors assertEventNew
  timing). Java maps underlying class → kind name: SupportBean→Struct,
  Map/HashMap→Map, Object[]→ObjectArray, GenericData.Record→Avro,
  JsonEventObject→JSON. Configured-json's Object.class assertions are
  tautological; the kind record still pins the observable JSON representation.
- Per-case fresh runtime (established convention; Java shares one runtime with
  undeploys between rounds — observably equivalent).

## Go surface (no engine work)

- Registrations: `RegisterStruct` (SupportBean mirror {theString,intPrimitive},
  SupportMarketDataBean mirror {symbol,price,volume,feed}, Bean_Stream over the
  SupportBean mirror), `RegisterMap` (MapOne/MapTwo/Map_Stream), `RegisterObjectArray`
  (OAOne/OATwo/OA_Stream), `RegisterAvro` (AvroOne/AvroTwo/Avro_Stream),
  `RegisterJSON` (JsonOne/JsonTwo/Json_Stream). Go requires pre-registered
  `X_Stream` schemas (no auto-create) — approved difference, kind mirrors origin.
- s1 implicit: `FromAny(env, origin).Select(Selection{Expr: Transpose[T](EventValue[T]())}).InsertInto(stream, StatementName("s1"))` for bean/map/OA/Avro; JSON uses explicit `Alias("one",...)/Alias("two",...)` columns (Transpose to a JSON target requires a string payload; explicit-Alias is the iupsNativeTranspose precedent).
- s2/insert: `FromAny(env, eventType).Select(Selection{Expr: Transpose[R](Func1("convertEvent*", conv, EventValue[T]()))}).InsertInto(target, StatementName("s2"|"insert"))`.
- s0 configured: `FromAny(env, target).Query(StatementName("s0"))` (empty selections = select *).
- Sends: `Send` (struct), `SendRecord` (map), `SendObjectArray`, `SendAvro` (*AvroRecord via NewAvroRecordFromMap), `SendJSON` (raw payload bytes).
- Kind names: runner-local `map[esper.SchemaKind]string` (SchemaKind has no exported String()).

## Deliverables

- `testdata/parity/epl-insert-into-populate-single-col-method-call.json` (9 cases / 18 steps)
- `tools/java-oracle/EplInsertIntoPopulateSingleColByMethodCallScenarioOracle.java`
- `tools/java-oracle/run-epl-insert-into-populate-single-col-method-call.sh`
- `internal/app/parity/epl_insert_into_populate_single_col_method_call.go` + run.go wiring (modes `epl-insert-into-populate-single-col-method-call[-diff]`)
- run_test.go family (passing-evidence + trace-mutation tests)
- Generated: `.trace.json` (Java), `.go.trace.json`, `.evidence.json`
- Manifest: new born-DV case `case.epl-insert-into-populate-single-col-method-call`; capability `epl.insertinto-pattern` DV IDs +1, remaining narrowed; roadmap + CHANGELOG.
