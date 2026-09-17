#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-expr-filter-in-and-between.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "expr-filter-in-and-between" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterInAndBetween.java" and
    (.javaRuntimes | type == "array" and length == 5) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["ExprFilterInDynamic", "ExprFilterReuse", "ExprFilterReuseNot", "ExprFilterInMultipleNonMatchingFirst", "ExprFilterInMultipleWithBool"]) and
    (.javaStaticIds | length == 5 and all(. == "java-17cece2bf9c2df0b27f1")) and
    (.javaFlags == ["OBSERVEROPS"]) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["in-dynamic-pattern", "in-reuse-undeploy", "not-in-reuse-undeploy", "in-multiple-nonmatching-first", "in-multiple-with-bool"]) and
    ([.cases[].ordinal] == [0, 5, 6, 7, 8]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 181) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "deploy")] | length == 37) and
    ([.steps[] | select(.op == "deployed")] | length == 37) and
    ([.steps[] | select(.op == "send")] | length == 67) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 65) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBeanNumeric")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 1) and
    ([.steps[] | select(.op == "undeploy")] | length == 31) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid expr-filter-in-and-between replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-expr-filter-in-and-between.XXXXXX")
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
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/ExprFilterInAndBetweenScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ExprFilterInAndBetweenScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def listener_rows($case):
        [.records[] | select(.case == $case and .operation == "listener")];
    def deployed_rows($case):
        [.records[] | select(.case == $case and .operation == "deployed")];
    .version == "esper-parity/v1" and
    .id == "expr-filter-in-and-between" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 101) and
    ([.records[].operation] | all(. == "listener" or . == "deployed")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # in-dynamic-pattern: 2 deployed + 5 listener (phase A: intPrimitive 10,20;
    # phase B: theString a,b,c).
    ([.records[0:7][] | .case] | all(. == "in-dynamic-pattern")) and
    (deployed_rows("in-dynamic-pattern") | length == 2) and
    (listener_rows("in-dynamic-pattern") | length == 5) and
    (listener_rows("in-dynamic-pattern") | all(.statement == "s0"
        and (.new | length == 1))) and
    ([listener_rows("in-dynamic-pattern")[].new[0].fields.b.intPrimitive]
        == [10, 20, 0, 0, 0]) and
    ([listener_rows("in-dynamic-pattern")[].new[0].fields.b.theString
        | (if type == "object" then .state else . end)]
        == ["null", "null", "a", "b", "c"]) and
    # in-reuse-undeploy: 20 deployed + 36 listener (per group n(n+1)/2).
    ([.records[7:63][] | .case] | all(. == "in-reuse-undeploy")) and
    (deployed_rows("in-reuse-undeploy") | length == 20) and
    (listener_rows("in-reuse-undeploy") | length == 36) and
    (listener_rows("in-reuse-undeploy") | all(.new[0].fields.intBoxed == 3)) and
    # not-in-reuse-undeploy: 11 deployed + 21 listener.
    ([.records[63:95][] | .case] | all(. == "not-in-reuse-undeploy")) and
    (deployed_rows("not-in-reuse-undeploy") | length == 11) and
    (listener_rows("not-in-reuse-undeploy") | length == 21) and
    (listener_rows("not-in-reuse-undeploy") | all(.new[0].fields.intBoxed == 3)) and
    # in-multiple-nonmatching-first: 2 deployed + 1 listener (B only).
    ([.records[95:98][] | .case] | all(. == "in-multiple-nonmatching-first")) and
    (deployed_rows("in-multiple-nonmatching-first") | map(.statement) == ["A", "B"]) and
    (listener_rows("in-multiple-nonmatching-first") | length == 1) and
    (listener_rows("in-multiple-nonmatching-first")[0].statement == "B") and
    (listener_rows("in-multiple-nonmatching-first")[0].new[0].fields.theString == "A") and
    (listener_rows("in-multiple-nonmatching-first")[0].new[0].fields.intPrimitive == 0) and
    # in-multiple-with-bool: 2 deployed + 1 listener (s2 only).
    ([.records[98:101][] | .case] | all(. == "in-multiple-with-bool")) and
    (deployed_rows("in-multiple-with-bool") | map(.statement) == ["s1", "s2"]) and
    (listener_rows("in-multiple-with-bool") | length == 1) and
    (listener_rows("in-multiple-with-bool")[0].statement == "s2") and
    (listener_rows("in-multiple-with-bool")[0].new[0].fields.theString == "A") and
    (listener_rows("in-multiple-with-bool")[0].new[0].fields.intPrimitive == 1)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid expr-filter-in-and-between trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
