#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-on-merge" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraOnMergeSimpleInsert{namedWindow=true}", "InfraOnMergeSimpleInsert{namedWindow=false}", "InfraOnMergeMatchNoMatch{namedWindow=true}", "InfraOnMergeMatchNoMatch{namedWindow=false}"]) and
    (.javaStaticIds == ["java-0fdb4e6490ae6d9e9103", "java-0fdb4e6490ae6d9e9103", "java-5f5d122bcbf66d59da6c", "java-5f5d122bcbf66d59da6c"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["simple-nw", "simple-table", "matchnomatch-nw", "matchnomatch-table"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 52) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 8) and
    ([.steps[] | select(.op == "deployed")] | length == 8) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 16) and
    ([.steps[] | select(.op == "snapshot")] | length == 12) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge.XXXXXX")
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
    "$script_root/InfraNWTableOnMergeScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergeScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def deployed_at($idx; $name):
        .records[$idx].operation == "deployed" and .records[$idx].statement == $name
            and .records[$idx].sequence == 1;
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 39) and
    ([.records[].operation] | all(. == "listener" or . == "deployed" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # simple-nw: 2 deployed + 2 listener + 2 snapshot = 6.
    ([.records[0:6][].case] | all(. == "simple-nw")) and
    deployed_at(0; "create") and deployed_at(1; "merge") and
    ([.records[2:6][].operation] == ["listener", "snapshot", "listener", "snapshot"]) and
    ([.records[2:6][] | select(.operation == "listener") | .statement] | all(. == "merge")) and
    ([.records[2:6][] | select(.operation == "listener") | .sequence] == [1, 2]) and
    (.records[2].new[0].fields == {"p0": "E1", "p1": 1}) and
    (.records[2] | has("old") | not) and
    ([.records[3].new[].fields] == [{"p0": "E1", "p1": 1}]) and
    (.records[4].new[0].fields == {"p0": "E2", "p1": 2}) and
    ([.records[5].new[].fields] == [{"p0": "E1", "p1": 1}, {"p0": "E2", "p1": 2}]) and
    # simple-table: 2 deployed + 2 listener + 2 snapshot = 6.
    ([.records[6:12][].case] | all(. == "simple-table")) and
    deployed_at(6; "create") and deployed_at(7; "merge") and
    ([.records[8:12][].operation] == ["listener", "snapshot", "listener", "snapshot"]) and
    ([.records[8:12][] | select(.operation == "listener") | .statement] | all(. == "merge")) and
    (.records[8].new[0].fields == {"p0": "E1", "p1": 1}) and
    ([.records[9].new[].fields] == [{"p0": "E1", "p1": 1}]) and
    (.records[10].new[0].fields == {"p0": "E2", "p1": 2}) and
    ([.records[11].new[].fields] == [{"p0": "E1", "p1": 1}, {"p0": "E2", "p1": 2}]) and
    # matchnomatch-nw: 2 deployed + 10 listener + 4 snapshot = 16; each
    # merge mutation reaches the create listener before the merge listener.
    ([.records[12:28][].case] | all(. == "matchnomatch-nw")) and
    deployed_at(12; "create") and deployed_at(13; "merge") and
    ([.records[14:28][].operation] == ["listener", "listener", "snapshot", "listener", "listener", "snapshot", "listener", "listener", "snapshot", "listener", "listener", "listener", "listener", "snapshot"]) and
    ([.records[14:28][] | select(.operation == "listener") | .statement] == ["create", "merge", "create", "merge", "create", "merge", "create", "merge", "create", "merge"]) and
    ([.records[14:28][] | select(.operation == "listener" and .statement == "create") | .sequence] == [1, 2, 3, 4, 5]) and
    ([.records[14:28][] | select(.operation == "listener" and .statement == "merge") | .sequence] == [1, 2, 3, 4, 5]) and
    (.records[14].new[0].fields.theString == "E2" and .records[14].new[0].fields.intPrimitive == 2) and
    (.records[14] | has("old") | not) and
    (.records[15].new[0].fields.theString == "E2" and .records[15].new[0].fields.intPrimitive == 2) and
    (.records[15] | has("old") | not) and
    ([.records[16].new[].fields] == [{"intPrimitive": 2, "theString": "E2"}]) and
    (.records[17].new[0].fields.theString == "E2" and .records[17].new[0].fields.intPrimitive == 12) and
    (.records[17].old[0].fields.theString == "E2" and .records[17].old[0].fields.intPrimitive == 2) and
    (.records[18].new[0].fields.theString == "E2" and .records[18].new[0].fields.intPrimitive == 12) and
    (.records[18].old[0].fields.theString == "E2" and .records[18].old[0].fields.intPrimitive == 2) and
    ([.records[19].new[].fields] == [{"intPrimitive": 12, "theString": "E2"}]) and
    (.records[20] | has("new") | not) and
    (.records[20].old[0].fields.theString == "E2" and .records[20].old[0].fields.intPrimitive == 12) and
    (.records[21] | has("new") | not) and
    (.records[21].old[0].fields.theString == "E2" and .records[21].old[0].fields.intPrimitive == 12) and
    (.records[22] | has("new") | not) and
    (.records[23].new[0].fields.theString == "E3" and .records[23].new[0].fields.intPrimitive == 3) and
    (.records[24].new[0].fields.theString == "E3" and .records[24].new[0].fields.intPrimitive == 3) and
    (.records[25].new[0].fields.theString == "E3" and .records[25].new[0].fields.intPrimitive == 7) and
    (.records[25].old[0].fields.theString == "E3" and .records[25].old[0].fields.intPrimitive == 3) and
    (.records[26].new[0].fields.theString == "E3" and .records[26].new[0].fields.intPrimitive == 7) and
    (.records[26].old[0].fields.theString == "E3" and .records[26].old[0].fields.intPrimitive == 3) and
    ([.records[27].new[].fields] == [{"intPrimitive": 7, "theString": "E3"}]) and
    # matchnomatch-table: 2 deployed + 5 listener + 4 snapshot = 11; the
    # create listener is attached but never fires for a table.
    ([.records[28:39][].case] | all(. == "matchnomatch-table")) and
    deployed_at(28; "create") and deployed_at(29; "merge") and
    ([.records[30:39][].operation] == ["listener", "snapshot", "listener", "snapshot", "listener", "snapshot", "listener", "listener", "snapshot"]) and
    ([.records[30:39][] | select(.operation == "listener") | .statement] | all(. == "merge")) and
    ([.records[30:39][] | select(.operation == "listener") | .sequence] == [1, 2, 3, 4, 5]) and
    (.records[30].new[0].fields == {"intPrimitive": 2, "theString": "E2"}) and
    (.records[30] | has("old") | not) and
    ([.records[31].new[].fields] == [{"intPrimitive": 2, "theString": "E2"}]) and
    (.records[32].new[0].fields == {"intPrimitive": 12, "theString": "E2"}) and
    (.records[32].old[0].fields == {"intPrimitive": 2, "theString": "E2"}) and
    ([.records[33].new[].fields] == [{"intPrimitive": 12, "theString": "E2"}]) and
    (.records[34] | has("new") | not) and
    (.records[34].old[0].fields == {"intPrimitive": 12, "theString": "E2"}) and
    (.records[35] | has("new") | not) and
    (.records[36].new[0].fields == {"intPrimitive": 3, "theString": "E3"}) and
    (.records[37].new[0].fields == {"intPrimitive": 7, "theString": "E3"}) and
    (.records[37].old[0].fields == {"intPrimitive": 3, "theString": "E3"}) and
    ([.records[38].new[].fields] == [{"intPrimitive": 7, "theString": "E3"}])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
