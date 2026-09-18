# Draft 4.456 contract — context-variables

Source: `/root/app/esper` @ `9e1b9f1cc9117fea4bf33ab043762c045d73839c`,
`regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextVariables.java`.
5 executions, shared static ID `java-2443c804eb31da7902e7`.

Full Java contract: `agent://NextJavaContract456` (frozen).
Full Go surface map: `agent://NextGoSurface456`.

## Runtime IDs (per execution)
- ord 0 ContextVariablesSegmentedByKey: `java-runtime-700973f399356686a11e`
- ord 1 ContextVariablesOverlapping: `java-runtime-821c37bbc3256af33468`
- ord 2 ContextVariablesIterateAndListen: `java-runtime-ea85235554e99c41e863`
- ord 3 ContextVariablesGetSetAPI: `java-runtime-4d98fe6487ed39e7d1de` (RUNTIMEOPS)
- ord 4 ContextVariablesInvalid: `java-runtime-c7b860b3ff30ed78f3f5` (no INVALIDITY flag declared)

## Key semantics (from frozen contract)
- ord 0: segmented key context; uncorrelated `on set` fires only for the
  partition the event keys into, allocating it if absent; same event can
  allocate AND set.
- ord 1: overlapping initiated/terminated; correlated set via
  `context.s0.p00`; UNCORRELATED set (`intPrimitive < 0`) writes ALL live
  partitions; terminated partitions reset to initial on re-initiation.
  Module tail: deploy+undeploy a module with `integer` keyword +
  `distinct(theString)` initiator (no events/assertions).
- ord 2: `terminated after 24 hours`; listener on `create variable`
  statement fires IR pair per set (new=assigned, old=PREVIOUS incl.
  initial 5) only for updated partition; iterators on 'upd'/'var' yield
  one row per live partition (upd ordered by partition id, var any-order).
- ord 3: partition-scoped get/set API by agentInstanceId (0,1 in
  allocation order); global var via these APIs throws
  VariableNotFoundException "is a global variable and not
  context-partitioned".
- ord 4: 8 compile-failure probes, prefix-match messages (context not
  found; wrong-context variable; out-of-context variable in select,
  expr-window, limit, offset, output-every, reclaim hint).

## Go API mapping (from scout)
- `CreateKeyContextByStreams` (ord 0); `CreateOverlappingInitiatedTerminatedContext`
  (ord 1); `CreateOverlappingPatternTerminatedContext` + `TimerInterval`
  end (ords 2/3 — Java `initiated by` is overlapping-by-default).
- `env.RegisterContextVariable(ctx, name, initial)` for `context X create
  variable` (env-scoped; deployable-statement gap is the established
  approved difference — registration fixture + silent statement).
- `context.s0.p00` → `Property[string](ContextInitiatingEvent(), "p00")`.
- `on ... set` → `OnEvent(...).SetVariables(...).Query(WithContext("MyCtx"))`.
- 'var' listener IR pair → `AddVariableChangeListener` (VariableChangeEvent
  carries Old/New/PartitionID/PartitionKey).
- 'var'/'upd' iterators → `Statement.Snapshot` / `ContextVariableStates`.
- ord 3 API → `SetContextVariablesByID` (strict active-partition),
  `ContextVariableStates` + `SelectContextPartitionIDs`.
- Milestones are documented no-ops.

## Shared-core change already applied by primary agent
`visitQueryExpressions` now visits `query.output.CountExpr` and
`IntervalExpr` so context variables inside output-rate expressions hit
`validateContextVariableScope` (ord-4 probe 7).

## Approved differences
- Error wording: pin Go error category/wording per invalidity policy;
  Java prefix-messages asserted inside the oracle only.
- `create variable` statement is registration fixture (no deployable
  statement type) — established precedent.
- Hint-parameter probe (ord 4 #8, `reclaim_group_aged=myctxone_int`): Go
  hints are string-typed, no expression surface — unrepresentable; oracle
  asserts it internally, scenario may pin a Go-side equivalent or mark it.
- ord 1 module tail (deploy+undeploy, no assertions): registration fixture.

## File ownership
- Asset worker: `testdata/parity/context-variables.json`,
  `tools/java-oracle/ContextVariablesScenarioOracle.java`,
  `tools/java-oracle/run-context-variables.sh`,
  `internal/app/parity/context_variables.go`, run.go wiring.
- Primary agent: `internal/esper/plan.go` (done), manifest, evidence,
  roadmap, CHANGELOG, PLANS.md.
- Worker MUST NOT touch internal/esper, manifest, evidence, or run gates.
