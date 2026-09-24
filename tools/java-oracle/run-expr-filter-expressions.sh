#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-expr-filter-expressions.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "expr-filter-expressions" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/expr/filter/ExprFilterExpressions.java" and
    (.javaRuntimes | type == "array" and length == 3) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["ExprFilterOverInClause", "ExprFilterStaticFunc", "ExprFilterInstanceMethodWWildcard"]) and
    (.javaStaticIds == ["java-9d2f2743881d949bbddc", "java-314bbbdca28d0445fdd0", "java-576e4853b035fd6064f5"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["over-in-clause", "static-func", "instance-method-wildcard"]) and
    ([.cases[].ordinal] == [7, 17, 27]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 48) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 13) and
    ([.steps[] | select(.op == "deployed")] | length == 13) and
    ([.steps[] | select(.op == "send")] | length == 14) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportTradeEvent")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportInstanceMethodBean")] | length == 9) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid expr-filter-expressions replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-expr-filter-expressions.XXXXXX")
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
# Avro is a common-avro dependency needed by the compiler's event-type
# resolution (GenericData$Record referenced during filter compilation).
"$mvn_bin" -q -f "$esper_root/common-avro/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/common-avro-cp.txt" -Dmdep.includeScope=runtime \
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
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp$cp_sep$(tr -d '\r\n' < "$work/common-avro-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/ExprFilterExpressionsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ExprFilterExpressionsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def listener_rows($case):
        [.records[] | select(.case == $case and .operation == "listener")];
    def deployed_rows($case):
        [.records[] | select(.case == $case and .operation == "deployed")];
    def count_rows($case):
        [.records[] | select(.case == $case and .operation == "count")];
    .version == "esper-parity/v1" and
    .id == "expr-filter-expressions" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 49) and
    ([.records[].operation] | all(. == "listener" or . == "deployed" or . == "count")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # over-in-clause: 2 deployed + 3 listener (s0 fires twice, s1 once).
    ([.records[0:5][] | .case] | all(. == "over-in-clause")) and
    (deployed_rows("over-in-clause") | map(.statement) == ["s0", "s1"]) and
    (listener_rows("over-in-clause") | map(.statement) == ["s0", "s0", "s1"]) and
    (listener_rows("over-in-clause") | all(.new | length == 1)) and
    ([listener_rows("over-in-clause")[].new[0].fields.event1.id] == [1, 2, 2]) and
    # static-func: 8 deployed + 7 listener + 17 listener-not-invoked counts.
    ([.records[5:37][] | .case] | all(. == "static-func")) and
    (deployed_rows("static-func") | map(.statement) == ["s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7"]) and
    (listener_rows("static-func") | length == 7) and
    (listener_rows("static-func") | all(.statement != "s7" and .new[0].fields.theString == "b")) and
    (count_rows("static-func") | length == 17) and
    (count_rows("static-func") | all(.name == "listener-not-invoked" and .count == 0)) and
    ([count_rows("static-func")[] | select(.statement == "s7")] | length == 3) and
    # instance-method-wildcard: 3 deployed + 5 listener + 4 counts.
    ([.records[37:49][] | .case] | all(. == "instance-method-wildcard")) and
    (deployed_rows("instance-method-wildcard") | length == 3) and
    (deployed_rows("instance-method-wildcard") | all(.statement == "s0")) and
    (listener_rows("instance-method-wildcard") | length == 5) and
    ([listener_rows("instance-method-wildcard")[].new[0].fields.x] == [0, 1, 2, 1, 1]) and
    (count_rows("instance-method-wildcard") | length == 4)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid expr-filter-expressions trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
