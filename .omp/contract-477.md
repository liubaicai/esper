# Contract — Draft 4.477 'context-admin-listen-partition-addremove'

Oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (NEVER modify).
Source: ContextAdminListen.java ord 5 ContextAdminPartitionAddRemoveListener
(java-runtime-206c08a7f3d6c239050c, RUNTIMEOPS, static java-3a026095a61c4060c91b).

## Java observable contract (scout + reviewer verified)
- `runAssertionPartitionAddRemoveListener` runs TWICE on one runtime with
  `env.undeployAll()` between: scenario A flat, scenario B nested.
- EPL A: `@name('ctx') @public create context MyContextStartEnd start SupportBean_S0 as s0
  end SupportBean_S1` + `@name('s0') context MyContextStartEnd select count(*) from
  SupportBean`.
- EPL B: `@name('ctx') @public create context MyContextStartEndWithNeverEnding context
  NeverEndingStory start @now, context ABSession start SupportBean_S0 as s0 end
  SupportBean_S1` + `@name('s0') context MyContextStartEndWithNeverEnding select count(*)
  from SupportBean`.
- `start @now` = ContextSpecConditionImmediate: non-overlapping initiated parent with
  immediate start and no end. Go equivalent: NewInitiatedContext("NeverEndingStory",
  Literal("global"), Literal(true)) — activates on first event rather than at deploy;
  observably equivalent because listeners register after deploy and the parent emits no
  partition events (Java fires allocated only at LEAF instantiation).
- Sequence (both scenarios): deploy ctx+s0 (no listeners yet); register 3 partition-state
  listeners l0/l1/l2 via addContextPartitionStateListener(depIdCtx, contextName, l_i);
  S0(1) → each gets ContextStateEventContextPartitionAllocated (partitionId=0, runtimeURI
  "default", contextDeploymentId=ctx deploy id); removeContextPartitionStateListener(l0);
  S1(1) → l0 silent, l1/l2 get ContextStateEventContextPartitionDeallocated (partitionId=0,
  NO identifier); getContextPartitionStateListeners → [l1,l2] registration order;
  removeContextPartitionStateListeners → empty; S0(2)/S1(2) → leaf id 1 silently
  allocated/deallocated, all listeners assertNotInvoked; undeployAll.
- Identifier shapes (oracle renderIdentifier): A allocated →
  {"type":"initiatedTerminated","initiatingEvent":"SupportBean_S0"}; B allocated →
  {"type":"nested","identifiers":[{"type":"initiatedTerminated"},
  {"type":"initiatedTerminated","initiatingEvent":"SupportBean_S0"}]} — the @now parent
  renders no initiatingEvent (no triggering event). Deallocated events carry only the id.
- In-callback getContextProperties(depId,name,id) must return non-null.

## Ownership
- Shared core: none expected (scout: zero or near-zero; leaf-initiated path handles the
  never-ending parent). If a gap surfaces, primary agent only.
- Asset worker: extend tools/java-oracle/ContextAdminListenScenarioOracle.java +
  testdata/parity/context-admin-listen.json + internal/app/parity/context_admin_listen.go
  with a 'partition-add-remove-listener' case covering BOTH scenarios A and B.
- Runner additions needed: remove-partition-listener, remove-partition-listeners ops,
  admin:partition-listeners snapshot (iterator over registered listeners in registration
  order), identifier rendering for the `start @now` parent level (no initiatingEvent key).
- Primary agent generates traces + evidence; updates manifest/roadmap/CHANGELOG/PLANS.

## Validation
- tools/java-oracle/run-context-admin-listen.sh → trace
- go run ./cmd/parity -mode context-admin-listen[-diff]
- make check; targeted internal/esper tests.
