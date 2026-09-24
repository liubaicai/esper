# Draft 4.535 contract — infra-nwtable-on-select-aggregation

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelect.java (5 execution classes × namedWindow={true,false} = 10 executions, no flags except namedWindow).

## Executions

### 1. InfraSelectAggregation{namedWindow=T/F} — java-runtime-192de61f3c60d848e6c2 / java-runtime-99d89e9beadeae52ce8b
Infra: NW '@name('create') @public create window MyInfraSA#keepall as select theString as a, intPrimitive as b from SupportBean' | table '@name('create') @public create table MyInfraSA (a string primary key, b int primary key)' (composite PK a,b → duplicate a allowed).
On-select verbatim: "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSA".
Insert same as create projection.
Sequence: SupportBean(E1,1),(E2,2),(E3,3) → not invoked; milestone; trigger A('A1') → {sumb=6}; deploy 'on SupportBean_B delete from MyInfraSA where id = a'; B('E2') deletes E2; milestone; A('A2') → {sumb=4}; SupportBean(E4,10); A('A3') → {sumb=14}. Output type: 1 property 'sumb' Integer.

### 2. InfraSelectAggregationCorrelated{namedWindow=T/F} — java-runtime-fa42452055bf6fa1e4fd / java-runtime-485188699c51be24f2cb
Infra MyInfraSAC same composite-PK shape.
On-select verbatim: "@name('select') on SupportBean_A select sum(b) as sumb from MyInfraSAC where a = id".
Sequence: SupportBean(E1,1),(E2,2); milestone; (E3,3) → not invoked; trigger A('A1') → {sumb=null} (correlated aggregate over EMPTY match set emits one row with null sum); milestone; A('E2') → {sumb=2}; SupportBean(E2,10) (second row a=E2 allowed by composite PK); milestone; A('E2') → {sumb=12}. Output type: 1 property 'sumb' Integer.

### 3. InfraSelectAggregationGrouping{namedWindow=T/F} — java-runtime-36a62fff425f4c9b8cb6 / java-runtime-964dbe543e45c3f14f00
Infra MyInfraSAG same composite-PK shape.
Two on-selects in one module: "@name('select') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a order by a desc" and "@name('selectTwo') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a having sum(b) > 5 order by a desc"; insert '@name('insert') insert into MyInfraSAG select theString as a, intPrimitive as b from SupportBean'.
Sequence: trigger A('A1') on empty infra → NEITHER listener invoked (group-by over empty set emits zero rows — contrast with ungrouped #2 which emits null row); SupportBean(E1,1),(E2,2); milestone; (E1,5) → not invoked; A('A1') → select newData [{E2,2},{E1,6}] (a desc), selectTwo [{E1,6}] (having sum>5); milestone; SupportBean(E4,-1),(E2,10),(E1,100) → not invoked; milestone; A('A2') → select [{E4,-1},{E2,12},{E1,106}], selectTwo [{E1,106}]; deploy 'on SupportBean_B delete from MyInfraSAG where id = a'; B('E2') removes BOTH E2 rows (2 and 10); A('A3') → select [{E4,-1},{E1,106}], selectTwo [{E1,106}]. Output type: 2 props, a:String, sumb:Integer.

### 4. InfraSelectAggregationHavingStreamWildcard{namedWindow=T/F} — java-runtime-a83c289aeff7f22c5d10 / java-runtime-113be8d12970feaf2fd0
Infra: NW '@public create window MyInfraSHS#keepall as (a string, b int)' | table '@public create table MyInfraSHS as (a string primary key, b int primary key)'; insert 'insert into MyInfraSHS select theString as a, intPrimitive as b from SupportBean'.
On-select verbatim: "@name('select') on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc where id = a group by a having sum(b) = 20".
Sequence: SupportBean(E1,16),(E2,2); milestone; SupportBean(E1,4); trigger A('E1') → newData length 2, both events mwcwin.a='E1' (stream-wildcard fragment 'mwcwin' emits per-row events for the group, not one row per group). Asserts statement SPI isStatelessSelect()==false.

### 5. InfraOnSelectMultikeyWArray{namedWindow=T/F} — java-runtime-d10a8674ccc6ecc5c70d / java-runtime-5b8dddb5496cfbefa7d7
Infra MyInfraPC: NW '@name('create') @public create window MyInfraPC#keepall as (id string, array int[], value int)' | table '@name('create') @public create table MyInfraPC(id string primary key, array int[], value int)'.
On-select verbatim: "@name('s0') on SupportBean select array, sum(value) as thesum from MyInfraPC group by array".
Rows loaded via FAF: env.compileExecuteFAFNoResult("insert into MyInfraPC values('E1', {1, 2}, 10)") and ('E2',{1,2},11); milestone; send SupportBean trigger → newData {thesum=21} (int[] {1,2} is a single array group key — multikey semantics); FAF inserts ('E3',{1,2},21), ('E4',{1},22); milestone; trigger → lastNew any-order [{thesum=42},{thesum=22}].

## Shared-core gaps (GoSurface535)
1. Ungrouped aggregation over on-select snapshot (sum(b) with/without where; empty-match → null row vs no row).
2. Table-side grouped on-select (SelectFromTableGroupBy/Rollup + having) — no counterpart to NW pair.
3. HAVING over on-select (having sum(b)>5, having sum(b)=20, having count>0).
4. Stream-wildcard/fragment projection (mwc.* as mwcwin → per-row fragment events).
5. Multikey/array group keys (group by array over int[] column, content-equality).
6. FAF insert with array literals (insert into MyInfraPC values('E1',{1,2},10)).

## Deliverables (asset worker)
- testdata/parity/infra-nwtable-on-select-aggregation.json (10 cases; metadata pins; verbatim EPL)
- internal/app/parity/infra_nwtable_on_select_aggregation.go (runner)
- tools/java-oracle/InfraNWTableOnSelectAggregationScenarioOracle.java + run-infra-nwtable-on-select-aggregation.sh
- run.go mode registration + run_test.go wiring

## Forbidden
Subagent: no formatter/lint/tests, no manifest/roadmap/CHANGELOG/PLANS edits, no commit/push, no internal/esper edits, no hand-authored traces/evidence.
