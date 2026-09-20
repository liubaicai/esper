#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge-nested.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "failed to resolve Esper commit at $esper_root" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper commit mismatch: want $expected_commit, got $actual_commit" >&2
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
    echo "Java 17 is required; detected version: $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-nested" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 6) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraUpdateNestedEvent{namedWindow=true}", "InfraUpdateNestedEvent{namedWindow=true}", "InfraUpdateNestedEvent{namedWindow=false}", "InfraUpdateNestedEvent{namedWindow=false}", "InfraOnMergeInsertStream{namedWindow=true}", "InfraOnMergeInsertStream{namedWindow=false}"]) and
    (.javaStaticIds == ["java-712b26dbe20bbda50f37", "java-712b26dbe20bbda50f37", "java-712b26dbe20bbda50f37", "java-712b26dbe20bbda50f37", "java-9bcebf3cac321ec7aee2", "java-9bcebf3cac321ec7aee2"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["nested-nw-map", "nested-nw-oa", "nested-table-map", "nested-table-oa", "insertstream-nw", "insertstream-table"]) and
    ([.cases[].ordinal] == [4, 4, 5, 5, 6, 7]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 74) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 6) and
    ([.steps[] | select(.op == "deployed")] | length == 38) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_ST0")] | length == 4) and
    ([.steps[] | select(.op == "snapshot")] | length == 2) and
    ([.steps[] | select(.op == "faf")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 6) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "faf" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge-nested replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge-nested.XXXXXX")
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
    MINGW*|MSYS*|CYGWIN*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
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
    "$script_root/InfraNWTableOnMergeNestedScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergeNestedScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def deployed_at($idx; $name):
        .records[$idx].operation == "deployed" and .records[$idx].statement == $name
            and .records[$idx].sequence == 1;
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-nested" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 58) and
    ([.records[].operation] | all(. == "listener" or . == "deployed" or . == "snapshot" or . == "faf")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # nested-nw-map: 6 deployed + 1 faf = 7.
    ([.records[0:7][].case] | all(. == "nested-nw-map")) and
    deployed_at(0; "schema-composite") and deployed_at(1; "schema-ainfra") and
    deployed_at(2; "infra") and deployed_at(3; "insert") and
    deployed_at(4; "schema-myevent") and deployed_at(5; "merge") and
    .records[6].operation == "faf" and .records[6].statement == "faf" and
    .records[6].sequence == 1 and
    ([.records[6].new[].fields] == [{"ca0": 1, "ca1": 2, "cf0": 1}]) and
    # nested-nw-oa: 6 deployed + 1 faf = 7.
    ([.records[7:14][].case] | all(. == "nested-nw-oa")) and
    deployed_at(7; "schema-composite") and deployed_at(8; "schema-ainfra") and
    deployed_at(9; "infra") and deployed_at(10; "insert") and
    deployed_at(11; "schema-myevent") and deployed_at(12; "merge") and
    .records[13].operation == "faf" and
    ([.records[13].new[].fields] == [{"ca0": 1, "ca1": 2, "cf0": 1}]) and
    # nested-table-map: 6 deployed + 1 faf = 7.
    ([.records[14:21][].case] | all(. == "nested-table-map")) and
    deployed_at(14; "schema-composite") and deployed_at(15; "schema-ainfra") and
    deployed_at(16; "infra") and deployed_at(17; "insert") and
    deployed_at(18; "schema-myevent") and deployed_at(19; "merge") and
    .records[20].operation == "faf" and
    ([.records[20].new[].fields] == [{"ca0": 1, "ca1": 2, "cf0": 1}]) and
    # nested-table-oa: 6 deployed + 1 faf = 7.
    ([.records[21:28][].case] | all(. == "nested-table-oa")) and
    deployed_at(21; "schema-composite") and deployed_at(22; "schema-ainfra") and
    deployed_at(23; "infra") and deployed_at(24; "insert") and
    deployed_at(25; "schema-myevent") and deployed_at(26; "merge") and
    .records[27].operation == "faf" and
    ([.records[27].new[].fields] == [{"ca0": 1, "ca1": 2, "cf0": 1}]) and
    # insertstream-nw: 7 deployed + 7 listener + 1 snapshot = 15.
    ([.records[28:43][].case] | all(. == "insertstream-nw")) and
    deployed_at(28; "schema") and deployed_at(29; "create") and
    deployed_at(30; "merge") and deployed_at(31; "s1") and
    deployed_at(32; "s2") and deployed_at(33; "s3") and deployed_at(34; "s4") and
    ([.records[35:43][].operation] == ["listener", "listener", "listener", "listener", "listener", "listener", "listener", "snapshot"]) and
    ([.records[35:42][] | .statement] == ["s1", "s2", "s3", "s1", "s2", "s3", "s4"]) and
    ([.records[35:42][] | .sequence] == [1, 1, 1, 2, 2, 2, 1]) and
    (.records[35].new[0].fields == {"id": "ID1", "key0": "K1", "p00": 1, "p01Long": {"state": "null"}, "pcommon": {"state": "null"}}) and
    (.records[36].new[0].fields == {"id": "ID1", "key0": "K1"}) and
    (.records[37].new[0].fields == {"id": "ID1", "key0": "K1"}) and
    (.records[38].new[0].fields == {"id": "ID1", "key0": "K2", "p00": 2, "p01Long": {"state": "null"}, "pcommon": {"state": "null"}}) and
    (.records[39].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[40].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[41].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[42].statement == "Create" and .records[42].sequence == 0) and
    ([.records[42].new[].fields] == [{"v1": "K1", "v2": 1}, {"v1": "K2", "v2": 2}]) and
    # insertstream-table: 7 deployed + 7 listener + 1 snapshot = 15.
    ([.records[43:58][].case] | all(. == "insertstream-table")) and
    deployed_at(43; "schema") and deployed_at(44; "create") and
    deployed_at(45; "merge") and deployed_at(46; "s1") and
    deployed_at(47; "s2") and deployed_at(48; "s3") and deployed_at(49; "s4") and
    ([.records[50:58][].operation] == ["listener", "listener", "listener", "listener", "listener", "listener", "listener", "snapshot"]) and
    ([.records[50:57][] | .statement] == ["s1", "s2", "s3", "s1", "s2", "s3", "s4"]) and
    ([.records[50:57][] | .sequence] == [1, 1, 1, 2, 2, 2, 1]) and
    (.records[50].new[0].fields == {"id": "ID1", "key0": "K1", "p00": 1, "p01Long": {"state": "null"}, "pcommon": {"state": "null"}}) and
    (.records[51].new[0].fields == {"id": "ID1", "key0": "K1"}) and
    (.records[52].new[0].fields == {"id": "ID1", "key0": "K1"}) and
    (.records[53].new[0].fields == {"id": "ID1", "key0": "K2", "p00": 2, "p01Long": {"state": "null"}, "pcommon": {"state": "null"}}) and
    (.records[54].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[55].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[56].new[0].fields == {"id": "ID1", "key0": "K2"}) and
    (.records[57].statement == "Create" and .records[57].sequence == 0) and
    ([.records[57].new[].fields] == [{"v1": "K1", "v2": 1}, {"v1": "K2", "v2": 2}])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge-nested trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
