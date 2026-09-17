#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-delete.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-on-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnDelete.java" and
    (.javaRuntimes | type == "array" and length == 6) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 6) and
    (.javaNames == ["InfraDeleteCondition{namedWindow=true}", "InfraDeleteCondition{namedWindow=false}", "InfraDeletePattern{namedWindow=true}", "InfraDeletePattern{namedWindow=false}", "InfraDeleteAll{namedWindow=true}", "InfraDeleteAll{namedWindow=false}"]) and
    (.javaStaticIds == ["java-eb28fd3f17513a399ac9", "java-eb28fd3f17513a399ac9", "java-132a1f9e7f2c434a19e9", "java-132a1f9e7f2c434a19e9", "java-e3aeca986a0cfee60dc7", "java-e3aeca986a0cfee60dc7"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 6) and
    ([.cases[].case] == ["cond-nw", "cond-table", "pattern-nw", "pattern-table", "deleteall-nw", "deleteall-table"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3, 4, 5]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 144) and
    ([.steps[] | select(.op == "case")] | length == 6) and
    ([.steps[] | select(.op == "deploy")] | length == 22) and
    ([.steps[] | select(.op == "deployed")] | length == 22) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 18) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 10) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 4) and
    ([.steps[] | select(.op == "snapshot")] | length == 30) and
    ([.steps[] | select(.op == "faf")] | length == 26) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 6) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "faf" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-delete replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-delete.XXXXXX")
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
    "$script_root/InfraNWTableOnDeleteScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnDeleteScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def head_deployed($off; $names):
        ([.records[$off:$off+($names | length)][].operation] | all(. == "deployed")) and
        ([.records[$off:$off+($names | length)][].statement] == $names) and
        ([.records[$off:$off+($names | length)][].sequence] == [1, 1, 1, 1][0:($names | length)]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 106) and
    ([.records[].operation] | all(. == "listener" or . == "deployed" or . == "snapshot" or . == "faf")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # cond-nw: 4 deployed + 6 CreateInfra listener + 4 faf + 4 snapshot = 18.
    ([.records[0:18][].case] | all(. == "cond-nw")) and
    head_deployed(0; ["CreateInfra", "DeleteCondA", "DeleteCondB", "Insert"]) and
    ([.records[4:18][].operation] == ["listener", "listener", "listener", "faf", "snapshot", "listener", "snapshot", "faf", "listener", "snapshot", "faf", "listener", "snapshot", "faf"]) and
    ([.records[4:18][] | select(.operation == "listener") | .statement] | all(. == "CreateInfra")) and
    ([.records[4:18][] | select(.operation == "listener") | .sequence] == [1, 2, 3, 4, 5, 6]) and
    ([.records[4:18][] | select(.operation == "listener" and has("new")) | .new[0].fields] == [{"a": "E1", "b": 1}, {"a": "E2", "b": 2}, {"a": "E3", "b": 3}, {"a": "E7", "b": 7}]) and
    (.records[9].old[0].fields == {"a": "E2", "b": 2}) and
    (.records[9] | has("new") | not) and
    (.records[15].old | length == 2) and
    (.records[15].old[0].fields == {"a": "E1", "b": 1}) and
    (.records[15].old[1].fields == {"a": "E3", "b": 3}) and
    ([.records[4:18][] | select(.operation == "faf") | .statement] | all(. == "count")) and
    ([.records[4:18][] | select(.operation == "faf") | .new[0].fields.c0] == [3, 2, 3, 1]) and
    ([.records[4:18][] | select(.operation == "snapshot") | .statement] | all(. == "CreateInfra")) and
    ([.records[4:18][] | select(.operation == "snapshot") | .new | length] == [3, 2, 3, 1]) and
    # cond-table: 4 deployed + 0 listener + 4 faf + 4 snapshot = 12.
    ([.records[18:30][].case] | all(. == "cond-table")) and
    head_deployed(18; ["CreateInfra", "DeleteCondA", "DeleteCondB", "Insert"]) and
    ([.records[22:30][].operation] == ["faf", "snapshot", "snapshot", "faf", "snapshot", "faf", "snapshot", "faf"]) and
    ([.records[22:30][] | select(.operation == "faf") | .new[0].fields.c0] == [3, 2, 3, 1]) and
    ([.records[22:30][] | select(.operation == "snapshot") | .new | length] == [3, 2, 3, 1]) and
    # pattern-nw: 3 deployed + 6 listener + 6 snapshot + 4 faf = 19.
    ([.records[30:49][].case] | all(. == "pattern-nw")) and
    head_deployed(30; ["CreateInfra", "OnDelete", "Insert"]) and
    ([.records[33:49][].operation] == ["listener", "snapshot", "snapshot", "faf", "listener", "listener", "snapshot", "snapshot", "faf", "listener", "snapshot", "faf", "listener", "listener", "snapshot", "faf"]) and
    ([.records[33:49][] | select(.operation == "listener") | .statement] == ["CreateInfra", "CreateInfra", "OnDelete", "CreateInfra", "CreateInfra", "OnDelete"]) and
    ([.records[33:49][] | select(.operation == "listener" and .statement == "CreateInfra" and has("old")) | .old[0].fields] == [{"a": "E1", "b": 1}, {"a": "E2", "b": 2}]) and
    ([.records[33:49][] | select(.operation == "snapshot") | .statement] == ["OnDelete", "CreateInfra", "OnDelete", "CreateInfra", "CreateInfra", "CreateInfra"]) and
    ([.records[33:49][] | select(.operation == "snapshot" and .statement == "CreateInfra") | .new | length] == [1, 0, 1, 0]) and
    ([.records[33:49][] | select(.operation == "snapshot" and .statement == "OnDelete") | has("new") | not] | all) and
    ([.records[33:49][] | select(.operation == "faf") | .new[0].fields.c0] == [1, 0, 1, 0]) and
    # pattern-table: 3 deployed + 2 listener + 4 snapshot + 4 faf = 13.
    ([.records[49:62][].case] | all(. == "pattern-table")) and
    head_deployed(49; ["CreateInfra", "OnDelete", "Insert"]) and
    ([.records[52:62][].operation] == ["snapshot", "faf", "listener", "snapshot", "faf", "snapshot", "faf", "listener", "snapshot", "faf"]) and
    ([.records[52:62][] | select(.operation == "listener") | .statement] | all(. == "OnDelete")) and
    ([.records[52:62][] | select(.operation == "listener") | .new[0].fields] == [{"a": "E1", "b": 1}, {"a": "E2", "b": 2}]) and
    ([.records[52:62][] | select(.operation == "snapshot") | .new | length] == [1, 0, 1, 0]) and
    ([.records[52:62][] | select(.operation == "faf") | .new[0].fields.c0] == [1, 0, 1, 0]) and
    # deleteall-nw: 4 deployed + 12 listener + 7 snapshot + 5 faf = 28.
    ([.records[62:90][].case] | all(. == "deleteall-nw")) and
    head_deployed(62; ["CreateInfra", "OnDelete", "Insert", "Select"]) and
    ([.records[66:90][].operation] == ["faf", "listener", "listener", "snapshot", "snapshot", "faf", "listener", "listener", "listener", "snapshot", "snapshot", "faf", "listener", "listener", "listener", "listener", "snapshot", "faf", "listener", "listener", "listener", "snapshot", "snapshot", "faf"]) and
    ([.records[66:90][] | select(.operation == "listener") | .statement] == ["CreateInfra", "Select", "CreateInfra", "OnDelete", "Select", "CreateInfra", "Select", "CreateInfra", "Select", "CreateInfra", "OnDelete", "Select"]) and
    ([.records[66:90][] | select(.operation == "listener" and .statement == "OnDelete") | .new | length] == [1, 2]) and
    (([.records[66:90][] | select(.operation == "listener" and .statement == "OnDelete") | .new] | .[1] | map(.fields)) == [{"a": "E2", "b": 2}, {"a": "E3", "b": 3}]) and
    ([.records[66:90][] | select(.operation == "listener" and .statement == "CreateInfra" and has("old")) | .old[0].fields] == [{"a": "E1", "b": 1}, {"a": "E2", "b": 2}]) and
    ([.records[66:90][] | select(.operation == "listener" and .statement == "Select" and has("old")) | .old[0].fields] == [{"a": "E1", "b": 1}, {"a": "E2", "b": 2}]) and
    ([.records[66:90][] | select(.operation == "snapshot") | .statement] == ["CreateInfra", "OnDelete", "OnDelete", "CreateInfra", "CreateInfra", "OnDelete", "CreateInfra"]) and
    ([.records[66:90][] | select(.operation == "snapshot" and .statement == "CreateInfra") | .new | length] == [1, 0, 2, 0]) and
    ([.records[66:90][] | select(.operation == "snapshot" and .statement == "OnDelete") | has("new") | not] | all) and
    ([.records[66:90][] | select(.operation == "faf") | .new[0].fields.c0] == [0, 1, 0, 2, 0]) and
    # deleteall-table: 4 deployed + 2 listener + 5 snapshot + 5 faf = 16.
    ([.records[90:106][].case] | all(. == "deleteall-table")) and
    head_deployed(90; ["CreateInfra", "OnDelete", "Insert", "Select"]) and
    ([.records[94:106][].operation] == ["faf", "snapshot", "snapshot", "faf", "listener", "snapshot", "faf", "snapshot", "faf", "listener", "snapshot", "faf"]) and
    ([.records[94:106][] | select(.operation == "listener") | .statement] | all(. == "OnDelete")) and
    ([.records[94:106][] | select(.operation == "listener") | .new | length] == [1, 2]) and
    (([.records[94:106][] | select(.operation == "listener") | .new] | .[1] | map(.fields)) == [{"a": "E2", "b": 2}, {"a": "E3", "b": 3}]) and
    ([.records[94:106][] | select(.operation == "snapshot" and .statement == "CreateInfra") | .new | length] == [1, 0, 2, 0]) and
    ([.records[94:106][] | select(.operation == "faf") | .new[0].fields.c0] == [0, 1, 0, 2, 0])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-delete trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
