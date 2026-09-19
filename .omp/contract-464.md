# Contract — Draft 4.464 'epl-insert-into-typed-columns'

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (do not modify).
Java files: EPLInsertIntoEmptyPropType.java (ords 0-1), EPLInsertIntoEventTypedColumnFromProp.java (ords 0-1).
New runner: internal/app/parity/epl_insert_into_typed_columns.go; mode `epl-insert-into-typed-columns` (+`-diff`).
Scenario: testdata/parity/epl-insert-into-typed-columns.json (LIGHT format: version/id/description/javaRuntimes/cases/steps).
Oracle tool: tools/java-oracle/EplInsertIntoTypedColumnsScenarioOracle.java + run-epl-insert-into-typed-columns.sh (copy run-infra-table-insert-into.sh shape).
Evidence: testdata/parity/epl-insert-into-typed-columns.{trace.json,go.trace.json,evidence.json}.
Manifest: new `case.epl-insert-into-typed-columns` mapped to capability `epl.insertinto-pattern`.

## Cases (6 labels, 4 runtime IDs)

| case | runtime ID | Java execution | static ID |
|---|---|---|---|
| named-window-model-after | java-runtime-0870abe075dc95308a27 | EPLInsertIntoNamedWindowModelAfter | java-00bd26d73c1bd2e1695a |
| create-schema-map | java-runtime-d585492dbeef1deee74f | EPLInsertIntoCreateSchemaInsertInto | java-f054ac226586064128ca |
| create-schema-objectarray | java-runtime-d585492dbeef1deee74f | (same, OA sub-round) | (same) |
| create-schema-bean | java-runtime-d585492dbeef1deee74f | (same, bean sub-round) | (same) |
| event-typed-column-on-merge | java-runtime-2fb237744f8a8b010414 | EPLInsertIntoEventTypedColumnOnMerge | java-5eeac65322c142a5ed7c |
| pojo-typed-column-on-merge | java-runtime-700d690d1c5c019ec414 | EPLInsertIntoPOJOTypedColumnOnMerge | java-85b735959564286e59a4 |

The three create-schema-* labels are the sub-rounds of ONE Java execution (shared runtime ID; the map sub-round's soda=true/false double run is compile-path-only, replayed once — approved difference).

## Ops
`case`, `send` (eventType+payload), `advance-time` (ms), `snapshot` (statement label -> window/table via per-case map), `faf-insert` (statement=window name; Java compileExecuteFAFNoResult "insert into W select null"), `faf-delete` (statement=window/table name; "delete from W"), `value` (statement label; emits the event-type name of the first row of a fresh snapshot — pins Java's getEventType().getName()/instanceof assertions). Listener records emitted by subscribed statements (s0); subscriber record emitted by the OA case's s0 subscriber.

## Case contracts

### named-window-model-after (EmptyPropType ord 0, EPLInsertIntoEmptyPropType.java:44-78)
EPL: `@public create schema EmptyPropSchema()`; `@name('window') @public create window EmptyPropWin#keepall as EmptyPropSchema`; `insert into EmptyPropWin() select null from SupportBean`; `on SupportBean_S0 merge EmptyPropWin when not matched then insert select null`; `on SupportBean_S1 insert into EmptyPropWin select null`.
Go: RegisterMap("EmptyPropWin", nil) [schema registered under the WINDOW name so the row type name is EmptyPropWin]; CreateNamedWindow KeepAll; From[bean].InsertInto("EmptyPropWin") (zero-selection route projects {} into empty schema); OnEvent(S0).MergeIntoNamedWindowWhen("EmptyPropWin", nil, WhenNotMatchedAny(CopyMatchingFields())); OnEvent(S1).InsertIntoNamedWindow("EmptyPropWin", CopyMatchingFields()).
Steps: send SupportBean{} -> snapshot window [1 row {}] -> value window ["EmptyPropWin"] -> faf-insert -> snapshot [2 rows] -> faf-delete -> snapshot [0] -> send S0{id:0} -> snapshot [1] -> send S1{id:0} -> snapshot [2].
Go faf-insert: FromNamedWindow("EmptyPropWin").OnDemand().InsertRows(InsertValues()); faf-delete: OnDemand().DeleteAll().

### create-schema-map (EmptyPropType ord 1 sub-round a)
EPL: `@public create map schema EmptyMapSchema as ()`; `insert into EmptyMapSchema() select null from SupportBean`; `@name('s0') select * from EmptyMapSchema` + listener.
Go: RegisterMap("EmptyMapSchema", nil); From[bean].InsertInto("EmptyMapSchema"); FromAny("EmptyMapSchema").Query(StatementName("s0")) subscribed.
Steps: send SupportBean{} -> listener record (new row {} fields) + value record "EmptyMapSchema" (emitted by listener callback on first delivery).

### create-schema-objectarray (sub-round c)
EPL: `@public create objectarray schema EmptyOASchema()`; `@public insert into EmptyOASchema select null from SupportBean`; `@name('s0') select * from EmptyOASchema` + listener AND subscriber.
Go: RegisterObjectArray("EmptyOASchema", nil); same as map + stmt.SetSubscriber emitting a "subscriber" record (New=[{"kind":"row","fields":{}}]).
Steps: send SupportBean{} -> listener record + subscriber record + value record "EmptyOASchema".

### create-schema-bean (sub-round d)
EPL: `@public create schema MyBeanWithoutProps as com.espertech.esper.regressionlib.support.bean.SupportBeanWithoutProps`; `@public insert into MyBeanWithoutProps select null from SupportBean`; `@name('s0') select * from MyBeanWithoutProps` + listener.
Go: RegisterStruct[emptyStruct]("MyBeanWithoutProps"); same as map.
Steps: send SupportBean{} -> listener record + value record "MyBeanWithoutProps".

### event-typed-column-on-merge (EventTypedColumnFromProp ord 0, .java:66-96)
EPL (verbatim, single module):
```
@public @buseventtype create schema CarEvent(carId string, tracked boolean);
create table StatusTable(carId string primary key, lastevent CarEvent);
on CarEvent(tracked=true) as ce merge StatusTable as st where ce.carId = st.carId 
  when matched 
    then update set lastevent = ce 
  when not matched 
    then insert(carId, lastevent) select ce.carId, ce 
    then insert into CarOutputStream select 'online' as status, ce as outputevent;
insert into CarTimeoutStream select e.* 
  from pattern[every e=CarEvent(tracked=true) -> (timer:interval(1 minutes) and not CarEvent(carId = e.carId, tracked=true))];
on CarTimeoutStream as cts merge StatusTable as st where cts.carId = st.carId 
  when matched 
    then delete 
    then insert into CarOutputStream select 'offline' as status, lastevent as outputevent;
@name('s0') select * from CarOutputStream
```
Go: RegisterMap CarEvent{carId string, tracked bool}; RegisterMap CarOutputStream{status string, outputevent Event+nested CarEvent}; RegisterMap CarTimeoutStream{carId string, tracked bool}; CreateTable StatusTable{PrimaryKeyColumn[string]("carId"), TableColumnOf[esper.Event]("lastevent")+nested CarEvent}.
Merge1: OnEvent(FromAny("CarEvent").Filter(Equal(Field[any,bool]("tracked"),Literal(true)))).MergeIntoTableWhen("StatusTable", []Expr{Field[any,string]("carId")}, WhenMatchedAny(SetColumn("lastevent",EventValue[esper.Event]())), WhenNotMatchedActions(ThenInsertIntoTarget(SetColumn("carId",Field[any,string]("carId")),SetColumn("lastevent",EventValue[esper.Event]())), ThenInsertInto("CarOutputStream",Alias("status",Literal("online")),Alias("outputevent",EventValue[esper.Event]())))).
Pattern: every e=CarEvent(tracked=true) -> (timer:interval(1m) and not CarEvent(carId=e.carId,tracked=true)); select e.* -> explicit Alias per prop (approved difference: no wildcard-tag helper). InsertInto("CarTimeoutStream").
Merge2: OnEvent(FromAny("CarTimeoutStream")).MergeIntoTableWhen("StatusTable", []Expr{Field[any,string]("carId")}, WhenMatchedActions(ThenInsertInto("CarOutputStream",Alias("status",Literal("offline")),Alias("outputevent",TableField[esper.Event]("lastevent"))), ThenDelete())) — insert BEFORE delete (Go ThenDelete terminates the chain; observably identical since lastevent reads the pre-delete row).
s0: FromAny("CarOutputStream").Query(StatementName("s0")) subscribed.
Steps: send CarEvent{carId:"C1",tracked:true} -> listener {status:"online",outputevent:{carId:"C1",tracked:true}} -> advance-time 60000 -> listener {status:"offline",outputevent:{carId:"C1",tracked:true}}.

### pojo-typed-column-on-merge (ord 1, .java:33-64)
Same shape; EPL verbatim:
```
create schema CarOutputStream(status string, outputevent com.espertech.esper.common.internal.support.SupportBean);
create table StatusTable(theString string primary key, lastevent com.espertech.esper.common.internal.support.SupportBean);
on SupportBean as ce merge StatusTable as st where ce.theString = st.theString 
  when matched 
    then update set lastevent = ce 
  when not matched 
    then insert select ce.theString as theString, ce as lastevent
    then insert into CarOutputStream select 'online' as status, ce as outputevent;
insert into CarTimeoutStream select e.* 
  from pattern[every e=SupportBean -> (timer:interval(1 minutes) and not SupportBean(theString = e.theString))];
on CarTimeoutStream as cts merge StatusTable as st where cts.theString = st.theString 
  when matched 
    then delete 
    then insert into CarOutputStream select 'offline' as status, lastevent as outputevent;
@name('s0') select * from CarOutputStream
```
Go: minimal SupportBean struct {theString} only (intPrimitive unobservable — asserted fields only, approved difference); outputevent/lastevent Event+nested SupportBean schema; CarTimeoutStream{theString}. Oracle renders outputevent projected to asserted fields {theString:"E1"} (map case renders full {carId,tracked}).
Steps: send SupportBean{theString:"E1",intPrimitive:1} -> listener {status:"online",outputevent:{theString:"E1"}} -> advance-time 60000 -> listener {status:"offline",outputevent:{theString:"E1"}}.

## Beans
SupportBean{theString,intPrimitive,longPrimitive}; SupportBean_S0{id,p00}; SupportBean_S1{id,p10}; CarEvent map{carId,tracked}; pojo-case SupportBean{theString} minimal.

## Normalization
- Empty rows render {"kind":"row","fields":{}} both sides.
- Event-typed columns render as nested rows (map: full props; pojo: asserted-field projection {theString}).
- value records: {"case":..,"operation":"value","statement":..,"sequence":0,"value":"<event type name>"}.
- listener/subscriber records: standard shape; Time = engine now RFC3339Nano (epoch 0 -> "1970-01-01T00:00:00Z"; after advance-time 60000 -> "1970-01-01T00:01:00Z").
- advance-time: engine.AdvanceTime(ms) — virtual time; timer:interval fires at 60000.

## Forbidden
- No Select(Alias(...)) into empty targets (phantom field leak); use zero-selection InsertInto / CopyMatchingFields.
- No zero-assignment InsertIntoNamedWindow (Build-rejected); use CopyMatchingFields.
- Do not modify the Java oracle repo.
