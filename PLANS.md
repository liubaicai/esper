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
  `896ed1312`; the next semantic change must preserve that baseline.
- Status: like/regexp is committed and pushed. The scalar IN/BETWEEN contract
  is implemented and differential-verified for the frozen five-execution
  slice; independent parity review and all applicable local gates pass.
- Current work unit: `expr.core` / `case.expr-core-in-between`
- Exact next action: create the semantic commit, push `master`, and verify the
  local and remote refs read-only.
- Worktree notes: expected unit files are modified/untracked; the pinned Java
  runner accepts the current scenario with send counts 22/36/48/48/10. Do not
  modify `/root/app/esper` or `goal.txt`. Previous commit identity is owned by
  Git.

## Delegation checkpoint

- Collaboration facility: available; the fresh IN/BETWEEN scouts were resumed
  and completed concurrently before implementation.
- Java contract scout: `01a01e4f-3b78-7e72-81df-74beac83c9a5` (`Russell`),
  completed. Confirmed source order 0/11/13/14/19, exact runtime IDs
  `791d833152f1266fa139`, `ffe28bce1e8e2a0dc4a0`,
  `143c2fcf5bb1e4e6aa4a`, `1145ffc38eea9ce1624e`, and
  `31934462edbc04c83973`; frozen five-case values, null/negated/range
  contracts, and excluded executions.
- Go surface scout: `01a01e4f-3cd5-7eb0-a713-5b17da488e25` (`Kierkegaard`),
  completed. Confirmed reusable `InOf`/`NotInOf`, `BetweenOf`/
  `NotBetweenOf`, and range builders with no public API drift; identified
  nullable replay schema needs and the endpoint-policy plan identity defect.
- Independent parity reviewer: `01a01ea1-56e9-7f10-bbef-105d7f21b48e`
  (`Meitner`), completed the first read-only review with no lifecycle or
  statement-sequence mismatch. Findings were exact payload-vector validation,
  stale baseline wording, shared provenance override and field-map ordering
  scope decisions, plus focused nullable/negated/reversed/string-range test
  coverage. The first follow-up closed the payload and scope findings, while
  retaining the focused-test P2. The final follow-up completed with no P0, P1,
  or P2 findings after the new runtime coverage; residual risk is limited to
  the documented shared provenance/map-order protocol choices.
- Previous like/regexp scouts/review: retained in Git history; like/regexp was
  pushed as `896ed1312`.
- Serial exception: none; both required IN/BETWEEN scouts were started through
  the collaboration facility with disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, scalar IN/BETWEEN matching and Null
  propagation
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreInBetween.java`;
  `ExprCoreInNumeric` / `java-runtime-791d833152f1266fa139`;
  `ExprCoreInStringExpr` / `java-runtime-ffe28bce1e8e2a0dc4a0`;
  `ExprCoreBetweenStringExpr` / `java-runtime-143c2fcf5bb1e4e6aa4a`;
  `ExprCoreBetweenNumericExpr` / `java-runtime-1145ffc38eea9ce1624e`;
  `ExprCoreInRange` / `java-runtime-31934462edbc04c83973`.
- Differential scope: five ordered isolated cases named `in-numeric`,
  `in-string`, `between-string`, `between-numeric`, and `in-range`.
  The scenario must include Java's numeric/string vectors, explicit null
  values and bounds, negated IN/BETWEEN projections, reversed bounds, all
  four range endpoint policies, reversed range bounds, and string range
  inputs. No timers, windows, old-stream rows, or time advances are expected.
  Collection/object/map/array, substitution, SODA/OM/compile, BigNumber,
  Boolean IN, numeric-coercion variants, and invalid compilation remain
  implemented-only; no runtime IDs are claimed for them.
- Observable contract: Java boxed Boolean result fields, exact projection and
  listener order, Null propagation for IN, false-on-null value/bound behavior
  for BETWEEN, numeric equality/coercion, lexicographic string ranges,
  reversed-bound normalization, fresh per-case lifecycle, and distinct range
  endpoint policies in both values and Plan identity.
- Allowed production files: `internal/esper/expr_in_between.go` only for the
  confirmed endpoint-policy Plan identity fix, plus focused tests in the same
  expression surface.
- Allowed Go test/parity files: a new
  `internal/app/parity/expr_core_in_between.go`, focused additions under
  `internal/esper`, and minimal dispatcher/test additions in `run.go` and
  `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreInBetweenScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-in-between.sh`, and
  `testdata/parity/expr-core-in-between.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused IN/BETWEEN
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
- 2026-08-20: The focused-test P2 was addressed with direct runtime coverage
  for negated endpoint policies, reversed mixed endpoints, string ranges,
  nullable/negated string and numeric expressions, null bounds, and
  deploy/undeploy lifecycle isolation. The new tests are listed in the target
  manifest entry and the final parity review confirmed no remaining findings.

## Validation evidence

- Investigation baseline before this unit on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and matches
  `origin/master` at `896ed1312`. Manifest summary reports 522 cases, 128
  differential-verified cases, 393 differential runtimes, 2,901 referenced
  runtimes, and 1,235 unreferenced runtime IDs. The IN/BETWEEN target is
  implemented-only with 27 inventoried runtime associations and existing Go
  unit coverage; no differential trace or evidence was present at that
  baseline.
- Current unit result: the pinned Java oracle and Go differential replay pass
  with exact payload vectors and 164 byte-identical listener records; checked-in
  trace/evidence identity, malformed-shape and payload rejection, six trace
  mutations, focused expression/lifecycle tests, manifest validation, JSON
  validation, shell syntax, and `git diff --check` pass. The full
  `go test ./... -count=1 -timeout 240s`, `go vet ./...`, and `make check` gates
  pass. The focused expression race selection and
  `go test -race ./internal/app/parity ./internal/compat` pass. A clean baseline
  worktree at `896ed1312` independently reproduces the unrelated
  `internal/esper/join_representation_test.go:271`
  `TestJoinInsertedWrapperRepresentationSupportsNonUniqueKeys` race timeout
  after 45 seconds; the full race gate is therefore recorded as a pre-existing
  baseline failure, not a unit regression. Independent parity review completed
  with no P0, P1, or P2 findings. Final reruns also pass
  `go test ./internal/esper -run 'InBetween|ExprCoreInRange|ExprCoreNullable' -count=1`,
  `go test ./internal/app/parity -run 'ExprCoreInBetween' -count=1`, and
  `go test ./internal/compat ./internal/app/manifest -count=1`; shell syntax,
  JSON, and `git diff --check` were revalidated after the final diff audit.

## Delivery

- The previous unit is committed and pushed to `origin/master` at
  `896ed1312`; Git history is the authoritative source for its identity. The
  IN/BETWEEN unit may be committed only after independent review and all
  required gates pass.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
