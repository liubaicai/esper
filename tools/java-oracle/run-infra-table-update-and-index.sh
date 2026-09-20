#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-update-and-index.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-table-update-and-index" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableUpdateAndIndex.java" and
    (.javaRuntimes | type == "array" and length == 5) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraEarlyUniqueIndexViolation", "InfraLateUniqueIndexViolation", "InfraFAFUpdate", "InfraTableKeyUpdateSingleKey", "InfraTableKeyUpdateMultiKey"]) and
    (.javaStaticIds == ["java-c8309a6f377c34ffcdb4", "java-8db9b9cd27d04bb86d69", "java-4a2462a159e9d648257a", "java-ad9c04a3fc0d98d4e4dd", "java-97fc9971820f1e5b947d"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["early-unique-violation", "late-unique-violation", "faf-update", "key-update-single", "key-update-multi"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3, 4]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 84) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "deploy")] | length == 20) and
    ([.steps[] | select(.op == "deployed")] | length == 17) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 6) and
    ([.steps[] | select(.op == "send-error" and .eventType == "SupportBean_S1")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 11) and
    ([.steps[] | select(.op == "deploy-error")] | length == 2) and
    ([.steps[] | select(.op == "build-error")] | length == 1) and
    ([.steps[] | select(.op == "faf-error")] | length == 1) and
    ([.steps[] | select(.op == "undeploy")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "FafUpdateCol0E1" and .epl == "update MyTableFAFU set col0 = 1 where pkey0='"'"'E1'"'"'")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .statement == "FafSelectCol0" and .epl == "select pkey0 from MyTableFAFU where col0=1")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "send-error" and .op != "snapshot" and .op != "deploy-error" and .op != "build-error" and .op != "faf-error" and .op != "undeploy" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-update-and-index replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-update-and-index.XXXXXX")
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
    CYGWIN*|MINGW*|MSYS*)
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
    "$script_root/InfraTableUpdateAndIndexScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableUpdateAndIndexScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def iserror($index; $case; $op; $statement; $value):
        .records[$index].operation == $op and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 1 and
        .records[$index].value == $value;
    def issnapshot($index; $case; $statement; $rows):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows);
    def earlycase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        iserror($base+2; $case; "deploy-error"; "sec-index-pkey0";
            "Failed to deploy: Unique index violation, index '"'"'SecIndex'"'"' is a unique index and key '"'"'E1'"'"' already exists") and
        iserror($base+3; $case; "faf-error"; "faf-update-pkey1";
            "Unique index violation, index '"'"'MyTableEUIV'"'"' is a unique index and key '"'"'MultiKey[E1,0]'"'"' already exists") and
        issnapshot($base+4; $case; "create";
            [{"pkey0": "E1", "pkey1": 10}, {"pkey0": "E1", "pkey1": 20}]) and
        isdeployed($base+5; $case; "on-update"; 1) and
        iserror($base+6; $case; "send-error"; "on-update-send";
            "Unexpected exception in statement '"'"'on-update'"'"': Unique index violation, index '"'"'MyTableEUIV'"'"' is a unique index and key '"'"'MultiKey[E1,0]'"'"' already exists") and
        issnapshot($base+7; $case; "create";
            [{"pkey0": "E1", "pkey1": 10}, {"pkey0": "E1", "pkey1": 20}]) and
        iserror($base+8; $case; "build-error"; "on-merge-pkey1";
            "Validation failed in when-matched (clause 1): On-merge statements may not update unique keys of tables");
    def latecase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        isdeployed($base+2; $case; "on-merge"; 1) and
        iserror($base+3; $case; "deploy-error"; "sec-index-col0";
            "Failed to deploy: Create-index adds a unique key on columns that are updated by one or more on-merge statements") and
        isdeployed($base+4; $case; "on-update"; 1) and
        isdeployed($base+5; $case; "sec-index-pkey1"; 1) and
        iserror($base+6; $case; "send-error"; "on-update-send";
            "Unexpected exception in statement '"'"'on-update'"'"': Unique index violation, index '"'"'MyUniqueSecondary'"'"' is a unique index and key '"'"'0'"'"' already exists") and
        issnapshot($base+7; $case; "create";
            [{"pkey0": "E1", "pkey1": 10}, {"pkey0": "E2", "pkey1": 20}]);
    def fafcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "create-index"; 1) and
        isdeployed($base+2; $case; "into"; 1) and
        issnapshot($base+3; $case; "FafSelectCol0"; [{"pkey0": "E1"}]) and
        issnapshot($base+4; $case; "FafSelectCol1"; [{"pkey0": "E1"}]);
    def singlecase($base; $case):
        isdeployed($base; $case; "s0"; 1) and
        isdeployed($base+1; $case; "insert"; 1) and
        isdeployed($base+2; $case; "on-update"; 1) and
        issnapshot($base+3; $case; "s0";
            [{"c0": 10, "pkey0": "E1"}, {"c0": 20, "pkey0": "E20"}, {"c0": 30, "pkey0": "E3"}]) and
        issnapshot($base+4; $case; "s0";
            [{"c0": 10, "pkey0": "E10"}, {"c0": 20, "pkey0": "E20"}, {"c0": 30, "pkey0": "E3"}]) and
        issnapshot($base+5; $case; "s0";
            [{"c0": 10, "pkey0": "E10"}, {"c0": 20, "pkey0": "E20"}, {"c0": 30, "pkey0": "E30"}]);
    def multicase($base; $case):
        isdeployed($base; $case; "s1"; 1) and
        isdeployed($base+1; $case; "insert"; 1) and
        isdeployed($base+2; $case; "on-update"; 1) and
        issnapshot($base+3; $case; "s1";
            [{"c0": 100, "pkey0": "E1", "pkey1": 10}, {"c0": 200, "pkey0": "E20", "pkey1": 20}, {"c0": 300, "pkey0": "E3", "pkey1": 30}]) and
        issnapshot($base+4; $case; "s1";
            [{"c0": 100, "pkey0": "E10", "pkey1": 10}, {"c0": 200, "pkey0": "E20", "pkey1": 20}, {"c0": 300, "pkey0": "E3", "pkey1": 30}]) and
        issnapshot($base+5; $case; "s1";
            [{"c0": 100, "pkey0": "E10", "pkey1": 10}, {"c0": 200, "pkey0": "E20", "pkey1": 20}, {"c0": 300, "pkey0": "E30", "pkey1": 30}]);
    .version == "esper-parity/v1" and
    .id == "infra-table-update-and-index" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 34) and
    ([.records[].operation] | all(. == "deployed" or . == "snapshot" or . == "deploy-error" or . == "faf-error" or . == "send-error" or . == "build-error")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    earlycase(0; "early-unique-violation") and
    latecase(9; "late-unique-violation") and
    fafcase(17; "faf-update") and
    singlecase(22; "key-update-single") and
    multicase(28; "key-update-multi")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-update-and-index trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
