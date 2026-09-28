#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-timerinterval-wharness-592.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternTimerIntervalWHarness592ScenarioOracle against the
pinned Esper 9.0.0 checkout (javaCommit
9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
PatternObserverTimerInterval ord 0 PatternOp — the single-execution
31-leg timer:interval W-harness over
EventCollectionFactory.getEventSetOne(0,1000) as ONE case (all
thirty-one legs deploy inside ONE deploy-all step — thirty as
`@name("S<i>") select * from pattern [<atom>]` plus S1 through the
SODA model path — then each of the twelve send steps advances the
external clock to the event's pinned instant before sendEventBean) —
and writing the normalized Java trace to --output. The trace carries
the listener records of every firing leg including timer expirations
attributed to the upcoming send's bucket (the S7 every-3.001-sec
re-arms at each advance instant for F1/D3, the S8 every-5000-msec for
B3, and the S17/S18/S19 timer-armed d branches at D1/D2/D3); the
silent legs S27/S29/S30 fire zero times.
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
    echo "Esper checkout is at $actual_commit, expected $expected_commit" >&2
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
    .id == "pattern-timerinterval-wharness-592" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternObserverTimerInterval.java" and
    (.javaSourceFiles | type == "array" and length == 11) and
    (.javaRuntimes == ["java-runtime-9b41fb5951301c0de979"]) and
    (.javaNames == ["PatternOp"]) and
    (.javaStaticIds == ["java-1422565b568236b2bfea"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 1) and
    ([.cases[].case] == ["w-harness"]) and
    ([.cases[].ordinal] == [0]) and
    ([.cases[].epls | length] == [31]) and
    (.steps | type == "array" and length == 15) and
    ([.steps[] | select(.op == "case")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "all")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_C")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_D")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_E")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_F")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_G")] | length == 1) and
    ([.steps[] | select(.op == "send" and has("at"))] | length == 12) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-timerinterval-wharness-592 replay: $scenario" >&2
    exit 1
fi
if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-timerinterval-wharness-592.XXXXXX")
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
    "$script_root/PatternTimerIntervalWHarness592ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternTimerIntervalWHarness592ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-timerinterval-wharness-592" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 32) and
    ([.records[] | select(.case == "w-harness" and .operation == "listener")] | length == 32) and
    ([.records[] | select(.statement == "S0" and .time == "1970-01-01T00:00:02Z")] | length == 1) and
    ([.records[] | select(.statement == "S1" and .time == "1970-01-01T00:00:02Z")] | length == 1) and
    ([.records[] | select(.statement == "S7")] | length == 3) and
    ([.records[] | select(.statement == "S8")] | length == 2) and
    ([.records[] | select(.statement == "S17" and .time == "1970-01-01T00:00:09Z")] | length == 1) and
    ([.records[] | select(.statement == "S19")] | length == 2) and
    ([.records[] | select(.statement == "S24" and .time == "1970-01-01T00:00:11Z")] | length == 1) and
    ([.records[] | select(.statement == "S27" or .statement == "S29" or .statement == "S30")] | length == 0)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
