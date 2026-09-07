#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-start-stop.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-start-stop" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableStartStop.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    ([.javaNames[] | startswith("InfraStartStop")] | all) and
    (.javaStaticIds | type == "array" and length == 4) and
    ([.javaStaticIds[] | . == "java-2fe58125cb5f265d5fd1" or . == "java-c4a5793bec169880d651"] | all) and
    (.javaFlags == ["OBSERVEROPS"]) and
    (.cases | type == "array" and length == 4) and
    ([.cases[] | select(.observation == "listener")] | length == 4) and
    ([.cases[].case] == ["consumer-nw", "consumer-table", "inserter-nw", "inserter-table"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].iteratorSnapshots] == [5, 4, 3, 2]) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 16) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 6) and
    ([.steps[] | select(.op == "deploy" and .statement == "select")] | length == 6) and
    ([.steps[] | select(.op == "undeploy")] | length == 8) and
    ([.steps[] | select(.op == "snapshot")] | length == 14) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 62)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-start-stop replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-start-stop.XXXXXX")
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
    "$script_root/InfraNWTableStartStopScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableStartStopScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def row($a; $b): {"kind": "row", "fields": {"a": $a, "b": $b}};
    def lis($c; $s; $q; $r): {"case": $c, "operation": "listener", "statement": $s, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": [$r]};
    # snapany compares a mode:"any" snapshot: the captured rows must equal the
    # expected rows regardless of the engine iteration order.
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-start-stop" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 24) and
    ([.records[].case] == ["consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw", "consumer-nw",
                           "consumer-table", "consumer-table", "consumer-table", "consumer-table",
                           "inserter-nw", "inserter-nw", "inserter-nw", "inserter-nw", "inserter-nw", "inserter-nw", "inserter-nw",
                           "inserter-table", "inserter-table"]) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "listener") | .sequence >= 1] | all) and
    ([.records[] | select(.operation == "snapshot") | .sequence == 0] | all) and
    ([.records[] | has("new")] | all) and
    ([.records[] | select(has("old"))] | length == 0) and
    ([.records[] | select(has("new")) | .new[] | .kind == "row" and ((.fields | keys) == ["a", "b"])] | all) and
    # consumer-nw: 6 listener records (create E1-E4, select E1 and E3) and
    # 5 iterator snapshots; the stopped consumer misses E2 and E4 while the
    # create listener keeps firing, and the redeployed consumer immediately
    # sees E1 and E2 through its iterator.
    (.records[0] == lis("consumer-nw"; "create"; 1; row("E1"; 1))) and
    (.records[1] == lis("consumer-nw"; "select"; 1; row("E1"; 1))) and
    (.records[2] == {"case": "consumer-nw", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1)]}) and
    (.records[3] == lis("consumer-nw"; "create"; 2; row("E2"; 2))) and
    ((.records[4].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2)]) and
    (.records[4] == {"case": "consumer-nw", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[4].new}) and
    ((.records[5].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2)]) and
    (.records[5] == {"case": "consumer-nw", "operation": "snapshot", "statement": "select", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[5].new}) and
    (.records[6] == lis("consumer-nw"; "create"; 3; row("E3"; 3))) and
    (.records[7] == lis("consumer-nw"; "select"; 2; row("E3"; 3))) and
    (.records[8] == {"case": "consumer-nw", "operation": "snapshot", "statement": "select", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1), row("E2"; 2), row("E3"; 3)]}) and
    ((.records[9].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2), row("E3"; 3)]) and
    (.records[9] == {"case": "consumer-nw", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[9].new}) and
    (.records[10] == lis("consumer-nw"; "create"; 4; row("E4"; 4))) and
    # consumer-table: both listeners stay silent over the table; only the
    # iterator snapshots observe the growing table contents.
    (.records[11] == {"case": "consumer-table", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1)]}) and
    ((.records[12].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2)]) and
    (.records[12] == {"case": "consumer-table", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[12].new}) and
    ((.records[13].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2)]) and
    (.records[13] == {"case": "consumer-table", "operation": "snapshot", "statement": "select", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[13].new}) and
    ((.records[14].new | sort_by(.fields.a)) == [row("E1"; 1), row("E2"; 2), row("E3"; 3)]) and
    (.records[14] == {"case": "consumer-table", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[14].new}) and
    # inserter-nw: 4 listener records (create and select for E1 and E3; E2 and
    # E4 are sent while the insert is undeployed) and 3 iterator snapshots.
    (.records[15] == lis("inserter-nw"; "create"; 1; row("E1"; 1))) and
    (.records[16] == lis("inserter-nw"; "select"; 1; row("E1"; 1))) and
    (.records[17] == {"case": "inserter-nw", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1)]}) and
    (.records[18] == lis("inserter-nw"; "create"; 2; row("E3"; 3))) and
    (.records[19] == lis("inserter-nw"; "select"; 2; row("E3"; 3))) and
    (.records[20] == {"case": "inserter-nw", "operation": "snapshot", "statement": "select", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1), row("E3"; 3)]}) and
    ((.records[21].new | sort_by(.fields.a)) == [row("E1"; 1), row("E3"; 3)]) and
    (.records[21] == {"case": "inserter-nw", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[21].new}) and
    # inserter-table: silent listeners; iterator snapshots only.
    (.records[22] == {"case": "inserter-table", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": [row("E1"; 1)]}) and
    ((.records[23].new | sort_by(.fields.a)) == [row("E1"; 1), row("E3"; 3)]) and
    (.records[23] == {"case": "inserter-table", "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": .records[23].new})
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-start-stop trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
