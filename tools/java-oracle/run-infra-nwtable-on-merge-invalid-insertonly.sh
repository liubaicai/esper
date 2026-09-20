#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge-invalid-insertonly.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "Esper checkout is not the pinned oracle commit $expected_commit (got $actual_commit)" >&2
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
    .id == "infra-nwtable-on-merge-invalid-insertonly" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 6) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraInvalid{namedWindow=true}", "InfraInvalid{namedWindow=false}", "InfraInsertOnly{namedWindow=true, useEquivalent=true, soda=false, useColumnNames=false}", "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=false}", "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=false, useColumnNames=true}", "InfraInsertOnly{namedWindow=true, useEquivalent=false, soda=true, useColumnNames=false}"]) and
    (.javaStaticIds == ["java-42d23ac20541998f0a55", "java-42d23ac20541998f0a55", "java-0b6e7bd4eb235001d140", "java-0b6e7bd4eb235001d140", "java-0b6e7bd4eb235001d140", "java-0b6e7bd4eb235001d140"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["invalid-nw", "invalid-table", "insertonly-nw-equivalent", "insertonly-nw", "insertonly-nw-colnames", "insertonly-nw-soda"]) and
    ([.cases[].ordinal] == [40, 41, 42, 43, 44, 45]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 81) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 18) and
    ([.steps[] | select(.op == "deployed")] | length == 8) and
    ([.steps[] | select(.op == "build-error")] | length == 25) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 8) and
    ([.steps[] | select(.op == "snapshot")] | length == 8) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 8) and
    ([.steps[] | select(.op == "build-error" and .case == "invalid-nw") | .statement] == ["notmatched-filter-windowevent", "insert-unknown-column", "notmatched-update-action", "matched-insert-wrong-type", "missing-clauses", "missing-then", "and-then-delete", "ambiguous-where-prop", "invalid-select-prop", "invalid-match-where-prop", "variable-lhs", "indexed-lhs", "nested-event-assign"]) and
    ([.steps[] | select(.op == "build-error" and .case == "invalid-table") | .statement] == ["notmatched-filter-windowevent", "insert-unknown-column", "notmatched-update-action", "missing-clauses", "missing-then", "and-then-delete", "ambiguous-where-prop", "invalid-select-prop", "invalid-match-where-prop", "variable-lhs", "indexed-lhs", "nested-event-assign"]) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "build-error" and .op != "send" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge-invalid-insertonly replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge-invalid-insertonly.XXXXXX")
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
# regression-lib classes provide the SupportBean_A bean the InfraInvalid
# probes reference.
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergeInvalidInsertOnlyScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def islistener($index; $case; $seq):
        .records[$index].operation == "listener" and
        .records[$index].case == $case and
        .records[$index].statement == "on" and
        .records[$index].sequence == $seq;
    def issnapshot($index; $case; $rows):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == "Window" and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows);
    def iscompileerror($index; $case; $statement):
        .records[$index].operation == "compile-error" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 0 and
        (.records[$index].value | type == "string" and length > 0);
    def invalidnw($base; $case):
        iscompileerror($base; $case; "notmatched-filter-windowevent") and
        iscompileerror($base+1; $case; "insert-unknown-column") and
        iscompileerror($base+2; $case; "notmatched-update-action") and
        iscompileerror($base+3; $case; "matched-insert-wrong-type") and
        iscompileerror($base+4; $case; "missing-clauses") and
        iscompileerror($base+5; $case; "missing-then") and
        iscompileerror($base+6; $case; "and-then-delete") and
        iscompileerror($base+7; $case; "ambiguous-where-prop") and
        iscompileerror($base+8; $case; "invalid-select-prop") and
        iscompileerror($base+9; $case; "invalid-match-where-prop") and
        iscompileerror($base+10; $case; "variable-lhs") and
        iscompileerror($base+11; $case; "indexed-lhs") and
        iscompileerror($base+12; $case; "nested-event-assign");
    def invalidtable($base; $case):
        iscompileerror($base; $case; "notmatched-filter-windowevent") and
        iscompileerror($base+1; $case; "insert-unknown-column") and
        iscompileerror($base+2; $case; "notmatched-update-action") and
        iscompileerror($base+3; $case; "missing-clauses") and
        iscompileerror($base+4; $case; "missing-then") and
        iscompileerror($base+5; $case; "and-then-delete") and
        iscompileerror($base+6; $case; "ambiguous-where-prop") and
        iscompileerror($base+7; $case; "invalid-select-prop") and
        iscompileerror($base+8; $case; "invalid-match-where-prop") and
        iscompileerror($base+9; $case; "variable-lhs") and
        iscompileerror($base+10; $case; "indexed-lhs") and
        iscompileerror($base+11; $case; "nested-event-assign");
    def insertonly($base; $case):
        isdeployed($base; $case; "Window"; 1) and
        isdeployed($base+1; $case; "on"; 1) and
        islistener($base+2; $case; 1) and
        (.records[$base+2].new[0].fields | .p0 == "E1" and .p1 == 1) and
        issnapshot($base+3; $case; [{"p0": "E1", "p1": 1}]) and
        islistener($base+4; $case; 2) and
        (.records[$base+4].new[0].fields | .p0 == "E2" and .p1 == 2) and
        issnapshot($base+5; $case; [{"p0": "E1", "p1": 1}, {"p0": "E2", "p1": 2}]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-invalid-insertonly" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 49) and
    ([.records[].operation] | all(. == "deployed" or . == "listener" or . == "snapshot" or . == "compile-error")) and
    ([.records[] | select(.operation != "compile-error") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    invalidnw(0; "invalid-nw") and
    invalidtable(13; "invalid-table") and
    insertonly(25; "insertonly-nw-equivalent") and
    insertonly(31; "insertonly-nw") and
    insertonly(37; "insertonly-nw-colnames") and
    insertonly(43; "insertonly-nw-soda")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge-invalid-insertonly trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
