# Contract 537 — Oracle assets for pattern-guard-timerwithin-forms-595

## Target
Author exactly two new files, modeled byte-for-byte on the 594 pair:
1. `tools/java-oracle/PatternGuardTimerWithinForms595ScenarioOracle.java`
2. `tools/java-oracle/run-pattern-guard-timerwithin-forms-595.sh`

## Context
Pinned Java commit `9e1b9f1cc9117fea4bf33ab043762c045d73839c` (oracle at
`/root/app/esper`). Scenario `testdata/parity/pattern-guard-timerwithin-forms-595.json`
(6 cases / 43 steps) replays `PatternGuardTimerWithin.java` ordinals 1–6:

| case | ordinal | runtimeId | execution | EPLs |
|---|---|---|---|---|
| interval-10-min | 1 | java-runtime-34555c4a9823a346d710 ← CHECK actual per-case | (see scenario `javaNames`/`runtimeId` per case) | `select * from pattern [(every SupportBean) where timer:within(1 days 2 hours 3 minutes 4 seconds 5 milliseconds)]` |

Read the scenario JSON for the authoritative per-case `runtimeId`,
`executionName`, `observation`, and `epls` — the loader pins them.

## Operational semantics (frozen)
- Steps are grouped into case blocks by `{"op":"case","case":name}`.
- `{"op":"deploy","statement":"s0",["at":...]}`: if `at` present call
  `runtime.getEventService().advanceTime(Instant.parse(at).toEpochMilli())`
  FIRST, then compile the NEXT unpinned EPL of that case (each case's
  `epls` array is consumed in order across its deploys — only
  `may-max-month` has 2), deploy, attach a TraceWriter listener named
  by the EPL's `@name('s0')`.
- `{"op":"send","at":...,"eventType":...,"payload":{...}}`:
  `advanceTime(parse(at).toEpochMilli())` BEFORE `sendEventBean`.
  Payload → ctor: `SupportBean` → `new SupportBean(theString, intPrimitive)`
  where theString may be JSON null → `null`; `SupportMarketDataBean` →
  `new SupportMarketDataBean(symbol, id, price)` (price double).
- `{"op":"undeploy-all"}`: `runtime.getDeploymentService().undeployAll()`.
- Fresh `EPRuntimeProvider.getDefaultRuntime(new Configuration())` per
  case. Before ord-2's replay register suite variables: Java
  `TestSuitePattern.configure` does `config.addVariable("D", double.class, 1)`,
  `("H",double.class,2)`, `("M",double.class,3)`, `("S",double.class,4)`,
  `("MS",double.class,5)` — set these on the Configuration for EVERY case
  (they are inert elsewhere; verify in
  `/root/app/esper/regression-lib/src/main/java/.../support/regression/execution/TestSuitePattern.java`
  or equivalent — locate the configure method that adds D/H/M/S/MS and
  mirror its declared types exactly).
- Ord 3 deploys with substitution parameters: `DeploymentOptions
  .setStatementSubstitutionParameter(...)` five positional `?:` params
  `(1,2,3,4,5)` — read the Java ord3 source in PatternGuardTimerWithin.java
  for the exact binding call and mirror it.

## Trace format (identical to 594)
Records `{case, operation:"listener", statement:"s0", sequence,
time:<ISO millis>, new:[{kind:"row",fields:{name:normalized}}], old:[]}`,
listener-invocable only when new/old non-empty; property names TreeSet-
sorted; `normalize` identical to the 594 helper (null→{state:null},
NaN→{state:nan}, BigDecimal→plain string, EventBean→underlying, etc.) —
copy the helper verbatim. Expected record count: assert in code, verify
against an actual run (Go currently yields 13 — Java MUST agree; do NOT
hard-code 13 into the script, assert it inside the Java main like 594's
EXPECTED_RECORDS with the count you observe, and report it).

## Forbidden
- Do NOT touch `/root/app/esper` sources (oracle is read-only).
- Do NOT modify the scenario JSON, Go files, manifest, PLANS.md, CHANGELOG.
- Do NOT run mvn install of the whole suite; use the script's own
  `-pl common,compiler,runtime,regression-lib -am install -DskipTests`.
- No generated-trace authoring by hand — the primary agent produces the
  checked-in Java trace by running your script.

## Deliverable
- The two files, modeled on the 594 originals (same imports, same
  JsonObject/minimaljson usage, same usage()/jq-validation shell shape
  with 595-specific jq assertions: id, javaCommit, javaSource, per-case
  names/ordinals/epls counts, 43 steps, op whitelists, `at` present on
  sends and exactly the two ord-6 deploys).
- Run the script once: `--esper-root /root/app/esper --scenario
  testdata/parity/pattern-guard-timerwithin-forms-595.json --output
  /tmp/595-java-trace.json` and report exit status + record count +
  the per-case record times (case, sequence, time) in your final message.
