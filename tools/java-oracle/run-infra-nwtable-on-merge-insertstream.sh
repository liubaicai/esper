#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-merge-insertstream.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "failed to resolve Esper commit at $esper_root" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper commit mismatch: want $expected_commit, got $actual_commit" >&2
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
    echo "Java 17 is required; detected version: $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-insertstream" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnMerge.java" and
    (.javaRuntimes | type == "array" and length == 12) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=OBJECTARRAY}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=OBJECTARRAY}", "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=MAP}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=MAP}", "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=AVRO}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=AVRO}", "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSON}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSON}", "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=JSONCLASSPROVIDED}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=JSONCLASSPROVIDED}", "InfraInsertOtherStream{namedWindow=true, eventRepresentationEnum=DEFAULT}", "InfraInsertOtherStream{namedWindow=false, eventRepresentationEnum=DEFAULT}"]) and
    (.javaStaticIds | length == 12 and all(. == "java-8304a4459a4ea865bf1f")) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 12) and
    ([.cases[].case] == ["insertstream-nw-objectarray", "insertstream-table-objectarray", "insertstream-nw-map", "insertstream-table-map", "insertstream-nw-avro", "insertstream-table-avro", "insertstream-nw-json", "insertstream-table-json", "insertstream-nw-jsonclassprovided", "insertstream-table-jsonclassprovided", "insertstream-nw-default", "insertstream-table-default"]) and
    ([.cases[].ordinal] == [8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 132) and
    ([.steps[] | select(.op == "case")] | length == 12) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deployed")] | length == 72) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent")] | length == 24) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 12) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-merge-insertstream replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-merge-insertstream.XXXXXX")
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
# regression-lib classes provide the @JsonSchema MyLocalJsonProvided* classes
# referenced by the JSONCLASSPROVIDED variant's create-schema annotations.
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableOnMergeInsertStreamScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnMergeInsertStreamScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def deployed6($base; $case):
        ([.records[$base:$base+6][] | .operation] | all(. == "deployed")) and
        ([.records[$base:$base+6][] | .case] | all(. == $case)) and
        ([.records[$base:$base+6][] | .statement]
            == ["schema-myevent", "infra", "insert", "schema-input", "merge", "s0"]) and
        ([.records[$base:$base+6][] | .sequence] | all(. == 1));
    def nwcase($base; $case):
        deployed6($base; $case) and
        ([.records[$base+6:$base+9][] | .operation] | all(. == "listener")) and
        ([.records[$base+6:$base+9][] | .case] | all(. == $case)) and
        ([.records[$base+6:$base+9][] | .statement] | all(. == "s0")) and
        ([.records[$base+6:$base+9][] | .sequence] == [1, 2, 3]) and
        ([.records[$base+6:$base+9][] | .new[0].fields]
            == [{"event_name": "name1", "status": 0},
                {"event_name": "name1", "status": 10},
                {"event_name": "name1", "status": 11}]) and
        ([.records[$base+6:$base+9][] | has("old")] | all(. == false));
    def tblcase($base; $case):
        deployed6($base; $case) and
        .records[$base+6].operation == "listener" and
        .records[$base+6].case == $case and
        .records[$base+6].statement == "s0" and
        .records[$base+6].sequence == 1 and
        (.records[$base+6].new[0].fields == {"event_name": "name1", "status": 10}) and
        (.records[$base+6] | has("old") | not);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-merge-insertstream" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 96) and
    ([.records[].operation] | all(. == "listener" or . == "deployed")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    nwcase(0; "insertstream-nw-objectarray") and
    tblcase(9; "insertstream-table-objectarray") and
    nwcase(16; "insertstream-nw-map") and
    tblcase(25; "insertstream-table-map") and
    nwcase(32; "insertstream-nw-avro") and
    tblcase(41; "insertstream-table-avro") and
    nwcase(48; "insertstream-nw-json") and
    tblcase(57; "insertstream-table-json") and
    nwcase(64; "insertstream-nw-jsonclassprovided") and
    tblcase(73; "insertstream-table-jsonclassprovided") and
    nwcase(80; "insertstream-nw-default") and
    tblcase(89; "insertstream-table-default")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-merge-insertstream trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
