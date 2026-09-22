# Draft 4.495 — resultset-output-when-then-closure

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c
File: regression-lib/.../resultset/outputlimit/ResultSetOutputLimitCrontabWhen.java
Closes the file: ords 9, 10, 13 are the last 3 unreferenced of 14.

## Executions

- ord 9 `ResultSetOutputWhenThenExpressionSODA` = java-runtime-c8af5811fef1d7f582d6
  - runtimeSetVariable(myvar,0); advanceTime(2008-02-01T08:00 local);
    deploy `on SupportBean set myvar = intPrimitive` (inert, never triggered);
    SODA model -> toEPL == `@name('s0') select symbol from SupportMarketDataBean#length(2) output when myvar=1 then set myvar=0, count_insert_var=count_insert`;
    compileDeploy(model) + listener; undeployAll.
  - ONLY assertion: toEPL byte-equality. No events, no output assertions.
  - Go: toEPL unrepresentable -> `unrepresentable` step; deploy/deployed/undeploy-all records.
- ord 10 `ResultSetOutputWhenThenSameVarTwice` = java-runtime-feee4a26544010fc7fed
  - advanceTime(0); deploy s1 + s2 (separate modules) `select * from SupportMarketDataBean output last when myvar=100`;
    send MD(ABC,E1,100), MD(ABC,E2,100); advanceTime(1000) -> neither invoked;
    set myvar=100; advanceTime(2000) -> BOTH invoked (each 1 new row = E2, old null);
    undeploy both.
  - ENGINE GAP: Go whenPending emits all buffered rows for OutputLastPolicy;
    must reduce to last row (Java emits only E2).
- ord 13 `ResultSetInvalid` = java-runtime-54ed0a4e11d7714b8289
  - 8 tryInvalidCompile probes (prefix match):
    1. `output when myvardummy` (non-bool) -> covered, contains-guard
    2. `then set myvardummy='b'` (type mismatch) -> covered, contains-guard
    3. `then set myvardummy=sum(myvardummy)` -> NEW: aggregate in then-set
    4. `then set 1` -> UNREPRESENTABLE (SetOutputVariable requires name)
    5. `output when sum(price)>0` -> covered (variables-only), contains-guard
    6. `output when sum(count_insert)>0` -> NEW: aggregate in when
    7. `output when prev(1,count_insert)=0` -> NEW: prev in when
    8. `output all every 0 seconds` -> covered (interval positive), contains-guard

## File ownership

- Shared core (primary agent): internal/esper/{expr,plan,runtime,stream}.go,
  facade_generated.go, internal/esper/*_test.go
- Asset worker: internal/app/parity/resultset_output_limit_crontab_when_closure.go,
  run.go + run_test.go wiring, testdata/parity/resultset-output-limit-crontab-when-closure.json,
  tools/java-oracle/ResultSetOutputLimitCrontabWhenClosureScenarioOracle.java,
  tools/java-oracle/run-resultset-output-limit-crontab-when-closure.sh
- Primary agent generates traces + evidence after integration.

## Conventions

- Runner mirrors resultset_output_limit_row_per_group_first.go (multi-case) +
  view_group_closure.go (build-error/unrepresentable probes, contains-guard).
- count_insert_var registers as int64 (OutputCountInsert is int64; narrowing rejected).
- myvar/myvardummy register as int.
- SupportMarketDataBean needs all 5 fields; Volume *int64 / Feed *string nullable.
- sendTimer(ms) = advance-time at epoch+ms; sendTimeEvent(2008-02-01T08:00) pinned absolute.
