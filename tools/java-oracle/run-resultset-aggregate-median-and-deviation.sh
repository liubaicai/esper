#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-median-and-deviation.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "resultset-aggregate-median-and-deviation" and
    (.description | type == "string") and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMedianAndDeviation.java" and
    .javaRuntimes == ["java-runtime-101db664ea346721e986", "java-runtime-2f533a8a1bc0d93ae649", "java-runtime-2667a7eadeb7a458d7ce"] and
    .javaNames == ["ResultSetAggregateStmt", "ResultSetAggregateStmtJoinOM", "ResultSetAggregateStmtJoin"] and
    .javaStaticIds == ["java-b11b233b0ea7da05217f", "java-66a516ce6e14456627b2", "java-a5c65b307ecdb2d0355b"] and
    (.cases | type == "array" and length == 3) and
    .cases[0].case == "stmt-main" and .cases[0].ordinal == 0 and .cases[0].runtimeId == "java-runtime-101db664ea346721e986" and .cases[0].executionName == "ResultSetAggregateStmt" and .cases[0].observation == "listener" and
    .cases[1].case == "join-om" and .cases[1].ordinal == 1 and .cases[1].runtimeId == "java-runtime-2f533a8a1bc0d93ae649" and .cases[1].executionName == "ResultSetAggregateStmtJoinOM" and .cases[1].observation == "listener" and
    .cases[2].case == "join-epl" and .cases[2].ordinal == 2 and .cases[2].runtimeId == "java-runtime-2667a7eadeb7a458d7ce" and .cases[2].executionName == "ResultSetAggregateStmtJoin" and .cases[2].observation == "listener" and
    (.steps | type == "array" and length == 30) and
    .steps[0] == {"op":"case","case":"stmt-main"} and
    .steps[8] == {"op":"case","case":"join-om"} and
    .steps[19] == {"op":"case","case":"join-epl"} and
    ([.steps[1:8][] | select(.op == "send" and .eventType == "SupportMarketDataBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["price","symbol","volume"]) and .payload.symbol == "DELL" and (.payload.volume | type == "number") and (.payload.volume == 0))] | length) == 7 and
    ([.steps[9:19][] | select(.op == "send")] | length) == 10 and
    ([.steps[20:30][] | select(.op == "send")] | length) == 10 and
    ([.steps[1:8][] | .payload.price]) == [10,20,20,90,5,90,30] and
    ([.steps[12:19][] | .payload.price]) == [10,20,20,90,5,90,30] and
    ([.steps[23:30][] | .payload.price]) == [10,20,20,90,5,90,30] and
    ([.steps[9:12][] | .payload.theString]) == ["DELL","IBM","AAA"] and
    ([.steps[20:23][] | .payload.theString]) == ["DELL","IBM","AAA"]
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-median-and-deviation replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-median-and-deviation.XXXXXX")
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
    "$script_root/ResultSetAggregateMedianAndDeviationScenarioOracle.java"
mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateMedianAndDeviationScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-median-and-deviation" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17.")) and
    (.records | type == "array" and length == 21) and
    ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array" and length == 1) and (.old | type == "array" and length == 1))] | length) == 21 and
    ([.records[] | select(.case == "stmt-main")] | map(.sequence) | sort) == [1,2,3,4,5,6,7] and
    ([.records[] | select(.case == "join-om")] | map(.sequence) | sort) == [1,2,3,4,5,6,7] and
    ([.records[] | select(.case == "join-epl")] | map(.sequence) | sort) == [1,2,3,4,5,6,7] and
    (all(.records[]; (.time | startswith("1970-01-01T00:00:00"))))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
