# Draft 4.470 contract — epl-other-select-expr-stream-selector-remainder

Frozen from read-only scouts `NextJavaContract470` (java-oracle-scout) and
`NextGoSurface470` (scout), both completed during Draft 4.469's review window.
Oracle pinned at `/root/app/esper` commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c` (verified by scout).

## Scope: EPLOtherSelectExprStreamSelector.java unreferenced ords

| ord | execution | runtime ID | static ID | verdict |
|-----|-----------|------------|-----------|---------|
| 0 | EPLOtherInvalidSelectWildcardProperty | java-runtime-0fe17586d141b87f3cff | java-53aaecf8c98ebdd1c538 | intentionally-different (compile-only; NO Go rejection boundary — named Transpose is accepted; record unrepresentable) |
| 1 | EPLOtherInsertTransposeNestedProperty | java-runtime-784378ea15f390e587b2 | java-5dcc72bb024241a1c003 | DV-able, expressible today (Transpose(Field nested) → struct target) |
| 2 | EPLOtherInsertFromPattern | java-runtime-68b49a5d4a1630d85f75 | java-587f1eac414f59c3f257 | DV-able, needs new surface (pattern-source transpose of tagged-event underlying) |
| 3 | EPLOtherObjectModelJoinAlias | java-runtime-ee69f9fe048d908e8a1b | java-2da944929b02a2eb610c | intentionally-different (SODA/eplToModel JVM-only; runtime slice already DV'd by ords 8/9) |
| 16 | EPLOtherInvalidSelect | java-runtime-fb00a9a05a09d998736d | java-0b8c93bdcb738bd64543 | intentionally-different (4 compile-only probes; Go boundaries exist for dup-alias and multi-transpose, partial for unknown stream selector) |

Suite static id java-045df7417e4eba449a41; flags [] on all five ords.
Regression env disables the internal timer — timer:within(30 sec) never fires;
matches are immediate at t=0.

## Java observable contract (byte-exact)

- ord 0: `select simpleProperty.* as a from SupportBeanComplexProps as s0` →
  prefix `The property wildcard syntax must be used without column name`.
- ord 1: EPL1 `@name('l1') @public insert into StreamA select nested.* from
  SupportBeanComplexProps as s0` (listener; underlying ==
  SupportBeanSpecialGetterNested); EPL2 `@name('l2') select nestedValue from
  StreamA` (listener; nestedValue String). One send
  SupportBeanComplexProps.makeDefaultBean() → both listeners new[nestedValue]=
  "nestedValue". undeployAll twice (second no-op → one undeploy-all step).
- ord 2: l1 `insert into streamA select a.* from pattern [every a=SupportBean]`
  (listener fires but unasserted — trace must still record both deliveries);
  l2 `insert into streamA select a.* from pattern [every a=SupportBean where
  timer:within(30 sec)]` (listener; underlying == SupportBean); sends
  SupportBean("E1",10), ("E2",10) → l2 new event underlying assertSame sent
  bean; l3 deployed AFTER sends `insert into streamB select a.*, 'abc' as abc
  from pattern [every a=SupportBean where timer:within(30 sec)]` — no listener,
  zero deliveries, underlying Pair, abc/theString String. One undeployAll.
  streamA (lowercase) ≠ StreamA of ord 1 — separate fresh runtimes per case.
- ord 3: SODA model; toEPL byte-equals `@name('s0') select s0.*, s1.* as
  s1stream, theString as sym from SupportBean#keepall as s0,
  SupportMarketDataBean#keepall as s1`; eplToModel round-trip equal. Runtime:
  send SupportBean("E1") → listener NOT invoked (cumulative); send
  SupportMarketDataBean("E1",0d,0L,"") → one new event, s1stream assertSame
  market bean. intentionally-different; runtime contract already covered.
- ord 16 probes (prefix match):
  (a) `select theString.* as theString, theString from SupportBean#length(3) as
      theString` → `Column name 'theString' appears more then once in select
      clause` — Go boundary EXISTS (plan.go:4606 duplicate alias).
  (b) `select s1.* as abc from SupportBean#length(3) as s0` → `Stream selector
      's1.*' does not match any stream name in the from clause [` (trailing '[')
      — partial boundary only (positional join API).
  (c) `select s0.* as abc, s0.* as abc from SupportBean#length(3) as s0` →
      `Column name 'abc' appears more then once in select clause` — boundary
      EXISTS (plan.go:4554).
  (d) `select s0.*, s1.* from SupportBean#keepall as s0, SupportBean#keepall as
      s1` → `A column name must be supplied for all but one stream if multiple
      streams are selected via the stream.* notation` — boundary EXISTS
      (plan.go:4555 / 1690 identical sentence).

## Go work list

- Ord 1: zero engine work. RegisterStruct complexProps (nested *complexNested),
  RegisterStruct complexNested as "StreamA", Transpose(Field nested) route,
  FromAny consumer; `types` step op for event-type assertions.
- Ord 2: NEW SURFACE — pattern-source transpose of tagged-event underlying.
  Touch points: plan.go validatePattern (6627) + resultSchema pattern branch
  (4507) unnamed-transpose allowance; validateRoute transpose block (1681);
  runtime.go transposeRouteInfo (22824) + pattern-match transpose projection +
  Event unwrap. streamB pair shape: Event→map flattening or explicit TagField
  enumeration (types record must pin a defensible shape — Java asserts Pair).
  LATENT BUG to close: named Transpose in a pattern select currently builds but
  misroutes — support or explicitly reject.
- Ords 0/16: build-error steps → compile-error records pinning Java prefixes;
  ord 0 recorded as unrepresentable (no boundary claim).
- Ord 3: intentionally-different case; SODA noted as JVM-only compile path.

## File ownership

- Primary agent (shared core): internal/esper/plan.go, runtime.go, expr.go
  (if typed pattern-underlying expression wins), pattern.go, engine parity
  tests, run.go, run_test.go, manifest, PLANS.md, CHANGELOG, roadmap.
- Parity-asset writer (disjoint, new files only):
  testdata/parity/epl-other-select-expr-stream-selector-remainder.json,
  tools/java-oracle/<New>ScenarioOracle.java + run script,
  internal/app/parity/<new_runner>.go.

## Risks (ranked)

1. Pattern-transpose route absent — three coordinated touch points; Event
   payload typing decision affects Build-time coercion fidelity.
2. streamB Pair shape — Event→map flatten vs TagField enumeration.
3. Ord 0 has NO Go rejection boundary — record unrepresentable, never claim.
4. Ord 16(b) partial boundary only.
5. Latent misroute of named Transpose in pattern selects must be closed.
