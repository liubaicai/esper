#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-view-expression-window-time-573.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the ViewExpressionWindowTime573ScenarioOracle against the pinned Esper
9.0.0 checkout (javaCommit 9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
ViewExpressionWindow ord 3 (ViewExpressionWindowTimeWindow — the keep-form
timestamp window) and writing the normalized Java trace to --output. The
bundle's two compile-error-only siblings (ViewExpressionWindowInvalid ord 5,
ViewExpressionBatchInvalid ord 4) are intentionally-different manifest rows
and produce no trace records.
EOF
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
    case "$1" in
        --esper-root) esper_root=$2; shift 2 ;;
        --scenario) scenario=$2; shift 2 ;;
        --output) output=$2; shift 2 ;;
        --skip-build) skip_build=1; shift ;;
        -h|--help) usage; exit 0 ;;
        *) usage; exit 2 ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "could not resolve Esper commit under $esper_root" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is at $actual_commit; expected $expected_commit" >&2
    exit 1
fi

java_bin=${JAVA:-java}
javac_bin=${JAVAC:-javac}
mvn_bin=${MAVEN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then
    java_bin="$JAVA_HOME/bin/java"
    javac_bin="$JAVA_HOME/bin/javac"
fi
if [ -n "${MAVEN_HOME:-}" ]; then
    mvn_bin="$MAVEN_HOME/bin/mvn"
fi
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable was not found: $java_bin" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac executable was not found: $javac_bin" >&2; exit 1; }
command -v "$mvn_bin" >/dev/null 2>&1 || { echo "Maven executable was not found: $mvn_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the scenario and oracle trace" >&2; exit 1; }

java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "view-expression-window-time-573" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/view/ViewExpressionWindow.java" and
    (.javaSourceFiles | type == "array" and length == 2) and
    (.javaRuntimes == ["java-runtime-5d04258cf2eb6e0025fd"]) and
    (.javaNames == ["ViewExpressionWindowTimeWindow"]) and
    (.javaStaticIds == ["java-06e6b1f6c905b8f12b82"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 1) and
    ([.cases[].case] == ["time-window"]) and
    ([.cases[].ordinal] == [3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    (.steps | type == "array" and length == 28) and
    ([.steps[] | select(.op == "case")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 1) and
    ([.steps[] | select(.op == "deployed" and .statement == "s0")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 8) and
    ([.steps[] | select(.op == "advance-time")] | length == 8) and
    ([.steps[] | select(.op == "snapshot" and .mode == "ordered")] | length == 8) and
    ([.steps[] | select(.op == "snapshot" and .mode == "any")] | length == 0) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "advance-time" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid view-expression-window-time-573 replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-view-expr-win-time-573.XXXXXX")
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    CYGWIN*|MINGW*|MSYS*) cp_sep=";" ;;
    *) cp_sep=":" ;;
esac
native_path() {
    if [ "$cp_sep" = ";" ]; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/ViewExpressionWindowTime573ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ViewExpressionWindowTime573ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "view-expression-window-time-573" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 17) and
    ([.records[] | select(.operation == "deployed")] | length == 1) and
    ([.records[] | select(.operation == "listener")] | length == 8) and
    ([.records[] | select(.operation == "snapshot")] | length == 8)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
