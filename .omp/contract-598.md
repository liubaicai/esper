# Contract: Draft 4.598 'expr enum invalid-args' (FROZEN)

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c.
Five executions, one scenario family `expr-enum-invalid-args` — invalid-compile
probes only, no sends/listeners, no virtual time, no flags on any execution.
All Java assertions go through `SupportMessageAssertUtil.tryInvalidCompile`
(exception caught, `startsWith` on the asserted text).

| case | Java source | execution / ordinal | runtime ID | static ID |
|---|---|---|---|---|
| min-invalid | ExprEnumMinMax.java | ExprEnumInvalid ord 4 | java-runtime-67deab0a6d57e2bc57fa | java-167415d6e7af87a7a111 |
| minby-invalid | ExprEnumMinMaxBy.java | ExprEnumMinMaxByInvalid ord 2 | java-runtime-5556be14d552b0228acc | java-4083b006f50db9c59884 |
| orderby-invalid | ExprEnumOrderBy.java | ExprEnumOrderByInvalid ord 4 | java-runtime-f721caa1c77ad596eca4 | java-0277b2cbf963ec07d2c0 |
| take-invalid | ExprEnumTakeAndTakeLast.java | ExprEnumTakeInvalid ord 2 | java-runtime-3c4f6374416fe5a2ee04 | java-3f7b6e1e84fe78b4a816 |
| takewhile-invalid | ExprEnumTakeWhileAndWhileLast.java | ExprEnumTakeWhileInvalid ord 2 | java-runtime-50b5bc269985fbfd9ed2 | java-0fec7ea37236a9b16857 |

## Probe list (byte-exact EPLs + Java asserted text)

Beans (TestSuiteExprEnum.configure): `SupportBean_ST0_Container` with field
`contained : List<SupportBean_ST0>`; `SupportCollection` with field
`strvals : List<String>` (Java actually uses List; Go `[]string`).

### min-invalid (ExprEnumMinMax.java:134-143)
1. `min-event-input`: `select contained.min() from SupportBean_ST0_Container`
   → Java asserts prefix
   `Failed to validate select-clause expression 'contained.min()': Invalid input for built-in enumeration method 'min' and 0-parameter footprint, expecting collection of values (typically scalar values) as input, received collection of events of type 'com.espertech.esper.regressionlib.support.bean.SupportBean_ST0'`
   Go: UNREPRESENTABLE — `EnumMin[T EnumOrdered]` cannot take `[]SupportBean_ST0`
   (generics reject at compile). Pin clause only, no Go boundary claim.
2. `min-null-selector`: `select contained.min(x => null) from SupportBean_ST0_Container`
   → clause `Null-type is not allowed`.
   Go: BOUNDARY — `EnumMinOf[st0,int64](contained, NullLiteral[int64]())` must
   reject at Build (new shared-core validation).

### minby-invalid (ExprEnumMinMaxBy.java:102-109)
1. `minby-null-selector`: `select contained.minBy(x => null) from SupportBean_ST0_Container`
   → clause `Null-type is not allowed`.
   Go: BOUNDARY — `EnumMinBy[st0,int64]` null selector rejects.

### orderby-invalid (ExprEnumOrderBy.java:172-181)
1. `orderby-event-input`: `select contained.orderBy() from SupportBean_ST0_Container`
   → Java asserts the same 0-parameter-footprint shape as min-event-input
   with method 'orderBy'.
   Go: UNREPRESENTABLE — `EnumOrderByNatural[T EnumOrdered]` generics reject.
2. `orderby-null-selector`: `select strvals.orderBy(v => null) from SupportCollection`
   → clause `Null-type is not allowed`.
   Go: BOUNDARY — `EnumOrderBy[string,int64]` null selector rejects.

### take-invalid (ExprEnumTakeAndTakeLast.java:120-126)
1. `take-null-count`: `select strvals.take(null) from SupportCollection`
   → Java asserts `Failed to validate enumeration method 'take', expected a
   non-null result for expression parameter 0 but received a null-typed
   expression`.
   Go: BOUNDARY — `EnumTakeExpr[string]` null count rejects.

### takewhile-invalid (ExprEnumTakeWhileAndWhileLast.java:147-155)
1. `takewhile-null-predicate`: `select strvals.takeWhile(x => null) from SupportCollection`
   → Java asserts the same null-parameter clause with method 'takeWhile'.
   Go: BOUNDARY — `EnumTakeWhile[string]` null predicate rejects.

## Shared-core change (primary agent, internal/esper only)
Currently `EnumTakeExpr`/`EnumTakeWhile`/`EnumMinBy`/`EnumOrderBy`/`EnumMinOf`
accept `NullLiteral[T]()` parameters silently (verified: `take(null)` builds
clean). Add Java-mirrored rejection, visible via `enumInvalidReason` inside
`validateEnumExpressionNodes` (Build-time, ErrorInvalidRule):
- expression-valued scalar params (take/take-last expr count, take-while/
  take-while-last predicate): `enumeration method %q expected a non-null
  result for expression parameter 0 but received a null-typed expression`
- selector-null (min/minBy/max/maxBy/orderBy/orderByDesc etc.): `enumeration
  method %q: Null-type is not allowed`
A parameter child whose node kind is "null" triggers it. Engine test pins
each method family's message; no other parameter kind changes.

## Step/record protocol (mirror context-init-term-remainder)
- case (4 cases + 1 … actually 5 cases, one per execution)
- build-error → compile-error record, value = pinned Java assertion clause
- undeploy-all per case
- No deploy/send/advance-time steps. 5 cases × (case + probes + undeploy-all).

## File ownership (disjoint)
- Shared-core writer (primary): internal/esper/enum_expr.go +
  internal/esper/enum_*_test.go engine pins ONLY.
- Asset worker: testdata/parity/expr-enum-invalid-args.json,
  tools/java-oracle/ExprEnumInvalidArgsScenarioOracle.java,
  tools/java-oracle/run-expr-enum-invalid-args.sh ONLY.
- Primary agent: internal/app/parity/expr_enum_invalid_args.go runner +
  run.go/run_test.go wiring, traces, evidence, manifest/roadmap/CHANGELOG/PLANS.

## Forbidden
- No edits to /root/app/esper. No formatter/lint/full-suite runs by
  subagents. No manifest/roadmap/PLANS edits by subagents.
- Runner records `expectError` verbatim as the compile-error value
  (the contract-pinned assertion clause, NOT necessarily a Java prefix —
  same convention as context-init-term-remainder).
