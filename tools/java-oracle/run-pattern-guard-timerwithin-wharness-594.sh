#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-guard-timerwithin-wharness-594.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternGuardTimerWithinWHarness594ScenarioOracle against the
pinned Esper 9.0.0 checkout (javaCommit
9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
PatternGuardTimerWithin ord 0 PatternOp — the single-execution
34-leg timer:within W-harness over
EventCollectionFactory.getEventSetOne(0,1000) as ONE case (all
thirty-four legs deploy inside ONE deploy-all step — thirty-three as
`@name("S<i>") select * from pattern [<atom>]` plus S3 through the
SODA model path — then each of the twelve send steps advances the
external clock to the event's pinned instant before sendEventBean) —
and writing the normalized Java trace to --output. The trace carries
the listener records of every firing leg including guard-expiry
effects attributed to the upcoming send's bucket (the exclusive
deadline drops legs whose event lands at exactly arm+period, the
accumulating every((every X) where G) legs deliver B3's four rows in
one record, the per-branch every of S23 respawns inside the 6000
advance to see D1, and S31's or survives the b side's expiry with b
unbound); the silent legs S0/S2/S5/S6/S17/S20/S21/S24/S27/S28/S33
fire zero times.
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
    .id == "pattern-guard-timerwithin-wharness-594" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardTimerWithin.java" and
    (.javaSourceFiles | type == "array" and length == 11) and
    (.javaRuntimes == ["java-runtime-bb8113cb979826cff927"]) and
    (.javaNames == ["PatternOp"]) and
    (.javaStaticIds == ["java-0dec801a426fed297402"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 1) and
    ([.cases[].case] == ["w-harness"]) and
    ([.cases[].ordinal] == [0]) and
    ([.cases[].epls | length] == [34]) and
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
    echo "scenario is not a valid pattern-guard-timerwithin-wharness-594 replay: $scenario" >&2
    exit 1
fi
if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-guard-timerwithin-wharness-594.XXXXXX")
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
    "$script_root/PatternGuardTimerWithinWHarness594ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternGuardTimerWithinWHarness594ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-guard-timerwithin-wharness-594" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 47) and
    ([.records[] | select(.case == "w-harness" and .operation == "listener")] | length == 47) and
    ([.records[] | select(.statement == "S0")] | length == 0) and
    ([.records[] | select(.statement == "S1" and .time == "1970-01-01T00:00:02Z")] | length == 1) and
    ([.records[] | select(.statement == "S3" and .time == "1970-01-01T00:00:10Z")] | length == 1) and
    ([.records[] | select(.statement == "S4" and .time == "1970-01-01T00:00:10Z")] | length == 1) and
    ([.records[] | select(.statement == "S7")] | length == 1) and
    ([.records[] | select(.statement == "S8")] | length == 2) and
    ([.records[] | select(.statement == "S11")] | length == 3) and
    ([.records[] | select(.statement == "S11" and .time == "1970-01-01T00:00:10Z" and (.new | length) == 4)] | length == 1) and
    ([.records[] | select(.statement == "S23")] | length == 3) and
    ([.records[] | select(.statement == "S30" and .time == "1970-01-01T00:00:02Z")] | length == 1) and
    ([.records[] | select(.statement == "S31" and .time == "1970-01-01T00:00:06Z" and .new[0].fields.b.state == "null")] | length == 1) and
    ([.records[] | select(.statement == "S32")] | length == 4) and
    ([.records[] | select(.statement == "S2" or .statement == "S5" or .statement == "S6" or .statement == "S17" or .statement == "S20" or .statement == "S21" or .statement == "S24" or .statement == "S27" or .statement == "S28" or .statement == "S33")] | length == 0)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
