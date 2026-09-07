#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-on-update.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-on-update" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    ([.javaNames[] | startswith("Infra")] | all) and
    (.javaStaticIds | type == "array" and length == 4) and
    ([.javaStaticIds[] | startswith("java-")] | all) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["intersect", "union", "multikey-array", "multikey-two-fields"]) and
    ([.cases[].ordinal] == [1, 2, 6, 7]) and
    ([.cases[].runtimeId] == ["java-runtime-f921cf2543cbb10b4150", "java-runtime-0947ec873ea298b60209", "java-runtime-9870ee394d3099ef85e8", "java-runtime-a25a18f2754aeae08015"]) and
    ([.cases[].executionName] == ["InfraMultipleDataWindowIntersect", "InfraMultipleDataWindowUnion", "InfraUpdateMultikeyWArrayPrimitiveArray", "InfraUpdateMultikeyWArrayTwoFields"]) and
    ([.cases[] | select(.observation == "listener")] | length == 4) and
    ([.cases[].iteratorSnapshots] == [1, 1, 1, 1]) and
    ([.steps[] | select(.case == "intersect")] | length == 9) and
    ([.steps[] | select(.case == "union")] | length == 9) and
    ([.steps[] | select(.case == "multikey-array")] | length == 14) and
    ([.steps[] | select(.case == "multikey-two-fields")] | length == 14) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "update")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportEventWithManyArray")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportEventWithIntArray")] | length == 9) and
    ([.steps[] | select(.op == "snapshot")] | length == 4) and
    ([.steps[] | select(.op == "snapshot" and .mode == "any")] | length == 3) and
    ([.steps[] | select(.op == "snapshot" and (has("mode") | not))] | length == 1) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 46)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-on-update replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-on-update.XXXXXX")
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
    "$script_root/InfraNamedWindowOnUpdateScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowOnUpdateScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def rowp($i; $t): {"kind": "row", "fields": {"intPrimitive": $i, "theString": $t}};
    def rowm($i; $v): {"kind": "row", "fields": {"id": $i, "value": $v}};
    def lis($c; $q; $new): {"case": $c, "operation": "listener", "statement": "create", "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    # liso is a listener record carrying both update new and old delivery.
    def liso($c; $q; $new; $old): {"case": $c, "operation": "listener", "statement": "create", "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    # snap($c; $rows) checks a snapshot record; pass .records[N].new as $rows
    # for mode-any snapshots so the engine iteration order is not pinned.
    def snap($c; $rows): {"case": $c, "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    .version == "esper-parity/v1" and
    .id == "infra-named-window-on-update" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 10) and
    ([.records[].case] == ["intersect", "intersect", "intersect", "intersect",
                           "union", "union", "union", "union",
                           "multikey-array", "multikey-two-fields"]) and
    ([.records[].operation] == ["listener", "listener", "listener", "snapshot",
                                "listener", "listener", "listener", "snapshot",
                                "snapshot", "snapshot"]) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "listener") | .statement] | all(. == "create")) and
    ([.records[] | select(.operation == "listener") | .sequence] == [1, 2, 3, 1, 2, 3]) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == [0, 0, 0, 0]) and
    ([.records[] | has("new")] | all) and
    ([.records[] | select(has("old"))] | length == 2) and
    ([.records[] | select(.case == "intersect" or .case == "union") | .new[] | .kind == "row" and ((.fields | keys) == ["intPrimitive", "theString"])] | all) and
    ([.records[] | select(.case == "multikey-array" or .case == "multikey-two-fields") | .new[] | .kind == "row" and ((.fields | keys) == ["id", "value"])] | all) and
    # intersect: the create listener sees the two inserts and then the
    # on-update delivery (new E2/300, old E2/3); the mode-any snapshot holds
    # the updated rows regardless of engine iteration order.
    (.records[0] == lis("intersect"; 1; [rowp(2; "E1")])) and
    (.records[1] == lis("intersect"; 2; [rowp(3; "E2")])) and
    (.records[2] == liso("intersect"; 3; [rowp(300; "E2")]; [rowp(3; "E2")])) and
    ((.records[3].new | sort_by(.fields.theString)) == [rowp(2; "E1"), rowp(300; "E2")]) and
    (.records[3] == snap("intersect"; .records[3].new)) and
    # union: same listener shape; the snapshot pin is ordered by theString,
    # mirroring EPAssertionUtil.sort in InfraMultipleDataWindowUnion.
    (.records[4] == lis("union"; 1; [rowp(2; "E1")])) and
    (.records[5] == lis("union"; 2; [rowp(3; "E2")])) and
    (.records[6] == liso("union"; 3; [rowp(300; "E2")]; [rowp(3; "E2")])) and
    (.records[7] == snap("union"; [rowp(2; "E1"), rowp(300; "E2")])) and
    # multikey-array: single mode-any snapshot; the single-field int[] update
    # sets E2=10 via [3,4], E3=11 via [1], E4=12 via [], and E1=13 via [1,2],
    # compared as a sorted-by-id multiset.
    ((.records[8].new | sort_by(.fields.id)) == [rowm("E1"; 13), rowm("E2"; 10), rowm("E3"; 11), rowm("E4"; 12)]) and
    (.records[8] == snap("multikey-array"; .records[8].new)) and
    # multikey-two-fields: single mode-any snapshot; the id + int[] update
    # sets ID2=10, ID3=11, and ID1=12, while IDX and the [1,2,3] trigger miss.
    ((.records[9].new | sort_by(.fields.id)) == [rowm("ID1"; 12), rowm("ID2"; 10), rowm("ID3"; 11)]) and
    (.records[9] == snap("multikey-two-fields"; .records[9].new))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-on-update trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
