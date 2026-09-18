# Draft 4.457 — context-category

Born-differential case `case.context-category` covering all 9 ContextCategory
executions. Java oracle fixed at /root/app/esper @ 9e1b9f1cc9117fea4bf33ab043762c045d73839c.

## Shared-core change (already applied by primary)
Category contexts now fan out: `partitionsForEvent` returns every matching
`category:<name>` key; `partitionRuntime` signals fan-out for
ContextCategorySegmented; `processContextCategoryFanOut` dispatches in
declaration order; `filterEventsForPartition`, `contextJoinPartitionEvents`,
FAF grouped paths, and `processNamedWindowContextLocked` use the plural form.
Residual gap (documented): context-bound named-window INSERT routes to the
first matching partition only — unexercised by these executions.

## Frozen contract (scout NextJavaContract457)
Shared semantics: `create context X group <bool-expr> as <label>, ... from
<EventType>` eagerly allocates one partition per declared group at deploy
(ids 0..n-1 in declaration order). Each event is evaluated against EVERY
group; processed in each matching partition, none if no match.
`context.label`/`context.name`/`context.id` per partition. All events
SupportBean(theString, intPrimitive).

- ord 0 SceneOne `java-runtime-edb899dd318dc4e8711e`: EPL with trailing space
  after `cat2 `; admin assertions (statement names, nesting level, partition
  ids {0,1}, count 2, statement props CONTEXTNAME/CONTEXTDEPLOYMENTID);
  sends A/B/C with count(*) + context.label; C matches nothing.
- ord 1 SceneTwo `java-runtime-440efde2e13b0063a968`: `group by` spelling;
  partition identifiers ContextPartitionIdentifierCategory, labels
  {cat1,cat2}; fields c1..c5 incl. context.name/id; sum per category.
- ord 2 WContextProps `java-runtime-cbe8b6c2ba86887f6807`: 3 categories
  (between 10 and 20 inclusive); filterSvcCountApprox==3, 3 eager agent
  instances; iterators over all partitions incl. empty (null sum);
  undeploy → counts 0.
- ord 3 BooleanExprFilter `java-runtime-ad038edc0893e86eba46`: two
  deployments shared path; `like 'A%'` patterns; CONTEXTDEPLOYMENTID is the
  ctx deployment (different module).
- ord 4 ContextPartitionSelection `java-runtime-fe484daf28031390497b`:
  iterator-only (no listener); keepall + group by theString; selector
  assertions: ById, ByCategory, filtered category selector, null/empty sets;
  Segmented selector → InvalidContextPartitionSelector prefix.
- ord 5 SingleCategorySODAPrior `java-runtime-96843edb4ce366e1d74e`: single
  category; prior(1) per-partition; non-matching event does NOT update prior;
  SODA round-trip = redeploy + resend (Go equivalent: second deploy cycle).
- ord 6 Invalid `java-runtime-51b59dd76ca097972003`: three compile-error
  prefixes (bad filter prop, non-boolean predicate, statement stream type not
  in category context).
- ords 7/8 DeclaredExpr isAlias=false/true `java-runtime-629da2acd413b0688d68`
  / `java-runtime-76b9f0c6ca0a53be2ab0`: declared expressions resolving
  context.label; script-call form vs alias form.

## Approved differences
- ord 5 SODA round-trip maps to a second deploy+send cycle (Go has no EPL
  text/model surface).
- ord 6 pins Go error wording; Java prefixes asserted oracle-side.
- ord 4 selector-error record pins Go wording (Java prefix oracle-side).
- Admin/filter-service counts (filterSvcCountApprox, instance counts) map to
  Go admin equivalents where they exist; otherwise oracle-only.

## Files
- Scenario: testdata/parity/context-category.json
- Oracle: tools/java-oracle/ContextCategoryScenarioOracle.java +
  run-context-category.sh
- Runner: internal/app/parity/context_category.go + run.go wiring
- Traces/evidence: testdata/parity/context-category.{trace,go.trace,evidence}.json
- Manifest: new case.context-category born-DV with all 9 runtime IDs
