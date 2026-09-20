# Contract — Draft 4.481 `infra-nwtable-on-merge-insert-other-stream`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 8-19: InfraInsertOtherStream{namedWindow,rep} — the
event-representation matrix, 12 executions = namedWindow∈{true,false} ×
EventRepresentationChoice∈{OBJECTARRAY,MAP,AVRO,JSON,JSONCLASSPROVIDED,DEFAULT}
(namedWindow=true added first per rep). One parameterized family sharing one harness.
Asset-only (Go scout confirmed all six representations have registration, send, nw/table,
and merge paths; TestTriggerMergeInsertOtherStreamRepresentationMatrix already exercises
all 12 cells with a simplified topology).

Runtime IDs (ord → id):
- 8  nw=true  OBJECTARRAY        java-runtime-c91b3df37512cfac454f
- 9  nw=false OBJECTARRAY        java-runtime-b54b14998ff59e65f2fc
- 10 nw=true  MAP                java-runtime-945ba0790a1cfa2b1033
- 11 nw=false MAP                java-runtime-fef4b3a46f9b7da65a72
- 12 nw=true  AVRO               java-runtime-d8cb24fc9e29421236e2
- 13 nw=false AVRO               java-runtime-e22957dc02c3ac4b8bb7
- 14 nw=true  JSON               java-runtime-284497e8a9729bb76ac2
- 15 nw=false JSON               java-runtime-f6718d2a96a3e3668239
- 16 nw=true  JSONCLASSPROVIDED  java-runtime-51773f11823a58156447
- 17 nw=false JSONCLASSPROVIDED  java-runtime-b891f1f54423ccb8a628
- 18 nw=true  DEFAULT            java-runtime-dd95faed8e31c95bec9f
- 19 nw=false DEFAULT            java-runtime-6e68e972f23cef8dde85
Static ID java-8304a4459a4ea865bf1f (OBSERVEROPS heuristic); runtime flags [].

## Verbatim EPL (single compileDeploy, RegressionPath)
```
<REP_ANN(MyLocalJsonProvidedMyEvent)> @public @buseventtype @public create schema MyEvent as (name string, value double);
namedWindow=true:  <REP_ANN(MyLocalJsonProvidedMyEvent)> @public create window MyInfraIOS#unique(name) as MyEvent;
namedWindow=false: @public create table MyInfraIOS (name string primary key, value double primary key);   [no rep annotation]
insert into MyInfraIOS select * from MyEvent;
<REP_ANN(MyLocalJsonProvidedInputEvent)> create schema InputEvent as (col1 string, col2 double);   [dead declaration]
on MyEvent as eme merge MyInfraIOS as MyInfraIOS where MyInfraIOS.name = eme.name
  when matched then insert into OtherStreamOne select eme.name as event_name, MyInfraIOS.value as status
  when not matched then insert into OtherStreamOne select eme.name as event_name, 0d as status;
@name('s0') select * from OtherStreamOne;
```
REP_ANN prefixes: OBJECTARRAY=`@EventRepresentation('objectarray')`,
MAP=`@EventRepresentation('map')`, AVRO=`@EventRepresentation('avro')`,
JSON=`@EventRepresentation('json')`,
JSONCLASSPROVIDED=`@JsonSchema(className='...MyLocalJsonProvidedMyEvent') @EventRepresentation('json')`
(InputEvent uses MyLocalJsonProvidedInputEvent), DEFAULT=`` (empty → leading space).

## Steps + expected (assertPropsNew 's0' on [event_name,status], new/istream only)
1. send MyEvent(name='name1', value=10d) → s0 new {name1, nw?0d:10d}
2. namedWindow only: send ('name1',11d) → {name1,10d}; send ('name1',12d) → {name1,11d}
3. undeployAll

Semantics: table — insert-into routes the row before the on-merge trigger evaluates, so
the first event MATCHES (status=10d). Named window — on-merge fires BEFORE the insert-into
route, so first event is NOT MATCHED (status=0d); the row is then inserted and #unique(name)
replaces on each subsequent same-key event, so each new event matches the PREVIOUS value.
Merge never mutates the target.

Send payloads per rep: OBJECTARRAY → object array {name,value}; MAP/DEFAULT → map;
AVRO → GenericData.Record over preconfigured schema; JSON/JSONCLASSPROVIDED → JSON object.

## Harness
One scenario `infra-nwtable-on-merge-insertstream`, twelve cases
(insertstream-{nw,table}-{objectarray,map,avro,json,jsonclassprovided,default}), fresh
engine per case with WithRuntimeURI(runtimeID). Ops: deploy/deployed/send/snapshot/
undeploy-all. Listener on 's0' only. Per-representation payload decoders follow the
infra_namedwindow_processing_order.go / infra_nwtable_faf_join.go precedent.

## Go surface (asset-only)
RegisterMap/RegisterObjectArray/RegisterAvro/RegisterJSON/RegisterJSONFor/RegisterStruct;
MergeIntoNamedWindowWhen/MergeIntoTableWhen + WhenMatchedActions/WhenNotMatchedActions +
ThenInsertInto side-stream; composite-PK table mapping (name+value both primary key).
Deploy-order swap for nw feeder fidelity is asset-level.

## Allowed files (asset lane)
- tools/java-oracle/InfraNWTableOnMergeInsertStreamScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-insertstream.sh (new)
- testdata/parity/infra-nwtable-on-merge-insertstream.json (new)
- internal/app/parity/infra_nwtable_on_merge_insertstream.go (new)
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/**, internal/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.
- Do NOT modify the checked-in infra-nwtable-on-merge{,-nested}.* assets or runners.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-insertstream-diff` zero-diff;
run_test.go pins; make check; parity review; commit/push.
