#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-subq-filtered-correl.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-subq-filtered-correl" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqFilteredCorrel.java" and
    (.javaRuntimes | type == "array" and length == 7) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 7) and
    ([.javaNames[] | . == "InfraNWTableSubqFilteredCorrelAssertion"] | all) and
    (.javaStaticIds | type == "array" and length == 7) and
    ([.javaStaticIds[] | . == "java-cef495b8aa35bb19a5ac"] | all) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 7) and
    ([.cases[] | select(.observation == "listener" and .iteratorSnapshots == 0)] | length == 7) and
    ([.cases[].case] == ["nw-no-share", "nw-no-share-index", "nw-share", "nw-share-disable", "nw-share-disable-index", "table-no-share", "table-no-share-index"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3, 4, 5, 6]) and
    ([.steps[] | select(.op == "case")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 28) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 28) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 7) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 7) and
    ([.steps[] | select(.op == "deploy" and .statement == "index")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 7) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 7) and
    (.steps | type == "array" and length == 94)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-subq-filtered-correl replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-subq-filtered-correl.XXXXXX")
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
    "$script_root/InfraNWTableSubqFilteredCorrelScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableSubqFilteredCorrelScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def consumecase($off; $name):
        ([.records[$off:$off+4][].case] | all(. == $name)) and
        ([.records[$off:$off+4][].operation] | all(. == "listener")) and
        ([.records[$off:$off+4][].statement] | all(. == "consume")) and
        ([.records[$off:$off+4][].sequence] == [1, 2, 3, 4]) and
        ([.records[$off:$off+4][].time] | all(. == "1970-01-01T00:00:00Z")) and
        ([.records[$off:$off+4][] | has("new")] | all) and
        ([.records[$off:$off+4][] | has("old")] | all(. == false)) and
        ([.records[$off:$off+4][] | .new | length] | all(. == 1)) and
        ([.records[$off:$off+4][] | .new[0].kind] | all(. == "row")) and
        ([.records[$off:$off+4][] | .new[0].fields | keys] | all(. == ["val"])) and
        ([.records[$off:$off+4][] | .new[0].fields.val] == [{"state": "null"}, -2, -3, {"state": "null"}]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-subq-filtered-correl" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 28) and
    ([.records[].operation] | all(. == "listener")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[].statement] | all(. == "consume")) and
    # nw-no-share: keepall named window without index share; 4 records, one per
    # S0 send, with val null, -2, -3, null as the intPrimitive<0 filter admits
    # only E2 and E3.
    consumecase(0; "nw-no-share") and
    # nw-no-share-index: explicit create index on the correlated column; same listener trace.
    consumecase(4; "nw-no-share-index") and
    # nw-share: enable_window_subquery_indexshare create hint; same listener trace.
    consumecase(8; "nw-share") and
    # nw-share-disable: disable_window_subquery_indexshare consumer hint; same listener trace.
    consumecase(12; "nw-share-disable") and
    # nw-share-disable-index: consumer hint plus explicit index; same listener trace.
    consumecase(16; "nw-share-disable-index") and
    # table: composite-key table correlates identically through its primary-key index.
    consumecase(20; "table-no-share") and
    # table-no-share-index: explicit table index; same listener trace.
    consumecase(24; "table-no-share-index")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-subq-filtered-correl trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
