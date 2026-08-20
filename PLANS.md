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
- Baseline: `master` at `30828761e`; verify before use.
- Status: Codex adapter ready; no migration work unit selected by this change.
- Current work unit: none
- Exact next action: inspect current roadmap priorities and manifest remaining,
  then select a closed-loop group of 1 to 5 related Java executions.
- Worktree notes: verify with `git status`; do not assume it is still clean.

## Work-unit contract

- Capability/subdomain: not selected
- Java executions and runtime IDs: not selected
- Observable contract: not frozen
- Allowed production files: not selected
- Allowed parity asset files: not selected
- Forbidden/conflicting files: `PLANS.md`, manifest, roadmap, CHANGELOG,
  generated traces/evidence, and `goal.txt` are primary-agent owned
- Targeted scenario and test commands: not selected
- Milestone gates required: determine from the roadmap and quality strategy

## Progress

- [ ] Reconfirm baseline, current manifest summary, and relevant existing gates.
- [ ] Select the next natural work unit from roadmap priority and manifest gaps.
- [ ] Freeze the Java observable contract, runtime IDs, file ownership, and
      validation commands using read-only investigation.
- [ ] Implement the smallest Go/API/parity-asset change that satisfies the
      frozen contract.
- [ ] Generate and compare Java/Go traces; run targeted tests and required
      mutation checks.
- [ ] Update manifest, evidence, roadmap, CHANGELOG, and public summary only
      where verified facts changed.
- [ ] Run independent parity review, resolve findings, and run complete local
      gates plus applicable milestone gates.
- [ ] Review the final diff, commit one semantic work unit, push `master`, and
      record the commit and actual validation.

## Discoveries and decisions

- 2026-08-20: Codex uses root `AGENTS.md` for persistent repository
  instructions and this file as its living execution checkpoint. `.omp/`
  remains an OMP adapter rather than the shared source of project rules.

## Validation evidence

No migration behavior changed in this adapter-only checkpoint. Adapter
validation on 2026-08-20:

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

## Handoff

Start or resume a Codex task from the repository root with the starter prompt
in `docs/esper-go-port-codex-workflows.md`. The first migration action is plan
selection and contract discovery, not an implementation guess.
