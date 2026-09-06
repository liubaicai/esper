#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-subq-uncorrel.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-subq-uncorrel" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqUncorrel.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    ([.javaNames[] | startswith("InfraNWTableSubqUncorrelAssertion{")] | all) and
    (.javaStaticIds | type == "array" and length == 4) and
    ([.javaStaticIds[] | . == "java-5af40542812749ae30e4"] | all) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 4) and
    ([.cases[] | select(.observation == "listener" and .iteratorSnapshots == 0)] | length == 4) and
    ([.cases[].case] == ["nw-no-share", "nw-index-share", "nw-disable-share", "table"]) and
    ([.cases[].ordinal] == [0, 1, 0, 1]) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 24) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "select")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "selectTwo")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 72)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-subq-uncorrel replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-subq-uncorrel.XXXXXX")
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
    "$script_root/InfraNWTableSubqUncorrelScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableSubqUncorrelScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nwcase($off; $name):
        ([.records[$off:$off+18][].case] | all(. == $name)) and
        ([.records[$off:$off+18][].operation] | all(. == "listener")) and
        ([.records[$off:$off+18][].time] | all(. == "1970-01-01T00:00:00Z")) and
        ([.records[$off:$off+18][] | has("new") or has("old")] | all) and
        ([.records[$off:$off+18][] | select(has("new")) | .new[0].kind] | all(. == "row")) and
        ([.records[$off:$off+18][] | select(has("new")) | .new | length] | all(. == 1)) and
        ([.records[$off:$off+18][] | select(has("old")) | .old | length] | all(. == 1)) and
        ([.records[$off:$off+18][].statement] == ["select", "create", "select", "selectTwo", "create", "select", "selectTwo", "create", "delete", "select", "selectTwo", "create", "delete", "select", "selectTwo", "create", "select", "selectTwo"]) and
        ([.records[$off:$off+18][].sequence] == [1, 1, 2, 1, 2, 3, 2, 3, 1, 4, 3, 4, 2, 5, 4, 5, 6, 5]) and
        ([.records[$off:$off+18][] | select(.statement == "select" or .statement == "selectTwo") | .new[0].fields | keys] | all(. == ["symbol", "value"])) and
        ([.records[$off:$off+18][] | select(.statement == "create" or .statement == "delete") | ((.new // .old)[0].fields | keys)] | all(. == ["a", "b", "c"])) and
        (.records[$off].new[0].fields == {"symbol": "M1", "value": {"state": "null"}}) and
        (.records[$off+1].new[0].fields == {"a": "S1", "b": 1, "c": 2}) and
        (.records[$off+2].new[0].fields == {"symbol": "M1", "value": "S1"}) and
        (.records[$off+3].new[0].fields == {"symbol": "M1", "value": "S1"}) and
        (.records[$off+4].new[0].fields == {"a": "S2", "b": 10, "c": 20}) and
        (.records[$off+5].new[0].fields == {"symbol": "M2", "value": {"state": "null"}}) and
        (.records[$off+6].new[0].fields == {"symbol": "M2", "value": {"state": "null"}}) and
        (.records[$off+7].old[0].fields == {"a": "S1", "b": 1, "c": 2}) and
        (.records[$off+7] | has("new") | not) and
        (.records[$off+8].new[0].fields == {"a": "S1", "b": 1, "c": 2}) and
        (.records[$off+8] | has("old") | not) and
        (.records[$off+9].new[0].fields == {"symbol": "M3", "value": "S2"}) and
        (.records[$off+10].new[0].fields == {"symbol": "M3", "value": "S2"}) and
        (.records[$off+11].old[0].fields == {"a": "S2", "b": 10, "c": 20}) and
        (.records[$off+11] | has("new") | not) and
        (.records[$off+12].new[0].fields == {"a": "S2", "b": 10, "c": 20}) and
        (.records[$off+12] | has("old") | not) and
        (.records[$off+13].new[0].fields == {"symbol": "M4", "value": {"state": "null"}}) and
        (.records[$off+14].new[0].fields == {"symbol": "M4", "value": {"state": "null"}}) and
        (.records[$off+15].new[0].fields == {"a": "S3", "b": 100, "c": 200}) and
        (.records[$off+16].new[0].fields == {"symbol": "M5", "value": "S3"}) and
        (.records[$off+17].new[0].fields == {"symbol": "M5", "value": "S3"});
    def tablecase($off; $name):
        ([.records[$off:$off+13][].case] | all(. == $name)) and
        ([.records[$off:$off+13][].operation] | all(. == "listener")) and
        ([.records[$off:$off+13][].time] | all(. == "1970-01-01T00:00:00Z")) and
        ([.records[$off:$off+13][] | has("new") or has("old")] | all) and
        ([.records[$off:$off+13][] | select(has("new")) | .new[0].kind] | all(. == "row")) and
        ([.records[$off:$off+13][] | select(has("new")) | .new | length] | all(. == 1)) and
        ([.records[$off:$off+13][] | select(has("old")) | .old | length] | all(. == 1)) and
        ([.records[$off:$off+13][].statement] == ["select", "select", "selectTwo", "select", "selectTwo", "delete", "select", "selectTwo", "delete", "select", "selectTwo", "select", "selectTwo"]) and
        ([.records[$off:$off+13][].sequence] == [1, 2, 1, 3, 2, 1, 4, 3, 2, 5, 4, 6, 5]) and
        ([.records[$off:$off+13][] | select(.statement == "select" or .statement == "selectTwo") | .new[0].fields | keys] | all(. == ["symbol", "value"])) and
        ([.records[$off:$off+13][] | select(.statement == "delete") | .new[0].fields | keys] | all(. == ["a", "b", "c"])) and
        (.records[$off].new[0].fields == {"symbol": "M1", "value": {"state": "null"}}) and
        (.records[$off+1].new[0].fields == {"symbol": "M1", "value": "S1"}) and
        (.records[$off+2].new[0].fields == {"symbol": "M1", "value": "S1"}) and
        (.records[$off+3].new[0].fields == {"symbol": "M2", "value": {"state": "null"}}) and
        (.records[$off+4].new[0].fields == {"symbol": "M2", "value": {"state": "null"}}) and
        (.records[$off+5].new[0].fields == {"a": "S1", "b": 1, "c": 2}) and
        (.records[$off+5] | has("old") | not) and
        (.records[$off+6].new[0].fields == {"symbol": "M3", "value": "S2"}) and
        (.records[$off+7].new[0].fields == {"symbol": "M3", "value": "S2"}) and
        (.records[$off+8].new[0].fields == {"a": "S2", "b": 10, "c": 20}) and
        (.records[$off+8] | has("old") | not) and
        (.records[$off+9].new[0].fields == {"symbol": "M4", "value": {"state": "null"}}) and
        (.records[$off+10].new[0].fields == {"symbol": "M4", "value": {"state": "null"}}) and
        (.records[$off+11].new[0].fields == {"symbol": "M5", "value": "S3"}) and
        (.records[$off+12].new[0].fields == {"symbol": "M5", "value": "S3"});
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-subq-uncorrel" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 67) and
    ([.records[].operation] | all(. == "listener")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[].statement] | all(. == "create" or . == "select" or . == "selectTwo" or . == "delete")) and
    # nw-no-share: create window without index share; 18 records.  Engine-pinned
    # intra-send order: on SupportBean_A deletes the create remove-stream record
    # precedes the delete record.
    nwcase(0; "nw-no-share") and
    # nw-index-share: enable_window_subquery_indexshare create hint; same listener trace.
    nwcase(18; "nw-index-share") and
    # nw-disable-share: disable_window_subquery_indexshare consumer hints; same listener trace.
    nwcase(36; "nw-disable-share") and
    # table: create table keeps the create listener silent; 13 records.
    tablecase(54; "table")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-subq-uncorrel trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
