#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-variable-output-rate.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java 9.0.0 oracle commit. Java 17
and Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
EOF
    exit 2
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
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

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required for the Java 9.0.0 oracle; found ${java_version:-unknown}" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "epl-variable-output-rate" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    (.javaStaticIds | type == "array" and length == 4) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 4) and
    ([.cases[] | select(.observation == "listener" and .iteratorSnapshots == 0)] | length == 4) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 47) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 17) and
    ([.steps[] | select(.op == "deploy")] | length == 8) and
    ([.steps[] | select(.op == "set-variable")] | length == 4) and
    ([.steps[] | select(.op == "advance-time")] | length == 14) and
    ([.steps[] | select(.op == "advance-time-error")] | length == 1) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 99)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid epl-variable-output-rate replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-variable-output-rate.XXXXXX")
cleanup() {
    status=$?
    rm -rf "$work"
    exit $status
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
    MINGW* | MSYS* | CYGWIN*)
        cp_sep=";"
        native_path() { cygpath -m "$1"; }
        ;;
    *)
        cp_sep=":"
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/EPLVariablesOutputRateScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLVariablesOutputRateScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "epl-variable-output-rate" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 24) and
    ([.records[].statement] | all(. == "s0")) and
    ([.records[0:6][].case] | all(. == "events")) and
    ([.records[0:6][].operation] | all(. == "listener")) and
    ([.records[0:6][].sequence] == [range(1; 7)]) and
    ([.records[0:6][].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[0:6][].new | length] | all(. == 1)) and
    ([.records[0:6][].new[0].fields.cnt] == [3, 8, 10, 11, 12, 13]) and
    ([.records[6:12][].case] | all(. == "events-om")) and
    ([.records[6:12][].operation] | all(. == "listener")) and
    ([.records[6:12][].sequence] == [range(1; 7)]) and
    ([.records[6:12][].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[6:12][].new | length] | all(. == 1)) and
    ([.records[6:12][].new[0].fields.cnt] == [3, 8, 10, 11, 12, 13]) and
    ([.records[12:18][].case] | all(. == "events-compile")) and
    ([.records[12:18][].operation] | all(. == "listener")) and
    ([.records[12:18][].sequence] == [range(1; 7)]) and
    ([.records[12:18][].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[12:18][].new | length] | all(. == 1)) and
    ([.records[12:18][].new[0].fields.cnt] == [3, 8, 10, 11, 12, 13]) and
    ([.records[18:24][].case] | all(. == "time-snapshot")) and
    ([.records[18:23][].operation] | all(. == "listener")) and
    ([.records[18:23][].sequence] == [range(1; 6)]) and
    ([.records[18:23][].time] == ["1970-01-01T00:00:03Z", "1970-01-01T00:00:04Z", "1970-01-01T00:00:05Z", "1970-01-01T00:00:08Z", "1970-01-01T00:00:12Z"]) and
    ([.records[18:23][].new | length] | all(. == 1)) and
    ([.records[18:23][].new[0].fields.cnt] == [2, 4, 4, 4, 6]) and
    (.records[23].case == "time-snapshot") and
    (.records[23].operation == "advance-time-error") and
    (.records[23].statement == "s0") and
    (.records[23].sequence == 6) and
    (.records[23].time == "1970-01-01T00:00:14Z") and
    (.records[23].value == "Unexpected exception in statement '\''s0'\'': Failed to evaluate time period, received a null value for '\''Received null value evaluating time period'\''")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid epl-variable-output-rate trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
