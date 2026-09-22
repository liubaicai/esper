# Draft 4.496 — resultset-aggregate-invalid-closure

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c
4 invalid-form executions across 4 files, all compile-time validation (11 probes).

## Executions

- `ext-invalid` ResultSetAggregateExtInvalid = java-runtime-98f0ac5299780e2d6656
  (single-execution file, closes it). `select rate(10) from SupportBean` with
  extendedAggregation=false -> unknown function. UNREPRESENTABLE: Go has no
  extended-aggregation toggle; Rate compiles. Pin as unrepresentable.
- `aggregate-invalid` ResultSetAggregateInvalid (SortedMinMaxBy ord 7) =
  java-runtime-09530eec55a704b01c96. 2 probes:
  1. `maxBy(p00||p10)` cross-stream -> NEW: same-stream criteria check
  2. `sorted(p00)` no window -> NEW: sorted window requirement (exempt
     into-table/create-table)
- `window-invalid` ResultSetAggregateWindowInvalid (MethodWindow ord 4) =
  java-runtime-fbbdc48ea2da6d4bc426. Needs @public table MyTable(windowcol
  window(*) @type('SupportBean')). 2 probes:
  1. `windowcol.first(id)` -> NEW: arg validated against contained type
     (SupportBean has no id), not outer stream
  2. `windowcol.listReference(intPrimitive)` -> NEW: arity check (zero params)
- `filter-named-param-invalid` ResultSetAggregateFilterNamedParamInvalid
  (FilterNamedParameter ord 20) = java-runtime-50935b7efcc1fdced314. 5 probes:
  1. multi-value filter -> UNREPRESENTABLE (no tuple expressions)
  2. multiple filters -> UNREPRESENTABLE (typed API)
  3. non-bool filter -> UNREPRESENTABLE (typed API)
  4. create-table filter -> NEW: CreateTable rejects TableAggDecl.Filter/GroupBy
  5. correlated subquery filter -> COVERED (plan.go:3368)

## File ownership

- Shared core (primary): internal/esper/{expr,plan,state,runtime}.go,
  facade_generated.go, internal/esper/*_test.go
- Asset worker: internal/app/parity/resultset_aggregate_invalid_closure.go,
  run.go + run_test.go wiring,
  testdata/parity/resultset-aggregate-invalid-closure.json,
  tools/java-oracle/ResultSetAggregateInvalidClosureScenarioOracle.java,
  tools/java-oracle/run-resultset-aggregate-invalid-closure.sh
- Primary generates traces + evidence after integration.

## Conventions

- Runner mirrors resultset_output_limit_crontab_when_closure.go.
- build-error probes: Go call asserts rejection + contains-guard, records
  pinned Java prefix. unrepresentable: pinned EPL + prefix, no Go claim.
- window-invalid probes carry path (table pre-deployed); others path-less.
- Java error text uses normalized expression (lowercase, no spaces, truncated
  at 35 chars + '...(N chars)'). Go messages differ -> contains-guard.
