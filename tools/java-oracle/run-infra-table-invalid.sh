#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-invalid.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "Esper commit mismatch: expected $expected_commit, found $actual_commit" >&2
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
    .id == "infra-table-invalid" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableInvalid.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraInvalidAggMatchSingleFunc", "InfraInvalidAggMatchMultiFunc", "InfraInvalidAnnotations", "InfraInvalid"]) and
    (.javaStaticIds == ["java-dc8058ccbd099b1d1361", "java-ecf59e6ef7727ab987fa", "java-53cb0410fe21932803bc", "java-c44ff6d4e86c09522b35"]) and
    (.javaFlags == ["INVALIDITY"]) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["agg-match-single-func", "agg-match-multi-func", "annotations", "invalid"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 219) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 61) and
    ([.steps[] | select(.op == "build-error")] | length == 104) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 50) and
    ([.steps[] | select(.op == "build-error" and .compileWithoutPath == true)] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .compileWithoutPath == true)] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "rate-interval" and .epl == "into table var1 select rate(11) as value from SupportBean")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "window-event-type" and .epl == "into table var1 select window(*) as value from SupportBean#time(1000)")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "annotation-duplicate" and .epl == "create table v1 (abc window(*) @type(SupportBean) @type(SupportBean))")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "table-pattern-atom" and .epl == "select * from pattern[aggvar_ungrouped]")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "build-error" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-invalid replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-invalid.XXXXXX")
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
    "$script_root/InfraTableInvalidScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableInvalidScenarioOracle "$scenario" > "$output"
if ! jq -e \
    --arg err_rate "Incompatible aggregation function for table 'var1' column 'value', expecting 'rate(20)' and received 'rate(11)': The interval-time is 20000 and provided is 11000 [" \
    --arg err_contains "Incompatible aggregation function for table" \
    --arg err_annotation "For column 'abc' multiple annotations provided named 'type' [" \
    --arg err_pattern "Tables cannot be used in pattern filter atoms [" \
    '
    def iscompileerror($index; $case; $statement; $value):
        .records[$index].operation == "compile-error" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].value == $value;
    .version == "esper-parity/v1" and
    .id == "infra-table-invalid" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 104) and
    ([.records[].operation] | all(. == "compile-error")) and
    ([.records[] | select(.case == "agg-match-single-func")] | length == 43) and
    ([.records[] | select(.case == "agg-match-multi-func")] | length == 6) and
    ([.records[] | select(.case == "annotations")] | length == 5) and
    ([.records[] | select(.case == "invalid")] | length == 50) and
    iscompileerror(41; "agg-match-single-func"; "leaving-name-mismatch"; "Incompatible aggregation function for table") and
    iscompileerror(44; "agg-match-multi-func"; "window-vs-sorted"; "Failed to validate select-clause expression '"'"'sorted(intPrimitive)'"'"': When specifying into-table a sort expression cannot be provided [") and
    iscompileerror(45; "agg-match-multi-func"; "window-event-type"; "Incompatible aggregation function for table '"'"'var1'"'"' column '"'"'value'"'"', expecting '"'"'window(*)'"'"' and received '"'"'window(*)'"'"': The required event type is '"'"'SupportBean_S0'"'"' and provided is '"'"'SupportBean'"'"' [") and
    iscompileerror(51; "annotations"; "annotation-duplicate"; $err_annotation) and
    iscompileerror(101; "invalid"; "table-pattern-atom"; $err_pattern) and
    ([.records[] | select(.case == "agg-match-single-func" and (.value // "") == $err_contains)] | length == 31) and
    ([.records[] | select(has("value") | not)] | length == 2)
    ' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
