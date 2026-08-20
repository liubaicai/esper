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
- Baseline: `master` is clean and matches `origin/master` at the pushed logical
  parity checkpoint; the next semantic change must preserve that baseline.
- Status: coalesce implementation, oracle assets, differential replay, and
  focused validation are complete; central facts are updated, focused review
  is complete, and all required local milestone gates pass; semantic delivery
  remains.
- Current work unit: `expr.core` / `case.expr-core-coalesce`
- Exact next action: review the final diff and deliver one semantic commit,
  then verify the pushed remote ref without tracked edits.
- Worktree notes: the baseline files are unchanged; only the expected coalesce
  implementation, parity assets, and this checkpoint are modified/untracked.
  Do not modify `/root/app/esper` or `goal.txt`.

## Delegation checkpoint

- Collaboration facility: available; concurrent scout fan-out was attempted
  before implementation.
- Java oracle scout: `01a01dca-a6af-7f73-beea-ca22badc21ac` (`Einstein`),
  read-only task started concurrently but remained running through repeated
  waits and an interrupt/wrap-up request; it was closed without a report. The
  fixed source was independently read locally, so no Java contract is inferred
  from the missing agent output.
- Go surface scout: `01a01dca-a6ea-7ef2-bf8c-10972df33778` (`Kepler`), report
  received and reviewed. It confirmed the existing `Coalesce`/`CoalesceOf`
  builders, value-state handling, build validation, focused unit coverage, and
  the parity extension points; no production change is indicated statically.
- Parity asset worker: `01a01dd3-d757-7293-925f-5673b1f36438` (`Curie`),
  started after contract freeze but timed out and left no files; the primary
  agent created and reviewed the assets directly.
- Independent parity reviewer: `01a01de7-74f9-7900-8819-da17795b881d`
  (`Zeno`), started after targeted validation but remained running through an
  interrupt/finalization request and was closed without a report. The primary
  agent completed the same read-only parity/evidence review and found no
  blocking finding. A second concise review request was sent to the previously
  available `01a01da9-3bba-7973-a220-6f33a1cccde9` (`Rawls`), which also timed
  out and was closed without a report.
- Serial exception: none for the scout gate. The Java scout and later asset
  worker both timed out; the primary agent completed the missing read-only
  inspection/asset work and recorded those outcomes above.

## Work-unit contract

- Capability/subdomain: `expr.core`, `coalesce` value selection and promotion
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreCoalesce.java`;
  `ExprCoreCoalesceBeans` / `java-runtime-6464df255cd4e23a89e7`;
  `ExprCoreCoalesceLong` / `java-runtime-0a352fd0c30e1b3a175c`;
  `ExprCoreCoalesceLongOM` / `java-runtime-0ee4f2c3f1d9129e2598`;
  `ExprCoreCoalesceLongCompile` / `java-runtime-7cac5278b06087f8e7f2`;
  `ExprCoreCoalesceDouble` / `java-runtime-9341a1983c1f4fb3fdda`;
  `ExprCoreCoalesceNull` / `java-runtime-c9475d47ffbc6275e330`;
  `ExprCoreCoalesceInvalid` / `java-runtime-32ea122af7f0205bcb19`.
- Differential scope: the six observable executions above through
  `ExprCoreCoalesceNull` are replayable; `ExprCoreCoalesceInvalid` is retained
  as implemented-only because the current trace protocol has no compile-error
  record. Its Go Build-time invalid-builder tests remain required evidence for
  the boundary, but it is not included in differential runtime metadata.
- Observable contract: coalesce returns the first non-null/non-missing
  operand, preserves the original bean/event identity, promotes boxed numeric
  operands to the widest required result (`long` and `double` cases), returns
  null when all operands are null, and rejects fewer than two operands or
  incompatible/narrowing operands at build/compile time. Field order, null
  values, result type where exposed, event identity, and error phase are
  observable. Java EPL/SODA/compile syntax, exact boxed metadata, and parser
  diagnostics are outside the typed fluent API. The replay must use fixed
  inputs and exercise normal values, explicit nulls, missing values, typed nil
  pointers, bean identity, numeric promotion, and invalid-builder behavior.
- Allowed production files: existing typed coalesce code under `internal/esper`
  only if replay proves a semantic regression; prefer no production change.
- Allowed Go test/parity files: `internal/app/parity/expr_core_coalesce.go`,
  focused additions to `internal/esper/expr_coalesce_test.go`, and minimal
  dispatcher/test additions in `internal/app/parity/run.go` and `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreCoalesceScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-coalesce.sh`, and
  `testdata/parity/expr-core-coalesce.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused
  `go test ./internal/esper -run 'Coalesce' -count=1`; focused parity replay
  and mutation tests; and
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
- 2026-08-20: The selected unit is the implemented but unverified coalesce
  expression case. The fixed Java source has bean, long-promotion, object-model,
  compile, double-promotion, all-null, and invalid executions; the six
  observable executions are the differential scope and the invalid execution
  remains implemented-only because build errors have no trace record.
- 2026-08-20: The existing typed `Coalesce`/`CoalesceOf` surface satisfied the
  six replayable executions without production changes. The scenario covers 22
  records across beans, long promotion, object-model/compile variants, double
  promotion, and all-null behavior.
- 2026-08-20: Pinned Java and Go traces compare with zero differences. The
  checked-in trace SHA-256 is
  `9ff55902a543bc9d61d700ca06fae21a5f64afc78d18387badec2be669c97b3c`.
- 2026-08-20: The independent reviewer timed out and was closed without a
  report; primary read-only review found no blocking parity issue. Manifest,
  roadmap, and CHANGELOG now record the six-runtime promotion. The validator
  confirmed runtime association totals remain 3,067 / 2,901 / 1,235 because
  all seven inventory runtimes were already associated before promotion.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839b`; `master` is clean and matches
  `origin/master` at `cf92134a0a6c14b079e7da8ffcf5b60084b2db45`. Manifest
  summary reports 522 cases, 125 differential-verified cases, 381
  differential runtimes, 2,901 referenced runtimes, and 1,235 unreferenced
  runtime IDs. The target case is implemented-only with seven runtime IDs and
  existing Go unit coverage; no differential trace or evidence is present.
- Current unit result: the Go scout found no static production gap, the Java
  scout timed out and was closed, and the six-execution differential boundary
  was frozen. Oracle/scenario assets, replay traces, evidence, and focused
  mutation checks now pass; central metadata is consistent and focused review
  found no blocking issue. The pinned Java replay is byte-identical to the
  checked-in trace; `go vet ./...`, `go test ./... -count=1 -timeout 240s`,
  `make check`, focused `go test -race ./internal/app/parity ./internal/esper
  -count=1 -timeout 600s`, manifest/evidence checks, and `git diff --check`
  all pass.

## Delivery

- The previous unit is committed and pushed to `origin/master`; Git history is
  the authoritative source for its commit identity. This unit may be committed
  only after independent review and all required gates pass.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
