# Draft 4.511 contract — view-systime-trio

Frozen 2026-09-23. Java oracle pinned at commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c` (`/root/app/esper`).

## Scope

Three executions, one capability subdomain (time views: ref-point
anchoring + system-time variants). All runtime/static IDs unreferenced;
flags=[] for all.

| Case name | Execution | Runtime ID | Static ID |
|---|---|---|---|
| `timebatch-refpoint` | ViewTimeBatch$ViewTimeBatchRefPoint (ord 9) | java-runtime-7d3c38fa4d2477a0be87 | java-890952520f19d33fb6d9 |
| `timebatch-uni-systime` | ViewTimeBatchWSystemTime (ord 0) | java-runtime-90902912cde58a38fafd | java-6020404113b2fcd36186 |
| `timewin-weightedavg-systime` | ViewTimeWinWSystemTime (ord 0) | java-runtime-486b77a63e1a3518f362 | java-949fe2671907c0c3e961 |

## Byte-exact EPLs

- `@name('s0') select * from SupportBean#time_batch(10 minutes, 10L)`
- `@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time_batch(2)#uni(volume)`
- `@name('s0') select * from SupportMarketDataBean(symbol='CSCO.O')#time(3.0)#weighted_avg(price, volume, symbol, feed)`

## Step sequences

### timebatch-refpoint (virtual time)
1. advance-time 1970-01-01T00:00:00.000Z (before deploy)
2. deploy s0
3. advance-time +10ms
4. send SupportBean{theString:null, intPrimitive:0}
5. advance-time to 600009ms → silent
6. advance-time to 600010ms → listener 1 row (the SupportBean)
7. undeploy-all

Ref-point: `time_batch(interval, refPointMillis)` — absolute epoch-ms
anchor; boundaries = refPoint + k*interval; strictly-greater rule (a
boundary exactly at now is skipped). Anchor latches on FIRST EVENT
arrival. Here refPoint=10 == first-event time → boundary 600010.

### timebatch-uni-systime (Java wall-clock → virtual mirror)
SupportMarketDataBean{symbol:"CSCO.O", price:0, volume:V, feed:""}.
- deploy; snapshot → 1 row average=NaN (uni zero-state)
- send 500, send 1000 → snapshot NaN; silent
- advance +1000; send 1000, send 1200 → snapshot NaN; silent
- advance +1500 (T0+2000) → snapshot 925.0; listener avg 925.0
- send 500, 600, 1000 → snapshot 925.0; silent
- advance +1000; send 200 → snapshot 925.0; silent
- advance +1500 (T0+4000) → snapshot 575.0; listener 575.0
- send 1200 → snapshot 575.0; silent
- advance +2000 (T0+6000) → snapshot 1200.0; listener 1200.0
- undeploy-all

Iterator on #uni always yields 1 stats row (NaN before first release).
Each boundary posts ONE listener update: new=[stats], old=[prev stats].
select * over #uni exposes: datapoints(Long), total, stddev, stddevpa,
variance, average (all Double).

### timewin-weightedavg-systime (same wall-clock caveat)
SupportMarketDataBean{symbol:"CSCO.O", price:P, volume:V, feed:"feed1"}.
- deploy; types op: property "average" == Double
- send (10,500) → 10.0; send (11,500) → 10.5
- advance +1500; send (10,1000) → 10.25; send (10.5,2000) → 10.375
- advance +2000 (E1,E2 expire T0+3000) → 10.333333333 (listener update)
- send (10.2,1000) → 10.3
- advance +2500 (E3,E4 expire T0+4500) → 10.2
- advance +1000 (E5 expires T0+6500) → NaN
- undeploy-all

checkValue = iterator 1 row + listener lastNew[0], avg at precision 6.
Every row asserts feed="feed1", symbol="CSCO.O" — weighted_avg params
≥3 are passthrough props evaluated on the LAST new event, retained even
on the NaN row. Each send = 1 listener record; each expiry advance = 1
listener record (E1+E2 expire together → single update).

## Scenario conventions

`testdata/parity/view-systime-trio.json`, id `view-systime-trio`,
version esper-parity/v1, javaCommit pinned, javaSource =
ViewTimeBatch.java, javaSource2 = ViewTimeBatchWSystemTime.java +
ViewTimeWinWSystemTime.java (check oracle precedent for multi-source
header shape — rowrecog-prev uses javaSource+javaSource2 for two files;
three files may need a javaSource3 or a list — follow what the oracle
validator supports), javaRuntimes = 3 IDs, javaNames = 3 names,
javaStaticIds = 3 IDs, javaFlags = [].

Steps: case/deploy/advance-time/send/snapshot/types/undeploy-all.
advance-time `at` = RFC3339 UTC ms. send payload = object:
SupportBean{theString,intPrimitive}; SupportMarketDataBean{symbol,
price,volume,feed}. NaN renders {"state":"nan"} (minimal-json can't
encode raw NaN — ViewGroupMergeViewScenarioOracle precedent).

## Go-side decisions (primary agent)

- `timebatch-refpoint`: add `TimeBatchRefPoint(duration, refPoint
  time.Time)` — faithful API for the second time_batch argument.
  Implementation: TimeBatchWindowSpec.ReferencePoint field; arm paths
  anchor state.start via advanceTimeBatchReference(refPoint,...)
  (strictly-greater rule). Additive; existing paths unchanged when unset.
- `timebatch-uni-systime`: FromAny + Filter(symbol='CSCO.O') +
  TimeBatch(2s) + Aggregate with six uni accessors aliased to the Java
  ViewFieldEnum names (datapoints,total,average,stddevpa,stddev,
  variance).
- `timewin-weightedavg-systime`: Filter + TimeWindow(3s) + Aggregate
  with WeightedAvg(price,volume) + LastEver passthrough for symbol/feed
  (bare aliases would suppress the expiry NaN row).
- No WithOldStream; new-only listener recording.

## File ownership

- Asset worker: `tools/java-oracle/ViewSystimeTrioScenarioOracle.java`,
  `tools/java-oracle/run-view-systime-trio.sh`,
  `testdata/parity/view-systime-trio.json`.
- Primary agent: `internal/app/parity/view_systime_trio.go`, run.go
  dispatch, run_test.go family, `internal/esper/stream.go` +
  `runtime.go` TimeBatchRefPoint (shared core — sole writer),
  manifest/roadmap/CHANGELOG/PLANS.

## Validation

- `run-view-systime-trio.sh --esper-root /root/app/esper --scenario ...
  --output ...trace.json`
- `go run ./cmd/parity -mode view-systime-trio-diff ...` → passing / 0
- `make check` exit 0; compat manifest validation green.
