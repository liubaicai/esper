#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-subquery.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "failed to resolve Esper commit in $esper_root" >&2
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
    echo "Java 17 is required; resolved java reports version $java_version" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-named-window-subquery" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowSubquery.java" and
    (.description | test("InfraNamedWindowSubquery")) and
    (.javaRuntimes == ["java-runtime-901e88676d86ad580af1", "java-runtime-4261803b5e9dcef8a867", "java-runtime-95adfcdb21f327e1cc4e"]) and
    (.javaNames == ["InfraSubqueryTwoConsumerWindow", "InfraSubqueryLateConsumerAggregation", "InfraSubqueryWithFilterInParens"]) and
    (.javaStaticIds == ["java-f8f4ec7164371451341d", "java-ff6a2dd6de98a3f1bb81", "java-557b2408e6489254dd8e"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["two-consumer-window", "late-consumer-aggregation", "filter-in-parens"]) and
    ([.cases[].ordinal] == [0, 1, 2]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "op", "statement"] or . == ["case", "name", "op"] or . == ["case", "op"])) and
    # per-case step counts: two-consumer-window = 1 case + 4 deploys +
    # 4 deployed + 1 send + 1 read-variable + 1 undeploy-all = 12;
    # late-consumer-aggregation = 1 + 3 deploys + 3 deployed + 3 sends +
    # 1 undeploy-all = 11; filter-in-parens = 1 + 3 deploys + 3 deployed +
    # 5 sends + 1 undeploy-all = 13 (12 + 11 + 13 = 36 total).
    ([.steps[] | select(.case == "two-consumer-window")] | length == 12) and
    ([.steps[] | select(.case == "late-consumer-aggregation")] | length == 11) and
    ([.steps[] | select(.case == "filter-in-parens")] | length == 13) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 10) and
    ([.steps[] | select(.op == "deployed")] | length == 10) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert-count")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "variable")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "assign")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "send")] | length == 9) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 6) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 3) and
    ([.steps[] | select(.op == "read-variable" and .name == "myvar" and .case == "two-consumer-window")] | length == 1) and
    # byte-exact EPL pins, including the @Name/@name and @public annotations
    # the Java source writes.
    ([.steps[] | select(.op == "deploy" and .case == "two-consumer-window") | .epl] == ["create window MyWindowTwo#length(1) as (mycount long)", "@Name('"'"'insert-count'"'"') insert into MyWindowTwo select 1L as mycount from SupportBean", "create variable long myvar = 0", "@Name('"'"'assign'"'"') on MyWindowTwo set myvar = (select mycount from MyWindowTwo)"]) and
    ([.steps[] | select(.op == "deploy" and .case == "late-consumer-aggregation") | .epl] == ["@public create window MyWindow#keepall as SupportBean", "insert into MyWindow select * from SupportBean", "@name('"'"'s0'"'"') select * from MyWindow where (select count(*) from MyWindow) > 0"]) and
    ([.steps[] | select(.op == "deploy" and .case == "filter-in-parens") | .epl] == ["create window MyWindow#keepall as SupportBean", "@name('"'"'insert'"'"') insert into MyWindow select * from SupportBean", "@name('"'"'s0'"'"') select exists (select * from MyWindow(theString='"'"'E1'"'"')) as c0 from SupportBean_S0"]) and
    # send payload shapes: theString + intPrimitive for SupportBean, id only
    # for the S0 trigger.
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | (.payload | keys)] | all(. == ["id"])) and
    (.steps | type == "array" and length == 36)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-subquery replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-subquery.XXXXXX")
cleanup() {
    rm -rf "$work"
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
    CYGWIN*|MINGW*|MSYS*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
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
    "$script_root/InfraNamedWindowSubqueryScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowSubqueryScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nul: {"state": "null"};
    def bean($s; $i): {"kind": "row", "fields": {
        "bigDecimal": nul, "bigInteger": nul, "boolBoxed": nul,
        "boolPrimitive": false, "byteBoxed": nul, "bytePrimitive": 0,
        "charBoxed": nul, "charPrimitive": "\u0000", "doubleBoxed": nul,
        "doublePrimitive": 0, "enumValue": nul, "floatBoxed": nul,
        "floatPrimitive": 0, "intBoxed": nul, "intPrimitive": $i,
        "longBoxed": nul, "longPrimitive": 0, "shortBoxed": nul,
        "shortPrimitive": 0, "theString": $s}};
    def dep($c; $st; $q): {"case": $c, "operation": "deployed", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z"};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def crec($c): [.records[] | select(.case == $c)];
    # two-consumer-window: four deployed markers (window, insert-count,
    # create variable, on-set assign) and the myvar read the Java
    # assertRuntime pins at 1L.
    def two: [
        dep("two-consumer-window"; "create"; 1),
        dep("two-consumer-window"; "insert-count"; 1),
        dep("two-consumer-window"; "variable"; 1),
        dep("two-consumer-window"; "assign"; 1),
        {"case": "two-consumer-window", "operation": "variable", "sequence": 0, "name": "myvar", "value": 1}
    ];
    # late-consumer-aggregation: three deployed markers and the single s0
    # listener invocation for the post-attach E3 insert (the E1/E2 preload
    # output is never observed).
    def late: [
        dep("late-consumer-aggregation"; "create"; 1),
        dep("late-consumer-aggregation"; "insert"; 1),
        dep("late-consumer-aggregation"; "s0"; 1),
        lisn("late-consumer-aggregation"; "s0"; 1; [bean("E3"; 1)])
    ];
    # filter-in-parens: three deployed markers and the three s0 listener
    # invocations, one new c0 row each (false, false, true).
    def parens: [
        dep("filter-in-parens"; "create"; 1),
        dep("filter-in-parens"; "insert"; 1),
        dep("filter-in-parens"; "s0"; 1),
        lisn("filter-in-parens"; "s0"; 1; [{"kind": "row", "fields": {"c0": false}}]),
        lisn("filter-in-parens"; "s0"; 2; [{"kind": "row", "fields": {"c0": false}}]),
        lisn("filter-in-parens"; "s0"; 3; [{"kind": "row", "fields": {"c0": true}}])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-subquery" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 15) and
    ((crec("two-consumer-window") | length) == 5) and
    ((crec("late-consumer-aggregation") | length) == 4) and
    ((crec("filter-in-parens") | length) == 6) and
    ([.records[].case] == (["two-consumer-window", "two-consumer-window", "two-consumer-window", "two-consumer-window", "two-consumer-window"]
        + ["late-consumer-aggregation", "late-consumer-aggregation", "late-consumer-aggregation", "late-consumer-aggregation"]
        + ["filter-in-parens", "filter-in-parens", "filter-in-parens", "filter-in-parens", "filter-in-parens", "filter-in-parens"])) and
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | select(.operation == "variable")] == [{"case": "two-consumer-window", "operation": "variable", "sequence": 0, "name": "myvar", "value": 1}]) and
    (crec("two-consumer-window") == two) and
    (crec("late-consumer-aggregation") == late) and
    (crec("filter-in-parens") == parens)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-subquery trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
