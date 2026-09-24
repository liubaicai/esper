# Draft 4.536 contract — epl-other-create-expression (FROZEN)

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c
File: regression-lib/.../suite/epl/other/EPLOtherCreateExpression.java
Scouts: JavaContract536 (contract), GoSurface536 (surface). Full reports in
agent history; key facts below.

## Executions (5, all unreferenced)

| ord | name | runtimeId | staticId | flags |
|-----|------|-----------|----------|-------|
| 0 | EPLOtherInvalid | java-runtime-e3078c54652f7f72cd73 | java-683caba87edd0bf7ecea | [] |
| 1 | EPLOtherParseSpecialAndMixedExprAndScript | java-runtime-2e66cbcd61d52f258cb4 | java-dd3e693c0a24401dc305 | [] |
| 2 | EPLOtherExprAndScriptLifecycleAndFilter | java-runtime-c0c5df502bf2651e6acf | java-e292f8a1c482bb84b1e4 | [OBSERVEROPS] |
| 3 | EPLOtherScriptUse | java-runtime-c2595ccced4ddaea854d | java-ae1621ba24392298ec2f | [] |
| 4 | EPLOtherExpressionUse | java-runtime-61a0f3f3916ac25443d7 | java-089123ca8c5a8e4a85b6 | [] |

Static manifest lists OBSERVEROPS on all 5 (stale class-level); runtime
inventory is authoritative (only ord 2). No virtual time, no threading, no FAF.

## Representable coverage (Go typed API)

- ord 0: duplicate declared-expression rejection. Java: `Expression 'E1' has
  already been declared`. Go DefineExpression rejects duplicates → compile-error
  record carries the pinned Java message (convention: infra_nwtable_event_type
  buildError). Script-duplicate half (name+arity) unrepresentable.
- ord 1: declared-expression halves only. Part A: `myexpr{sb=>'--'||theString
  ||'--'}` → select myexpr(sb) → c1='--E1--' on SupportBean(E1,1). Part B:
  `scalarfilter{s=>strvals.where(y=>y!='E1')}` chained `.where(x=>x!='E2')`
  over SupportCollection.makeString("E1,E2,E3,E4") → val1=[E3,E4], statement
  stateless. JS parts (myscript, callIt bean factory) unrepresentable.
- ord 2: declared-expression pass only. MyFilter{sb=>intPrimitive=1} + select *
  from SupportBean(MyFilter(sb)): E1/0 silent, E2/1 fires; undeployAll;
  redeploy =2 variant; E3/0 silent, E4/1 silent (rebind proof), E4/2 fires;
  old listener never fires post-undeploy. Script pass unrepresentable.
- ord 3: intentionally-different — script overloading by arity (Go registry
  rejects same-name scripts), int coercion of script result, SODA round-trip.
  No Go surface.
- ord 4: TwoPi{Math.PI*2} + factorPi{sb=>Math.PI*intPrimitive}; select
  TwoPi(), (select TwoPi() from SupportBean_S0#lastevent), factorPi(sb) →
  [2π,2π,3π] on SupportBean(E1,3) after S0(10). Part B: statement-local
  `expression TwoPi{Math.PI*10}` shadows @public → c0=10π (needs new
  WithExpression shared-core API — primary agent). Part C: JoinMultiplication
  {(s1,s2)=>s1.intPrimitive*s2.id} over two #lastevent streams — deploy only.
  Part D (×2: namedWindow true/false): myexpr{(select intPrimitive from
  MyInfra)} defined BEFORE MyInfra exists (deferred binding); then window or
  table + insert-into; select myexpr() from SupportBean_S0 → c0=100,
  stateful. SODA round-trips unrepresentable.

## Shared-core additions (primary agent)

- `WithExpression(name, expr)` QueryOption: statement-local declared
  expression shadowing env registrations (ord 4 part B). Implementation:
  exprNode.expressionOverride set by a Build-time walk; validation + eval
  prefer the override.
- Verify deferred subquery binding works (DefineExpression with SubqueryValue
  body over not-yet-existing MyInfra, resolved at consumer Build).

## Assets (parity-asset-worker)

- testdata/parity/epl-other-create-expression.json — cases: invalid,
  parse-mixed-expr, lifecycle-filter, script-use (intentionally-different
  marker case or omitted per convention), expression-use-nw,
  expression-use-table. Byte-exact EPL pins in deploy steps.
- tools/java-oracle/EPLOtherCreateExpressionScenarioOracle.java + run.sh —
  replay only the representable steps; oracle asserts the same records.
- internal/app/parity/epl_other_create_expression.go + run.go/run_test.go
  wiring (6-test family).
- Records: deployed markers, listener rows (new only), compile-error records
  with pinned Java messages, stateless/stateful flags where Java asserts
  them (ord1-B stateless=true, ord4-D stateful=false→isStatelessSelect false).

## Forbidden

- No JS/MVEL evaluation, no script overloads, no SODA/EPL text round-trip —
  those stay unrepresentable/intentionally-different.
- Do not modify the Java oracle.
