# Draft 4.510 contract — rowrecog-multikey-warray

Frozen 2026-09-23. Java oracle pinned at commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c` (`/root/app/esper`).

## Scope

Two executions, one capability subdomain (match_recognize multi-key
partition family), both in
`regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/rowrecog/RowRecogMultikeyWArray.java`:

| Case name | Execution | Ordinal | Runtime ID | Static ID |
|---|---|---|---|---|
| `partition-multikey-warray` | RowRecogPartitionMultikeyWArray | 0 | java-runtime-7a2e1b6edbc6c814817e | java-fa41ea09e7c778bf34af |
| `partition-multikey-plain` | RowRecogPartitionMultikeyPlain | 1 | java-runtime-11baac617fb4b4f194c6 | java-cec9c8f3503e647e5830 |

Both runtime IDs unreferenced; flags []. Inventory `id` field is the
shared outer-class quirk `java-cec9c8f3503e647e5830` for both rows;
per-execution static IDs above come from static-manifest.json.

## Byte-exact EPLs

ord 0 (single spaces at every concat boundary):
```
@name('s0') select * from SupportEventWithIntArray match_recognize ( partition by array measures A.id as a, B.id as b pattern (A B) define A as A.value = 1, B as B.value = 2)
```

ord 1:
```
@name('s0') select * from SupportBean match_recognize ( partition by intPrimitive, longPrimitive measures A.theString as a, B.theString as b pattern (A B) define A as A.doublePrimitive = 1, B as B.doublePrimitive = 2)
```

## Event types

- `SupportEventWithIntArray`: `id` String, `array` int[] (primitive int
  array — pin int[] so Java selects MultiKeyArrayInt), `value` int.
- `SupportBean`: `theString` String, `intPrimitive` int,
  `longPrimitive` long, `doublePrimitive` double.

## Step sequences

### partition-multikey-warray
1. deploy s0
2. send {id:E1, array:[1,2], value:1} → none
3. send {id:E2, array:[1], value:1} → none
4. send {id:E3, array:null, value:1} → none
5. send {id:E4, array:[], value:1} → none
6. send {id:E10, array:[1,2], value:2} → {a:E1, b:E10}
7. send {id:E11, array:[], value:2} → {a:E4, b:E11}
8. send {id:E12, array:[1], value:2} → {a:E2, b:E12}
9. send {id:E13, array:null, value:2} → {a:E3, b:E13}
10. undeploy-all

### partition-multikey-plain
1. deploy s0
2. send {theString:E1, intPrimitive:1, longPrimitive:2, doublePrimitive:1} → none
3. send {theString:E2, intPrimitive:1, longPrimitive:3, doublePrimitive:1} → none
4. send {theString:E3, intPrimitive:2, longPrimitive:2, doublePrimitive:1} → none
5. send {theString:E10, intPrimitive:2, longPrimitive:2, doublePrimitive:2} → {a:E3, b:E10}
6. send {theString:E11, intPrimitive:1, longPrimitive:3, doublePrimitive:2} → {a:E2, b:E11}
7. send {theString:E12, intPrimitive:1, longPrimitive:2, doublePrimitive:2} → {a:E1, b:E12}
8. undeploy-all

## Semantics

- Single array-typed partition expr → MultiKeyArrayInt: deep content
  equality (Arrays.equals/hashCode); distinct instances with same
  content share one partition. null array → one shared null partition;
  null ≠ empty.
- Two scalar exprs → generated MultiKey over boxed types, per-component
  Objects.equals.
- `select *` over match_recognize yields only the measure columns {a,b}.
- No virtual time, no order by, no iterator assertions.
- milestone(0) is a harness no-op — not a scenario step.

## Scenario conventions

Mirror rowrecog-prev/rowrecog-interval: `testdata/parity/
rowrecog-multikey-warray.json`, id `rowrecog-multikey-warray`, same
header fields (javaCommit, javaSource single file, javaRuntimes 2 IDs,
javaNames 2 names, javaStaticIds 2 IDs, javaFlags []), cases[] metadata,
steps: case/deploy/send/undeploy-all. Send payloads are objects;
`array` is a JSON array of ints or null.

## File ownership

- Asset worker: `tools/java-oracle/RowRecogMultikeyWArrayScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-multikey-warray.sh`,
  `testdata/parity/rowrecog-multikey-warray.json`.
- Primary agent: `internal/app/parity/rowrecog_multikey_warray.go`,
  run.go dispatch, run_test.go family, manifest/roadmap/CHANGELOG/PLANS.
- Shared core: ZERO predicted delta — encodeKey already canonicalizes
  []int content (nil vs empty distinct) and multi-component keys;
  existing unit tests mirror both executions.

## Validation

- `run-rowrecog-multikey-warray.sh --esper-root /root/app/esper
  --scenario ... --output ...trace.json`
- `go run ./cmd/parity -mode rowrecog-multikey-warray-diff ...` →
  passing / 0 differences
- `make check` exit 0; compat manifest validation green.
