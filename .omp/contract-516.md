# Contract — Draft 4.516 `context-init-term-prioritized`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.
Source file: `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/context/ContextInitTermPrioritized.java` (2 executions, both in scope).

## Unit

Two executions from `ContextInitTermPrioritized`, one runtime surface: initiated/terminated
contexts interacting with named-window subqueries and same-event termination.

- ord 0 `ContextInitTermPrioNonOverlappingSubqueryAndInvalid` — runtime `java-runtime-bb247dc87cf118eb8661`, static `java-41c13254dc50886c2dd2`, flags [].
  Java order: `sendTimeEvent("2002-05-1T10:00:00.000")` FIRST, then ONE `compileDeploy` of a
  7-statement module (path-shared), listener on `out`, then `sendEventBean(SupportProductIdEvent("A1"))`,
  then a `tryInvalidCompile` probe, then `undeployAll`.
  Module EPL (byte-exact, newline-prefixed statements):
  ```
  @Name('ctx') @public create context RuleActivityTime as start (0, 9, *, *, *) end (0, 17, *, *, *);
  @Name('window') @public context RuleActivityTime create window EventsWindow#firstunique(productID) as SupportProductIdEvent;
  @Name('variable') create variable boolean IsOutputTriggered_2 = false;
  @Name('A') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));
  @Name('B') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));
  @Name('C') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));
  @Name('D') context RuleActivityTime insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow));
  @Name('out') context RuleActivityTime select * from EventsWindow
  ```
  Observable: at 10:00 the cron context (9:00–17:00) is active; the first `A1` event passes the
  `not exists` filter on exactly one insert statement, lands in `EventsWindow`, and `out` emits
  ONE row `{productID=A1}`. The other three inserts see the now-nonempty window and filter out.
  Invalid probe EPL: `insert into EventsWindow select * from SupportProductIdEvent(not exists (select * from EventsWindow))`
  Pinned expectError prefix: `Failed to validate subquery number 1 querying EventsWindow: Named window by name 'EventsWindow' has been declared for context 'RuleActivityTime' and can only be used within the same context`

- ord 1 `ContextInitTermPrioAtNowWithSelectedEventEnding` — runtime `java-runtime-0c822c80cf402d017d61`, static `java-517175a60c987d2e2397`, flags [].
  EPL (byte-exact):
  ```
  @Priority(1) create context C1 start @now end SupportBean;
  @name('s0') @Priority(0) context C1 select * from SupportBean;
  ```
  Steps: deploy module, listener `s0`; send SupportBean(E1,1) → s0 emits `{theString=E1}`;
  send SupportBean(E2,1) → s0 emits `{theString=E2}`; undeployAll.
  `@Priority` annotations are unobservable ordering metadata — Go has no equivalent and needs none.

## Harness

One scenario `context-init-term-prioritized`, two cases (`nonoverlapping-subquery`,
`terminating-same-event`), fresh engine per case with `WithRuntimeURI(runtimeID)`.
Ops: `advance-time`, `deploy` (statement + epl pinned), `send`, `build-error`
(statement + epl + expectError pinned), `undeploy-all`.

Case `nonoverlapping-subquery` step order (mirrors Java):
1. `advance-time` at `2002-05-01T10:00:00.000Z`
2. `deploy` ctx / window / variable / A / B / C / D / out — each step carries its pinned EPL
   substring; listener attaches to `out` only.
3. `send` SupportProductIdEvent `{productID:"A1"}`
4. `build-error` probe `subquery-context-mismatch` with the pinned EPL + expectError prefix above.
5. `undeploy-all`

Case `terminating-same-event`:
1. `deploy` ctx (`@Priority(1) create context C1 start @now end SupportBean`)
2. `deploy` s0 (`@name('s0') @Priority(0) context C1 select * from SupportBean`), listener `s0`
3. `send` SupportBean `{theString:"E1",intPrimitive:1}` → s0 emits `{theString=E1}`
4. `send` SupportBean `{theString:"E2",intPrimitive:1}` → s0 emits `{theString=E2}`
5. `undeploy-all`

## Go surface (already verified by primary agent — engine fixes landed)

- `CreateCronTimeContext(env, name, NewCronSchedule(min, hour, dom, mon, dow), end)` —
  Esper cron field order is (minute, hour, dom, month, dow): `start (0,9,*,*,*)` = 09:00 daily.
- `CreateNamedWindow(env, "EventsWindow", schema, NamedWindowContext("RuleActivityTime"),
  NamedWindowRetention(FirstUnique(Field[ProductEvent,string]("ProductID"))))`
- `env.RegisterVariable("IsOutputTriggered_2", false)` for the variable statement.
- Insert: `src.Filter(Not(SubqueryExists(FromNamedWindow(env,"EventsWindow"), nil))).InsertInto("EventsWindow", StatementName(name), WithContext("RuleActivityTime"))`
- `out`: `FromNamedWindow(env,"EventsWindow").Query(StatementName("out"), WithContext("RuleActivityTime"))`
- ord 1 ctx: `CreateInitiatedTerminatedContext(env, "C1", Literal("global"), Literal(true), end)` where
  `end = Equal[string](TypeName(EventValue[Event]()), Literal("SupportBean"))` (event-type-filter end).
- ord 1 s0: `From[Bean](env,"SupportBean").Query(StatementName("s0"), WithContext("C1"), StatementPriority(0))`
- Deploy order: advance-time BEFORE deploys (Java order); Go creates temporal partitions lazily — verified.
- The invalid probe builds the same insert WITHOUT `WithContext` → Go now rejects with
  `ErrorInvalidRule` containing `has been declared for context`; record the pinned Java prefix
  (build-error convention from context_key_segmented_invalid.go: verify Go rejection code+substring,
  then record `step.ExpectError`).

## Allowed files (asset lane)

- `tools/java-oracle/ContextInitTermPrioritizedScenarioOracle.java` (new)
- `tools/java-oracle/run-context-init-term-prioritized.sh` (new)
- `testdata/parity/context-init-term-prioritized.json` (new)
- `internal/app/parity/context_init_term_prioritized.go` (new)
- `internal/app/parity/run.go` (mode wiring only: `context-init-term-prioritized` + `-diff`)

## Forbidden

- `internal/esper/**` (shared core — primary agent owns; fixes already landed)
- `internal/compat/**`, manifest, roadmap, CHANGELOG, PLANS.md
- No tests, no formatter, no commit/push, no hand-authored traces/evidence.

## Acceptance

- Oracle replays both cases and emits a trace; runner replays the same scenario.
- `build-error` step records the pinned Java prefix after verifying the Go rejection.
- Listener records carry `case`/`operation`/`statement`/`sequence`/`time`/`new`/`old` matching
  the established TraceWriter shape.
