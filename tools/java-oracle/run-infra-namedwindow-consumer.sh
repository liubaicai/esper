#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-namedwindow-consumer.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-namedwindow-consumer" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowConsumer.java" and
    (.javaRuntimes | type == "array" and length == 3) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraNamedWindowConsumerKeepAll", "InfraNamedWindowConsumerLengthWin", "InfraNamedWindowConsumerWBatch"]) and
    (.javaStaticIds | length == 3 and all(. == "java-04f7e86e470bf275affa")) and
    (.javaFlags == ["EXCLUDEWHENINSTRUMENTED"]) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["keepall", "lengthwin", "wbatch"]) and
    ([.cases[].ordinal] == [0, 1, 2]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 35) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 11) and
    ([.steps[] | select(.op == "deployed")] | length == 11) and
    ([.steps[] | select(.op == "send")] | length == 6) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 6) and
    ([.steps[] | select(.op == "send-batch" and .eventType == "IncomingEvent" and .count == 10000)] | length == 1) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "send-batch" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-namedwindow-consumer replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-namedwindow-consumer.XXXXXX")
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
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNamedWindowConsumerScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowConsumerScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def listener_rows($case):
        [.records[] | select(.case == $case and .operation == "listener")];
    .version == "esper-parity/v1" and
    .id == "infra-namedwindow-consumer" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 17) and
    ([.records[].operation] | all(. == "listener" or . == "deployed")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # keepall: 3 deployed + 2 listener (E1, E2 new-only).
    ([.records[0:5][] | .case] | all(. == "keepall")) and
    ([.records[0:5][] | select(.operation == "deployed") | .statement]
        == ["create", "insert", "select"]) and
    (listener_rows("keepall") | length == 2) and
    (listener_rows("keepall") | all(.statement == "select"
        and (.new | length == 1) and (.old == null or (.old | length == 0)))) and
    ([listener_rows("keepall")[].new[0].fields.theString] == ["E1", "E2"]) and
    # lengthwin: 3 deployed + 4 listener (aggregate new/old pairs).
    ([.records[5:12][] | .case] | all(. == "lengthwin")) and
    ([.records[5:12][] | select(.operation == "deployed") | .statement]
        == ["create", "insert", "s0"]) and
    (listener_rows("lengthwin") | length == 4) and
    (listener_rows("lengthwin") | all(.statement == "s0" and (.new | length == 1))) and
    ([listener_rows("lengthwin")[].new[0].fields.c0] == ["E1", "E2", "E3", "E4"]) and
    ([listener_rows("lengthwin")[].new[0].fields.c1] == [10, 30, 45, 51]) and
    # wbatch: 5 deployed, no listener records.
    ([.records[12:17][] | .case] | all(. == "wbatch")) and
    ([.records[12:17][] | select(.operation == "deployed") | .statement]
        == ["schema-incoming", "schema-retained", "insert-retained", "create-window", "insert-window"]) and
    (listener_rows("wbatch") | length == 0)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-namedwindow-consumer trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
