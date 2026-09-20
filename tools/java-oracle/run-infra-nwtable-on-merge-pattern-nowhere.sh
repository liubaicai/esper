#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge-pattern-nowhere.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "could not resolve Esper checkout commit: $esper_root" >&2
    exit 1
}
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required, found version $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-pattern-nowhere" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 6) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraPatternMultimatch{namedWindow=true}", "InfraPatternMultimatch{namedWindow=false}", "InfraNoWhereClause{namedWindow=true}", "InfraNoWhereClause{namedWindow=false}", "InfraMultipleInsert{namedWindow=true}", "InfraMultipleInsert{namedWindow=false}"]) and
    (.javaStaticIds == ["java-74cbcb4f6ad520a0f3fd", "java-74cbcb4f6ad520a0f3fd", "java-2b5b9c17389359bc78f7", "java-2b5b9c17389359bc78f7", "java-227e4a044ce4b9c1de81", "java-227e4a044ce4b9c1de81"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["patternmultimatch-nw", "patternmultimatch-table", "nowhere-nw", "nowhere-table", "multipleinsert-nw", "multipleinsert-table"]) and
    ([.cases[].ordinal] == [26, 27, 28, 29, 30, 31]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 102) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 8) and
    ([.steps[] | select(.op == "deployed")] | length == 22) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent")] | length == 24) and
    ([.steps[] | select(.op == "snapshot")] | length == 20) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 6) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge-pattern-nowhere replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge-pattern-nowhere.XXXXXX")
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
        native_path() { cygpath -w "$1"; }
        cp_sep=';'
        ;;
    *)
        native_path() { printf '%s' "$1"; }
        cp_sep=':'
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/avro-1.11.3.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-annotations-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-core-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-databind-2.14.2.jar")"
# regression-lib classes provide the SupportBean_A bean the
# InfraNoWhereClause delete trigger fires on.
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableOnMergePatternNoWhereScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergePatternNoWhereScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def issnapshot($index; $case; $rows):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == "create" and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows);
    def ismerge($index; $case; $seq; $col1; $col2):
        .records[$index].operation == "listener" and
        .records[$index].case == $case and
        .records[$index].statement == "Merge" and
        .records[$index].sequence == $seq and
        (.records[$index] | has("old") | not) and
        (.records[$index].new | length == 1) and
        (.records[$index].new[0].fields | .col1 == $col1 and .col2 == $col2);
    def pmcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "merge"; 1) and
        issnapshot($base+2; $case; [{"c1": "A1", "c2": "B1"},
            {"c1": "A2", "c2": "B1"}]) and
        issnapshot($base+3; $case; [{"c1": "A1", "c2": "B1"},
            {"c1": "A2", "c2": "B1"},
            {"c1": "A3", "c2": "B2"},
            {"c1": "A4", "c2": "B2"}]);
    def nwccase($base; $case):
        isdeployed($base; $case; "schema-myevent"; 1) and
        isdeployed($base+1; $case; "schema-myschema"; 1) and
        isdeployed($base+2; $case; "create"; 1) and
        isdeployed($base+3; $case; "delete"; 1) and
        isdeployed($base+4; $case; "merge"; 1) and
        issnapshot($base+5; $case; [{"col1": "xE1x", "col2": -2}]) and
        issnapshot($base+6; $case; [{"col1": "xE1x", "col2": -2}]) and
        issnapshot($base+7; $case; []) and
        issnapshot($base+8; $case; [{"col1": "A1", "col2": 4}]) and
        issnapshot($base+9; $case; [{"col1": "A1", "col2": 4}]) and
        issnapshot($base+10; $case; []) and
        issnapshot($base+11; $case; [{"col1": "B1", "col2": 5}]) and
        issnapshot($base+12; $case; [{"col1": "Z", "col2": -1}]);
    def micase($base; $case):
        isdeployed($base; $case; "schema-myevent"; 1) and
        isdeployed($base+1; $case; "schema-myschema"; 1) and
        isdeployed($base+2; $case; "infra"; 1) and
        isdeployed($base+3; $case; "merge"; 1) and
        ismerge($base+4; $case; 1; "A1"; 1) and
        ismerge($base+5; $case; 2; "B1"; 2) and
        ismerge($base+6; $case; 3; "Z"; -1) and
        ismerge($base+7; $case; 4; "xD1x"; -4);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-pattern-nowhere" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 50) and
    ([.records[].operation] | all(. == "deployed" or . == "listener" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    pmcase(0; "patternmultimatch-nw") and
    pmcase(4; "patternmultimatch-table") and
    nwccase(8; "nowhere-nw") and
    nwccase(21; "nowhere-table") and
    micase(34; "multipleinsert-nw") and
    micase(42; "multipleinsert-table")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge-pattern-nowhere trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
