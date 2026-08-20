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
- Baseline: `master` is clean and matches `origin/master` at the current
  pushed checkpoint; the next semantic change must preserve that baseline.
- Status: reviewed findings are repaired; Maxwell's follow-up review found no
  findings, and the fixed Java oracle and Go replay now match at 68 records
  with zero differences. All required local gates pass; delivery remains.
- Current work unit: `expr.core` / `case.expr-dt-between`
- Exact next action: review the final diff, create one semantic commit, push
  `master`, and verify the remote ref without tracked edits.
- Worktree notes: work-unit files and central facts are intentionally dirty;
  no unrelated user changes are present. Do not edit tracked files after the
  semantic commit merely to add its hash.

## Delegation checkpoint

- Collaboration facility: available; first scout fan-out completed
  concurrently for this work unit.
- Java oracle scout: `01a01d28-670f-7822-aba7-fdc3d44d1cff` (`Gibbs`),
  read-only report received. The three executions produce the scoped listener
  rows later expanded to 68 by the replay's supplemental null/missing cases;
  inclusive bounds normalize reversed constants, while runtime endpoint-flag
  expressions retain comparison-position ordering. Long, Date, Calendar,
  LocalDateTime, and ZonedDateTime compare by the same UTC epoch millisecond.
- Go surface scout: `01a01d28-6752-7b32-a8ea-106b494e83e2` (`Ramanujan`),
  read-only report received. Existing `CurrentTimestamp`, typed fields,
  `VariableRef[bool]`, and `BetweenRangeOf` support the work; the scout
  recommends a focused `expr_dt_between.go` with private epoch normalization,
  typed endpoint expressions, and no shared `value.go` change. No serial
  fallback was used.
- Parity asset worker: `01a01d36-450f-7b13-9510-4d3dbffcbd6d` (`Avicenna`),
  bounded asset task was interrupted after repeated waits with no patch
  returned or visible process. The primary agent assumes the exact asset
  files below and records this as a worker-continuity exception; no semantic
  scope is expanded.
- Parity reviewer: `01a01d4e-620f-73e2-8187-2bdf2c85f402` (`Fermat`),
  read-only review completed with findings: static constant endpoint flags
  must normalize reversed bounds for all four inclusivity combinations; the
  parity test must not derive its Java fixture from persisted evidence; the
  type replay should model Java's latest-bound lifecycle; and null/missing
  inputs need differential exercise or a narrower manifest claim.
- Review resolution: static constant endpoint flags now normalize reversed
  bounds for all four inclusivity combinations; the differential test loads
  the independently generated Java trace artifact rather than evidence; the
  exclude cases share one Go engine and deployment lifecycle, the type case
  uses a unidirectional `SupportDateTime` plus `SupportBean#lastevent` join,
  and the scenario/oracle cover null and missing date-time fields and a null
  endpoint flag. The checked-in evidence test now compares that independent
  Java trace and a fresh Go replay, while map-backed adapters preserve absent
  fields as Missing instead of collapsing them into explicit nulls.
- Follow-up parity reviewer: `01a01d73-f002-7260-841d-c89b606f4242`
  (`Maxwell`), read-only follow-up completed with no findings; evidence
  linkage, map-backed null/missing handling, manifest consistency, and the
  68-record zero-difference artifacts are delivery-ready.
- Serial exception: asset-worker continuity failure after delegation; the
  primary agent writes only the worker's pre-frozen oracle/runner/scenario
  files and will review them before generating traces.

## Work-unit contract

- Capability/subdomain: `expr.core`, date-time `between`
- Java source and executions/runtime IDs:
  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/datetime/ExprDTBetween.java`;
  `ExprDTBetweenIncludeEndpoints` /
  `java-runtime-6593e0f0cc79ed906f52`;
  `ExprDTBetweenExcludeEndpoints` /
  `java-runtime-5e744f4720368eb6585b`;
  `ExprDTBetweenTypes` / `java-runtime-c0d2bebfa077f7e51474`.
- Observable contract: fixed Java `ExprDTBetween` evaluates inclusive and
  independently configurable exclusive endpoint checks against virtual
  current time, supports reversed bounds and boolean variables for endpoint
  flags, and compares the same instant across epoch-millisecond, date, and
  calendar-like representations. The replay must cover before/at/inside/after
  boundaries, both endpoint flags, variable-backed flags, reversed bounds,
  mixed value/bound representations, and projected boolean field order. Go
  uses typed fluent expressions; Java EPL/SODA/model syntax is not a Go API.
- Allowed production files: the existing typed between implementation and
  directly adjacent date-time comparison helpers under `internal/esper`
  (prefer a new focused file); modify shared runtime code only if the frozen
  replay proves a real regression.
- Allowed Go test/parity files: focused `internal/esper` date-time between
  tests, `internal/app/parity/expr_dt_between.go`, and minimal dispatcher/test
  additions in `internal/app/parity/run.go` and `run_test.go`.
- Allowed parity asset files: `tools/java-oracle/DTBetweenScenarioOracle.java`,
  `tools/java-oracle/run-dt-between.sh`, and
  `testdata/parity/dt-between.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused
  `go test ./internal/esper -run 'ExprDTBetween|Between' -count=1`; focused
  parity replay and mutation tests; and
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
- [x] Repair review findings, regenerate independent traces/evidence, and rerun
      targeted validation.
- [x] Review the final diff, record actual validation, create one semantic
      commit, push `master`, and verify the remote ref without tracked edits.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.
- 2026-08-20: The next selected unit is the unlinked Java `ExprDTBetween`
  trio, adjacent to the verified `ExprDTIntervalOps` surface. Its contract
  requires date-time-to-epoch normalization, endpoint flags, reversed-bound
  behavior, and mixed date-time representations.
- 2026-08-20: The typed Go date-time builders, public facade, parity runner,
  fixed Java oracle assets, and focused tests now cover the frozen contract.
  The pinned oracle and Go replay emit 68 listener records including the
  supplemental null/missing cases; differential evidence is passing with zero
  differences and value/order/field/time mutations rejected. The replay uses
  map-backed schemas where omitted properties remain Missing.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` is clean and matches
  `origin/master` at the current pushed checkpoint before this work unit.
  Manifest summary now reports 522 cases, 124 differential-verified cases,
  378 differential runtimes, 2,901 referenced runtimes, and 1,235
  unreferenced runtime IDs.
- Previous work-unit result: `case.expr-core-current-evaluation-context` is
  persisted as `differential-verified` with two runtime IDs, zero-difference
  evidence, mutation rejection, complete local gates, review, commit, and push.
- Independent parity review on 2026-08-20 initially found static reversed
  constant bounds, circular evidence-derived Java fixtures, an inaccurate
  type/lifecycle replay, and unexercised null claims. After repair, the pinned
  Java runner and Go replay emit 68 matching records with zero differences;
  static reversed bounds cover all four endpoint policies, exclude cases share
  one runtime lifecycle, types use the unidirectional latest-bound join, and
  null/missing input cases are replayed. The mutation suite loads the
  independently generated Java trace artifact, and the checked-in evidence
  test verifies both persisted traces against independent/current sources.
- Follow-up review on 2026-08-20: Maxwell reported no findings after the
  evidence-linkage and map-backed null/missing repairs.
- Current unit result: `testdata/parity/dt-between.trace.json` and
  `dt-between.evidence.json` contain the regenerated pinned Java/Go replay and
  zero differences across 68 records for the three Java executions plus the
  null/missing supplemental cases. Focused Esper/parity tests,
  compatibility/manifest tests, layout, whitespace, `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, and
  `go test -race ./internal/app/parity ./internal/esper -count=1
  -timeout 600s` all pass. The checked-in evidence linkage test and direct
  null-versus-missing decoder test also pass.

## Delivery

- The previous unit is committed and pushed to `origin/master`; Git history is
  the authoritative source for its commit identity.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
