#!/usr/bin/env sh
set -eu

# Convenience wrapper for the rollup-output-first oracle. The underlying
# RollupOutputLastScenarioOracle also supports the "first" case; this script
# keeps the invocation name aligned with the scenario.
exec "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/run-rollup-output-last.sh" "$@"
