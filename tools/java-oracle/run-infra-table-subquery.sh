#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-subquery.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "Esper commit drift: expected $expected_commit, found $actual_commit" >&2
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
    .id == "infra-table-subquery" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableSubquery.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraTableSubqueryAgainstKeyed", "InfraTableSubqueryAgainstUnkeyed", "InfraTableSubquerySecondaryIndex", "InfraTableSubqueryInFilter"]) and
    (.javaStaticIds == ["java-84c3e4b24f1621c4e20f", "java-8a837e4f238a838a719c", "java-22c9a3e16812af54b583", "java-05c24f1601cc75b1683e"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["subquery-keyed", "subquery-unkeyed", "subquery-secondary-index", "subquery-in-filter"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 54) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 11) and
    ([.steps[] | select(.op == "deployed")] | length == 11) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 14) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 10) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "merge" and .epl == "on SupportBean_S0 merge MyTable where p00 = k0 and p01 = k1 when not matched then insert select p00 as k0, p01 as k1, p02 as p2, id as value when matched then update set p2 = p02, value = id ")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "module" and .epl == "create table MyTable(tablecol string primary key);\ninsert into MyTable select p00 as tablecol from SupportBean_S0;\n@name('"'"'s0'"'"') select * from SupportBean(theString=(select tablecol from MyTable).orderBy().firstOf())")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-subquery replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-subquery.XXXXXX")
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
    "$script_root/InfraTableSubqueryScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableSubqueryScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def islistener($index; $case; $seq; $fields):
        .records[$index].operation == "listener" and
        .records[$index].case == $case and
        .records[$index].statement == "s0" and
        .records[$index].sequence == $seq and
        ([.records[$index].new[]?.fields] == $fields) and
        ((.records[$index] | has("old")) | not);
    def isbeanlistener($index; $case; $seq; $theString):
        .records[$index].operation == "listener" and
        .records[$index].case == $case and
        .records[$index].statement == "s0" and
        .records[$index].sequence == $seq and
        (.records[$index].new | length == 1) and
        (.records[$index].new[0].kind == "row") and
        (.records[$index].new[0].fields | length == 20) and
        (.records[$index].new[0].fields.theString == $theString) and
        (.records[$index].new[0].fields.intPrimitive == 0) and
        ((.records[$index] | has("old")) | not);
    def keyedcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "into"; 1) and
        isdeployed($base+2; $case; "s0"; 1) and
        islistener($base+3; $case; 1; [{"value": {"state": "null"}}]) and
        islistener($base+4; $case; 2; [{"value": 200}]) and
        islistener($base+5; $case; 3; [{"value": 100}]) and
        islistener($base+6; $case; 4; [{"value": 200}]);
    def unkeyedcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "s0"; 1) and
        isdeployed($base+2; $case; "insert"; 1) and
        islistener($base+3; $case; 1; [{"c0": 10}]);
    def secidxcase($base; $case):
        isdeployed($base; $case; "create"; 1) and
        isdeployed($base+1; $case; "index"; 1) and
        isdeployed($base+2; $case; "merge"; 1) and
        isdeployed($base+3; $case; "s0"; 1) and
        islistener($base+4; $case; 1; [{"c0": 10}]) and
        islistener($base+5; $case; 2; [{"c0": {"state": "null"}}]) and
        islistener($base+6; $case; 3; [{"c0": 11}]);
    def filtercase($base; $case):
        isdeployed($base; $case; "module"; 1) and
        isbeanlistener($base+1; $case; 1; "E") and
        isbeanlistener($base+2; $case; 2; "C") and
        isbeanlistener($base+3; $case; 3; "C") and
        isbeanlistener($base+4; $case; 4; "A");
    .version == "esper-parity/v1" and
    .id == "infra-table-subquery" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 23) and
    ([.records[].operation] | all(. == "deployed" or . == "listener")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    keyedcase(0; "subquery-keyed") and
    unkeyedcase(7; "subquery-unkeyed") and
    secidxcase(11; "subquery-secondary-index") and
    filtercase(18; "subquery-in-filter")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-subquery trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
