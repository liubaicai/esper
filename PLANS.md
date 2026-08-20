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
  `a448dbd4f`; the next semantic change must preserve that baseline.
- Status: relop is committed and pushed. The like/regexp replay is
  zero-difference, independently reviewed, and all local gates pass; the
  semantic commit and push remain.
- Current work unit: `expr.core` / `case.expr-core-like-regexp`
- Exact next action: perform the final staged diff audit, create one semantic
  commit, push `master`, and verify the remote ref without tracked edits.
- Worktree notes: expected like/regexp implementation, parity assets, central
  facts, this checkpoint, and the Java-contract correction in
  `expr_filter_optimizable_or_rewrite_parity_test.go` are modified/untracked;
  do not modify `/root/app/esper` or `goal.txt`. Previous commit identity is
  owned by Git.

## Delegation checkpoint

- Collaboration facility: available; the like/regexp scouts were started
  concurrently before implementation.
- Java oracle scout: `01a01e35-f5b6-7661-8fa8-2162b48b42b8` (`Hypatia`),
  completed. Confirmed the four-execution ordinal 0-3 slice, runtime IDs
  `376f347aa8fc8367fcbc`, `09f15b6eb39cbee59b0d`,
  `7dc8b98263cf5e9e4c3f`, and `8a8794063a0b9c4bfd0e`; ten source-derived
  listener rows with fixed field/order contracts; Java full-match REGEXP,
  numeric formatting, and no timers/old stream.
- Go surface scout: `01a01e35-f46f-77d0-ab28-1a4eb60c567a` (`Volta`),
  completed. Confirmed existing typed Like/Regexp APIs and Null/Missing/
  typed-nil handling; no API-shape change is needed. Identified a likely
  production gap: Go `regexp.MatchString` searches substrings while Java
  `Pattern.matcher(...).matches()` requires a full match; the replay includes
  a distinguishing input before any production fix.
- Independent parity reviewer: `01a01e4f-3e36-7811-97f5-1daffab14ce9`
  (`Erdos`), completed read-only review with one P1 finding. The finding was
  resolved by correcting three unchanged filter parity test expressions from
  `RegexpMatch(^prefix)` to the Java-source-equivalent `Like(prefix%)`; the
  reviewer confirmed no remaining findings on follow-up.
- N+1 Java contract scout: `01a01e4f-3b78-7e72-81df-74beac83c9a5`
  (`Russell`), completed read-only `expr-core-in-between` scope; identified a
  safe five-execution candidate and its exact runtime IDs.
- N+1 Go surface scout: `01a01e4f-3cd5-7eb0-a713-5b17da488e25`
  (`Kierkegaard`), completed read-only `expr-core-in-between` scope; confirmed
  reusable typed builders and flagged endpoint-plan-identity/nullability risks.
- Previous relop scouts/review: retained in Git history; relop was pushed as
  `a448dbd4f`.
- Serial exception: none; both required like/regexp scouts were started
  through the collaboration facility with disjoint read-only scopes.

## Work-unit contract

- Capability/subdomain: `expr.core`, LIKE/REGEXP matching and Null
  propagation
- Java source and executions/runtime IDs:
-  `regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/exprcore/ExprCoreLikeRegexp.java`;
  `ExprCoreLikeWConstants` / `java-runtime-376f347aa8fc8367fcbc`;
  `ExprCoreLikeWExprs` / `java-runtime-09f15b6eb39cbee59b0d`;
  `ExprCoreRegexpWConstants` / `java-runtime-7dc8b98263cf5e9e4c3f`;
  `ExprCoreRegexpWExprs` / `java-runtime-8a8794063a0b9c4bfd0e`.
- Differential scope: four ordered isolated cases for the executions above.
  The scenario includes the Java source inputs plus Null and a regex
  substring/full-match discriminator inside the dynamic REGEXP statement.
  The invalid compile execution, escaped-character case, numeric/Null case,
  and SODA/compile duplicate remain implemented-only for later units or Go
  unit evidence; no runtime IDs are claimed for them here.
- Observable contract: full-string SQL-like `%`/`_` matching, Java-compatible
  numeric text conversion, dynamic value/pattern operands, Java full-match
  REGEXP semantics, Null/Missing/typed-nil propagation, boxed Boolean result
  fields, field and listener order, and fresh lifecycle boundaries. Fixed
  inputs include matches, non-matches, Null values, numeric values, and a
  dynamic pattern that must reject a substring-only match. No timers, windows,
  old-stream rows, or time advances are expected.
- Allowed production files: existing typed LIKE/REGEXP code under
  `internal/esper` only if differential replay proves a semantic regression;
  prefer no production change.
- Allowed Go test/parity files: a new
  `internal/app/parity/expr_core_like_regexp.go`, focused additions only if
  needed under `internal/esper`, and minimal dispatcher/test additions in
  `internal/app/parity/run.go` and `run_test.go`.
- Allowed parity asset files:
  `tools/java-oracle/ExprCoreLikeRegexpScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-like-regexp.sh`, and
  `testdata/parity/expr-core-like-regexp.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: changes under `/root/app/esper`; unrelated
  semantic surfaces; `goal.txt`; generated evidence before trace validation;
  and central facts outside this unit's manifest/roadmap/CHANGELOG updates.
  `PLANS.md`, the manifest, roadmap, CHANGELOG, traces, and evidence remain
  primary-agent owned. Scouts do not format, build, test, commit, or push.
- Targeted validation: the pinned Java oracle runner; focused LIKE/REGEXP
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
- 2026-08-20: `case.expr-core-like-regexp` reached zero-difference replay:
  four differential runtime IDs, four isolated cases, and 19 ordered records.
  The checked-in Java and Go traces both hash to
  `086c1aa32be7ae5bf2b23ec9750853c911187e3512eac9edd8e32d8917ea6f2f`;
  the full-match fix is covered by a unit assertion and dynamic REGEXP input.
- 2026-08-20: After targeted validation, independent reviewer Erdos and the
  two `expr-core-in-between` read-only scouts Russell/Kierkegaard were started
  concurrently. Erdos found and then cleared one P1 test-modeling issue;
  Russell/Kierkegaard identified a safe five-execution N+1 slice and its
  endpoint-plan-identity/nullability risks without modifying files.

## Validation evidence

- Investigation baseline on 2026-08-20: `/root/app/esper` is fixed at
  `9e1b9f1cc9117fea4bf33ab043762c045d73839c`; `master` was clean and matched
  `origin/master` at `a448dbd4fd59f7b97ded37f7f6e1f090ed7c1a0b`. Manifest
  summary reports 522 cases, 127 differential-verified cases, 389
  differential runtimes, 2,901 referenced runtimes, and 1,235 unreferenced
  runtime IDs. The like/regexp target is implemented-only with ten runtime
  IDs and existing Go unit coverage; no differential trace or evidence is
  present.
- Current unit result: the pinned Java runner produced 19 records; checked-in
  Java and Go traces are byte-identical; differential evidence is `passing`
  with zero differences and exact four-runtime/source/execution metadata.
  Focused LIKE/REGEXP Esper tests, parity replay/evidence/malformed-shape and
  six trace mutation tests, compat/manifest tests, `sh -n`, JSON validation,
  and `git diff --check` pass. The two pre-existing filter parity failures
  were reproduced on clean `a448dbd4f`, then corrected to use the Java
  source's LIKE semantics. After that correction, `go vet ./...`,
  `go test ./... -count=1 -timeout 240s`, `make check`, and
  `make test-race` all pass. The independent parity reviewer confirmed no
  remaining findings.

## Delivery

- The previous unit is committed and pushed to `origin/master` at
  `a448dbd4f`; Git history is the authoritative source for its identity. The
  like/regexp unit may be committed only after independent review and all
  required gates pass.
- Do not create another tracked checkpoint update after this unit's semantic
  commit. Verify the pushed ref without editing tracked files.

## Handoff

Start or resume from the repository root with the starter prompt in
`docs/esper-go-port-codex-workflows.md`. After this unit is committed and
pushed, select the next closed-loop work unit and repeat concurrent scout
fan-out before implementation.
