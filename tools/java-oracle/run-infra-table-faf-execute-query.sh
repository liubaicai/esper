#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-faf-execute-query.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
        --esper-root=*)
            esper_root=${1#*=}
            shift
            ;;
        --scenario)
            [ "$#" -ge 2 ] || usage
            scenario=$2
            shift 2
            ;;
        --scenario=*)
            scenario=${1#*=}
            shift
            ;;
        --output)
            [ "$#" -ge 2 ] || usage
            output=$2
            shift 2
            ;;
        --output=*)
            output=${1#*=}
            shift
            ;;
        --skip-build)
            skip_build=1
            shift
            ;;
        *)
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
    .id == "infra-table-faf-execute-query" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableFAFExecuteQuery.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraFAFInsert", "InfraFAFDelete", "InfraFAFUpdate", "InfraFAFSelect"]) and
    (.javaStaticIds == ["java-1899404b366f6f3c1a31", "java-11e9c7fcb6e1617929a9", "java-db32f6072d9b36a84471", "java-c87f698c077e7dd7ada7"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["faf-insert", "faf-delete", "faf-update", "faf-select"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 44) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 10) and
    ([.steps[] | select(.op == "deployed")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 14) and
    ([.steps[] | select(.op == "snapshot")] | length == 5) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "FafInsert" and .epl == "insert into MyTableINS (p0, p1) select '"'"'a'"'"', 1")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "FafDelete" and .epl == "delete from MyTableDEL")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "FafUpdate" and .epl == "update MyTableUPD set p1 = '"'"'ABC'"'"'")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .statement == "FafSelect" and .epl == "select * from MyTableSEL")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .count == 10)] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .count == 0)] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-faf-execute-query replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-faf-execute-query.XXXXXX")
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
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/avro-1.11.3.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-annotations-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-core-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-databind-2.14.2.jar")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraTableFAFExecuteQueryScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableFAFExecuteQueryScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def issnapshot($index; $case; $statement; $rows):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows) and
        ((.records[$index] | has("new")) == (($rows | length) > 0)) and
        ((.records[$index] | has("count")) | not);
    def issnapshotcount($index; $case; $statement; $rows; $count):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows) and
        ((.records[$index] | has("new")) == (($rows | length) > 0)) and
        (.records[$index].count == $count);
    def insertcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        issnapshot($base+1; $case; "create"; [{"p0": "a", "p1": 1}]);
    def deletecase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        issnapshotcount($base+2; $case; "create";
            [{"p0": "G0", "thesum": 0}, {"p0": "G1", "thesum": 1},
             {"p0": "G2", "thesum": 2}, {"p0": "G3", "thesum": 3},
             {"p0": "G4", "thesum": 4}, {"p0": "G5", "thesum": 5},
             {"p0": "G6", "thesum": 6}, {"p0": "G7", "thesum": 7},
             {"p0": "G8", "thesum": 8}, {"p0": "G9", "thesum": 9}]; 10) and
        issnapshotcount($base+3; $case; "create"; []; 0);
    def updatecase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        issnapshot($base+2; $case; "TheTable";
            [{"p0": "E1", "p1": "ABC"}, {"p0": "E2", "p1": "ABC"}]);
    def selectcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        issnapshot($base+2; $case; "FafSelect"; [{"p0": "E1"}, {"p0": "E2"}]);
    .version == "esper-parity/v1" and
    .id == "infra-table-faf-execute-query" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 12) and
    ([.records[].operation] | all(. == "deployed" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    insertcase(0; "faf-insert") and
    deletecase(2; "faf-delete") and
    updatecase(6; "faf-update") and
    selectcase(9; "faf-select")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-faf-execute-query trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
