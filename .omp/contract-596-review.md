# Batch contract: Draft 4.596 parity review + Draft 4.597 prefetch scouts

## Goal
Independent parity review of work unit 4.596 (infra-nwtable-join-select-delete)
plus read-only contract prefetch for the next unit (context init-term
remainder: ContextInitTermTemporalFixed / ContextInitTermWithDistinct /
ContextInitTermWithNow / ContextHashSegmented / ContextDocExamples
unreferenced executions).

## Constraints
- All three agents are READ-ONLY: no writes, no formatters, no tests beyond
  targeted re-runs the reviewer needs, no manifest/roadmap/PLANS edits.
- Java oracle is FIXED at /root/app/esper commit
  9e1b9f1cc9117fea4bf33ab043762c045d73839c — never request changes to it.
- Reviewer reports P0/P1/P2/P3 findings; scouts return frozen contract facts.
