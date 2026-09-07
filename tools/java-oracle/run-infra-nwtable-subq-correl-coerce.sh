#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-subq-correl-coerce.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-subq-correl-coerce" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableSubqCorrelCoerce.java" and
    (.javaRuntimes | type == "array" and length == 8) and
    (.javaRuntimes == [
      "java-runtime-0f3833a87dcd9b209935",
      "java-runtime-f1f621deec2314404f2a",
      "java-runtime-4f850a69b870e08ebfd5",
      "java-runtime-1616d20fc73fe4be679c",
      "java-runtime-0422895f8568f52e0956",
      "java-runtime-9161da818506b8194079",
      "java-runtime-3a94283dcac4e9ae4dea",
      "java-runtime-ce6e53f117adccdf9f59"]) and
    (.javaNames | type == "array" and length == 8) and
    ([.javaNames[] | startswith("InfraNWTableSubqCorrelCoerceSimple{")] | all) and
    (.javaStaticIds | type == "array" and length == 8) and
    ([.javaStaticIds[] | . == "java-5a8fbdfa647a5bef3660"] | all) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 8) and
    ([.cases[] | select(.observation == "listener" and .iteratorSnapshots == 0)] | length == 8) and
    ([.cases[].case] == ["nw-no-share", "nw-no-share-index", "nw-share", "nw-share-index", "nw-share-disable", "nw-share-disable-index", "table-no-share", "table-no-share-index"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3, 4, 5, 6, 7]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].epl | contains("disable_window_subquery_indexshare")] == [false, false, false, false, true, true, false, false]) and
    ([.steps[] | select(.op == "case")] | length == 8) and
    # four executions carry createExplicitIndex=true (ordinals 1, 3, 5, 7), so
    # the faithful step mix is 4x21 + 4x23 = 176 steps.
    ([.steps[] | select(.op != "case")] | group_by(.case) | map({(.[0].case): length}) | add) == {
      "nw-no-share": 20, "nw-no-share-index": 22, "nw-share": 20, "nw-share-index": 22,
      "nw-share-disable": 20, "nw-share-disable-index": 22, "table-no-share": 20,
      "table-no-share-index": 22} and
    ([.steps[] | select(.op == "deploy")] | group_by(.case) | map({(.[0].case): ([.[].statement] | unique)}) | add) == {
      "nw-no-share": ["c1", "c2", "consume", "create", "insert"],
      "nw-no-share-index": ["c1", "c2", "consume", "create", "index", "insert"],
      "nw-share": ["c1", "c2", "consume", "create", "insert"],
      "nw-share-index": ["c1", "c2", "consume", "create", "index", "insert"],
      "nw-share-disable": ["c1", "c2", "consume", "create", "insert"],
      "nw-share-disable-index": ["c1", "c2", "consume", "create", "index", "insert"],
      "table-no-share": ["c1", "c2", "consume", "create", "insert"],
      "table-no-share-index": ["c1", "c2", "consume", "create", "index", "insert"]} and
    ([.steps[] | select(.op == "send")] | group_by(.case) | map({(.[0].case): length}) | add) == {
      "nw-no-share": 11,
      "nw-no-share-index": 11,
      "nw-share": 11,
      "nw-share-index": 11,
      "nw-share-disable": 11,
      "nw-share-disable-index": 11,
      "table-no-share": 11,
      "table-no-share-index": 11} and
    ([.steps[] | select(.op == "undeploy")] | group_by(.case) | map({(.[0].case): length}) | add) == {
      "nw-no-share": 2,
      "nw-no-share-index": 3,
      "nw-share": 2,
      "nw-share-index": 3,
      "nw-share-disable": 2,
      "nw-share-disable-index": 3,
      "table-no-share": 2,
      "table-no-share-index": 3} and
    ([.steps[] | select(.op == "send" and .eventType == "WindowSchema")] | length == 32) and
    ([.steps[] | select(.op == "send" and .eventType == "EventSchema")] | length == 56) and
    ([.steps[] | select(.op == "deploy" and .statement == "c1")] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "c2")] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "create" and (.epl | contains("create window")))] | length == 6) and
    ([.steps[] | select(.op == "deploy" and .statement == "create" and (.epl | contains("create table")))] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "create" and (.epl | contains("enable_window_subquery_indexshare")))] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert" and (.epl | contains("insert into MyInfra select * from WindowSchema")))] | length == 8) and
    ([.steps[] | select(.op == "deploy" and .statement == "index" and (.epl | contains("create index MyIndex")))] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 16) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume" and (.epl | contains("disable_window_subquery_indexshare")))] | length == 4) and
    ([.steps[] | select(.op == "undeploy" and .statement == "s0")] | length == 16) and
    ([.steps[] | select(.op == "undeploy" and .statement == "index")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 8) and
    ([.steps[] | select(.op == "snapshot")] | length == 0) and
    (.steps | type == "array" and length == 176)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-subq-correl-coerce replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-subq-correl-coerce.XXXXXX")
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
    "$script_root/InfraNWTableSubqCorrelCoerceScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableSubqCorrelCoerceScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def s0case($off; $name):
        ([.records[$off:$off+7][].case] | all(. == $name)) and
        ([.records[$off:$off+7][].operation] | all(. == "listener")) and
        ([.records[$off:$off+7][].statement] | all(. == "s0")) and
        ([.records[$off:$off+7][].sequence] == [1, 2, 3, 4, 5, 6, 7]) and
        ([.records[$off:$off+7][].time] | all(. == "1970-01-01T00:00:00Z")) and
        ([.records[$off:$off+7][] | has("new")] | all) and
        ([.records[$off:$off+7][] | has("old")] | all(. == false)) and
        ([.records[$off:$off+7][] | .new | length] | all(. == 1)) and
        ([.records[$off:$off+7][] | .new[0].kind] | all(. == "row")) and
        ([.records[$off:$off+7][] | .new[0].fields | keys] | all(. == ["e0", "val"])) and
        ([.records[$off:$off+7][] | .new[0].fields.e0] == ["E1", "E2", "E3", "E4", "E5", "E6", "E6"]) and
        # val resolves to W1, null, W2, W3, W1, W4 and, after the mid-case
        # undeploy and redeploy of s0, W4 again (the subquery index
        # repopulates from the existing infra contents).
        ([.records[$off:$off+7][] | .new[0].fields.val] == ["W1", {"state": "null"}, "W2", "W3", "W1", "W4", "W4"]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-subq-correl-coerce" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 56) and
    ([.records[].operation] | all(. == "listener")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[].statement] | all(. == "s0")) and
    # nw-no-share: keepall named window without index share.
    s0case(0; "nw-no-share") and
    # nw-no-share-index: explicit two-key create index; same listener trace.
    s0case(7; "nw-no-share-index") and
    # nw-share: enable_window_subquery_indexshare create hint; same listener trace.
    s0case(14; "nw-share") and
    # nw-share-index: create hint plus explicit index; same listener trace.
    s0case(21; "nw-share-index") and
    # nw-share-disable: disable_window_subquery_indexshare consumer hint; same listener trace.
    s0case(28; "nw-share-disable") and
    # nw-share-disable-index: consumer hint plus explicit index; same listener trace.
    s0case(35; "nw-share-disable-index") and
    # table: primary-key table correlates identically through its key index,
    # with the int-to-long coercion on col1 = es.e1.
    s0case(42; "table-no-share") and
    # table-no-share-index: explicit table index; same listener trace.
    s0case(49; "table-no-share-index")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-subq-correl-coerce trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
