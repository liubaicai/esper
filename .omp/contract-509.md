# Draft 4.509 contract — rowrecog-interval

Frozen 2026-09-23. Java oracle pinned at commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c` (`/root/app/esper`).

## Scope

Five executions, one capability subdomain (match_recognize interval family):

| Case name | Execution | Runtime ID | Static ID |
|---|---|---|---|
| `interval-simple` | RowRecogIntervalSimple (ord 0) | java-runtime-02e57a7f151323edc4be | java-4f72d1e9bd1e9e0e91c3 |
| `interval-partitioned` | RowRecogPartitioned (ord 1) | java-runtime-b9a2b0afe61a2bfabcd0 | java-2b80de56360cd5f89976 |
| `interval-multicompleted` | RowRecogMultiCompleted (ord 2) | java-runtime-1361f7530aa059720913 | java-ecc5ea69ce7531fba427 |
| `interval-monthscoped` | RowRecogMonthScoped (ord 3) | java-runtime-373cb011a639d99148f0 | java-a3bc77ab51a1c2e468a8 |
| `orterminated-doc-sample` | RowRecogIntervalOrTerminated sub (a) | java-runtime-bd18c2cfd8ff34a0b5a3 | java-9c1e0161596faa9a387e |
| `orterminated-a-b` | sub (h) | same | same |
| `orterminated-a-bstar` | sub (b) | same | same |
| `orterminated-a-bstar-allmatches` | sub (c) | same | same |
| `orterminated-a-bstar-or-c` | sub (i) | same | same |
| `orterminated-a-bstar-or-cstar` | sub (f) | same | same |
| `orterminated-a-b-cstar` | sub (g) | same | same |
| `orterminated-a-bplus` | sub (e) | same | same |
| `orterminated-astar` | sub (d) | same | same |
| `orterminated-a-parens-bstar` | sub (j) | same | same |

All five runtime IDs were unreferenced before this unit. Flags: [] for all.

## Semantics (verified against RowRecogNFAView.java)

- Interval anchors to matchBeginEventTime (first event of the match);
  emission boundary INCLUSIVE: emit when currentTime - period >= beginKey.
- `interval` alone: completed matches are scheduled, not emitted; timer
  callback drains begin-keys <= cutoff. Iterator sees scheduled-but-unemitted
  matches.
- `or terminated`: dead-end end states emit immediately; termination states
  claim prefix-consistent scheduled end states under the same begin-key;
  sibling branches keep running.
- Misfit event kills non-matching strands silently (unless or-terminated +
  EndEval successor).
- Non-allMatches = one ranked row per begin-key (greedy-count compare);
  Go builder needs `.FirstMatch()`. allMatches emits all end states,
  asserted any-order in Java → scenario case step carries `mode: "any"`.
- Calendar interval (`interval 1 month`) uses Calendar.add semantics →
  Go `.IntervalCalendar(0, 1, 0)`.
- `sendTimer(Integer.MAX_VALUE)` → advance-time to 1970-01-25T20:31:23.647Z.
- Each or-terminated sub-scenario = fresh engine + clock reset (Java does
  sendTimer(0) + fresh deploy per sub-assertion).

## Scenario conventions (mirror rowrecog-prev)

- `testdata/parity/rowrecog-interval.json`, version `esper-parity/v1`,
  id `rowrecog-interval`, javaCommit pinned, javaSource =
  `regression-lib/.../rowrecog/RowRecogInterval.java`, javaSource2 =
  `.../RowRecogIntervalOrTerminated.java`, javaRuntimes = the 5 IDs,
  javaNames = the 5 execution names, javaStaticIds = the 5 static IDs,
  javaFlags = [].
- `cases[]` entries: {case, ordinal, runtimeId, executionName, observation,
  epl} — ordinal = position within its source file's executions (0-3 for
  RowRecogInterval, 0 for IntervalOrTerminated); epl = the pinned deploy EPL.
- Steps: `{"op":"case","case":NAME}` marker (add `"mode":"any"` for
  orterminated-a-bstar-allmatches), then deploy/advance-time/send/snapshot/
  undeploy-all. `advance-time` uses `at` RFC3339 UTC ms. `send` payload =
  object for SupportRecogBean/SupportBean, positional array for
  TemperatureSensorEvent (object-array type: id String, device int,
  temp double).
- `interval-simple` replays the identical send/advance sequence twice
  (compileDeploy + eplToModelCompileDeploy → deploy, undeploy-all, deploy,
  undeploy-all).
- Event types: SupportRecogBean{theString,value,cat}, SupportBean{theString,
  intPrimitive}, TemperatureSensorEvent object-array.

## File ownership

- Asset worker (parity-asset-worker): `tools/java-oracle/RowRecogIntervalScenarioOracle.java`,
  `tools/java-oracle/run-rowrecog-interval.sh`,
  `testdata/parity/rowrecog-interval.json`. Read-only references:
  `tools/java-oracle/RowRecogPrevScenarioOracle.java` + `run-rowrecog-prev.sh`
  (structure precedent), `testdata/parity/rowrecog-prev.json` (format).
- Primary agent: `internal/app/parity/rowrecog_interval.go`, `run.go`
  dispatch, `run_test.go` family, manifest/roadmap/CHANGELOG/PLANS.
- Shared core: ZERO predicted delta (4.508 termination-entry rewrite covers
  or-terminated; plain-interval path unit-verified).

## Validation

- `run-rowrecog-interval.sh --esper-root /root/app/esper --scenario
  testdata/parity/rowrecog-interval.json --output ...trace.json`
- `go run ./cmd/parity -mode rowrecog-interval-diff -java-trace ...
  -evidence ... -scenario ...` → passing / 0 differences
- `make check` exit 0; compat manifest validation green.
