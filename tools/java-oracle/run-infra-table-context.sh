#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-context.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "Esper commit mismatch: expected $expected_commit got $actual_commit" >&2
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
    .id == "infra-table-context" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableContext.java" and
    (.javaRuntimes | type == "array" and length == 3) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraPartitioned", "InfraNonOverlapping", "InfraTableContextInvalid"]) and
    (.javaStaticIds == ["java-62ab3ac014745d5ab5c5", "java-969b28d4058f010a19af", "java-69b99206b5ca50456737"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["context-partitioned", "context-nonoverlapping", "context-invalid"]) and
    ([.cases[].ordinal] == [0, 1, 2]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 46) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deployed")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 9) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 4) and
    ([.steps[] | select(.op == "build-error")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0" and .epl == "@name('"'"'s0'"'"') context CtxNowTillS0 select pkey as c0, thesum as c1 from MyTable output snapshot when terminated")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "subquery-table" and .epl == "select (select * from MyTable) from SupportBean")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "build-error" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-context replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-context.XXXXXX")
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
    "$script_root/InfraTableContextScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableContextScenarioOracle "$scenario" > "$output"
if ! jq -e \
    --arg err_vis "Table by name 'MyTable' has been declared for context 'SimpleCtx' and can only be used within the same context [" \
    --arg err_sub "Failed to plan subquery number 1 querying MyTable: Mismatch in context specification, the context for the table 'MyTable' is 'SimpleCtx' and the query specifies no context  [select (select * from MyTable) from SupportBean]" \
    '
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
    def iscompileerror($index; $case; $statement; $value):
        .records[$index].operation == "compile-error" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].value == $value;
    def partitionedcase($base; $case):
        isdeployed($base; $case; "ctx"; 1) and
        isdeployed($base+1; $case; "create"; 1) and
        isdeployed($base+2; $case; "into"; 1) and
        isdeployed($base+3; $case; "s0"; 1) and
        islistener($base+4; $case; 1; [{"c0": 110}]) and
        islistener($base+5; $case; 2; [{"c0": 20}]);
    def nonoverlappingcase($base; $case):
        isdeployed($base; $case; "ctx"; 1) and
        isdeployed($base+1; $case; "create"; 1) and
        isdeployed($base+2; $case; "into"; 1) and
        isdeployed($base+3; $case; "s0"; 1) and
        islistener($base+4; $case; 1; [{"c0": "E1", "c1": 110}, {"c0": "E2", "c1": 20}]) and
        isdeployed($base+5; $case; "index"; 1) and
        isdeployed($base+6; $case; "join"; 1) and
        islistener($base+7; $case; 2; [{"c0": "E1", "c1": 30}, {"c0": "E3", "c1": 100}]);
    def invalidcase($base; $case):
        isdeployed($base; $case; "ctx"; 1) and
        isdeployed($base+1; $case; "create"; 1) and
        iscompileerror($base+2; $case; "select-table"; $err_vis) and
        iscompileerror($base+3; $case; "subquery-table"; $err_sub) and
        iscompileerror($base+4; $case; "insert-table"; $err_vis);
    .version == "esper-parity/v1" and
    .id == "infra-table-context" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 19) and
    ([.records[].operation] | all(. == "deployed" or . == "listener" or . == "compile-error")) and
    ([.records[] | select(has("time")) | .time] | all(. == "1970-01-01T00:00:00Z")) and
    partitionedcase(0; "context-partitioned") and
    nonoverlappingcase(6; "context-nonoverlapping") and
    invalidcase(14; "context-invalid")
    ' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-context trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
