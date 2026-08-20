# Esper Go migration active ExecPlan

This is the resumable plan for Codex migration work. Keep it accurate while a
work unit is in progress. It is an operational checkpoint, not a second
roadmap or an append-only history.

## Plan maintenance

- At task start, verify this checkpoint against Git, the manifest, evidence,
  and the roadmap. Memory or an older conversation is not current evidence.
- Before editing production code, fill in the work-unit contract and mark the
  first incomplete progress item.
- After investigation, implementation, validation, review, or a material
  decision, update the relevant section with concrete paths and commands.
- Keep only the current work unit and at most five one-line recent outcomes.
  Detailed completed history belongs in CHANGELOG and Git.
- Maintain the delegation checkpoint for the active unit. Parallel shell
  commands are not subagent delegation. Before implementation, record the
  Java/Go scout agent IDs or the exact serial-fallback reason; after targeted
  validation, record the independent reviewer ID and result.
- On interruption, leave the worktree state, exact next action, failures, and
  unverified assumptions explicit enough for a new Codex task to resume.
- Record validation and outcome before the semantic commit. Never edit this
  file after committing only to add the new commit hash; Git owns commit
  identity and post-push verification is read-only.

## Outcome

Continue the fixed-commit Esper 9.0.0 to Go migration until the final
acceptance criteria in `docs/esper-go-port-quality-strategy.md` all pass.
Progress is measured by verified work units and manifest evidence, not agent
activity or a single coverage percentage.

## Active checkpoint

- Updated: 2026-08-20
- Baseline: `master` is clean and matches `origin/master` at pushed commit
  `033786cbd`; the next semantic change must preserve that baseline.
- Status: equality/identity implementation, checked-in parity assets, and
  zero-difference replay are complete for the four frozen listener executions.
  The invalid compile execution remains outside differential replay. Central
  facts, independent review, and complete local gates are complete; the
  semantic commit and push are the only remaining delivery steps.
- Current work unit: `expr.core` / `case.expr-core-equals-is`
- Exact next action: audit the staged final diff, create one semantic commit,
  push `master`, and verify the remote ref read-only.
- Worktree notes: equality implementation, parity runner/tests/assets, central
  manifest/docs, and this checkpoint are modified or untracked as expected.
  Do not modify `/root/app/esper` or `goal.txt`; previous commit identity is
  owned by Git.

## Delegation checkpoint

- Collaboration facility: available; current scouts were sent concurrently
  through the collaboration tools before implementation.
- Java contract scout: Russell `01a01e4f-3b78-7e72-81df-74beac83c9a5`, current
  submission `01a01ed4-bb07-7cf3-b7ec-1be78410ad84`; completed. Frozen source
  order 1/2/3/5, runtime IDs, exact scalar/array/Null vectors, fresh
  deployment lifecycle, no timers/windows/old stream, and compile-error
  exclusion for `ExprCoreEqualsInvalid`.
- Go surface scout: Kierkegaard `01a01e4f-3cd5-7eb0-a713-5b17da488e25`, current
  submission `01a01edd-9aae-7082-af1b-47c408fae777`; completed. Confirmed
  reusable `EqualOf`/`NotEqualOf`/`Is`/`IsNot`, deep slice equality and typed
  nil behavior; identified a literal type-distinction risk in description-based
  Plan identity and missing direct parity coverage for the four Java cases.
- Independent parity reviewer: `01a01ea1-56e9-7f10-bbef-105d7f21b48e`
  (`Meitner`) completed the current post-targeted-validation review with no
  findings. Residual risk is limited to Java-side semantic rather than raw-JSON
  lexical validation and broader object-array coercion coverage; no mismatch is
  observable in the current eight-record replay.
- Previous IN/BETWEEN scouts/review: retained in Git history; that unit was
  pushed as `033786cbd`.
- Serial exception: none; both required equality scouts were sent through the
  collaboration facility with disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, scalar equality/identity operators,
  coercion, arrays, and Null semantics
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreEqualsIs.java`;
  `ExprCoreEqualsIsCoercion` / `java-runtime-fdef3bed6ec0b16d36db`;
  `ExprCoreEqualsIsCoercionSameType` / `java-runtime-1eaef3b328c31a863b26`;
  `ExprCoreEqualsIsMultikeyWArray` / `java-runtime-2fb582ea3ac2dc026c82`;
  `ExprCoreEqualsInvalid` / `java-runtime-bc33c9283b85c18481fb`;
  `ExprCoreEqualsNull` / `java-runtime-d6084d5b7a191cbde6cd`.
- Differential scope: four ordered isolated cases named `equals-coercion`,
  `equals-same-type`, `equals-array`, and `equals-null`, matching source order
  executions 1/2/3/5. Each case uses a fresh deployment and emits one new row
  per send in send order; no timers, windows, old-stream rows, or time advances.
  `ExprCoreEqualsInvalid` remains a compile/build boundary and is not assigned a
  listener trace or differential runtime claim.
- Observable contract: boxed nullable Boolean result fields in exact `c0...`
  projection order; compatible int/long numeric equality; same-type string
  equality; deep primitive/boxed/two-dimensional/object-array content and shape
  equality; SQL-style three-valued `=` versus Null-safe `is`; explicit Null
  property vectors; and fresh deploy/undeploy lifecycle isolation.
- Allowed production files: `internal/esper/expr_equals.go` and, only if
  confirmed by the focused identity regression, the smallest shared literal/
  plan identity helper under `internal/esper`.
- Allowed Go test/parity files: a new
  `internal/app/parity/expr_core_equals_is.go`, focused additions under
  `internal/esper`, and minimal dispatcher/test additions in `run.go` and
  `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreEqualsIsScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-equals-is.sh`, and
  `testdata/parity/expr-core-equals-is.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused equality/identity
  Esper tests; focused parity replay and mutation tests; and
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: changed-file `gofmt`, `go vet ./...`,
  `go test ./... -count=1`, manifest/evidence validation, `make check`, and
  `git diff --check`; the focused expr race gate remains applicable.

## Progress

- [x] Reconfirm baseline, roadmap priorities, manifest summary, and relevant
      existing gates.
- [x] Select the next natural work unit from the `expr.core` manifest gap.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands using read-only investigation.
- [x] Run concurrent Java/Go read-only scouts and record their agent IDs and
      conclusions.
- [x] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [x] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [x] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts changed.
- [x] Run independent parity review, resolve findings, and run complete local
      gates plus the applicable milestone gate.
- [x] Review the final diff and record actual validation.
- [ ] Create one semantic commit, push `master`, and verify the remote ref
      without tracked edits.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The previous coalesce unit was delivered as `96a2aadf3`, with
  six zero-difference runtimes and 22 replay records; its invalid compile case
  remains implemented-only.
- 2026-08-20: `case.expr-core-relop` was delivered as `a448dbd4f` with two
  zero-difference runtime IDs and 30 replay records. Its scenario-shape review
  tightened exact case order and send counts.
- 2026-08-20: `case.expr-core-like-regexp` reached zero-difference replay and
  was delivered as `896ed1312`; the full-match fix and Java-LIKE test-modeling
  correction passed the complete local gates.
- 2026-08-20: Fresh concurrent IN/BETWEEN scouts Russell/Kierkegaard froze a
  safe five-execution scalar slice and identified the endpoint-policy Plan
  identity defect and nullable replay schema requirement.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records and exact `s0`/`s1`/`s2` range lifecycle; checked-in trace,
  evidence, mutation tests, manifest, roadmap, and CHANGELOG updates are in
  place. The endpoint-policy Plan identity regression is covered by a focused
  Go test.
- 2026-08-20: The first parity review's payload finding was resolved by
  symmetric frozen vectors in the Go runner and Java oracle. Validation now
  requires exact case/send order, event type, field set, and JSON value spelling
  for all 164 inputs, including the ten range pairs; a Go payload mutation test
  covers the rejection path.
- 2026-08-20: Shared differential provenance flags remain an intentional
  runner-level override used by existing modes and are out of scope for this
  unit; checked-in evidence still asserts the exact fixed commit, source,
  execution, and runtime metadata. Result fields remain maps because the
  protocol compares field identity/value structurally and JSON encoding gives
  deterministic lexicographic key order; field-map insertion order is likewise
  out of scope for this unit.
- 2026-08-20: `case.expr-core-in-between` reached zero-difference replay with
  164 records, passed independent review and local gates, and was pushed as
  `033786cbd`; Git is authoritative for that commit identity.
- 2026-08-20: The equality scouts confirmed the reusable typed
  `EqualOf`/`NotEqualOf`/`Is`/`IsNot` surface, deep slice equality, and typed-nil
  behavior. They also identified a Plan identity collision for interface-typed
  primitive literals and the missing direct differential replay.
- 2026-08-20: `case.expr-core-equals-is` implementation added a strict
  four-case scenario validator, fresh per-case Go deployments, and a pinned
  Esper Java oracle. The replay covers eight listener records in exact source
  order; `ExprCoreEqualsInvalid` remains a compile/build boundary.
- 2026-08-20: `Literal[T]` now includes the dynamic concrete type in Plan
  descriptions when `T` is an interface and the value is primitive. The
  focused regression keeps equivalent plans stable while distinguishing
  `Literal[any](1)` from `Literal[any]("1")`.
- 2026-08-20: Targeted validation passed: the pinned Java runner regenerated
  an eight-record trace byte-for-byte equal to the checked-in trace; Go
  differential replay/evidence reported zero differences; focused equality,
  parity, mutation, compat, and manifest tests passed. `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, focused expression and
  parity race tests, JSON/schema checks, shell syntax, layout, and
  `git diff --check` also passed.
- 2026-08-20: Independent parity review by Meitner returned no findings. The
  reviewer confirmed the Java source order, exact payload vectors, fresh case
  lifecycle, trace/evidence metadata, and central manifest references; residual
  risk is limited to semantic-vs-lexical payload validation and broader
  object-array coercion coverage.

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is
  fixed at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and
  matches `origin/master` at `033786cbd`. Manifest summary reports 522 cases,
  129 differential-verified cases, 398 differential runtimes, 2,901
  referenced runtimes, and 1,235 unreferenced runtime IDs. The equality target
  is implemented-only with five inventoried runtime associations and focused
  Go unit coverage; no differential trace or evidence is present at this
  baseline.
- Current unit result: the pinned Java oracle regenerated the checked-in trace
  byte-for-byte. Go differential replay and evidence report eight records and
  zero differences. Focused equality/identity Esper tests, parity replay,
  checked-in evidence, malformed-scenario, mutation, compat, and manifest tests
  pass. Repository-wide vet/test/check and focused race gates pass. JSON,
  layout, shell, replay-equivalence, and diff checks pass. Independent parity
  review returned no findings; its residual risks are documented above.

## Delivery

- The previous unit is committed and pushed to `origin/master` at
  `033786cbd`; Git history is the authoritative source for its identity. The
  equality unit has passed targeted validation, independent review, and all
  required local gates; it is ready for one semantic commit and push.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After the equality commit is pushed,
select the next closed-loop work unit and repeat concurrent scout fan-out.
