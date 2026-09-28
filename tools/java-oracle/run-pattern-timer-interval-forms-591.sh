#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-timer-interval-forms-591.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternTimerIntervalForms591ScenarioOracle against the pinned
Esper 9.0.0 checkout (javaCommit 9e1b9f1cc9117fea4bf33ab043762c045d73839c),
replaying PatternObserverTimerInterval ords 1/2/3/5/6 — the five
static-spec-resolution forms on the shared external-clock harness
(advanceTime(0) arms, 61999ms silent, milestone(0) savepoint, 62000ms
fires once, undeployAll): the literal 1 minute 2 seconds form
(PatternIntervalSpec), the RegressionPath double-variable form
(M_isv minute S_isv seconds), the double-arithmetic expression form
(MOne*60+SOne seconds), the prepared-statement form
(?::int minute ?::int seconds with substitution params add(1,1).add(2,2))
and the calendar month-scoped form (timer:interval(1 month) armed at
2002-02-01T09:00:00.000 firing at the 2002-03-01T09:00:00.000 boundary) —
and writing the normalized Java trace to --output.
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
    echo "Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
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
    .id == "pattern-timer-interval-forms-591" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java" and
    (.javaSourceFiles | type == "array" and length == 2) and
    (.javaRuntimes == ["java-runtime-d5ad6ad9d226628f8383", "java-runtime-bbb75d6bcc54f29c7652", "java-runtime-97c5e6b5e4c46cc0adb4", "java-runtime-ea394f5795b88ddd71c9", "java-runtime-28fc7f508485cbc765a2"]) and
    (.javaNames == ["PatternIntervalSpec", "PatternIntervalSpecVariables", "PatternIntervalSpecExpression", "PatternIntervalSpecPreparedStmt", "PatternMonthScoped"]) and
    (.javaStaticIds == ["java-1422565b568236b2bfea", "java-1422565b568236b2bfea", "java-1422565b568236b2bfea", "java-1422565b568236b2bfea", "java-1422565b568236b2bfea"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["interval-spec", "interval-spec-variables", "interval-spec-expression", "interval-spec-prepared-stmt", "month-scoped"]) and
    ([.cases[].ordinal] == [1, 2, 3, 5, 6]) and
    ([.cases[].epls | length] == [1, 3, 3, 1, 1]) and
    (.steps | type == "array" and length == 34) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "deploy")] | length == 9) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and has("payload"))] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0" and .payload == [1, 2])] | length == 1) and
    ([.steps[] | select(.op == "advance-time")] | length == 15) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "advance-time" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-timer-interval-forms-591 replay: $scenario" >&2
    exit 1
fi
if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-timer-interval-forms-591.XXXXXX")
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
    "$script_root/PatternTimerIntervalForms591ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternTimerIntervalForms591ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-timer-interval-forms-591" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 5) and
    ([.records[] | select(.operation == "listener" and .statement == "s0")] | length == 5) and
    ([.records[] | select((.new | length) == 1 and (.new[0].fields | length) == 0)] | length == 5) and
    ([.records[] | select(.case == "interval-spec" and .time == "1970-01-01T00:01:02Z")] | length == 1) and
    ([.records[] | select(.case == "interval-spec-variables" and .time == "1970-01-01T00:01:02Z")] | length == 1) and
    ([.records[] | select(.case == "interval-spec-expression" and .time == "1970-01-01T00:01:02Z")] | length == 1) and
    ([.records[] | select(.case == "interval-spec-prepared-stmt" and .time == "1970-01-01T00:01:02Z")] | length == 1) and
    ([.records[] | select(.case == "month-scoped" and .time == "2002-03-01T09:00:00Z")] | length == 1)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
