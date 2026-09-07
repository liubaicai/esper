#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-filter-named-parameter-linear-join.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-aggregate-filter-named-parameter-linear-join"
    and $s.description == "ResultSetAggregateFilterNamedParameter join and mixed-filter gap executions: filtered access aggregates (first, last, window, firstever, lastever, countever) over joined last-event and length-window or keepall streams with JoinField-bound filter predicates, plus mixed-filter and filter-leading window(sb) event-array projections rendered as event rows (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java)."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"
    and $s.javaRuntimes == ["java-runtime-d652fb38c70d8bcc778f", "java-runtime-4add988d1015cdae8bb2", "java-runtime-97101f0ae69f6fad8c29"]
    and $s.javaNames == ["ResultSetAggregateAccessAggLinearBound{join=true}", "ResultSetAggregateAccessAggLinearUnbound{join=true}", "ResultSetAggregateAccessAggLinearBoundMixedFilter"]
    and $s.javaStaticIds == ["java-85b8ba64c1be948a5676", "java-97884ae57325e3acd3f2", "java-9d0ccec4c0bb19e6a14c"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 3)
    and $s.cases[0] == {"case":"linear-join-bound","ordinal":9,"runtimeId":"java-runtime-d652fb38c70d8bcc778f","executionName":"ResultSetAggregateAccessAggLinearBound{join=true}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select first(intPrimitive, filter:theString like \u0027A%\u0027) as aFirst,last(intPrimitive, filter:theString like \u0027A%\u0027) as aLast,window(intPrimitive, filter:theString like \u0027A%\u0027) as aWindow,first(intPrimitive, filter:theString like \u0027B%\u0027) as bFirst,last(intPrimitive, filter:theString like \u0027B%\u0027) as bLast,window(intPrimitive, filter:theString like \u0027B%\u0027) as bWindow from SupportBean_S1#lastevent, SupportBean#length(5)"}
    and $s.cases[1] == {"case":"linear-join-unbound","ordinal":11,"runtimeId":"java-runtime-4add988d1015cdae8bb2","executionName":"ResultSetAggregateAccessAggLinearUnbound{join=true}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select first(intPrimitive, filter:theString like \u0027A%\u0027) as aFirst,firstever(intPrimitive, filter:theString like \u0027A%\u0027) as aFirstever,last(intPrimitive, filter:theString like \u0027A%\u0027) as aLast,lastever(intPrimitive, filter:theString like \u0027A%\u0027) as aLastever,countever(intPrimitive, filter:theString like \u0027A%\u0027) as aCountever from SupportBean_S1#lastevent, SupportBean#keepall"}
    and $s.cases[2] == {"case":"mixed-filter-window","ordinal":13,"runtimeId":"java-runtime-97101f0ae69f6fad8c29","executionName":"ResultSetAggregateAccessAggLinearBoundMixedFilter","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select window(sb, filter:theString like \u0027A%\u0027) as c0,window(sb) as c1,window(filter:theString like \u0027B%\u0027, sb) as c2 from SupportBean#keepall as sb"}
    and ($s.steps | type == "array" and length == 30)
    and $s.steps[0] == {"op":"case","case":"linear-join-bound"}
    and $s.steps[1] == {"op":"deploy","case":"linear-join-bound","statement":"s0","epl":"@name(\u0027s0\u0027) select first(intPrimitive, filter:theString like \u0027A%\u0027) as aFirst,last(intPrimitive, filter:theString like \u0027A%\u0027) as aLast,window(intPrimitive, filter:theString like \u0027A%\u0027) as aWindow,first(intPrimitive, filter:theString like \u0027B%\u0027) as bFirst,last(intPrimitive, filter:theString like \u0027B%\u0027) as bLast,window(intPrimitive, filter:theString like \u0027B%\u0027) as bWindow from SupportBean_S1#lastevent, SupportBean#length(5)"}
    and $s.steps[2] == {"op":"send","case":"linear-join-bound","eventType":"SupportBean_S1","payload":{"id":0}}
    and $s.steps[14] == {"op":"undeploy-all","case":"linear-join-bound"}
    and $s.steps[15] == {"op":"case","case":"linear-join-unbound"}
    and $s.steps[16] == {"op":"deploy","case":"linear-join-unbound","statement":"s0","epl":"@name(\u0027s0\u0027) select first(intPrimitive, filter:theString like \u0027A%\u0027) as aFirst,firstever(intPrimitive, filter:theString like \u0027A%\u0027) as aFirstever,last(intPrimitive, filter:theString like \u0027A%\u0027) as aLast,lastever(intPrimitive, filter:theString like \u0027A%\u0027) as aLastever,countever(intPrimitive, filter:theString like \u0027A%\u0027) as aCountever from SupportBean_S1#lastevent, SupportBean#keepall"}
    and $s.steps[17] == {"op":"send","case":"linear-join-unbound","eventType":"SupportBean_S1","payload":{"id":0}}
    and $s.steps[23] == {"op":"undeploy-all","case":"linear-join-unbound"}
    and $s.steps[24] == {"op":"case","case":"mixed-filter-window"}
    and $s.steps[25] == {"op":"deploy","case":"mixed-filter-window","statement":"s0","epl":"@name(\u0027s0\u0027) select window(sb, filter:theString like \u0027A%\u0027) as c0,window(sb) as c1,window(filter:theString like \u0027B%\u0027, sb) as c2 from SupportBean#keepall as sb"}
    and $s.steps[29] == {"op":"undeploy-all","case":"mixed-filter-window"}
    and ([ $s.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all") ] | length) == 0
    and ([ $s.steps[0:15][] | .case ] | unique) == ["linear-join-bound"]
    and ([ $s.steps[15:24][] | .case ] | unique) == ["linear-join-unbound"]
    and ([ $s.steps[24:30][] | .case ] | unique) == ["mixed-filter-window"]
    and ([ $s.steps[3:14][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","theString"])) ] | length) == 11
    and ([ $s.steps[18:23][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","theString"])) ] | length) == 5
    and ([ $s.steps[26:29][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["intPrimitive","theString"])) ] | length) == 3
    and ([ $s.steps[3:14][] | .payload.theString ]) == ["X1","B2","B3","A4","B5","A6","X2","X3","X4","X5","X6"]
    and ([ $s.steps[3:14][] | .payload.intPrimitive ]) == [1,2,3,4,5,6,7,8,9,10,11]
    and ([ $s.steps[18:23][] | .payload.theString ]) == ["X0","A1","X2","A3","X4"]
    and ([ $s.steps[18:23][] | .payload.intPrimitive ]) == [0,1,2,3,4]
    and ([ $s.steps[26:29][] | .payload.theString ]) == ["X1","A2","B3"]
    and ([ $s.steps[26:29][] | .payload.intPrimitive ]) == [1,2,3]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-filter-named-parameter-linear-join replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-filter-named-parameter-linear-join.XXXXXX")
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
    "$script_root/ResultSetAggregateFilterNamedParameterLinearJoinScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateFilterNamedParameterLinearJoinScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-aggregate-filter-named-parameter-linear-join"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | startswith("17."))
    and (.records | type == "array" and length == 19)
    and ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array"))] | length) == 19
    and ([.records[] | select(has("old"))] | length) == 0
    and ([.records[] | select(.case == "linear-join-bound")] | length) == 11
    and ([.records[] | select(.case == "linear-join-unbound")] | length) == 5
    and ([.records[] | select(.case == "mixed-filter-window")] | length) == 3
    and ([.records[] | select(.case != "linear-join-bound" and .case != "linear-join-unbound" and .case != "mixed-filter-window")] | length) == 0
    and ([.records[] | select(.case == "linear-join-bound") | .sequence]) == [range(1; 12)]
    and ([.records[] | select(.case == "linear-join-unbound") | .sequence]) == [range(1; 6)]
    and ([.records[] | select(.case == "mixed-filter-window") | .sequence]) == [range(1; 4)]
    and ([.records[] | select(.time != "1970-01-01T00:00:00Z")] | length) == 0
    and (.records[0].new[0].fields | keys_unsorted | sort) == ["aFirst","aLast","aWindow","bFirst","bLast","bWindow"]
    and (.records[11].new[0].fields | keys_unsorted | sort) == ["aCountever","aFirst","aFirstever","aLast","aLastever"]
    and (.records[16].new[0].fields | keys_unsorted | sort) == ["c0","c1","c2"]
    and .records[0].new[0].fields == {"aFirst":{"state":"null"},"aLast":{"state":"null"},"aWindow":{"state":"null"},"bFirst":{"state":"null"},"bLast":{"state":"null"},"bWindow":{"state":"null"}}
    and .records[1].new[0].fields == {"aFirst":{"state":"null"},"aLast":{"state":"null"},"aWindow":{"state":"null"},"bFirst":2,"bLast":2,"bWindow":[2]}
    and .records[2].new[0].fields == {"aFirst":{"state":"null"},"aLast":{"state":"null"},"aWindow":{"state":"null"},"bFirst":2,"bLast":3,"bWindow":[2,3]}
    and .records[3].new[0].fields == {"aFirst":4,"aLast":4,"aWindow":[4],"bFirst":2,"bLast":3,"bWindow":[2,3]}
    and .records[4].new[0].fields == {"aFirst":4,"aLast":4,"aWindow":[4],"bFirst":2,"bLast":5,"bWindow":[2,3,5]}
    and .records[5].new[0].fields == {"aFirst":4,"aLast":6,"aWindow":[4,6],"bFirst":2,"bLast":5,"bWindow":[2,3,5]}
    and .records[6].new[0].fields == {"aFirst":4,"aLast":6,"aWindow":[4,6],"bFirst":3,"bLast":5,"bWindow":[3,5]}
    and .records[7].new[0].fields == {"aFirst":4,"aLast":6,"aWindow":[4,6],"bFirst":5,"bLast":5,"bWindow":[5]}
    and .records[8].new[0].fields == {"aFirst":6,"aLast":6,"aWindow":[6],"bFirst":5,"bLast":5,"bWindow":[5]}
    and .records[9].new[0].fields == {"aFirst":6,"aLast":6,"aWindow":[6],"bFirst":{"state":"null"},"bLast":{"state":"null"},"bWindow":{"state":"null"}}
    and .records[10].new[0].fields == {"aFirst":{"state":"null"},"aLast":{"state":"null"},"aWindow":{"state":"null"},"bFirst":{"state":"null"},"bLast":{"state":"null"},"bWindow":{"state":"null"}}
    and .records[11].new[0].fields == {"aCountever":0,"aFirst":{"state":"null"},"aFirstever":{"state":"null"},"aLast":{"state":"null"},"aLastever":{"state":"null"}}
    and .records[12].new[0].fields == {"aCountever":1,"aFirst":1,"aFirstever":1,"aLast":1,"aLastever":1}
    and .records[13].new[0].fields == {"aCountever":1,"aFirst":1,"aFirstever":1,"aLast":1,"aLastever":1}
    and .records[14].new[0].fields == {"aCountever":2,"aFirst":1,"aFirstever":1,"aLast":3,"aLastever":3}
    and .records[15].new[0].fields == {"aCountever":2,"aFirst":1,"aFirstever":1,"aLast":3,"aLastever":3}
    and .records[16].new[0].fields == {"c0":{"state":"null"},"c1":[{"kind":"row","fields":{"intPrimitive":1,"theString":"X1"}}],"c2":{"state":"null"}}
    and .records[17].new[0].fields == {"c0":[{"kind":"row","fields":{"intPrimitive":2,"theString":"A2"}}],"c1":[{"kind":"row","fields":{"intPrimitive":1,"theString":"X1"}},{"kind":"row","fields":{"intPrimitive":2,"theString":"A2"}}],"c2":{"state":"null"}}
    and .records[18].new[0].fields == {"c0":[{"kind":"row","fields":{"intPrimitive":2,"theString":"A2"}}],"c1":[{"kind":"row","fields":{"intPrimitive":1,"theString":"X1"}},{"kind":"row","fields":{"intPrimitive":2,"theString":"A2"}},{"kind":"row","fields":{"intPrimitive":3,"theString":"B3"}}],"c2":[{"kind":"row","fields":{"intPrimitive":3,"theString":"B3"}}]}
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
