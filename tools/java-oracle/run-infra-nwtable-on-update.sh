#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-update.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-on-update" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnUpdate.java" and
    (.javaRuntimes | type == "array" and length == 6) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraNWTableOnUpdateSceneOne{namedWindow=true}", "InfraNWTableOnUpdateSceneOne{namedWindow=false}", "InfraSubquerySelf{namedWindow=true}", "InfraSubquerySelf{namedWindow=false}", "InfraSubqueryMultikeyWArray{namedWindow=true}", "InfraSubqueryMultikeyWArray{namedWindow=false}"]) and
    (.javaStaticIds == ["java-bc54a78188b2249a0a9f", "java-bc54a78188b2249a0a9f", "java-c67318a7541b42eb7940", "java-c67318a7541b42eb7940", "java-9ab08590ab6533e21139", "java-9ab08590ab6533e21139"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["sceneone-nw", "sceneone-table", "subqself-nw", "subqself-table", "multikey-nw", "multikey-table"]) and
    ([.cases[].ordinal] == [0, 1, 4, 5, 6, 7]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 92) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 18) and
    ([.steps[] | select(.op == "deployed")] | length == 16) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 14) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 6) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportEventWithIntArray")] | length == 8) and
    ([.steps[] | select(.op == "snapshot")] | length == 14) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 6) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-update replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-update.XXXXXX")
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
    "$script_root/InfraNWTableOnUpdateScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnUpdateScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def deployed_at($idx; $name):
        .records[$idx].operation == "deployed" and .records[$idx].statement == $name
            and .records[$idx].sequence == 1;
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-update" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 42) and
    ([.records[].operation] | all(. == "listener" or . == "deployed" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # sceneone-nw: 3 deployed + 6 listener + 3 snapshot = 12.
    ([.records[0:12][].case] | all(. == "sceneone-nw")) and
    deployed_at(0; "create") and deployed_at(1; "insert") and deployed_at(4; "update") and
    ([.records[2:12][].operation] == ["listener", "listener", "deployed", "listener", "listener", "snapshot", "listener", "listener", "snapshot", "snapshot"]) and
    ([.records[2:12][] | select(.operation == "listener") | .statement] == ["create", "create", "create", "update", "create", "update"]) and
    ([.records[2:12][] | select(.operation == "listener" and .statement == "create") | .sequence] == [1, 2, 3, 4]) and
    ([.records[2:12][] | select(.operation == "listener" and .statement == "update") | .sequence] == [1, 2]) and
    (.records[2].new[0].fields.theString == "A1" and .records[2].new[0].fields.intPrimitive == 1) and
    (.records[3].new[0].fields.theString == "B2" and .records[3].new[0].fields.intPrimitive == 2) and
    (.records[5].new[0].fields.theString == "X1" and .records[5].old[0].fields.theString == "A1") and
    (.records[6].new[0].fields.theString == "X1" and .records[6].old[0].fields.theString == "A1") and
    ([.records[7].new[].fields] == [{"intPrimitive": 2, "theString": "B2"}, {"intPrimitive": 1, "theString": "X1"}]) and
    (.records[8].new[0].fields.theString == "X2" and .records[8].old[0].fields.theString == "B2") and
    (.records[9].new[0].fields.theString == "X2" and .records[9].old[0].fields.theString == "B2") and
    ([.records[10].new[].fields] == [{"intPrimitive": 1, "theString": "X1"}, {"intPrimitive": 2, "theString": "X2"}]) and
    ([.records[11].new[].fields] == [{"intPrimitive": 1, "theString": "X1"}, {"intPrimitive": 2, "theString": "X2"}]) and
    # sceneone-table: 3 deployed + 2 listener + 3 snapshot = 8.
    ([.records[12:20][].case] | all(. == "sceneone-table")) and
    deployed_at(12; "create") and deployed_at(13; "insert") and deployed_at(14; "update") and
    ([.records[15:20][].operation] == ["listener", "snapshot", "listener", "snapshot", "snapshot"]) and
    ([.records[15:20][] | select(.operation == "listener") | .statement] | all(. == "update")) and
    (.records[15].new[0].fields.theString == "X1" and .records[15].old[0].fields.theString == "A1") and
    (.records[17].new[0].fields.theString == "X2" and .records[17].old[0].fields.theString == "B2") and
    ([.records[16].new[].fields] == [{"intPrimitive": 1, "theString": "X1"}, {"intPrimitive": 2, "theString": "B2"}]) and
    ([.records[18].new[].fields] == [{"intPrimitive": 1, "theString": "X1"}, {"intPrimitive": 2, "theString": "X2"}]) and
    ([.records[19].new[].fields] == [{"intPrimitive": 1, "theString": "X1"}, {"intPrimitive": 2, "theString": "X2"}]) and
    # subqself-nw: 3 deployed + 1 snapshot = 4.
    ([.records[20:24][].case] | all(. == "subqself-nw")) and
    deployed_at(20; "create") and deployed_at(21; "Insert") and deployed_at(22; "Self Update") and
    (.records[23].operation == "snapshot" and .records[23].statement == "create") and
    ([.records[23].new[].fields] == [{"intPrimitive": 3, "theString": "E1"}, {"intPrimitive": 7, "theString": "E2"}]) and
    # subqself-table: 3 deployed + 1 snapshot = 4.
    ([.records[24:28][].case] | all(. == "subqself-table")) and
    deployed_at(24; "create") and deployed_at(25; "Insert") and deployed_at(26; "Self Update") and
    (.records[27].operation == "snapshot" and .records[27].statement == "create") and
    ([.records[27].new[].fields] == [{"intPrimitive": 3, "theString": "E1"}, {"intPrimitive": 7, "theString": "E2"}]) and
    # multikey-nw: 2 deployed + 4 listener + 3 snapshot = 9.
    ([.records[28:37][].case] | all(. == "multikey-nw")) and
    deployed_at(28; "create") and deployed_at(29; "Update") and
    ([.records[30:37][].operation] == ["listener", "listener", "snapshot", "listener", "snapshot", "listener", "snapshot"]) and
    ([.records[30:37][] | select(.operation == "listener") | .statement] | all(. == "create")) and
    ([.records[29:37][] | select(.operation == "listener") | .sequence] == [1, 2, 3, 4]) and
    (.records[30].new[0].fields == {"value": 0}) and
    (.records[30] | has("old") | not) and
    (.records[31].new[0].fields == {"value": 21}) and
    (.records[31].old[0].fields == {"value": 0}) and
    ([.records[32].new[].fields] == [{"value": 21}]) and
    (.records[33].new[0].fields == {"value": 33}) and
    (.records[33].old[0].fields == {"value": 21}) and
    ([.records[34].new[].fields] == [{"value": 33}]) and
    (.records[35].new[0].fields == {"value": {"state": "null"}}) and
    (.records[35].old[0].fields == {"value": 33}) and
    ([.records[36].new[].fields] == [{"value": {"state": "null"}}]) and
    # multikey-table: 2 deployed + 3 snapshot = 5.
    ([.records[37:42][].case] | all(. == "multikey-table")) and
    deployed_at(37; "create") and deployed_at(38; "Update") and
    ([.records[39:42][].operation] | all(. == "snapshot")) and
    ([.records[39:42][] | .statement] | all(. == "create")) and
    ([.records[39].new[].fields] == [{"value": 21}]) and
    ([.records[40].new[].fields] == [{"value": 33}]) and
    ([.records[41].new[].fields] == [{"value": {"state": "null"}}])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-update trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
