#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-guard-timerwithin-forms-595.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternGuardTimerWithinForms595ScenarioOracle against the
pinned Esper 9.0.0 checkout (javaCommit
9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
PatternGuardTimerWithin ordinals 1-6 — the six remaining timer:within
executions after the ord-0 W-harness, each as its own case on a fresh
engine under the external clock: interval-10-min arms
`(every SupportBean) where timer:within(1 days 2 hours 3 minutes 4
seconds 5 milliseconds)` (93784005ms; fires at t=0 and deadline-1ms,
silent at the exclusive deadline), interval-10-min-variable reads the
same period from the suite variables D/H/M/S/MS=1..5,
interval-prepared composes it from five positional `?::int`
substitution parameters add(1,1)..add(5,5), within-from-expression
correlates the guard to a.intPrimitive seconds (E2@2000 and E3@2999
emit {id} rows, E4@3000 is silent past expiry),
pattern-not-followed-by respawns the outer every inside the 6000
advance so the E4+M1 pair emits exactly once, and may-max-month runs
two rounds — `timer:within(1 month)` then `timer:withinmax(1 month,
10)` — each pre-advanced to 2002-02-01T09:00:00Z so E1 fires at arm
and E2 at the calendar boundary minus 1ms while E3 at the exact
boundary stays silent. Every send step performs advanceTime(at)
BEFORE sendEventBean; the normalized listener trace is written to
--output.
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
    echo "Esper checkout $esper_root is at $actual_commit, expected $expected_commit" >&2
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
    .id == "pattern-guard-timerwithin-forms-595" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithin.java" and
    (.javaSourceFiles == [
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithin.java",
        "common/src/main/java/com/espertech/esper/common/internal/support/SupportBean.java",
        "regression-lib/src/main/java/com/espertech/esper/regressionlib/support/bean/SupportMarketDataBean.java"]) and
    (.javaRuntimes == [
        "java-runtime-f0649272cfb528ce731b",
        "java-runtime-2fe5370a7c1599bfb424",
        "java-runtime-66d65309509fac58bbfc",
        "java-runtime-36ef6f15f14a0e86ab93",
        "java-runtime-6a5e8128ae184e8a7249",
        "java-runtime-34555c4a9823a346d710"]) and
    (.javaNames == [
        "PatternInterval10Min",
        "PatternInterval10MinVariable",
        "PatternIntervalPrepared",
        "PatternWithinFromExpression",
        "PatternPatternNotFollowedBy",
        "PatternWithinMayMaxMonthScoped"]) and
    (.javaStaticIds == ["java-0dec801a426fed297402"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["interval-10-min", "interval-10-min-variable", "interval-prepared", "within-from-expression", "pattern-not-followed-by", "may-max-month"]) and
    ([.cases[].ordinal] == [1, 2, 3, 4, 5, 6]) and
    ([.cases[].epls | length] == [1, 1, 1, 1, 1, 2]) and
    (.steps | type == "array" and length == 43) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 7) and
    ([.steps[] | select(.op == "deploy" and .statement != "s0")] | length == 0) and
    ([.steps[] | select(.op == "deploy" and has("at"))] | length == 2) and
    ([.steps[] | select(.op == "deploy" and has("at") and .case == "may-max-month" and .at == "2002-02-01T09:00:00Z")] | length == 2) and
    ([.steps[] | select(.op == "send")] | length == 23) and
    ([.steps[] | select(.op == "send" and (has("at") | not))] | length == 0) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 22) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 1) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 7) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-guard-timerwithin-forms-595 replay: $scenario" >&2
    exit 1
fi
if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-guard-timerwithin-forms-595.XXXXXX")
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
    "$script_root/PatternGuardTimerWithinForms595ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternGuardTimerWithinForms595ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-guard-timerwithin-forms-595" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 13) and
    ([.records[] | select(.operation == "listener" and .statement == "s0")] | length == 13) and
    ([.records[] | select(.case == "interval-10-min")] | length == 2) and
    ([.records[] | select(.case == "interval-10-min" and .time == "1970-01-01T00:00:00Z" and .new[0].fields == {})] | length == 1) and
    ([.records[] | select(.case == "interval-10-min" and .time == "1970-01-02T02:03:04.004Z" and .new[0].fields == {})] | length == 1) and
    ([.records[] | select(.case == "interval-10-min-variable")] | length == 2) and
    ([.records[] | select(.case == "interval-10-min-variable" and .time == "1970-01-01T00:00:00Z")] | length == 1) and
    ([.records[] | select(.case == "interval-10-min-variable" and .time == "1970-01-02T02:03:04.004Z")] | length == 1) and
    ([.records[] | select(.case == "interval-prepared")] | length == 2) and
    ([.records[] | select(.case == "interval-prepared" and .time == "1970-01-01T00:00:00Z")] | length == 1) and
    ([.records[] | select(.case == "interval-prepared" and .time == "1970-01-02T02:03:04.004Z")] | length == 1) and
    ([.records[] | select(.case == "within-from-expression")] | length == 2) and
    ([.records[] | select(.case == "within-from-expression" and .time == "1970-01-01T00:00:02Z" and .new[0].fields.id == "E2")] | length == 1) and
    ([.records[] | select(.case == "within-from-expression" and .time == "1970-01-01T00:00:02.999Z" and .new[0].fields.id == "E3")] | length == 1) and
    ([.records[] | select(.case == "pattern-not-followed-by")] | length == 1) and
    ([.records[] | select(.case == "pattern-not-followed-by" and .time == "1970-01-01T00:00:06Z" and .new[0].fields == {})] | length == 1) and
    ([.records[] | select(.case == "may-max-month")] | length == 4) and
    ([.records[] | select(.case == "may-max-month" and .time == "2002-02-01T09:00:00Z" and .new[0].fields == {})] | length == 2) and
    ([.records[] | select(.case == "may-max-month" and .time == "2002-03-01T08:59:59.999Z" and .new[0].fields == {})] | length == 2)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
