#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-filter-named-parameter.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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

if ! jq -e -s '
    if length != 1 then false else .[0] as $s |
    (($s | keys_unsorted | sort) == ["cases","description","id","javaCommit","javaFlags","javaNames","javaRuntimes","javaSource","javaStaticIds","steps","version"])
    and $s.version == "esper-parity/v1"
    and $s.id == "resultset-aggregate-filter-named-parameter"
    and $s.description == "Named filter aggregate methods replaying ResultSetAggregateFilterNamedParameter executions: leaving, nth, virtual-time rate and timestamp rate."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"
    and $s.javaRuntimes == ["java-runtime-7bc068fcf2ea07b9c1f7", "java-runtime-dd319218418ee0418b21", "java-runtime-4b24ef27ade0eae24258", "java-runtime-3d732054eac8d5b8ba14"]
    and $s.javaNames == ["ResultSetAggregateMethodAggLeaving", "ResultSetAggregateMethodAggNth", "ResultSetAggregateMethodAggRateUnbound", "ResultSetAggregateMethodAggRateBound"]
    and $s.javaStaticIds == ["java-0c29efb6d43971aba5c4"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 4)
    and $s.cases[0] == {"case":"leaving","ordinal":4,"runtimeId":"java-runtime-7bc068fcf2ea07b9c1f7","executionName":"ResultSetAggregateMethodAggLeaving","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select leaving(filter:intPrimitive=1) as c0,leaving(filter:intPrimitive=2) as c1 from SupportBean#length(2)"}
    and $s.cases[1] == {"case":"nth","ordinal":5,"runtimeId":"java-runtime-dd319218418ee0418b21","executionName":"ResultSetAggregateMethodAggNth","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select nth(intPrimitive, 1, filter:theString like \u0027A%\u0027) as c0 from SupportBean"}
    and $s.cases[2] == {"case":"rate-unbound","ordinal":6,"runtimeId":"java-runtime-4b24ef27ade0eae24258","executionName":"ResultSetAggregateMethodAggRateUnbound","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select rate(1, filter:theString like \u0027A%\u0027) as c0 from SupportBean"}
    and $s.cases[3] == {"case":"rate-bound","ordinal":7,"runtimeId":"java-runtime-3d732054eac8d5b8ba14","executionName":"ResultSetAggregateMethodAggRateBound","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select rate(longPrimitive, filter:theString like \u0027A%\u0027) as myrate, rate(longPrimitive, intPrimitive, filter:theString like \u0027A%\u0027) as myqtyrate from SupportBean#length(3)"}
    and ($s.steps | type == "array" and length == 29)
    and $s.steps[0] == {"op":"case","case":"leaving"}
    and $s.steps[5] == {"op":"case","case":"nth"}
    and $s.steps[13] == {"op":"case","case":"rate-unbound"}
    and $s.steps[20] == {"op":"case","case":"rate-bound"}
    and ([ $s.steps[1:5][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","theString"])) ] | length) == 4
    and ([ $s.steps[6:13][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","theString"])) ] | length) == 7
    and ([ $s.steps[14:20][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["theString"])) ] | length) == 5
    and $s.steps[16] == {"op":"advance-time","at":"1970-01-01T00:00:01Z"}
    and ([ $s.steps[21:29][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","longPrimitive","theString"])) ] | length) == 8
    and ([ $s.steps[1:5][] | .payload.theString ]) == ["E1","E2","E3","E4"]
    and ([ $s.steps[1:5][] | .payload.intPrimitive ]) == [2,1,3,4]
    and ([ $s.steps[6:13][] | .payload.theString ]) == ["X1","X2","A3","A4","X3","A5","X4"]
    and ([ $s.steps[6:13][] | .payload.intPrimitive ]) == [0,0,1,2,0,3,0]
    and ([ $s.steps[14:16][] | .payload.theString ]) == ["X1","A1"]
    and ([ $s.steps[17:20][] | .payload.theString ]) == ["X2","A2","A3"]
    and ([ $s.steps[21:29][] | .payload.theString ]) == ["X1","X2","X2","A1","A2","A3","A4","A5"]
    and ([ $s.steps[21:29][] | .payload.longPrimitive ]) == [1000,1200,1300,1000,1200,1300,1500,2000]
    and ([ $s.steps[21:29][] | .payload.intPrimitive ]) == [10,0,0,10,0,0,14,11]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-filter-named-parameter replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-filter-named-parameter.XXXXXX")
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
    "$script_root/ResultSetAggregateFilterNamedParameterScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateFilterNamedParameterScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-aggregate-filter-named-parameter"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | startswith("17."))
    and (.records | type == "array" and length == 24)
    and ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array"))] | length) == 24
    and ([.records[] | select(.case == "leaving")] | length) == 4
    and ([.records[] | select(.case == "nth")] | length) == 7
    and ([.records[] | select(.case == "rate-unbound")] | length) == 5
    and ([.records[] | select(.case == "rate-bound")] | length) == 8
    and ([.records[] | select(.case != "leaving" and .case != "nth" and .case != "rate-unbound" and .case != "rate-bound")] | length) == 0
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
