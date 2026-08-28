#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-minmax-groupby-join-select-having.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
[ "$actual_commit" = "$expected_commit" ] || {
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
}

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
java_version=$($java_bin -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found ${java_version:-unknown}" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-minmax-groupby-join-select-having" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMaxMinGroupBy.java" and
    .javaRuntimes == ["java-runtime-802aec9425dc772e3aa5", "java-runtime-060f5af73edf420ef45a"] and
    .javaNames == ["ResultSetAggregateMinMaxJoin", "ResultSetAggregateMinNoGroupSelectHaving"] and
    (.cases | type == "array" and length == 2) and
    .cases[0].case == "minmax-join" and
    .cases[0].ordinal == 3 and
    .cases[0].runtimeId == "java-runtime-802aec9425dc772e3aa5" and
    .cases[0].executionName == "ResultSetAggregateMinMaxJoin" and
    .cases[0].observation == "listener" and
    .cases[1].case == "min-no-group-select-having" and
    .cases[1].ordinal == 5 and
    .cases[1].runtimeId == "java-runtime-060f5af73edf420ef45a" and
    .cases[1].executionName == "ResultSetAggregateMinNoGroupSelectHaving" and
    .cases[1].observation == "listener" and
    (.steps | type == "array" and length == 22) and
    .steps[0] == {"op":"case","case":"minmax-join"} and
    .steps[1] == {"op":"send","eventType":"SupportBeanString","payload":{"theString":"DELL"}} and
    .steps[2] == {"op":"send","eventType":"SupportBeanString","payload":{"theString":"IBM"}} and
    .steps[15] == {"op":"case","case":"min-no-group-select-having"} and
    ([.steps[3:15][] | select(.op == "send" and .eventType == "SupportMarketDataBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["price","symbol","volume"]))] | length) == 12 and
    ([.steps[16:22][] | select(.op == "send" and .eventType == "SupportMarketDataBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["price","symbol","volume"]))] | length) == 6 and
    (all(.steps[3:15][]; .payload.price == 0)) and
    (all(.steps[16:22][]; .payload.price == 0))
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-minmax-groupby-join-select-having replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-minmax-groupby-join-select-having.XXXXXX")
cleanup() { rm -rf "$work"; }
trap cleanup EXIT HUP INT TERM
"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ResultSetAggregateMaxMinGroupByJoinSelectHavingScenarioOracle.java"
mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateMaxMinGroupByJoinSelectHavingScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-minmax-groupby-join-select-having" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17.")) and
    (.records | type == "array" and length == 14) and
    ([.records[] | select(.case == "minmax-join" and .operation == "listener" and (.new | type == "array") and (.old | type == "array") and (.new | length) == (.old | length))] | length) == 12 and
    ([.records[] | select(.case == "min-no-group-select-having" and .operation == "listener" and (.new | type == "array" and length == 1) and (.old == null or (.old | length) == 0))] | length) == 2 and
    ([.records[] | select(.case == "minmax-join")] | map(.sequence) | sort) == [1,2,3,4,5,6,7,8,9,10,11,12] and
    ([.records[] | select(.case == "minmax-join")] | sort_by(.sequence) | map([(.new | length), (.old | length)])) == [[1,1],[1,1],[1,1],[1,1],[1,1],[2,2],[2,2],[2,2],[1,1],[1,1],[1,1],[1,1]] and
    ([.records[] | select(.case == "min-no-group-select-having")] | map(.sequence) | sort) == [1,2] and
    (all(.records[]; (.time | startswith("1970-01-01T00:00:00"))))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
