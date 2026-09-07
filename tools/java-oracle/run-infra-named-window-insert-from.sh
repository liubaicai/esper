#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-insert-from.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 1; }
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
    .id == "infra-named-window-insert-from" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowInsertFrom.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    ([.javaNames[] | startswith("Infra")] | all) and
    (.javaStaticIds | type == "array" and length == 4) and
    ([.javaStaticIds[] | startswith("java-")] | all) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["create-after-named", "insert-where-type-filter", "lenient-map", "lenient-objectarray"]) and
    ([.cases[].ordinal] == [0, 1, 5, 6]) and
    ([.cases[].runtimeId] == ["java-runtime-b3f6cb7b36c5211c8822", "java-runtime-e601b3cc7f827d578185", "java-runtime-46011542d6e9d34a87f5", "java-runtime-8138dd777290d00417d1"]) and
    ([.cases[].executionName] == ["InfraCreateNamedAfterNamed", "InfraInsertWhereTypeAndFilter", "InfraNamedWindowInsertLenientPropCount{rep=MAP}", "InfraNamedWindowInsertLenientPropCount{rep=OBJECTARRAY}"]) and
    ([.cases[] | select(.observation == "listener")] | length == 4) and
    ([.cases[].iteratorSnapshots] == [0, 3, 1, 1]) and
    # per-case step counts: the insert-where-type-filter enumeration is
    # 1 case + 9 deploys + 9 sends + 3 snapshots + 1 undeploy = 23 steps
    # (7 + 23 + 10 + 10 = 50 total).
    ([.steps[] | select(.case == "create-after-named")] | length == 7) and
    ([.steps[] | select(.case == "insert-where-type-filter")] | length == 23) and
    ([.steps[] | select(.case == "lenient-map")] | length == 10) and
    ([.steps[] | select(.case == "lenient-objectarray")] | length == 10) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .case == "create-after-named")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .case == "insert-where-type-filter")] | length == 9) and
    ([.steps[] | select(.op == "deploy" and .case == "lenient-map")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .case == "lenient-objectarray")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 2) and
    ([.steps[] | select(.op == "snapshot")] | length == 7) and
    ([.steps[] | select(.op == "snapshot" and .statement == "windowTwo" and .mode == "ordered")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .statement == "windowThree" and .mode == "ordered")] | length == 1) and
    # windowFour is the single mode-any pin: the Java assert is any-order.
    ([.steps[] | select(.op == "snapshot" and .statement == "windowFour" and .mode == "any")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .statement == "window" and .mode == "ordered")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 50)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-insert-from replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-insert-from.XXXXXX")
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
    "$script_root/InfraNamedWindowInsertFromScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowInsertFromScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nul: {"state": "null"};
    def rowb($i; $t): {"kind": "row", "fields": {"intPrimitive": $i, "theString": $t}};
    def rows1($t): {"kind": "row", "fields": {"theString": $t}};
    def row2($a; $b): {"kind": "row", "fields": {"c0": $a, "c1": $b}};
    def lis($c; $q; $st; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    # snap($c; $st; $rows) checks a snapshot record; pass .records[N].new as
    # $rows for the mode-any snapshot so the engine iteration order is not
    # pinned (the ordered snapshots pin the engine iterator order instead).
    def snap($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    .version == "esper-parity/v1" and
    .id == "infra-named-window-insert-from" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 18) and
    ([.records[].case] == ["create-after-named", "create-after-named",
        "insert-where-type-filter", "insert-where-type-filter", "insert-where-type-filter",
        "insert-where-type-filter", "insert-where-type-filter", "insert-where-type-filter",
        "insert-where-type-filter", "insert-where-type-filter", "insert-where-type-filter",
        "insert-where-type-filter", "insert-where-type-filter", "insert-where-type-filter",
        "lenient-map", "lenient-map", "lenient-objectarray", "lenient-objectarray"]) and
    ([.records[].operation] == ["listener", "listener",
        "listener", "listener", "listener", "listener", "listener",
        "snapshot", "snapshot", "snapshot",
        "listener", "listener", "listener", "listener",
        "snapshot", "snapshot", "snapshot", "snapshot"]) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "listener") | .statement] == ["windowOne", "selectOne", "window", "window", "window", "window", "window", "windowTwo", "window", "windowThree", "windowFour"]) and
    ([.records[] | select(.operation == "listener") | .sequence] == [1, 1, 1, 2, 3, 4, 5, 1, 6, 1, 1]) and
    ([.records[] | select(.operation == "snapshot") | .statement] == ["windowTwo", "windowThree", "windowFour", "window", "window", "window", "window"]) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == [0, 0, 0, 0, 0, 0, 0]) and
    ([.records[] | has("new")] | all) and
    ([.records[] | select(.operation == "listener" and (.statement == "windowOne" or .statement == "window" or .statement == "windowTwo" or .statement == "windowThree" or .statement == "windowFour")) | .new[]
        | .kind == "row" and ((.fields | keys) == ["intPrimitive", "theString"])] | all) and
    ([.records[] | select(.operation == "listener" and .statement == "selectOne") | .new[]
        | .kind == "row" and ((.fields | keys) == ["theString"])] | all) and
    ([.records[] | select(.operation == "snapshot" and (.statement == "windowTwo" or .statement == "windowThree" or .statement == "windowFour")) | .new[]
        | .kind == "row" and ((.fields | keys) == ["theString"])] | all) and
    ([.records[] | select(.case == "lenient-map" or .case == "lenient-objectarray") | .new[]
        | .kind == "row" and ((.fields | keys) == ["c0", "c1"])] | all) and
    # create-after-named: the shared insert delivers E1 to the windowOne
    # create-statement listener and to the selectOne subscriber, mirroring
    # assertPropsNew at InfraCreateNamedAfterNamed lines 97-98.
    (.records[0] == lis("create-after-named"; 1; "windowOne"; [rowb(1; "E1")])) and
    (.records[1] == lis("create-after-named"; 1; "selectOne"; [rows1("E1")])) and
    # insert-where-type-filter seeds: the window listener exists from its
    # deploy, so the five filtered seed sends each deliver one full row.
    (.records[2] == lis("insert-where-type-filter"; 1; "window"; [rowb(1; "A1")])) and
    (.records[3] == lis("insert-where-type-filter"; 2; "window"; [rowb(1; "B2")])) and
    (.records[4] == lis("insert-where-type-filter"; 3; "window"; [rowb(1; "C3")])) and
    (.records[5] == lis("insert-where-type-filter"; 4; "window"; [rowb(4; "A4")])) and
    (.records[6] == lis("insert-where-type-filter"; 5; "window"; [rowb(4; "C5")])) and
    # seed snapshots: keepall ordered A1..C5 and A-filtered ordered A1,A4
    # (mirroring the exact-order iterator asserts at lines 134 and 145), and
    # the unique(intPrimitive) mode-any snapshot holding C3/C5 (any-order
    # assert at line 155; content checked unordered below).
    (.records[7] == snap("insert-where-type-filter"; "windowTwo"; [rows1("A1"), rows1("B2"), rows1("C3"), rows1("A4"), rows1("C5")])) and
    (.records[8] == snap("insert-where-type-filter"; "windowThree"; [rows1("A1"), rows1("A4")])) and
    (.records[9] == snap("insert-where-type-filter"; "windowFour"; .records[9].new)) and
    ((.records[9].new | length) == 2) and
    ((.records[9].new | map(.fields.theString) | sort) == ["C3", "C5"]) and
    # routing: each filtered insert delivers exactly one row to its target
    # window (lines 172-218); the window sequence continues at 6 after the
    # five seed deliveries, and no other window fires.
    (.records[10] == lis("insert-where-type-filter"; 1; "windowTwo"; [rowb(-9; "B9")])) and
    (.records[11] == lis("insert-where-type-filter"; 6; "window"; [rowb(-8; "A8")])) and
    (.records[12] == lis("insert-where-type-filter"; 1; "windowThree"; [rowb(-7; "C7")])) and
    (.records[13] == lis("insert-where-type-filter"; 1; "windowFour"; [rowb(-6; "D6")])) and
    # lenient: the partial-column inserts leave the unassigned column null,
    # mirroring assertPropsPerRowIterator at lines 70 and 75.
    (.records[14] == snap("lenient-map"; "window"; [row2("E1"; nul)])) and
    (.records[15] == snap("lenient-map"; "window"; [row2("E1"; nul), row2(nul; 10)])) and
    (.records[16] == snap("lenient-objectarray"; "window"; [row2("E1"; nul)])) and
    (.records[17] == snap("lenient-objectarray"; "window"; [row2("E1"; nul), row2(nul; 10)]))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-insert-from trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
