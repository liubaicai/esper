#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-table-count-min-sketch.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-table-count-min-sketch" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/tbl/InfraTableCountMinSketch.java" and
    (.javaRuntimes == ["java-runtime-f09401ff1e3349b3497b", "java-runtime-87f8d0057f91e1dfa7ec", "java-runtime-4b4c531bbad3d4e1491d", "java-runtime-217779fa3c860cc1ea18"]) and
    (.javaNames == ["InfraFrequencyAndTopk", "InfraDocSamples", "InfraNonStringType", "InfraInvalid"]) and
    (.javaStaticIds == ["java-834a25e0038e5f5a8950", "java-d926cce1a816fb780ed8", "java-304d2b5b2913d2f02828", "java-91bcaf7a054d58304543"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["frequency-and-topk", "doc-samples", "non-string-type", "invalid"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 88) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 14) and
    ([.steps[] | select(.op == "deployed")] | length == 14) and
    ([.steps[] | select(.op == "send")] | length == 35) and
    ([.steps[] | select(.op == "build-error")] | length == 15) and
    ([.steps[] | select(.op == "undeploy")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "module" and .case == "frequency-and-topk")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "add-distinct" and .epl == "into table MyCMS select countMinSketchAdd(distinct '"'"'abc'"'"') as wordcms from SupportByteArrEventStringId")] | length == 1) and
    ([.steps[] | select(.op == "build-error" and .statement == "topk-param" and .epl == "select MyCMS.wordcms.countMinSketchTopk(theString) from SupportBean")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportByteArrEventStringId")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 17) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "build-error" and .op != "undeploy" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-table-count-min-sketch replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-table-count-min-sketch.XXXXXX")
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
    "$script_root/InfraTableCountMinSketchScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraTableCountMinSketchScenarioOracle "$scenario" > "$output"
if ! jq -e \
    --arg err_distinct "Failed to validate select-clause expression 'countMinSketchAdd(distinct \"abc\")': Count-min-sketch aggregation function 'countMinSketchAdd' is not supported with distinct [" \
    --arg err_topkparam "Failed to validate select-clause expression 'MyCMS.wordcms.countMinSketchTopk(th...(43 chars)': Count-min-sketch aggregation function 'countMinSketchTopk' requires a no parameter expressions [" \
    '
    def iscompileerror($case; $statement; $value):
        any(.records[]; .operation == "compile-error" and .case == $case
            and .statement == $statement and .value == $value);
    .version == "esper-parity/v1" and
    .id == "infra-table-count-min-sketch" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 57) and
    ([.records[] | select(.case == "frequency-and-topk")] | length == 29) and
    ([.records[] | select(.case == "doc-samples")] | length == 7) and
    ([.records[] | select(.case == "non-string-type")] | length == 5) and
    ([.records[] | select(.case == "invalid")] | length == 16) and
    ([.records[] | select(.operation == "deployed")] | length == 14) and
    ([.records[] | select(.operation == "listener")] | length == 28) and
    ([.records[] | select(.operation == "compile-error")] | length == 15) and
    ([.records[] | select(.case == "frequency-and-topk" and .operation == "listener" and .statement == "frequency")] | length == 17) and
    ([.records[] | select(.case == "frequency-and-topk" and .operation == "listener" and .statement == "topk")] | length == 7) and
    ([.records[] | select(.case == "frequency-and-topk" and .operation == "listener" and .statement == "join")] | length == 1) and
    ([.records[] | select(.case == "frequency-and-topk" and .operation == "listener" and .statement == "subq")] | length == 1) and
    ([.records[] | select(.case == "non-string-type" and .operation == "listener" and .statement == "s0")] | length == 2) and
    iscompileerror("invalid"; "add-distinct"; $err_distinct) and
    iscompileerror("invalid"; "topk-param"; $err_topkparam) and
    ([.records[] | select(.operation == "listener" and .statement == "topk" and .sequence == 7) | .new[0].fields.topk | map(.value)] == [["E2", "E4", "E1"]])
    ' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-table-count-min-sketch trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
