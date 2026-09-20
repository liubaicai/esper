# Contract — Draft 4.476 'context-selection-faf-nested'

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: ContextSelectionAndFireAndForget.java ord 3 ContextSelectionFAFNestedNamedWindowQuery
(java-runtime-f51a1493ad61c1f0d0d1, FIREANDFORGET).

## Java observable contract (scout-verified)
- EPL (byte-exact): `@public create context NestedContext context ACtx initiated by
  SupportBean_S0 as s0 terminated by SupportBean_S1(id=s0.id), context BCtx group by
  intPrimitive < 0 as grp1, group by intPrimitive = 0 as grp2, group by intPrimitive > 0 as
  grp3 from SupportBean`; `@public context NestedContext create window MyWindow#keepall as
  SupportBean`; `insert into MyWindow select * from SupportBean`.
- `initiated by` = OVERLAPPING init-term: every SupportBean_S0 creates a new concurrent ACtx
  partition. Each ACtx activation EAGERLY instantiates all 3 BCtx category leaves in declared
  order; leaf IDs allocated globally in instantiation order.
- Leaf map: id0=(s0=1,grp1) id1=(s0=1,grp2) id2=(s0=1,grp3) id3=(s0=2,grp1) id4=(s0=2,grp2)
  id5=(s0=2,grp3).
- Routing: SupportBean events broadcast to ALL live ACtx partitions, then land in one leaf per
  parent by BCtx category. Init-term level contributes no filter addendum. The triggering S0
  is evaluated into new leaf statements but S0 != SupportBean so never enters MyWindow.
- Events: S0(1,'S0_1'), E1(1), S0(2,'S0_2'), E2(-1), E3(5), E1(2). No S1 sent (no termination).
- Per-leaf window: leaf0={E2(-1)}, leaf1={}, leaf2={E1(1),E3(5),E1(2)}, leaf3={E2(-1)},
  leaf4={}, leaf5={E3(5),E1(2)}.
- FAF: Q1 `select theString as c1, sum(intPrimitive) as c2 from MyWindow group by theString`
  All+no-selector → {E1,5},{E2,-2},{E3,10} (merge all leaves into one collection, single
  group-by). Q2 same + ById{2} → {E1,3},{E3,5}. Q3 `context NestedContext select
  context.ACtx.s0.p00 as c1, context.BCtx.label as c2, theString as c3, sum(intPrimitive) as
  c4 from MyWindow group by theString` + ById{2} → {S0_1,grp3,E1,3},{S0_1,grp3,E3,5}
  (per-partition processing, no merge).
- Context props per leaf: {name:'NestedContext', id:cpid, ACtx:{name,startTime,endTime,
  s0:<triggering S0 event>}, BCtx:{name,id,label:'grpN'}}.
- Selector validity on nested: All/ById/Nested only; others → 'Invalid context partition
  selector, expected an implementation class of any of [ContextPartitionSelectorAll,
  ContextPartitionSelectorById, ContextPartitionSelectorNested] interfaces but received <fqcn>'.
  ById null/empty → empty result.

## Ownership
- Shared core (primary agent only): internal/esper/context.go, plan.go, runtime.go, state.go,
  faf.go, context_selector.go, context_test.go (gate pin).
- Asset worker: tools/java-oracle/ContextSelectionFAFNestedScenarioOracle.java (or extend the
  existing oracle), tools/java-oracle/run-context-selection-faf-nested.sh,
  testdata/parity/context-selection-faf-nested.json, internal/app/parity/context_selection_faf_nested.go,
  internal/app/parity/run.go (mode wiring only).
- Primary agent generates traces + evidence; updates manifest/roadmap/CHANGELOG/PLANS.

## Shared-core changes required
1. Remove NewNestedContext initiated-parent rejection (context.go:819) + validateContext gate
   (plan.go:2015-2018) + the test pin (context_test.go:2745).
2. Dispatch: events matching the leaf stream type broadcast to all live parent partitions,
   then route by child category — new processNestedInitiatedParent path before
   statementAcceptsEvent (runtime.go:8123); reuse nestedParents/nestedParentIDs/
   nestedParentKey/nestedLeafUnderParents helpers.
3. Eager leaf instantiation at parent initiation in category declaration order; global
   leaf-ID allocation (allocateContextPartitionIDLocked keyed by leaf context name).
4. Lifecycle: parent termination (S1 matching id=s0.id) cascades to its leaves;
   lifecycleManaged/suppressDeallocated gates need initiated-level chain walk;
   NamedWindow.releaseContextPartition gate needs chain walk.
5. FAF: nested-initiated leaf contexts need descriptor-driven path — iterate
   contextPartitionDescriptors in ID order + SnapshotContext per leaf for non-context
   queries (merge) and per-partition for context-clause queries; descriptors need
   parent.initiating_event (triggering S0) and child label.
6. Selector validation: nested contexts accept All/ById/Nested only.

## Validation
- tools/java-oracle/run-context-selection-faf-nested.sh → trace
- go run ./cmd/parity -mode context-selection-faf-nested[-diff]
- make check; targeted internal/esper tests.
