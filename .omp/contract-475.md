# Contract — Draft 4.475 'context-selection-faf'

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: regression-lib/.../context/ContextSelectionAndFireAndForget.java.

## Scope
IN: ord 0 ContextSelectionAndFireAndForgetInvalid (java-runtime-c8c49c4c40e41d383d25,
FIREANDFORGET+INVALIDITY), ord 1 ContextSelectionIterateStatement
(java-runtime-6dd5b9086935002cc50d), ord 2 ContextSelectionAndFireAndForgetNamedWindowQuery
(java-runtime-bf4cefd62580e2abd2c6, FIREANDFORGET).
OUT (prefetch N+1): ord 3 ContextSelectionFAFNestedNamedWindowQuery
(java-runtime-f51a1493ad61c1f0d0d1) — needs nested initiated-parent contexts, leaf-event
broadcast, eager category instantiation; different engine surface.

## Java observable contract (scout-verified)
- env.milestone = no-op; no virtual time. FAF asserts any-order; iterator asserts ordered.
- ord 0: two invalid FAF compiles. (1) `context SegmentedSB select * from WinSB, WinS0` →
  prefix 'Joins in runtime queries for context partitions are not supported'. (2) join of two
  context-bound windows without context clause → prefix 'Joins against named windows that are
  under context are not supported'. Events only create partitions.
- ord 1: `context PartitionedByString select context.key1 as c0, sum(intPrimitive) as c1 from
  SupportBean#length(5)`; events E1/10, E2/20, E2/21 → partitions id0={E1}, id1={E2}.
  Iterator rows {E1,10},{E2,41} for default/all/byId{0,1,2} (id 2 silently ignored);
  byId{1} → {E2,41}; byId{empty}/byId{null} → empty iterator, no error;
  iterator(null) → 'No selector provided'; non-context stmt + selector → 'Iterator with
  context selector is only supported for statements under context' (non-context check FIRST).
- ord 2: window MyWindow#keepall under PartitionedByString + insert-into (inherits context).
  Events E1/10, E2/20, E2/21 → id0 'E1'={10}, id1 'E2'={20,21}. Non-context FAF aggregates
  across ALL partitions (sum=51; >15 → 41); segmented selector [['E2']] → 41; byId{1} → 41;
  context-clause FAF exposes context.key1, raw per-partition rows; invalid selector (category
  on segmented ctx) → 'Invalid context partition selector, expected an implementation class
  of any of [ContextPartitionSelectorAll, ContextPartitionSelectorFiltered,
  ContextPartitionSelectorById, ContextPartitionSelectorSegmented] interfaces but received com'.

## Ownership
- Shared core (primary agent only): internal/esper/faf.go, runtime.go, state.go,
  context_selector.go, plan.go, split_stream.go.
- Asset worker (parity-asset-worker): tools/java-oracle/ContextSelectionAndFireAndForgetScenarioOracle.java,
  tools/java-oracle/run-context-selection-faf.sh, testdata/parity/context-selection-faf.json,
  internal/app/parity/context_selection_faf.go, internal/app/parity/run.go (mode wiring only).
- Primary agent generates Java/Go traces + evidence; updates manifest/roadmap/CHANGELOG/PLANS.

## Shared-core changes required
1. FAF join rejections at execute/prepare entry (nearest expressible to Java compileFAF):
   context clause + join → ErrorInvalidRule 'Joins in runtime queries for context partitions
   are not supported'; join over ≥1 context-bound named window without context clause →
   'Joins against named windows that are under context are not supported'. Remove now-dead
   executeContextJoinFireAndForget path if unreachable.
2. Non-context FAF over context-bound window: apply selector to window partitions (nil = all);
   validate selector kind vs window context kind → invalid-selector error.
3. Context-bound named-window partition creation registers allocation-order partition IDs +
   descriptors (retainContextPartitionLocked path) so by-id/segmented selectors work for
   FAF-only partitions.
4. SnapshotWithSelector: non-context check first, then nil selector → error 'No selector
   provided' equivalent; by-id intersects allocated IDs (unknown silently dropped); empty/nil
   id set → empty.

## Validation
- tools/java-oracle/run-context-selection-faf.sh → testdata/parity/context-selection-faf.trace.json
- go run ./cmd/parity -mode context-selection-faf → go trace; -diff → evidence, 0 differences
- make check; targeted internal/esper tests.
