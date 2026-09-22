# Contract — Draft 4.502 'expr-enum-sumof-remainder'

## Oracle
- Java repo: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (verified).
- Source: regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/enummethod/ExprEnumSumOf.java
- UDFs extractNum/extractBigDecimal: suite-wide single-row functions (TestSuiteExprEnum.java:177-178 -> ExprEnumMinMax.MyService): extractNum = Integer.parseInt(arg.substring(1)); extractBigDecimal = new BigDecimal(arg.substring(1)).

## Executions (4, all flags-free, no virtual time, listener-only)
1. ord 1 ExprEnumSumEventsPlus | java-runtime-8497175e9fc13501285c | static java-7443fe2db668140640df
   EPL: select beans.sumOf(x => intBoxed) c0, beans.sumOf( (x, i) => intBoxed + i*10) c1, beans.sumOf( (x, i, s) => intBoxed + i*10 + s*100) c2, beans.sumOf( (x, i) => case when i = 1 then null else 1 end) c3 from SupportBean_Container
   Inputs: SupportBean_Container(beans=null), (empty), [SB(E1,intBoxed=10)], [SB(10),SB(11)]
   Expects c0..c3 Integer: null-row->(null,null,null,null); empty->same; 1 bean->(10,10,110,1); 2 beans->(21,31,431,1)
2. ord 3 ExprEnumSumScalarStringValue | java-runtime-93ec466957ff19bc38a6 | static java-ce9a9b8124ed9b9cda09
   EPL: strvals.sumOf(v => extractNum(v)) c0, strvals.sumOf(v => extractBigDecimal(v)) c1, strvals.sumOf( (v, i) => extractNum(v) + i*10) c2, strvals.sumOf( (v, i, s) => extractNum(v) + i*10 + s*100) c3 from SupportCollection
   Inputs: makeString("E2,E1,E5,E4"), ("E1"), (null), ("")
   Expects: (12, BigDecimal(12), 72, 1672), (1, BD(1), 1, 101), all-null, all-null
3. ord 4 ExprEnumSumInvalid | java-runtime-bbe116cdf8ad7e13172f | static java-3bdebdfafa650da60077
   2 tryInvalidCompile probes:
   a) 'select beans.sumof() from SupportBean_Container' -> "Invalid input for built-in enumeration method 'sumof' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type '<SupportBean FQN>'"
   b) 'select strvals.sumOf(v => null) from SupportCollection' -> "expected a non-null result for expression parameter 0 but received a null-typed expression"
   Pin message PREFIXES only (JVM FQNs diverge).
4. ord 5 ExprEnumSumArray | java-runtime-b0455a5d1e4b447f34b1 | static java-2c99e8eabfc5cdb0b018
   EPL: {1d, 2d}.sumOf() c0, {BigInteger.valueOf(1), BigInteger.valueOf(2)}.sumOf() c1, {1L, 2L}.sumOf() c2, {1L, 2L, null}.sumOf() c3 from SupportBean
   Input: one new SupportBean()
   Expects: (3d, BigInteger 3, 3L, 3L)

## Semantics to pin
- null collection AND empty collection both yield null (not 0).
- sumOf skips null elements and null lambda results.
- Result type follows input: Integer->Integer, Long->Long, Double->Double, BigDecimal->BigDecimal, BigInteger->BigInteger.
- BigDecimal scale fidelity: SumOf expects BigDecimal(int) scale 0.
- Lambda footprints: i = 0-based index, s = collection size.
- SupportEvalRunner also runs a reflective non-compile pass; Go only needs the listener contract.

## Go surface (from GoSurface502)
- internal/esper/enum_expr.go: EnumSum/EnumSumOf with EnumElement/EnumIndex/EnumSize lambda context; EnumArrayOf for constant collections; Func1/Func2 UDFs; big-number collections exist.
- Predicted zero engine change. If a gap appears, REPORT it — do not work around.
- Build-error probes: precedent = context_key_segmented_invalid.go compile-error records (prefix-pin).

## File ownership
- Assets502 (parity-asset-worker) owns ONLY:
  - internal/app/parity/expr_enum_sumof_remainder.go (new)
  - testdata/parity/expr-enum-sumof-remainder.json (new)
  - tools/java-oracle/ExprEnumSumOfRemainderScenarioOracle.java (new)
  - tools/java-oracle/run-expr-enum-sumof-remainder.sh (new)
- Primary agent owns: run.go/run_test.go wiring, traces, evidence, manifest, roadmap, CHANGELOG, PLANS.md, commit.
- Worker MUST NOT: edit shared internal/esper, run tests/formatters, generate traces/evidence, update manifest/roadmap/CHANGELOG, commit/push.

## Scenario shape (suggested)
- id: expr-enum-sumof-remainder; 4 cases matching ord order; steps: case marker, deploy s0, sends per input rows, undeploy-all; build-error steps for the 2 invalid probes.
- Event types needed: SupportBean_Container (beans collection of SupportBean with intBoxed), SupportCollection (strvals), SupportBean, SupportBean_ST0_Container not needed.
- Follow existing scenario JSON conventions (version esper-parity/v1, javaCommit/javaSource/javaRuntimes/javaNames/javaStaticIds/javaFlags/cases/steps).
