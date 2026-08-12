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

root_go_files=$(find . -maxdepth 1 -type f -name '*.go' | wc -l | tr -d ' ')
[ "$root_go_files" = "3" ] || fail "repository root must contain exactly the public facade, package docs, and facade tests"
[ -f doc.go ] || fail "doc.go is required at the repository root"
[ -f facade_generated.go ] || fail "facade_generated.go is required at the repository root"
[ -f facade_test.go ] || fail "facade_test.go is required at the repository root"
[ -d internal/esper ] || fail "internal/esper is required for the private implementation"
if find internal/esper -maxdepth 1 -type f -name '*.go' -exec grep -L '^package esper$' {} + | grep -q .; then
    fail "all internal/esper Go files must declare package esper"
fi

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

facade=$(mktemp)
public_api=$(mktemp)
implementation_api=$(mktemp)
trap 'rm -f "$facade" "$public_api" "$implementation_api"' EXIT
go run ./internal/cmd/genfacade -source internal/esper -out "$facade"
cmp -s facade_generated.go "$facade" || fail "facade_generated.go is stale; run go generate ."
go run ./internal/cmd/apidump -package github.com/liubaicai/esper > "$public_api"
go run ./internal/cmd/apidump -package github.com/liubaicai/esper/internal/esper > "$implementation_api"
cmp -s "$public_api" "$implementation_api" || fail "public facade does not match internal/esper API"

go list ./... >/dev/null
