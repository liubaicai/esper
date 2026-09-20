# Contract - Draft 4.485 `infra-nwtable-on-merge-invalid-insertonly`

Java oracle fixed at /root/app/esper commit 9e1b9f1cc9117fea4bf33ab043762c045d73839c. NEVER modify.

## Unit
InfraNWTableOnMerge ordinals 40-45: InfraInvalid{nw,table} (ords 40-41, 13
tryInvalidCompile probes each) + InfraInsertOnly{namedWindow=true} x 4 variants
(ords 42-45: useEquivalent / plain / useColumnNames / soda). A born-DV unit.

Runtime IDs (java-execution-inventory.jsonl lines 2813-2818, all flags []):
- ord 40 `java-runtime-99b2d413519187b56232` InfraInvalid{namedWindow=true}
- ord 41 `java-runtime-33b1e837bc8b373bb198` InfraInvalid{namedWindow=false}
- ord 42 `java-runtime-651148621e89ec5465b0` InfraInsertOnly{namedWindow=true,useEquivalent=true,soda=false,useColumnNames=false}
- ord 43 `java-runtime-e51caa89fbde4ab34197` InfraInsertOnly{namedWindow=true,useEquivalent=false,soda=false,useColumnNames=false}
- ord 44 `java-runtime-8900ac7e3d063d8b8842` InfraInsertOnly{namedWindow=true,useEquivalent=false,soda=false,useColumnNames=true}
- ord 45 `java-runtime-b581753558f219b16a2a` InfraInsertOnly{namedWindow=true,useEquivalent=false,soda=true,useColumnNames=false}

## Java contract (frozen by NextJavaContract485 + NextGoSurface485)

### InfraInvalid (ords 40-41, InfraNWTableOnMerge.java:814-895)
Fixture: ONE multi-statement deploy via env.compileDeploy(epl, path), byte-exact
per variant:
- nw (ord 40): `@public create window MergeInfra#unique(theString) as SupportBean;\ncreate schema ABCSchema as (val int);\n@public create window ABCInfra#keepall as ABCSchema;\n`
- table (ord 41): `@public create table MergeInfra as (theString string, intPrimitive int, boolPrimitive bool);\ncreate schema ABCSchema as (val int);\n@public create table ABCInfra (val int);\n`

Then tryInvalidCompile(path, epl, prefix) probes — ALL compile WITH the runtime
path (no compileWithoutPath). assertMessage is a PREFIX match (startsWith) for
messages >10 chars; several pinned values end mid-text at `[`. The 13 probes
per variant are listed in the scout report (agent://NextJavaContract485);
transcribe EPL + prefix verbatim. Notable divergences:
- probe 2: nw vs table produce DIFFERENT messages (event-type-declared vs
  column-assignment wording).
- probe 4 is NW-ONLY (matched-insert select * type mismatch); the table variant
  has a different probe at that position — transcribe per-variant.
- probes 3/5/6/7 are parser errors (Incorrect syntax / Unexpected end-of-input);
  pin verbatim including the odd column numbers.
- probe 1: not-matched filter may not reference named-window-event properties.
- probes 8-13 cover ambiguous where-clause property, invalid select-clause
  property, match-where-clause property, insert-into unknown target,
  event-type-identity assignment, and subquery-correlated-to-target.

### InfraInsertOnly (ords 42-45, InfraNWTableOnMerge.java:464-528)
Fixture: `@Name('Window') @public create window InsertOnlyInfra#unique(p0) as
(p0 string, p1 int)` (namedWindow=true for all four). Then ONE merge statement
per variant:
- ord 42 (useEquivalent): `on SupportBean merge InsertOnlyInfra where 1=2 when
  not matched then insert select theString as p0, intPrimitive as p1`
- ord 43 (plain): `on SupportBean merge InsertOnlyInfra insert select theString
  as p0, intPrimitive as p1`
- ord 44 (useColumnNames): `on SupportBean as provider merge InsertOnlyInfra
  insert(p0, p1) select provider.theString, intPrimitive`
- ord 45 (soda): same EPL as ord 43 but compiled via the soda path — in Go this
  is the same fluent chain; the scenario pins the EPL text and the Java oracle
  uses compileDeploy(soda=true, epl, path).

Observable per variant: deploy Window + on, send SupportBean("E1",1), snapshot
{E1,1}, listener 'on' new {p0=E1,p1=1}, milestone, send SupportBean("E2",2),
snapshot {E1,1},{E2,2}, listener 'on' new {p0=E2,p1=2}, undeployAll. The
assertSame(windowType, onType) check is a Java-internal identity assertion —
not observable in the trace; the runner may assert the Go equivalent
(statement event type) if a cheap accessor exists, else skip.

## Go surface (frozen by NextGoSurface485)
- Every clause family already exists in internal/esper/trigger.go:
  MergeIntoNamedWindowWhen (predicate match, nil allowed), WhenNotMatchedAny /
  WhenNotMatched, ThenInsertInto / ThenInsertIntoTarget, SetColumn assignments.
  Insert-only merge = MergeIntoNamedWindowWhen(name, nil-or-false-predicate,
  WhenNotMatchedAny(...)) — the 'simple' cases in infra_nwtable_on_merge.go
  already run this exact form.
- Constant-false match predicate (ord 42 `where 1=2`): Literal(false) as the
  named-window match Expression[bool].
- Explicit insert column lists (ord 44 `insert(p0,p1) select`): SetColumn
  assignments — Java's insert(col,...) select is sugar.
- InfraInvalid compile-error probes replay through the established build-error
  op (internal/app/parity/context_key_segmented_invalid.go precedent): pinned
  EPL map per probe label, fluent Build/registration attempt, errors.As(
  *esper.Error) code+substring gate for expressible probes, unrepresentable
  probes pin expectError prefix verbatim; records Operation="compile-error",
  Value=step.ExpectError. Strict scenario loader with per-op field whitelists.
- Two contract-freeze decisions for InfraInvalid: (a) event-type-identity
  assignment check (probe 13) and (b) subquery-correlated-to-target not-matched
  filter (probe 1) — check whether Go's Build produces equivalent diagnostics;
  if not, pin expectError prefix verbatim as unrepresentable (documented).
- compat.Step already supports build-error / unrepresentable / value /
  deployed / snapshot ops — no internal/compat changes.

## Files (asset worker)
- tools/java-oracle/InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle.java (new)
- tools/java-oracle/run-infra-nwtable-on-merge-invalid-insertonly.sh (new)
- testdata/parity/infra-nwtable-on-merge-invalid-insertonly.json (new)
- internal/app/parity/infra_nwtable_on_merge_invalid_insertonly.go (new)
- internal/app/parity/run.go (add mode + -diff only)

## Forbidden
Do NOT modify internal/esper, internal/compat, manifest, docs, or the
checked-in infra-nwtable-on-merge* files. Do NOT run tests or formatters.

## Validation (primary agent)
Java trace via run script; `-mode infra-nwtable-on-merge-invalid-insertonly-diff`
zero-diff; run_test.go pins; make check; parity review; commit/push.
