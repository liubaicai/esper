#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge-flow-itv.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "could not resolve Esper checkout commit: $esper_root" >&2
    exit 1
}
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required, found version $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-flow-itv" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 8) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraFlow{namedWindow=true}", "InfraFlow{namedWindow=false}", "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}", "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}", "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=MAP}", "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=MAP}", "InfraInnerTypeAndVariable{namedWindow=true, eventRepresentationEnum=DEFAULT}", "InfraInnerTypeAndVariable{namedWindow=false, eventRepresentationEnum=DEFAULT}"]) and
    (.javaStaticIds == ["java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763", "java-097f9b8edb46959e0763"]) and
    (.javaFlags == ["OBSERVEROPS"]) and
    (.cases | type == "array" and length == 8) and
    ([.cases[].case] == ["flow-nw", "flow-table", "itv-nw-objectarray", "itv-table-objectarray", "itv-nw-map", "itv-table-map", "itv-nw-default", "itv-table-default"]) and
    ([.cases[].ordinal] == [32, 33, 34, 35, 36, 37, 38, 39]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 282) and
    ([.steps[] | select(.op == "case")] | length == 8) and
    ([.steps[] | select(.op == "deploy")] | length == 44) and
    ([.steps[] | select(.op == "deployed")] | length == 54) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 38) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEventSchema")] | length == 30) and
    ([.steps[] | select(.op == "snapshot")] | length == 68) and
    ([.steps[] | select(.op == "set-variable")] | length == 18) and
    ([.steps[] | select(.op == "undeploy")] | length == 10) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 8) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "set-variable" and .op != "undeploy" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge-flow-itv replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge-flow-itv.XXXXXX")
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
# regression-lib classes provide the SupportBean_A bean the
# InfraFlow delete trigger fires on.
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableOnMergeFlowITVScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergeFlowITVScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def isdeployed($index; $case; $statement; $seq):
        .records[$index].operation == "deployed" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def islistener($index; $case; $statement; $seq):
        .records[$index].operation == "listener" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == $seq;
    def issnapshot($index; $case; $statement; $rows):
        .records[$index].operation == "snapshot" and
        .records[$index].case == $case and
        .records[$index].statement == $statement and
        .records[$index].sequence == 0 and
        ([.records[$index].new[]?.fields] == $rows);
    def issetvar($index; $case; $seq; $value):
        .records[$index].operation == "set-variable" and
        .records[$index].case == $case and
        .records[$index].statement == "createvar" and
        .records[$index].sequence == $seq and
        .records[$index].name == "myvar" and
        .records[$index].value == $value;
    def flownwpass($base; $case; $ws; $ms):
        islistener($base; $case; "Window"; $ws) and
        (.records[$base].new[0].fields | .theString == "E1" and .intPrimitive == 10 and .intBoxed == 200) and
        issnapshot($base+1; $case; "Window"; [{"intBoxed": 200, "intPrimitive": 10, "theString": "E1"}]) and
        islistener($base+2; $case; "Window"; $ws+1) and
        (.records[$base+2].new[0].fields.intBoxed == 401 and .records[$base+2].old[0].fields.intBoxed == 200) and
        islistener($base+3; $case; "Merge"; $ms) and
        (.records[$base+3].new[0].fields.intBoxed == 401 and .records[$base+3].old[0].fields.intBoxed == 200) and
        issnapshot($base+4; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+5; $case; "Window"; $ws+2) and
        (.records[$base+5].new[0].fields.theString == "E2") and
        islistener($base+6; $case; "Merge"; $ms+1) and
        (.records[$base+6].new[0].fields.theString == "E2" and .records[$base+6].new[0].fields.intBoxed == 300) and
        issnapshot($base+7; $case; "Window"; [{"intBoxed": 300, "intPrimitive": 13, "theString": "E2"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+8; $case; "Window"; $ws+3) and
        islistener($base+9; $case; "Merge"; $ms+2) and
        (.records[$base+9].new[0].fields.intBoxed == 601) and
        issnapshot($base+10; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 601, "intPrimitive": 14, "theString": "E2"}]) and
        islistener($base+11; $case; "Window"; $ws+4) and
        islistener($base+12; $case; "Merge"; $ms+3) and
        (.records[$base+12].new[0].fields.intBoxed == 903) and
        issnapshot($base+13; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+14; $case; "Window"; $ws+5) and
        islistener($base+15; $case; "Merge"; $ms+4) and
        (.records[$base+15].new[0].fields.theString == "E3") and
        issnapshot($base+16; $case; "Window"; [{"intBoxed": 400, "intPrimitive": 40, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+17; $case; "Window"; $ws+6) and
        islistener($base+18; $case; "Merge"; $ms+5) and
        (.records[$base+18].new[0].fields | .intPrimitive == 0 and .intBoxed == 0) and
        issnapshot($base+19; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+20; $case; "Window"; $ws+7) and
        (.records[$base+20].old[0].fields.theString == "E2") and
        islistener($base+21; $case; "Merge"; $ms+6) and
        (.records[$base+21].old[0].fields.theString == "E2" and .records[$base+21].old[0].fields.intBoxed == 903) and
        issnapshot($base+22; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+23; $case; "Window"; $ws+8) and
        islistener($base+24; $case; "Merge"; $ms+7) and
        (.records[$base+24].old[0].fields.theString == "E1") and
        issnapshot($base+25; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}]);
    def flowtablepass($base; $case; $ms):
        issnapshot($base; $case; "Window"; [{"intBoxed": 200, "intPrimitive": 10, "theString": "E1"}]) and
        islistener($base+1; $case; "Merge"; $ms) and
        (.records[$base+1].new[0].fields.intBoxed == 401 and .records[$base+1].old[0].fields.intBoxed == 200) and
        issnapshot($base+2; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+3; $case; "Merge"; $ms+1) and
        (.records[$base+3].new[0].fields.theString == "E2" and .records[$base+3].new[0].fields.intBoxed == 300) and
        issnapshot($base+4; $case; "Window"; [{"intBoxed": 300, "intPrimitive": 13, "theString": "E2"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+5; $case; "Merge"; $ms+2) and
        (.records[$base+5].new[0].fields.intBoxed == 601) and
        issnapshot($base+6; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 601, "intPrimitive": 14, "theString": "E2"}]) and
        islistener($base+7; $case; "Merge"; $ms+3) and
        (.records[$base+7].new[0].fields.intBoxed == 903) and
        issnapshot($base+8; $case; "Window"; [{"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+9; $case; "Merge"; $ms+4) and
        (.records[$base+9].new[0].fields.theString == "E3") and
        issnapshot($base+10; $case; "Window"; [{"intBoxed": 400, "intPrimitive": 40, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+11; $case; "Merge"; $ms+5) and
        (.records[$base+11].new[0].fields | .intPrimitive == 0 and .intBoxed == 0) and
        issnapshot($base+12; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}, {"intBoxed": 903, "intPrimitive": 15, "theString": "E2"}]) and
        islistener($base+13; $case; "Merge"; $ms+6) and
        (.records[$base+13].old[0].fields.theString == "E2" and .records[$base+13].old[0].fields.intBoxed == 903) and
        issnapshot($base+14; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}, {"intBoxed": 401, "intPrimitive": 11, "theString": "E1"}]) and
        islistener($base+15; $case; "Merge"; $ms+7) and
        (.records[$base+15].old[0].fields.theString == "E1") and
        issnapshot($base+16; $case; "Window"; [{"intBoxed": 0, "intPrimitive": 0, "theString": "E3"}]);
    def flownw($base; $case):
        isdeployed($base; $case; "Window"; 1) and
        isdeployed($base+1; $case; "Insert"; 1) and
        isdeployed($base+2; $case; "Delete"; 1) and
        isdeployed($base+3; $case; "Merge"; 1) and
        flownwpass($base+4; $case; 1; 1) and
        islistener($base+30; $case; "Window"; 10) and
        (.records[$base+30].old[0].fields.theString == "E3") and
        isdeployed($base+31; $case; "Merge"; 2) and
        flownwpass($base+32; $case; 11; 9) and
        islistener($base+58; $case; "Window"; 20) and
        (.records[$base+58].old[0].fields.theString == "E3") and
        isdeployed($base+59; $case; "Merge"; 3) and
        islistener($base+60; $case; "Window"; 21) and
        (.records[$base+60].new[0].fields.theString == "E99") and
        islistener($base+61; $case; "Merge"; 17) and
        (.records[$base+61].new[0].fields.theString == "E99") and
        issnapshot($base+62; $case; "Window"; [{"intBoxed": 3, "intPrimitive": 2, "theString": "E99"}]) and
        isdeployed($base+63; $case; "schema-typeone"; 1) and
        isdeployed($base+64; $case; "infra-two"; 1) and
        isdeployed($base+65; $case; "merge-two"; 1);
    def flowtable($base; $case):
        isdeployed($base; $case; "Window"; 1) and
        isdeployed($base+1; $case; "Insert"; 1) and
        isdeployed($base+2; $case; "Delete"; 1) and
        isdeployed($base+3; $case; "Merge"; 1) and
        flowtablepass($base+4; $case; 1) and
        isdeployed($base+21; $case; "Merge"; 2) and
        flowtablepass($base+22; $case; 9) and
        isdeployed($base+39; $case; "Merge"; 3) and
        islistener($base+40; $case; "Merge"; 17) and
        (.records[$base+40].new[0].fields.theString == "E99") and
        issnapshot($base+41; $case; "Window"; [{"intBoxed": 3, "intPrimitive": 2, "theString": "E99"}]) and
        isdeployed($base+42; $case; "schema-typeone"; 1) and
        isdeployed($base+43; $case; "infra-two"; 1) and
        isdeployed($base+44; $case; "merge-two"; 1);
    def itvcase($base; $case):
        isdeployed($base; $case; "schema-inner"; 1) and
        isdeployed($base+1; $case; "schema-event"; 1) and
        isdeployed($base+2; $case; "infra"; 1) and
        isdeployed($base+3; $case; "createvar"; 1) and
        isdeployed($base+4; $case; "Merge"; 1) and
        issnapshot($base+5; $case; "infra"; [{"c1": "B", "c2.in1": "Y1", "c2.in2": 10}]) and
        issnapshot($base+6; $case; "infra"; []) and
        issetvar($base+7; $case; 1; true) and
        issnapshot($base+8; $case; "infra"; [{"c1": "X2", "c2.in1": "Y2", "c2.in2": 11}]) and
        issetvar($base+9; $case; 2; false) and
        issnapshot($base+10; $case; "infra"; [{"c1": "A", "c2.in1": {"state": "null"}, "c2.in2": {"state": "null"}}, {"c1": "X2", "c2.in1": "Y2", "c2.in2": 11}]) and
        isdeployed($base+11; $case; "Merge"; 2) and
        issetvar($base+12; $case; 3; true) and
        issnapshot($base+13; $case; "infra"; [{"c1": "A", "c2.in1": {"state": "null"}, "c2.in2": {"state": "null"}}, {"c1": "X2", "c2.in1": "Y2", "c2.in2": 11}, {"c1": "X4", "c2.in1": "Y4", "c2.in2": 11}]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-flow-itv" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 195) and
    ([.records[].operation] | all(. == "deployed" or . == "listener" or . == "snapshot" or . == "set-variable")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    flownw(0; "flow-nw") and
    flowtable(66; "flow-table") and
    itvcase(111; "itv-nw-objectarray") and
    itvcase(125; "itv-table-objectarray") and
    itvcase(139; "itv-nw-map") and
    itvcase(153; "itv-table-map") and
    itvcase(167; "itv-nw-default") and
    itvcase(181; "itv-table-default")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge-flow-itv trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
