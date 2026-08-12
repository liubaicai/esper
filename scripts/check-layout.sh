#!/usr/bin/env sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

fail() {
    echo "project layout: $*" >&2
    exit 1
}

[ -f go.mod ] || fail "go.mod must be at the repository root"
[ ! -d src ] || fail "top-level src/ is not a valid Go module layout"
[ -d cmd ] || fail "cmd/ is required for executable entry points"
[ -d internal ] || fail "internal/ is required for repository-private Go code"
[ -d testdata ] || fail "testdata/ is required for checked-in test assets"

for command in cmd/*; do
    [ -d "$command" ] || continue
    [ -f "$command/main.go" ] || fail "$command must contain main.go"
    count=$(find "$command" -maxdepth 1 -type f -name '*.go' | wc -l | tr -d ' ')
    [ "$count" = "1" ] || fail "$command must be a thin entry point with one Go file"
    grep -q '/internal/app/' "$command/main.go" || fail "$command must delegate to internal/app"
done

tracked_generated=$(git ls-files | while IFS= read -r path; do
    [ -e "$path" ] && echo "$path"
done | grep -E '(^|/)(coverage[^/]*|.*\.log|.*\.prof|.*\.test|.*\.out)$' || true)
[ -z "$tracked_generated" ] || fail "generated artifacts are tracked:\n$tracked_generated"

unformatted=$(gofmt -l $(find . -name '*.go' -not -path './vendor/*'))
[ -z "$unformatted" ] || fail "Go files require gofmt:\n$unformatted"

go list ./... >/dev/null
