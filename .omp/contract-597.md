# Contract: Draft 4.597 'context init-term remainder' (FROZEN)

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c.
Four executions, one scenario family `context-init-term-remainder`:

| case | Java source | runtime ID | notes |
|---|---|---|---|
| db-historical | ContextInitTermTemporalFixed ord 18 `ContextStartEndDBHistorical` | java-runtime-bc152186877c0a641b3d | static `java-06954b45a1979f495425` |
| distinct-invalid | ContextInitTermWithDistinct ord 0 `ContextInitTermWithDistinctInvalid` | java-runtime-19cc6b63d1614c49dfbf | static `java-1db75f8dcee67079871d` |
| now-invalid | ContextInitTermWithNow ord 2 `ContextInitTermWNowInvalid` | java-runtime-6b3caa8f5e3490b05507 | static `java-21fe1b2ee6da1a4f412c` |
| hash-invalid | ContextHashSegmented ord 8 `ContextHashInvalid` | java-runtime-25a58a30d6cbd02f00e7 | static `java-0564864de64ece6e7772` |

No flags on any execution. Java commit pinned. Live MySQL fixture
required for db-historical (mytesttable, DriverManagerConnection via
SupportDatabaseService DRIVER/FULLURL + SupportDatabaseURL.newProperties(),
POOLED lifecycle — mirror EPLDatabaseJoinScenarioOracle registration).

## Byte-exact EPLs (verbatim from Java source)

### db-historical (ord 18)
- `@public create context NineToFive as start (0, 9, *, *, *) end (0, 17, *, *, *)`
- `@name('s0') context NineToFive select * from SupportBean_S0 as s0, sql:MyDB ['select * from mytesttable where ${id} = mytesttable.mybigint'] as s1`
- env.addListener("s0") after compileDeploy.
- Sequence: sendTimeEvent 2002-05-1T08:00 → send S0(2) silent → sendTimeEvent
  2002-05-1T09:00 → send S0(2) → assertPropsNew s1.mychar="Y" →
  sendTimeEvent 2002-05-1T17:00 → send S0(2) silent →
  sendTimeEvent 2002-05-2T09:00 → send S0(3) → "X". undeployAll.
- Record shape: listener records `{"s1.mychar":"Y"}` / `{"s1.mychar":"X"}`.

### distinct-invalid (ord 0) — 5 probes, labels/expected prefixes
1. `distinct-no-as`: `create context MyContext initiated by distinct(theString) SupportBean terminated after 15 seconds` → "Distinct-expressions require that a stream name is assigned to the stream using 'as'"
2. `distinct-pattern`: `create context MyContext initiated by distinct(a.theString) pattern [a=SupportBean] terminated after 15 seconds` → "Distinct-expressions require a stream as the initiated-by condition"
3. `distinct-subselect`: `create context MyContext initiated by distinct((select * from MyWindow)) SupportBean as sb terminated after 15 seconds` → "Invalid context distinct-clause expression 'subselect_0': Aggregation, sub-select, previous or prior functions are not supported in this context"
4. `distinct-empty`: `create context MyContext initiated by distinct() SupportBean terminated after 15 seconds` → "Distinct-expressions have not been provided"
5. `start-distinct`: `create context MyContext start distinct(theString) SupportBean end after 15 seconds` → "Incorrect syntax near 'distinct' (a reserved keyword)"

### now-invalid (ord 2) — 3 probes
1. `now-alone-terminated`: `create context TimedImmediate initiated @now terminated after 10 seconds` → "Incorrect syntax near 'terminated' (a reserved keyword) expecting 'and'"
2. `now-and-nonoverlapping`: `create context TimedImmediate start @now and after 5 seconds end after 10 seconds` → "Incorrect syntax near 'and' (a reserved keyword)"
3. `now-with-filter`: `create context TimedImmediate initiated @now and SupportBean terminated after 10 seconds` → "Invalid use of 'now' with initiated-by stream"

### hash-invalid (ord 8) — sequence
1. build-error `hash-dummy-filter`: `create context ACtx coalesce hash_code(intPrimitive) from SupportBean(dummy = 1) granularity 10` → "Failed to validate filter expression 'dummy=1': Property named 'dummy' is not valid in any stream"
2. build-error `hash-bad-func`: `create context ACtx coalesce hash_code_xyz(intPrimitive) from SupportBean granularity 10` → "expected a hash function that is any of {consistent_hash_crc32, hash_code}"
3. build-error `hash-bare-prop`: `create context ACtx coalesce intPrimitive from SupportBean granularity 10` → same prefix
4. build-error `hash-no-params`: `create context ACtx coalesce hash_code() from SupportBean granularity 10` → "expected one or more parameters to the hash function"
5. deploy `@public create context ACtx coalesce hash_code(intPrimitive) from SupportBean granularity 10` (silent; needed as path context)
6. build-error `statement-stream-type`: `context ACtx select * from SupportBean_S0` → "requires that any of the event types that are listed in the segmented context also appear in any of the filter expressions of the statement, type 'SupportBean_S0' is not one of the types listed"
7. deploy `@public create window MyWindow#keepall as SupportBean` (silent)
8. build-error `partition-named-window`: `@public create context SegmentedByWhat partition by theString from MyWindow` → "Partition criteria may not include named windows"
9. undeploy-all.

## Step/record protocol (mirror context-lifecycle/context_category)
- `deploy` → silent (no record) unless listeners attached.
- `build-error` → `compile-error` record, value = pinned Java prefix.
- `advance-time` → advance record? Mirror context-init-term-temporal-fixed
  convention (check its trace: advance-time emits a record).
- `send` → listener records only when the statement emits.

## File ownership (disjoint)
- Shared-core writer: `internal/esper/context.go` +
  `internal/esper/context_hash_streams_test.go` ONLY.
- Asset worker: `testdata/parity/context-init-term-remainder.json`,
  `tools/java-oracle/ContextInitTermRemainderScenarioOracle.java`,
  `tools/java-oracle/run-context-init-term-remainder.sh` ONLY.
- Primary agent: Go runner + run.go/run_test.go wiring, traces, evidence,
  manifest/roadmap/CHANGELOG/PLANS.

## Forbidden
- No edits to /root/app/esper. No formatter/lint/full-suite runs by
  subagents. No manifest/roadmap/PLANS edits by subagents.
