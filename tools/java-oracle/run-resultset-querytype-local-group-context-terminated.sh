#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-context-terminated.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-context-terminated"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 17/23: context-terminated snapshots over fully aggregated, ungrouped, grouped and locally grouped variants plus aggregate order-by."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-ee681560ebeda52abbb1","java-runtime-377240544dc6ec554ab2"]
    and $s.javaNames == ["ResultSetAggregateFullyVersusNotFullyAgg","ResultSetLocalUngroupedOrderBy"]
    and $s.javaStaticIds == ["java-d7190dd845b29e121075","java-ae42d2957d2e50829f37"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"fully-agg-ungrouped","ordinal":17,"runtimeId":"java-runtime-ee681560ebeda52abbb1","executionName":"ResultSetAggregateFullyVersusNotFullyAgg","observation":"listener","iteratorSnapshots":0,"epl":"@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name(\u0027s0\u0027) context StartS0EndS1 select sum(group_by:(),intPrimitive) as c0 from SupportBean output snapshot when terminated;"},
        {"case":"agg-ungrouped","ordinal":17,"runtimeId":"java-runtime-ee681560ebeda52abbb1","executionName":"ResultSetAggregateFullyVersusNotFullyAgg","observation":"listener","iteratorSnapshots":0,"epl":"@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name(\u0027s0\u0027) context StartS0EndS1 select sum(group_by:theString, intPrimitive) as c0 from SupportBean#keepall output snapshot when terminated;"},
        {"case":"fully-agg-grouped","ordinal":17,"runtimeId":"java-runtime-ee681560ebeda52abbb1","executionName":"ResultSetAggregateFullyVersusNotFullyAgg","observation":"listener","iteratorSnapshots":0,"epl":"@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name(\u0027s0\u0027) context StartS0EndS1 select sum(intPrimitive, group_by:()) as c0, sum(group_by:theString, intPrimitive) as c1, theString from SupportBean group by theString output snapshot when terminated;"},
        {"case":"agg-grouped","ordinal":17,"runtimeId":"java-runtime-ee681560ebeda52abbb1","executionName":"ResultSetAggregateFullyVersusNotFullyAgg","observation":"listener","iteratorSnapshots":0,"epl":"@public create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name(\u0027s0\u0027) context StartS0EndS1 select sum(longPrimitive, group_by:()) as c0, sum(longPrimitive, group_by:theString) as c1,  sum(longPrimitive, group_by:intPrimitive) as c2,  theString from SupportBean#keepall group by theString output snapshot when terminated;"},
        {"case":"ungrouped-order-by","ordinal":23,"runtimeId":"java-runtime-377240544dc6ec554ab2","executionName":"ResultSetLocalUngroupedOrderBy","observation":"listener","iteratorSnapshots":0,"epl":"create context StartS0EndS1 start SupportBean_S0 end SupportBean_S1;@name(\u0027s0\u0027) context StartS0EndS1 select theString, sum(intPrimitive, group_by:theString) as c0  from SupportBean#keepall  output snapshot when terminated order by sum(intPrimitive, group_by:theString);"}
    ]
    and $s.steps == [
        {"op":"case","case":"fully-agg-ungrouped"},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":200}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":30,"longPrimitive":300}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":1}},
        {"op":"case","case":"agg-ungrouped"},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":200}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":30,"longPrimitive":300}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":1}},
        {"op":"case","case":"fully-agg-grouped"},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":200}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":30,"longPrimitive":300}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":1}},
        {"op":"case","case":"agg-grouped"},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":200}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":30,"longPrimitive":300}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":1}},
        {"op":"case","case":"ungrouped-order-by"},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":30,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E3","intPrimitive":40,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":50,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean_S1","payload":{"id":1}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-context-terminated replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-context-terminated.XXXXXX")
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
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ResultSetQueryTypeLocalGroupContextTerminatedScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupContextTerminatedScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-context-terminated"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records | length) == 6
    and ([.records[] | .case] == [
        "fully-agg-ungrouped","fully-agg-ungrouped",
        "agg-ungrouped","fully-agg-grouped","agg-grouped","ungrouped-order-by"])
    and ([.records[] | .operation] | unique == ["listener"])
    and ([.records[] | .statement] | unique == ["s0"])
    and ([.records[] | .sequence] == [1,2,1,1,1,1])
    and ([.records[] | .time] | unique == ["1970-01-01T00:00:00Z"])
    and ([.records[] | has("old")] | any | not)
    and ([.records[] | .new[0].fields | keys_unsorted] == [
        ["c0"],["c0"],["c0"],["c0","c1","theString"],["c0","c1","c2","theString"],["c0","theString"]])
    and ([.records[0:2][] | (.new | length)] == [1,1])
    and (.records[0].new[0].fields.c0 == 60)
    and (.records[1].new[0].fields.c0 == {"state":"null"})
    and ([.records[2].new[] | .fields.c0] == [10,50,50])
    and ([.records[3].new[] | [.fields.c0,.fields.c1,.fields.theString]] == [[60,10,"E1"],[60,50,"E2"]])
    and ([.records[4].new[] | [.fields.c0,.fields.c1,.fields.c2,.fields.theString]] == [
        [600,100,100,"E1"],[600,500,200,"E2"],[600,500,300,"E2"]])
    and ([.records[5].new[] | [.fields.theString,.fields.c0]] == [
        ["E1",40],["E1",40],["E3",40],["E2",70],["E2",70]])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
