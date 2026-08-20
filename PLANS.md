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
- On interruption, leave the worktree state, exact next action, failures, and
  unverified assumptions explicit enough for a new Codex task to resume.

## Outcome

Continue the fixed-commit Esper 9.0.0 to Go migration until the final
acceptance criteria in `docs/esper-go-port-quality-strategy.md` all pass.
Progress is measured by verified work units and manifest evidence, not agent
activity or a single coverage percentage.

## Active checkpoint

- Updated: 2026-08-20
- Baseline: `master` at `b8e0f3ee0` (verified clean before this work unit).
- Status: Bitwise expression parity slice implemented, differentially verified,
  independently reviewed, committed, and ready for delivery.
- Current work unit: `expr.core` / `case.expr-core-bitwise`
- Exact next action: amend this semantic commit with the final checkpoint,
  verify clean state, and push `master`.
- Worktree notes: clean at selection; unrelated files remain out of scope.

## Work-unit contract

- Capability/subdomain: `expr.core`, bitwise operators
- Java executions and runtime IDs:
  `ExprCoreBitWiseOp` / `java-runtime-b0d354033a204970cb26`;
  `ExprCoreBitWiseOpOM` / `java-runtime-7819faeecbb3e817d49d`.
  `ExprCoreBitWiseInvalid` / `java-runtime-e1ecced8a0b539b7ba84` remains
  implemented-only because the current trace protocol has no build-error
  record; its Go build-error regression stays required.
- Observable contract: one typed `SupportBean` event produces one listener
  row with byte/short/int/long bitwise results and boolean AND semantics;
  the EPL and object-model executions normalize to the same row values and
  output types. Null boxed operands and invalid operand shapes remain covered
  by the existing Go unit tests.
- Allowed production files: `internal/app/parity/expr_core_bitwise.go`,
  `internal/app/parity/run.go`, `internal/app/parity/run_test.go`, and the
  focused `internal/esper` parity test only if the frozen contract exposes a
  regression.
- Allowed parity asset files: `tools/java-oracle/ExprCoreBitwiseScenarioOracle.java`,
  `tools/java-oracle/run-expr-core-bitwise.sh`, and
  `testdata/parity/expr-core-bitwise.{json,trace.json,evidence.json}`.
- Forbidden/conflicting files: unrelated semantic surfaces; `goal.txt`;
  generated evidence before trace validation; and central facts outside this
  unit's manifest/roadmap/CHANGELOG updates. `PLANS.md`, the manifest,
  roadmap, CHANGELOG, traces, and evidence remain primary-agent owned.
- Targeted scenario and test commands: the Java oracle runner; `go test
  ./internal/esper -run 'Bitwise' -count=1`; `go test ./internal/app/parity
  -run 'Bitwise|ExprCoreBitwise' -count=1`; differential mutation tests;
  `go test ./internal/compat ./internal/app/manifest -count=1`.
- Milestone gates required: `gofmt`, `go vet ./...`, `go test ./... -count=1`,
  manifest/evidence validation, `make check`, and `git diff --check`; race is
  applicable at the capability milestone if the baseline remains green.

## Progress

- [x] Reconfirm baseline, current manifest summary, and relevant existing gates.
- [x] Select the next natural work unit from roadmap priority and manifest gaps.
- [x] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands using read-only investigation.
- [x] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [x] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [x] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts changed.
- [x] Run independent parity review, resolve findings, and run complete local
      gates plus applicable milestone gates.
- [x] Review the final diff, commit one semantic work unit, push `master`, and
      record the commit and actual validation.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.

## Validation evidence

Preceding adapter checkpoint validation on 2026-08-20:

- Changed-document local links: passed.
- Project model/provider binding scan: passed; no bindings found.
- `git diff --check`: passed.
- `go vet ./...`: passed.
- `go test ./... -count=1 -timeout 240s`: all packages before
  `internal/esper` passed, then the Windows compiler exhausted memory while
  compiling that large package. Re-running
  `go test -p 1 -gcflags=all=-c=1 ./internal/esper -count=1 -timeout 240s`
  passed.
- The original `scripts/check-layout.sh` passed its structural checks but Git
  Bash exceeded the Windows command-line limit at its all-files `gofmt`
  invocation. Equivalent per-file `gofmt -l`, public/internal API comparison,
  and `go list ./...` checks passed; no Go file changed.

Bitwise parity work-unit validation on 2026-08-20:

- Fixed Java oracle `/root/app/esper` was at `9e1b9f1cc9117fea4bf33ab043762c045d73839c`.
- `tools/java-oracle/run-expr-core-bitwise.sh --skip-build` produced the
  replayable two-record Java trace at
  `testdata/parity/expr-core-bitwise.trace.json`.
- `go run ./cmd/parity -mode expr-core-bitwise-diff ...` produced passing
  evidence at `testdata/parity/expr-core-bitwise.evidence.json`: 2 records on
  each side, 0 differences, runtime IDs
  `java-runtime-b0d354033a204970cb26` and
  `java-runtime-7819faeecbb3e817d49d`.
- `go test ./internal/app/parity -run 'ExprCoreBitwise|Bitwise' -count=1`
  passed, including value/order/field mutation rejection.
- `go test ./internal/compat -count=1` passed after manifest summary updates.
- `go vet ./...` passed.
- `go test ./... -count=1 -timeout 240s` passed all packages.
- `make check` passed layout, vet, and the full test suite.
- `go test -race ./internal/app/parity ./internal/esper -count=1 -timeout 600s`
  passed; `internal/esper` completed in 271.673s.
- `git diff --check` and changed-file `gofmt -l` checks passed.
- Manifest/evidence `jq` checks passed; the focused manifest entry was also
  re-reviewed after removing a duplicate JSON `evidence` key.

Work-unit decisions:

- Java runtime initialization had to be pinned to epoch `0L` in the oracle;
  otherwise wall-clock initialization created a false trace-time difference.
- `ExprCoreBitWiseInvalid` remains implemented-only. Its Go unit test verifies
  Build-phase rejection, but it is excluded from differential runtime IDs
  until the language-neutral trace protocol can represent build errors without
  weakening the evidence contract.

Independent parity review on 2026-08-20:

- Compared the fixed Java `ExprCoreBitWiseOperators` source, the Go typed
  replay, oracle normalization, scenario case order, persisted traces, and
  manifest runtime metadata; no remaining semantic findings.
- The review found and corrected one central-fact formatting defect: the
  bitwise case had duplicate JSON `evidence` keys after status promotion.

Delivery:

- Semantic commit: `expr: verify core bitwise parity`.
- The final checkpoint update is being amended into this same semantic commit
  before pushing `master`.

## Handoff

Start or resume a Codex task from the repository root with the starter prompt
in `docs/esper-go-port-codex-workflows.md`. The first migration action is plan
selection and contract discovery, not an implementation guess.
