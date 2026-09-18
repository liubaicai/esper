# Draft 4.459 — context-hash-segmented extension (ords 1/5/6)

## Scope

Extend `case.context-hash-segmented` (capability `context.partition`) with three
born-differential cases from `ContextHashSegmented.java` (Java commit
`9e1b9f1cc9117fea4bf33ab043762c045d73839c`, verified clean):

- ord 1 `ContextHashSegmentedFilter` — `java-runtime-514d6623af4af2d18516`
- ord 5 `ContextHashSegmentedBySingleRowFunc` — `java-runtime-7f880063fe0f23046c59`
- ord 6 `ContextHashScoringUseCase` — `java-runtime-74e1f67a0acf62775846`

Shared static id `java-13f0da7834ee65870fb0` (verify against static-manifest).

## Frozen Java contract (scout JavaContract459)

- ord 1: ctx `coalesce consistent_hash_crc32(theString) from
  SupportBean(intPrimitive > 10) granularity 4 preallocate`; stmt selects
  `context.name`, `intPrimitive` over `#lastevent`. Sends intPrimitive
  10,1,12,10,1,15 — only 12 and 15 produce output; filtered events never reach
  the partition or the lastevent window. Iterator asserts BOTH iterator() and
  safeIterator() any-order.
- ord 5: ctx `coalesce myHash(*) from SupportBean granularity 4 preallocate`
  (plug-in SRF registered in suite config); stmt selects `context.id`,
  `myHash(*)`, `mySecond(*, theString)`, qualified `mySecondFunc`. Sends
  intPrimitive 3,0,7 → context.id = value%4 (3,0,3), c2 = raw value.
- ord 6: two declared schemas (ScoreCycle, UserKeywordTotalStream); coalesce
  crc32(userId) context over both types granularity 1000000; context window
  unique(productId,keyword); insert-into window; group-by-keyword sum(score)
  insert into second type; on-delete trigger (sumScore>10000) never fires.
  5-event sequence → sumScore 100,115,30,40,50. Java iterates all 6 event
  representations with identical observable output; scenario pins DEFAULT/MAP.
- ord 8 `ContextHashInvalid` DEFERRED: six compile-error checks require named
  hash-function registry, context-level filter expressions, and
  named-window-in-partition-criteria surfaces that Go does not expose;
  partial coverage is not DV-able.

## Go mapping (scout GoSurface459)

- ord 1: statement-level `.Filter(Greater(intPrimitive, 10))` is observably
  identical under `preallocate` (all 4 buckets exist regardless; filtered
  events produce no output and leave lastevent untouched). Documented
  emulation.
- ord 5: `Func1[bean,int]("myHash", fn, EventValue[bean]())` as hash key with
  `HashAlgorithmJavaHashCode` (identity for int, abs%partitions); `Func2` for
  mySecond. context.id via `ContextID()`.
- ord 6: `NewMapSchema`+`RegisterSchema` for both types; shared
  `Field[map[string]any,string]("userId")` hash key; `CreateNamedWindow` with
  `NamedWindowContext`+`UniqueBy(productId,keyword)`; `InsertIntoNamedWindow`
  trigger; grouped aggregate over `FromNamedWindow` + `InsertInto` routed
  stream; `OnEvent(filtered).DeleteFromNamedWindow` trigger (never fires).
  Risk: grouped aggregate over context-bound named window + insert-into
  repartitioning under hash context is untested — smoke test first.

## Files

- Extend `testdata/parity/context-hash-segmented.json` (3 new cases)
- Extend `tools/java-oracle/ContextHashScenarioOracle.java` (3 new cases)
- Extend `internal/app/parity/context_hash.go` (3 new cases, new event types)
- Extend `internal/app/parity/run_test.go` (assert table + family)
- Update manifest case + summary, CHANGELOG, roadmap
- No `internal/esper` shared-core changes expected (emulation only)

## Delegation

- Java scout: JavaContract459 (java-oracle-scout) — done
- Go scout: GoSurface459 (scout) — done
- Implementation: primary agent (single interlocked asset bundle: oracle,
  scenario, runner, tests all co-evolve; no safe disjoint split)
- Reviewer: parity-reviewer after integration
