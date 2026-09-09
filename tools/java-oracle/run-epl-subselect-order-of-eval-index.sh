#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-subselect-order-of-eval-index.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "epl-subselect-order-of-eval-index" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/subselect/EPLSubselectOrderOfEval.java" and
    (.javaRuntimes | type == "array" and length == 5) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 5) and
    (.javaStaticIds | type == "array" and length == 5) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 5) and
    ([.cases[] | select(.observation == "listener" and .iteratorSnapshots == 0)] | length == 5) and
    ([.cases[].case] == ["correlated-subquery-order", "order-of-eval-subselect-first", "index-choices-overdefined-where", "unique-index-correlated", "order-of-eval-no-preeval"]) and
    ([.cases[].ordinal] == [0, 1, 0, 1, 0]) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportTradeEventTwo")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportSimpleBeanOne")] | length == 36) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportSimpleBeanTwo")] | length == 52) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 19) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 28) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 28) and
    (.steps | type == "array" and length == 177)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid epl-subselect-order-of-eval-index replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-subselect-order-of-eval-index.XXXXXX")
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
    "$script_root/EPLSubselectOrderOfEvalIndexScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLSubselectOrderOfEvalIndexScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "epl-subselect-order-of-eval-index" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 47) and
    ([.records[].operation] | all(. == "listener")) and
    ([.records[].statement] | all(. == "s0")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[].new | length] | all(. == 1)) and
    # correlated-subquery-order: window columns render as arrays of rows.
    ([.records[0:2][].case] | all(. == "correlated-subquery-order")) and
    ([.records[0:2][].sequence] == [1, 2]) and
    ([.records[0:2][].new[0].fields | keys] | all(. == ["longItems", "shortItems"])) and
    ([.records[0:2][].new[0].fields.longItems | length] == [1, 2]) and
    ([.records[0:2][].new[0].fields.shortItems | length] == [1, 2]) and
    ([.records[0:2][].new[0].fields.longItems[].fields | keys] | all(. == ["price", "securityID", "time", "volume"])) and
    ([.records[0:2][].new[0].fields.longItems[].fields.price] | all(. == 50.0)) and
    ([.records[0:2][].new[0].fields.longItems[].fields.securityID] | all(. == 1000)) and
    ([.records[0].new[0].fields.longItems[0].fields.time, .records[1].new[0].fields.longItems[1].fields.time] == [1000, 1010]) and
    ([.records[0:2][].new[0].fields.longItems[].fields.volume] | all(. == 1)) and
    (.records[0].new[0].fields.shortItems[0] == .records[0].new[0].fields.longItems[0]) and
    # order-of-eval-subselect-first emits no records: preeval-on not-in stays silent.
    # The preeval-off counterpart order-of-eval-no-preeval fires on its own runtime.
    # index-choices-overdefined-where: 36 records, sequences 1..36; null renders as the tagged state object.
    ([.records[2:38][].case] | all(. == "index-choices-overdefined-where")) and
    ([.records[2:38][].sequence] == [range(1; 37)]) and
    ([.records[2:38][].new[0].fields | keys] | all(. == ["c0", "c1"])) and
    ([.records[2].new[0].fields.c0, .records[2].new[0].fields.c1] == ["EX", "E1"]) and
    ([.records[3].new[0].fields.c0, .records[3].new[0].fields.c1] == ["EY", {"state": "null"}]) and
    ([.records[4:18][].new[0].fields.c0] | all(. == "EX")) and
    ([.records[4:18][].new[0].fields.c1] | all(. == "E3")) and
    ([.records[18:23][].new[0].fields.c0] == ["A", "B", "C", "D", "E"]) and
    ([.records[18:23][].new[0].fields.c1] == [{"state": "null"}, "E1", {"state": "null"}, {"state": "null"}, {"state": "null"}]) and
    ([.records[23:28][].new[0].fields.c0] == ["A", "B", "C", "D", "E"]) and
    ([.records[23:28][].new[0].fields.c1] == [{"state": "null"}, "E1", "E1", {"state": "null"}, {"state": "null"}]) and
    ([.records[28:33][].new[0].fields.c0] == ["A", "B", "C", "D", "E"]) and
    ([.records[28:33][].new[0].fields.c1] == [{"state": "null"}, "E2", {"state": "null"}, {"state": "null"}, {"state": "null"}]) and
    ([.records[33:38][].new[0].fields.c0] == ["A", "B", "C", "D", "E"]) and
    ([.records[33:38][].new[0].fields.c1] == [{"state": "null"}, "E2", "E2", {"state": "null"}, {"state": "null"}]) and
    # unique-index-correlated: 7 records over unique/firstunique/time+unique/groupwin+unique.
    ([.records[38:45][].case] | all(. == "unique-index-correlated")) and
    ([.records[38:45][].sequence] == [1, 2, 3, 4, 5, 6, 7]) and
    ([.records[38:45][].new[0].fields | keys] | all(. == ["c0", "c1"])) and
    ([.records[38:45][].new[0].fields.c0] == [10, 11, 10, 11, 10, 11, 1]) and
    ([.records[38:45][].new[0].fields.c1] == [4, 3, 2, 1, 4, 3, 102]) and
    # order-of-eval-no-preeval: 2 records on the selfSubselectPreeval=false runtime; the not-in filters fire.
    ([.records[45:47][].case] | all(. == "order-of-eval-no-preeval")) and
    ([.records[45:47][].sequence] == [1, 2]) and
    ([.records[45:47][].new[0].fields | keys] | all(. == ["bigDecimal", "bigInteger", "boolBoxed", "boolPrimitive", "byteBoxed", "bytePrimitive", "charBoxed", "charPrimitive", "doubleBoxed", "doublePrimitive", "enumValue", "floatBoxed", "floatPrimitive", "intBoxed", "intPrimitive", "longBoxed", "longPrimitive", "shortBoxed", "shortPrimitive", "theString"])) and
    ([.records[45:47][].new[0].fields.theString] | all(. == "E1")) and
    ([.records[45:47][].new[0].fields.intPrimitive] | all(. == 5)) and
    ([.records[45:47][].new[0].fields.boolPrimitive] | all(. == false)) and
    ([.records[45:47][].new[0].fields.charPrimitive] | all(. == "\u0000")) and
    ([.records[45:47][].new[0].fields.bytePrimitive, .records[45:47][].new[0].fields.shortPrimitive, .records[45:47][].new[0].fields.longPrimitive, .records[45:47][].new[0].fields.floatPrimitive, .records[45:47][].new[0].fields.doublePrimitive] | all(. == 0)) and
    ([.records[45:47][].new[0].fields.boolBoxed, .records[45:47][].new[0].fields.byteBoxed, .records[45:47][].new[0].fields.charBoxed, .records[45:47][].new[0].fields.doubleBoxed, .records[45:47][].new[0].fields.floatBoxed, .records[45:47][].new[0].fields.intBoxed, .records[45:47][].new[0].fields.longBoxed, .records[45:47][].new[0].fields.shortBoxed, .records[45:47][].new[0].fields.bigDecimal, .records[45:47][].new[0].fields.bigInteger, .records[45:47][].new[0].fields.enumValue] | all(. == {"state": "null"}))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid epl-subselect-order-of-eval-index trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
