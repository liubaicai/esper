# Contract: Draft 4.469 — epl-other-from-clause-optional

Frozen 2026-09-19. Java oracle pinned `9e1b9f1cc9117fea4bf33ab043762c045d73839c`
(verified via `git -C /root/app/esper rev-parse HEAD`). Scouts:
`NextJavaContract469` (Java) + `NextGoSurface469` (Go), both read-only.

## Scope

`EPLOtherFromClauseOptional.java`
(`regression-lib/.../suite/epl/other/EPLOtherFromClauseOptional.java`,
`executions()` L37-45). Five of six executions:

| ord | Java name | runtime ID | flags | in unit |
|-----|-----------|------------|-------|---------|
| 0 | `EPLOtherFromOptionalContext{soda=false}` | `java-runtime-0e96acf48376ed71c690` | [] | DV |
| 1 | `EPLOtherFromOptionalContext{soda=true}` | `java-runtime-f54b77f9c8381d0cc12c` | [] | DV |
| 2 | `EPLOtherFromOptionalNoContext` | `java-runtime-00d22c5518b7c57f1ba7` | [] | DV |
| 3 | `EPLOtherFromOptionalFAFNoContext` | `java-runtime-80d10cc89f5410ff790e` | FIREANDFORGET | DEFERRED — heterogeneous mix incl. JVM-only `inlined_class` + script-expression surface; separate unit |
| 4 | `EPLOtherFromOptionalFAFContext` | `java-runtime-4f6e15a0c30a5e95b1f6` | FIREANDFORGET | DV |
| 5 | `EPLOtherFromOptionalInvalid` | `java-runtime-6d948697a80bca6a0dac` | INVALIDITY | intentionally-different case |

Static IDs (from inventory `id` field, all six share the file-level id
`java-0c9a8913a4dafbcd91e2` — verify per-execution static ids in
`static-manifest.json` at manifest time).


## Java observable contract (verified against source)

### ords 0/1 — `EPLOtherFromOptionalContext{soda}`

Context: `@public create context MyContext initiated by SupportBean_S0 as s0
terminated by SupportBean_S1(id=s0.id)`.

- `s0`: `context MyContext select context.s0 as ctxs0` — listener fires at
  partition initiation with the initiating S0 event.
- `s1`: `context MyContext select context.s0 as ctxs0 output when terminated`
  — listener fires at partition termination.

- Sequence: S0(10,"A") → s0 listener ctxs0=s0A; iterator s0 = [s0A].
  S0(20,"B") → s0 ctxs0=s0B; iterators s0/s1 = [s0A, s0B] (one row per live
  partition, each row = that partition's initiating event).
  S1(10,"A") terminates the s0A partition (S1.id==s0.id) → s1 listener
  ctxs0=s0A; iterators s0/s1 = [s0B]. S1(20,"A") → s1 ctxs0=s0B; iterators
  empty. `env.milestone` calls are no-ops for the trace (no virtual time).

- Iterators: `assertIterator` on s0/s1 yields one row per live partition
  (each row = that partition's initiating event). After both partitions end,
  iterators are empty.
- `soda=true` (ord 1) is byte-identical behavior; the soda flag only changes
  the compile path (EPL→object model→EPL round trip).

### ord 2 — `EPLOtherFromOptionalNoContext`

`@name('s0') select 1 as value` deployed; iterator yields exactly one row
`{value: 1}`. No events sent.

### ord 4 — `EPLOtherFromOptionalFAFContext`

Same MyContext + `context MyContext select count(*) from SupportBean`
deployed (feeds partitions). S0(10,"A","x"), S0(20,"B","x") initiate two
partitions.

- FAF `context MyContext select context.s0.p00 as id` (no selector) → rows
  {A},{B} — one row per live partition.
- Same FAF with `SupportSelectorById(1)` → {B} only.
- SODA round trip → identical result (Go: same plan, no OM).
- FAF `context MyContext select distinct context.s0.p01 as p01` → {x}
  (distinct collapses the two partitions' identical p01).
- FAF `context MyContext select 1 as value where 'a'='b'` → 0 rows.
- FAF `context MyContext select context.s0.p00 as value where
  context.s0.id=10` → {A}.
- FAF `... having context.s0.id=10` → {A} (having ≡ where for source-less).

### ord 5 — `EPLOtherFromOptionalInvalid` (intentionally-different)

- `select (select 1)` compile error — unrepresentable: Go has no source-less
  subselect expression.
- `select *` + FAF `select *` — unrepresentable: `SelectOnce` requires named
  projections; nearest Go boundary is the empty-projection build error.
- Multi-selector FAF `IllegalArgumentException` — unrepresentable: Go's
  `ExecuteFireAndForgetWithSelector` takes a single selector by signature.
- Context+order-by FAF compile error — Go rejects ALL source-less order-by at
  Build ("order-by is not yet supported for source-less queries"); pin with a
  Go unit test.

## Go surface work (shared core — single writer)

1. `SelectOnce` options: `Query` has no fluent option methods. Add either
   `SelectOnceWith(env, selections, options...)` or fluent `Query` methods
   (`WithContext`, `WithOutput`, `WithStatementName`-equivalent). DECISION:
   add fluent methods on `Query` mirroring `OnDemandStream.WithContext`
   precedent: `Query.WithContext(name)`, `Query.WithOutput(policy)`,
   `Query.Named(name)` — pick minimal set; verify no method-name collisions.
2. Source-less deployed statement snapshot: `Snapshot` currently yields 0
   rows; Java yields 1 row of projected constants. Fix: source-less statement
   evaluates projections once per (partition) activation.
3. Source-less under context: per-partition instantiation; listener fires at
   initiation (each new partition delivers one row); `output when terminated`
   delivers at termination; iterator/snapshot yields one row per live
   partition. `context.s0` → `ContextInitiatingEvent()` (populated as
   `initiating_event` at runtime.go:8586/9395).
4. Source-less FAF + context: `executeFireAndForget` source-less branch must
   honor `contextName` (iterate live partitions, evaluate per partition),
   `selector` (filter partitions), `distinct`, and new where/having.
5. Source-less where/having: new `Query` fields + `QueryOption`s
   (`WithWhere`, `WithHaving`) restricted to source-less queries (build error
   otherwise). Evaluate per partition; false → no row.
6. FAF selector on source-less plan: honor single selector; document that
   multi-selector is unrepresentable.

## Files

- Runner: `internal/app/parity/epl_other_from_clause_optional.go` + run.go wiring.
- Scenario: `testdata/parity/epl-other-from-clause-optional.json`.
- Oracle: `tools/java-oracle/EplOtherFromClauseOptionalScenarioOracle.java` +
  `run-epl-other-from-clause-optional.sh`.
- Engine: `internal/esper/stream.go` (SelectOnce/Query options),
  `internal/esper/faf.go` (source-less context/selector/distinct/where/having),
  `internal/esper/runtime.go` (source-less per-partition lifecycle + snapshot),
  `internal/esper/plan.go` (where/having validation; order-by pin stays).
- Manifest: new case `case.epl-other-from-clause-optional` (ords 0,1,2,4 DV)
  + `case.epl-other-from-clause-optional-invalid` (ord 5,
  intentionally-different); capability `epl.other.from-clause-optional`
  created/updated; ord 3 stays in `remaining`.

## Validation

- `-mode epl-other-from-clause-optional-diff` → passing / 0 differences.
- Six-test family (runner + trace mutations + raw-scenario mutations).
- `make check` green; manifest validator green.
