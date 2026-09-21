# Contract — Draft 4.492 `infra-table-invalid`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraTableInvalid.java — ALL 4 executions (ords 0-3). Pure INVALIDITY file: zero events sent, zero listeners, zero virtual time. Every probe is a compile-time rejection asserted by message prefix/contains/skip.

Runtime IDs:
- ord 0 `java-runtime-70e525638c5fbaf37752` InfraInvalidAggMatchSingleFunc, flags [INVALIDITY]
- ord 1 `java-runtime-7c97277eedfa76b7e0cf` InfraInvalidAggMatchMultiFunc, flags [INVALIDITY]
- ord 2 `java-runtime-fd2a7de8e5606de63b80` InfraInvalidAnnotations, flags [INVALIDITY]
- ord 3 `java-runtime-b9b6435f81135ce15040` InfraInvalid, flags [INVALIDITY]

Static IDs:
- ord 0 `java-dc8058ccbd099b1d1361`
- ord 1 `java-ecf59e6ef7727ab987fa`
- ord 2 `java-53cb0410fe21932803bc`
- ord 3 `java-c44ff6d4e86c09522b35`

## Probe inventory
- ord 0: 43 tryInvalidAggMatch probes (declared vs provided agg signature mismatches: param type, distinct, filter, ignore-nulls, min/max direction, nth size, rate interval, ever direction, plugin names). 12 pinned startsWith prefixes; 31 null-message → contains `Incompatible aggregation function for table`.
- ord 1: 6 tryInvalidAggMatch probes (all unbound `#time(1000)`): window(*) @type event-type mismatch, sorted() sort-expr rejection, se1() plugin name mismatch.
- ord 2: 5 tryInvalidCompile probes (annotation syntax on table columns: unknown annotation, missing value, duplicate annotation, non-string value, unknown event type).
- ord 3: 50 tryInvalidCompile probes across declaration (PK-on-expression, PK-on-event-type, name collisions), into-table (group-by count/type, context visibility, write-only, unidirectional join, requires-aggregation), consumption (keyed-access count/type, unknown column, unknown function), misc (views on tables, unidirectional, retain, on-action, match-recognize, update-istream, context declaration, pattern atoms, schema/table collision).

## Shared-core changes (primary agent)
- `TableAggDecl` extended with signature-detail fields (ParamType, Distinct, Filter, IgnoreNulls, NthSize, RateInterval, EventType); `WithTableAggDecl` option.
- `intoTableAggInfo` unwraps `aggregate-filter`/`aggregate-distinct` wrappers; extracts param type, nth size, rate interval, event type.
- `validateIntoTableCompatible` extended with per-detail checks (param type, distinct, filter, ignore-nulls, nth size, rate interval, event type).
- `validateIntoTable` extended: group-by count/type vs PK, requires-aggregation, unidirectional join (Java precedence order). Write-only table-access and retain are unrepresentable in the fluent API (no bare table expression / no retain flag), so no guard exists.
- `NewTableDefinition`/`RegisterTableInModule`: PK-on-expression, PK-on-event-type, name collisions vs variables/schemas/event-types (named-window collision is Java-legal and not checked).
- Table-misuse guards: views on tables, unidirectional, on-action, match-recognize, update-istream, context declaration, pattern atoms (retain unrepresentable).
- Consumption: keyed-access count/type, unknown column, unknown table function.

## Allowed files (asset worker)
- tools/java-oracle/InfraTableInvalidScenarioOracle.java
- tools/java-oracle/run-infra-table-invalid.sh
- testdata/parity/infra-table-invalid.json
- internal/app/parity/infra_table_invalid.go
- internal/app/parity/run.go (mode wiring only)

## Forbidden
- internal/esper/** (shared core — primary agent owns)
- testdata/compat/**, manifest, roadmap, CHANGELOG, PLANS.md
- generated traces/evidence (primary agent generates)
- commits/pushes

## Validation (primary agent)
Java trace via run script; `-mode infra-table-invalid-diff` zero-diff; run_test.go pins; make check; parity review; commit/push.
