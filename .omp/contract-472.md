# Draft 4.472 — context-key-segmented-allocation-time

Java oracle: /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c (verified).
Source: regression-lib/.../suite/context/ContextKeySegmented.java. All three
executions flags []. Semantic cluster: partition-allocation-time behavior.

## Scope (3 executions)

### Ord 25 ContextKeySegmentedWPatternFireWhenAllocated — DV
- runtime `java-runtime-57199db349abfe7ad70e`, static `java-268501a6a96134271fc9`.
- Module EPL (byte-exact):
  `create context MyContext partition by theString from SupportBean;`
  `@name('s0') context MyContext select context.key1 as key1 from pattern[timer:interval(0)];`
  `context MyContext create variable String lastString = null;`
  `context MyContext on pattern[timer:interval(0)] set lastString = context.key1;`
- Semantics: timer:interval(0) fires synchronously AT partition allocation
  (not via advanceTime); s0 delivers {key1:allocating-key}; the on-pattern
  trigger sets per-partition variable lastString=key in the same allocation.
  Later events/milestones for the same key produce nothing. New key → new
  partition → fires again.
- Sends: SB(E1,0) -> s0 {key1:E1} + lastString[E1]=="E1"; SB(E1,1) -> none;
  milestone; SB(E1,1) -> none; SB(E2,0) -> {key1:E2} + lastString[E2]=="E2".
- NEW SURFACES REQUIRED:
  a) `OnPattern(pattern PatternStream)` trigger source feeding
     SetVariable/SetVariables (triggerDefinition gains a pattern field;
     trigger input nil).
  b) Context-typed events allocate partitions for statements whose pattern
     has no event inputs (timer-only patterns) — both select-from-pattern
     and on-pattern statements.
  c) Fire-on-allocation: after partitionRuntime initializeAt, a due pattern
     timer (timerNext <= now) emits immediately — select produces a Result,
     trigger executes its action.
  d) Variable readback: ContextVariableStates + SelectContextPartitionSegments
     (exists; runner op `read-variable`).

### Ord 28 ContextKeySegmentedRegExFilter — DV
- runtime `java-runtime-9356c517931472be6cad`, static `java-17286d97b0a5fbf0b0a3`.
- EPL: `@public @buseventtype create schema MyEventWPartition as (number int,
  description string, partitionId string);` + `create context MyContext
  partition by partitionId from MyEventWPartition terminated after 15 minutes;`
  + `context MyContext select * from MyEventWPartition(description like "%hello%");`
- Semantics (PR #285 regression): the ALLOCATING event is evaluated against
  the statement filter; matching allocating event outputs. `terminated after`
  never fires (no time advance) but must compile on a segmented context.
- NEW SURFACE: `terminated after <duration>` on segmented contexts —
  ContextDefinition gains terminatedAfter; expiry in the context-expire path
  (partition age > duration → deallocate). Compile-only for this scenario but
  implement the runtime path, not a stub.
- Like() exists; allocating-event filter evaluation already correct.

### Ord 6 ContextKeySegmentedSubtype — DV (reclaims dangling runtime ID)
- runtime `java-runtime-820bb6f72b84ad070ce4`, static `java-f9f4b6fe3adcdae63ef4`.
- EPL: `create context SegmentedByString partition by baseAB from
  ISupportBaseAB;` + `context SegmentedByString select count(*) as col1 from
  ISupportA;`
- Hierarchy: ISupportBaseAB{baseAB} <- ISupportA{a} <- ISupportAImpl.
- Sends: milestone; Impl(A1,AB1)->{col1:1}; Impl(A2,AB1)->{col1:2}; milestone;
  Impl(A3,AB2)->{col1:1}; Impl(A4,AB1)->{col1:3}.
- ENGINE FIX: contextKeysForEvent must resolve streamKeys through the event
  schema's parentNames (subtype event allocates via supertype-declared key).
  WithSchemaParent + acceptsEventType already route the subtype to the
  supertype-typed statement source.

## Files
- Shared core (primary agent only): internal/esper/context.go
  (terminatedAfter field + option + contextKeysForEvent parent walk),
  trigger.go (OnPattern + pattern triggerDefinition), runtime.go
  (context allocation for input-less pattern statements, fire-on-allocation,
  trigger-pattern expire path, terminatedAfter expiry), plan.go (validation).
- Parity assets (asset writer, disjoint): scenario(s)
  testdata/parity/context-key-segmented-allocation-time.json (3 cases),
  oracle + run script, runner internal/app/parity/, run.go/run_test.go wiring.
- Manifest: one DV case (3 runtime IDs) under context.partition.

## Validation
- diff mode passing/0 differences; mutation family; make check.
