#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-subselect-multirow.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
/tmp/esper-cp.txt are required; --skip-build is accepted for runner parity and
keeps the runner from attempting a Maven build.
EOF
    exit 2
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
classpath_file=${ESPER_CLASSPATH_FILE:-/tmp/esper-cp.txt}
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
    case "$1" in
        --esper-root)
            [ "$#" -ge 2 ] || usage
            esper_root=$2
            shift 2
            ;;
        --scenario)
            [ "$#" -ge 2 ] || usage
            scenario=$2
            shift 2
            ;;
        --output)
            [ "$#" -ge 2 ] || usage
            output=$2
            shift 2
            ;;
        --skip-build)
            skip_build=1
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage
            ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }
[ -f "$classpath_file" ] || { echo "Esper classpath was not found: $classpath_file" >&2; exit 1; }

esper_root=$(CDPATH= cd -- "$esper_root" && pwd)
scenario=$(CDPATH= cd -- "$(dirname -- "$scenario")" && pwd)/$(basename -- "$scenario")
output_dir=$(CDPATH= cd -- "$(dirname -- "$output")" && pwd)
output="$output_dir/$(basename -- "$output")"

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
fi
if [ -n "$(git -C "$esper_root" status --porcelain --untracked-files=all 2>/dev/null)" ]; then
    echo "Esper checkout has uncommitted or untracked changes: $esper_root" >&2
    exit 1
fi

java_bin=${JAVA:-java}
javac_bin=${JAVAC:-javac}
if [ -n "${JAVA_HOME:-}" ]; then
    java_bin="$JAVA_HOME/bin/java"
    javac_bin="$JAVA_HOME/bin/javac"
fi
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable was not found: $java_bin" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac executable was not found: $javac_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the oracle trace" >&2; exit 1; }

java_version=$("$java_bin" -version 2>&1 | awk -F'[\".]' '/version/ {print $2; exit}')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "subselect-multirow" and
    (.javaRuntimes | type == "array" and length == 2) and
    (.javaRuntimes == [
        "java-runtime-29c2087cc4243e9b7a50",
        "java-runtime-64eb1701d14bdbcefc86"
    ]) and
    (.cases | type == "array" and length == 2) and
    ([.cases[] | .case] == ["multirow-single-column", "multirow-underlying-correlated"]) and
    ([.steps[] | select(.op == "case") | .case] == [
        "multirow-single-column", "multirow-underlying-correlated"
    ]) and
    ([.steps[] | select(.op == "send")] | length) == 14 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length) == 8 and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length) == 6
    ' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid subselect-multirow replay: $scenario" >&2
    exit 1
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-subselect-multirow.XXXXXX")
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

classes="$work/classes"
mkdir -p "$classes"
base_cp=$(cat "$classpath_file")
classpath="$base_cp:$classes"

# The checked-in classpath contains paths relative to the Esper checkout, so
# both compilation and execution run with that checkout as their cwd.
(cd "$esper_root" && "$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/EPLSubselectMultirowScenarioOracle.java")
mkdir -p "$output_dir"
(cd "$esper_root" && "$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC \
    -Duser.language=en -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLSubselectMultirowScenarioOracle "$scenario") > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "subselect-multirow" and
    (.records | type == "array" and length == 6) and
    ([.records[] | select(.operation == "listener" and .statement == "s0" and
        (.new | type == "array" and length == 1))] | length) == 6 and
    ([.records[] | select(.case == "multirow-single-column")] | length) == 3 and
    ([.records[] | select(.case == "multirow-underlying-correlated")] | length) == 3
    ' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid subselect-multirow trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
