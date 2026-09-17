#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-namedwindow-on-delete-indexes.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-namedwindow-on-delete-indexes" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraStaggeredNamedWindow", "InfraCoercionKeyMultiPropIndexes", "InfraCoercionRangeMultiPropIndexes", "InfraCoercionKeyAndRangeMultiPropIndexes"]) and
    (.javaStaticIds == ["java-06bf0eb71230b3119293", "java-06bf0eb71230b3119293", "java-06bf0eb71230b3119293", "java-06bf0eb71230b3119293"]) and
    (.javaFlags == ["STATICHOOK"]) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["staggered", "coercion-key", "coercion-range", "coercion-key-range"]) and
    ([.cases[].ordinal] == [1, 2, 3, 4]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 185) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 32) and
    ([.steps[] | select(.op == "deployed")] | length == 32) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 34) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBeanTwo")] | length == 12) and
    ([.steps[] | select(.op == "index-count")] | length == 29) and
    ([.steps[] | select(.op == "snapshot")] | length == 15) and
    ([.steps[] | select(.op == "undeploy")] | length == 23) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.case == "staggered" and .op == "index-count") | .count] == [0, 0, 1, 1, 2, 2, 1]) and
    ([.steps[] | select(.case == "staggered" and .op == "index-count") | .of] | all(. == "rows")) and
    ([.steps[] | select(.case == "coercion-key" and .op == "index-count") | .count] == [1, 1, 2, 3, 4, 5, 6, 0, 1, 2]) and
    ([.steps[] | select(.case == "coercion-range" and .op == "index-count") | .count] == [1, 2, 3, 4, 4, 4, 0]) and
    ([.steps[] | select(.case == "coercion-key-range" and .op == "index-count") | .count] == [1, 2, 3, 4, 0]) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "index-count" and .op != "snapshot" and .op != "undeploy" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-namedwindow-on-delete-indexes replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-namedwindow-on-delete-indexes.XXXXXX")
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
    "$script_root/InfraNamedWindowOnDeleteIndexesScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowOnDeleteIndexesScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def deployed_at($idx; $name):
        .records[$idx].operation == "deployed" and .records[$idx].statement == $name
            and .records[$idx].sequence == 1;
    def index_counts($from; $to):
        [.records[$from:$to][] | select(.operation == "index-count") | .count];
    def old_strings($from; $to):
        [.records[$from:$to][] | select(.operation == "listener" and has("old"))
            | .old[0].fields.theString];
    .version == "esper-parity/v1" and
    .id == "infra-namedwindow-on-delete-indexes" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 118) and
    ([.records[].operation] | all(. == "listener" or . == "deployed"
        or . == "index-count" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # staggered: 5 deployed + 2 initial row counts + 6 listener + 5 snapshot
    # + 5 send-time row counts = 23 records.
    ([.records[0:23][].case] | all(. == "staggered")) and
    deployed_at(0; "createOne") and
    (.records[1].operation == "index-count" and .records[1].statement == "MyWindowSTAG"
        and .records[1].count == 0) and
    deployed_at(2; "createTwo") and
    (.records[3].operation == "index-count" and .records[3].statement == "MyWindowSTAGTwo"
        and .records[3].count == 0) and
    deployed_at(4; "delete") and deployed_at(5; "insert") and deployed_at(6; "insertTwo") and
    index_counts(0; 23) == [0, 0, 1, 1, 2, 2, 1] and
    ([.records[0:23][] | select(.operation == "snapshot")] | length == 5) and
    # E1(-10): createTwo new {a2:E1,b2:-10}.
    (.records[7].operation == "listener" and .records[7].statement == "createTwo"
        and .records[7].new[0].fields.a2 == "E1" and .records[7].new[0].fields.b2 == -10) and
    # E2(5): createOne new {a1:E2,b1:5}.
    (.records[10].operation == "listener" and .records[10].statement == "createOne"
        and .records[10].new[0].fields.a1 == "E2" and .records[10].new[0].fields.b1 == 5) and
    # E3(-1): createTwo new {E3,-1}.
    (.records[13].operation == "listener" and .records[13].statement == "createTwo"
        and .records[13].new[0].fields.a2 == "E3" and .records[13].new[0].fields.b2 == -1) and
    # E3(1): cross-window delete fires — createOne new {E3,1}, createTwo old
    # {E3,-1}, delete new {E3,-1}; the three deliveries share one send so the
    # engine picks their relative order.
    ([.records[16:19][] | .statement] | sort == ["createOne", "createTwo", "delete"]) and
    ([.records[16:19][] | select(.statement == "createOne") | .new[0].fields.a1] == ["E3"]) and
    ([.records[16:19][] | select(.statement == "createTwo") | .old[0].fields.a2] == ["E3"]
        and [.records[16:19][] | select(.statement == "createTwo") | .old[0].fields.b2] == [-1]) and
    ([.records[16:19][] | select(.statement == "delete") | .new[0].fields.a2] == ["E3"]) and
    (.records[19].operation == "snapshot" and .records[19].statement == "createOne"
        and ([.records[19].new[].fields.a1] == ["E2", "E3"])) and
    (.records[20].operation == "snapshot" and .records[20].statement == "createTwo"
        and ([.records[20].new[].fields.a2] == ["E1"])) and
    # coercion-key: 13 deployed + 10 index-count + 10 snapshot + 16 listener
    # = 49 records.
    ([.records[23:72][].case] | all(. == "coercion-key")) and
    ([.records[23:72][] | select(.operation == "deployed") | .statement]
        == ["createOne", "d1", "d2", "d3", "d4", "d5", "d6", "d7", "insert",
            "d0", "createTwo", "select1", "select2"]) and
    index_counts(23; 72) == [1, 1, 2, 3, 4, 5, 6, 0, 1, 2] and
    ([.records[23:72][] | select(.operation == "index-count") | .statement]
        == ["MyWindowCK", "MyWindowCK", "MyWindowCK", "MyWindowCK", "MyWindowCK",
            "MyWindowCK", "MyWindowCK", "MyWindowCK", "WinOne", "WinOne"]) and
    ([.records[23:72][] | select(.operation == "listener")] | length == 16) and
    ([.records[23:72][] | select(.operation == "listener" and has("new"))
        | .new[0].fields.theString] == ["E1", "E2", "E3", "E4", "E5", "E6", "E7", "E8"]) and
    (old_strings(23; 72) == ["E3", "E4", "E1", "E5", "E6", "E7", "E8", "E2"]) and
    ([.records[23:72][] | select(.operation == "snapshot")] | length == 10) and
    ([.records[23:72][] | select(.operation == "snapshot") | .new | length]
        == [4, 3, 2, 1, 3, 2, 1, 2, 1, 0]) and
    # coercion-range: 8 deployed + 7 index-count + 12 listener = 27 records.
    ([.records[72:99][].case] | all(. == "coercion-range")) and
    ([.records[72:99][] | select(.operation == "deployed") | .statement]
        == ["createOne", "insert", "d0", "d1", "d2", "d3", "d4", "d5"]) and
    index_counts(72; 99) == [1, 2, 3, 4, 4, 4, 0] and
    ([.records[72:99][] | select(.operation == "listener")] | length == 12) and
    (old_strings(72; 99) == ["E1", "E2", "E3", "E4", "E5", "E6"]) and
    # coercion-key-range: 6 deployed + 5 index-count + 8 listener = 19 records.
    ([.records[99:118][].case] | all(. == "coercion-key-range")) and
    ([.records[99:118][] | select(.operation == "deployed") | .statement]
        == ["createOne", "insert", "d0", "d1", "d2", "d3"]) and
    index_counts(99; 118) == [1, 2, 3, 4, 0] and
    ([.records[99:118][] | select(.operation == "listener")] | length == 8) and
    (old_strings(99; 118) == ["E1", "E2", "E3", "E4"])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-namedwindow-on-delete-indexes trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
