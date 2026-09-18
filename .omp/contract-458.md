# Draft 4.458 — context-admin-listen

Born-differential case `case.context-admin-listen` covering ContextAdminListen
ords 2,3,4,6 (ord 5 ContextAdminPartitionAddRemoveListener deferred: scenario B
needs nested initiated-parent contexts, which NewNestedContext rejects).
Java oracle fixed at /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c.

## Shared-core change (primary agent)
Populate `ContextStateEvent.RuntimeURI` from `e.runtimeURI` on all event
constructions (Java asserts "default"). `ContextDeploymentID` stays empty for
env-registered contexts; the runner normalizes to the ctx deploy-step label.

## Frozen contract (scout NextJavaContract458)
Listener surface: ContextStateListener (created/destroyed) +
ContextPartitionStateListener (activated/deactivated/statement-added/removed/
partition-allocated/deallocated). SupportContextListener implements both;
onContextCreated re-entrantly registers itself as partition listener.

- ord 2 Category `java-runtime-2021f021c6e12684fb81` (RUNTIMEOPS): context-state listener;
  deploy category ctx (pos/neg) + s0; NO sends; exactly 2 PartitionAllocated
  (eager); allocated[1] label "neg". Undeploy s0 then ctx.
- ord 3 Nested `java-runtime-4b1466f2815a381815b2` (RUNTIMEOPS): nested ctx
  (category parent pos/neg + keyed child by theString); deploy ctx →
  [Created]; deploy s0 → [StatementAdded, Activated]; send E1,1 → exactly 1
  Allocated (nested leaf = one event; identifier Nested, identifiers[1]
  Partitioned keys ["E1"]); undeploy s0 → [StatementRemoved,
  PartitionDeallocated, Deactivated]; undeploy ctx → [Destroyed].
- ord 4 AddRemoveListener `java-runtime-410d2c5d3daf011b6c7b` (RUNTIMEOPS+OBSERVEROPS): 3 context-state listeners
  before deploy; init-term ctx deploy → each [Created]; remove listeners[0];
  undeploy ctx → listeners[0] silent, [1],[2] [Destroyed]; iterator yields
  [1],[2] in order; removeContextStateListeners → empty; redeploy+undeploy →
  all silent.
- ord 6 MultipleStatements `java-runtime-6dd25578221d74ff6660` (RUNTIMEOPS): deploy init-term ctx; add ONE
  partition listener; deploy s0 'a' + 'b' → [StatementAdded(a), Activated,
  StatementAdded(b)] (Activated once, after first add); send S0(1) → exactly
  1 Allocated despite 2 statements.

## Approved differences
- contextDeploymentId normalizes to the ctx deploy-step label on both sides.
- Identifier-type assertions map to Go descriptor properties (label, key
  segments, initiating_event) — Java's typed identifier classes have no Go
  equivalent.
- runtimeURI asserted as "default" where Java asserts it.

## Files
- Scenario: testdata/parity/context-admin-listen.json
- Oracle: tools/java-oracle/ContextAdminListenScenarioOracle.java +
  run-context-admin-listen.sh
- Runner: internal/app/parity/context_admin_listen.go + run.go wiring
- Traces/evidence: testdata/parity/context-admin-listen.{trace,go.trace,evidence}.json
- Manifest: new case.context-admin-listen born-DV with 4 runtime IDs
