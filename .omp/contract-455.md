# Draft 4.455 contract — epl.variable-event-typed

Source: `/root/app/esper` @ `9e1b9f1cc9117fea4bf33ab043762c045d73839c`,
`regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/variable/EPLVariablesEventTyped.java`
(371 lines). Executions order (lines 41-48): SceneOne, SceneTwo, Config, SetProp, Invalid, CreateSchema.

Full Java contract: `agent://NextJavaContract455` (read-only scout report, frozen).
Full Go surface report: `agent://NextGoSurface455`.

## Suite-level preconfigured GLOBAL variables
From `regression-run/.../TestSuiteEPLVariable.java:122-129` — Go runner seeds via
`env.RegisterVariable` (null deployment id scope):
- `vars0_A` = event-typed 'SupportBean_S0', value SupportBean_S0(id=10)
- `vars1_A` = class SupportBean_S1, value SupportBean_S1(20)
- `varsobj1` = Object, Integer 123 (constant)
- `vars2` = 'SupportBean_S2', SupportBean_S2(30)
- `vars3` = SupportBean_S3, SupportBean_S3(40)
- `varsobj2` = Object, 'ABC' (constant)
- `myNonSerializable` = NonSerializable('abc')

Bean shapes per scout report: SupportBean{theString,intPrimitive,longPrimitive};
SupportBean_S0{id,p00..p03,value}; S1/S2{id}; S3{id, identity-equals}; A/B{id}.

## Per-execution contract (from scout; verify against source before pinning)
- ord 0 SceneOne: event-typed variable reads (property/method/nested access on
  `vars0_A`-style vars), listener assertions.
- ord 1 SceneTwo: on-set whole-event assignment + property mutation; milestones;
  `{'EX',-999,1,'S01',101L,null,null}` after on-set mutated bean; Update2
  `on SupportBean(intPrimitive=0) as sb set varbean = sb` replaces variable's event.
- ord 2 Config: preconfigured global variable reads.
- ord 3 SetProp: `set varbean.theString='X'`-form assignments emitting
  `varbean.theString`/`varbean.intPrimitive` output columns on the set statement.
- ord 4 Invalid: compile-error probes (implemented-only; assert error category,
  not byte-exact message — declared-type rendering differs).
- ord 5 CreateSchema: `create schema` + event-typed variable interactions.

## New shared-core API (already implemented by primary agent)
`esper.SetVariablePropExpr(name, property string, expression Expr) VariableAssignmentExpr`
- copy-on-write property write on struct/map/Event variable values
- null variable → silent no-op (Java semantics)
- emits `name.prop` output column with post-write property value
- sequential assignments see earlier writes via working map

## Approved differences
- ord 4: error category only, not byte-exact messages.
- ord 5: Java's consumed(variable-deployment)=[EVENTTYPE OrderEvent] dependency
  edge is unmodelable (variables carry reflect.Type, not event-type names);
  behavioral assertions only.
- No STATEMENTTYPE metadata (ON_SET) — consistent with prior contracts.
- Milestones: use the harness's established milestone/state round-trip convention.

## File ownership
- Asset worker (parity-asset-worker): `testdata/parity/epl-variables-event-typed.json`,
  `tools/java-oracle/EPLVariablesEventTypedScenarioOracle.java`,
  `tools/java-oracle/run-epl-variables-event-typed.sh`,
  `internal/app/parity/epl_variables_event_typed.go`, run.go wiring.
- Primary agent: `internal/esper/trigger.go`, `internal/esper/plan.go` (done),
  manifest, evidence, roadmap, CHANGELOG, PLANS.md.
- Worker MUST NOT touch internal/esper, manifest, evidence, or run gates.
