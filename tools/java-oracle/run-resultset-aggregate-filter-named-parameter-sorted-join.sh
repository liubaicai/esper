#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-filter-named-parameter-sorted-join.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-aggregate-filter-named-parameter-sorted-join"
    and $s.description == "ResultSetAggregateFilterNamedParameter sorted-aggregate join executions: maxby/minby/maxbyever/minbyever with theString value projections and sorted event-array columns under named filter parameters over a last-event join with length(4) or keepall SupportBean windows, plus two-key sorted(intPrimitive, doublePrimitive) multicriteria ordering with null rendered for empty filtered sets (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java)."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateFilterNamedParameter.java"
    and $s.javaRuntimes == ["java-runtime-6b6e0d2290261cb8cd93", "java-runtime-236d99a9d77ed3932510", "java-runtime-398a780be4d755650285"]
    and $s.javaNames == ["ResultSetAggregateAccessAggSortedBound{join=true}", "ResultSetAggregateAccessAggSortedUnbound{join=true}", "ResultSetAggregateAccessAggSortedMulticriteria"]
    and $s.javaStaticIds == ["java-ea6830fd215ba36a098b", "java-d726134be3446919675e", "java-a26ba8e7bb8f4e6c9f2d"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 3)
    and $s.cases[0] == {"case":"sorted-join-bound","ordinal":15,"runtimeId":"java-runtime-6b6e0d2290261cb8cd93","executionName":"ResultSetAggregateAccessAggSortedBound{join=true}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select maxby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxby,minby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinby,sorted(intPrimitive, filter:theString like \u0027A%\u0027) as aSorted,maxby(intPrimitive, filter:theString like \u0027B%\u0027).theString as bMaxby,minby(intPrimitive, filter:theString like \u0027B%\u0027).theString as bMinby,sorted(intPrimitive, filter:theString like \u0027B%\u0027) as bSorted from SupportBean_S1#lastevent, SupportBean#length(4)"}
    and $s.cases[1] == {"case":"sorted-join-unbound","ordinal":17,"runtimeId":"java-runtime-236d99a9d77ed3932510","executionName":"ResultSetAggregateAccessAggSortedUnbound{join=true}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select maxby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxby,maxbyever(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxbyever,minby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinby,minbyever(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinbyever from SupportBean_S1#lastevent, SupportBean#keepall"}
    and $s.cases[2] == {"case":"sorted-multicriteria","ordinal":18,"runtimeId":"java-runtime-398a780be4d755650285","executionName":"ResultSetAggregateAccessAggSortedMulticriteria","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select sorted(intPrimitive, doublePrimitive, filter:theString like \u0027A%\u0027) as aSorted,sorted(intPrimitive, doublePrimitive, filter:theString like \u0027B%\u0027) as bSorted from SupportBean#keepall"}
    and ($s.steps | type == "array" and length == 29)
    and $s.steps[0] == {"op":"case","case":"sorted-join-bound"}
    and $s.steps[1] == {"op":"deploy","case":"sorted-join-bound","statement":"s0","epl":"@name(\u0027s0\u0027) select maxby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxby,minby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinby,sorted(intPrimitive, filter:theString like \u0027A%\u0027) as aSorted,maxby(intPrimitive, filter:theString like \u0027B%\u0027).theString as bMaxby,minby(intPrimitive, filter:theString like \u0027B%\u0027).theString as bMinby,sorted(intPrimitive, filter:theString like \u0027B%\u0027) as bSorted from SupportBean_S1#lastevent, SupportBean#length(4)"}
    and $s.steps[2] == {"op":"send","case":"sorted-join-bound","eventType":"SupportBean_S1","payload":{"id":0,"doublePrimitive":-1}}
    and $s.steps[12] == {"op":"undeploy-all","case":"sorted-join-bound"}
    and $s.steps[13] == {"op":"case","case":"sorted-join-unbound"}
    and $s.steps[14] == {"op":"deploy","case":"sorted-join-unbound","statement":"s0","epl":"@name(\u0027s0\u0027) select maxby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxby,maxbyever(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMaxbyever,minby(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinby,minbyever(intPrimitive, filter:theString like \u0027A%\u0027).theString as aMinbyever from SupportBean_S1#lastevent, SupportBean#keepall"}
    and $s.steps[15] == {"op":"send","case":"sorted-join-unbound","eventType":"SupportBean_S1","payload":{"id":0,"doublePrimitive":-1}}
    and $s.steps[21] == {"op":"undeploy-all","case":"sorted-join-unbound"}
    and $s.steps[22] == {"op":"case","case":"sorted-multicriteria"}
    and $s.steps[23] == {"op":"deploy","case":"sorted-multicriteria","statement":"s0","epl":"@name(\u0027s0\u0027) select sorted(intPrimitive, doublePrimitive, filter:theString like \u0027A%\u0027) as aSorted,sorted(intPrimitive, doublePrimitive, filter:theString like \u0027B%\u0027) as bSorted from SupportBean#keepall"}
    and $s.steps[28] == {"op":"undeploy-all","case":"sorted-multicriteria"}
    and ([ $s.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all") ] | length) == 0
    and ([ $s.steps[0:13][] | .case ] | unique) == ["sorted-join-bound"]
    and ([ $s.steps[13:22][] | .case ] | unique) == ["sorted-join-unbound"]
    and ([ $s.steps[22:29][] | .case ] | unique) == ["sorted-multicriteria"]
    and ([ $s.steps[] | select(.op == "deploy") | .statement ] | unique) == ["s0"]
    and ([ $s.steps[] | select(.op == "deploy") ] | length) == 3
    and ([ $s.steps[] | select(.op == "undeploy-all") ] | length) == 3
    and ([ $s.steps[3:12][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["doublePrimitive","intPrimitive","theString"])) ] | length) == 9
    and ([ $s.steps[16:21][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["doublePrimitive","intPrimitive","theString"])) ] | length) == 5
    and ([ $s.steps[24:28][] | select(.op == "send" and .eventType == "SupportBean" and (.payload | type == "object") and ((.payload | keys_unsorted | sort) == ["doublePrimitive","intPrimitive","theString"])) ] | length) == 4
    and ([ $s.steps[3:12][] | .payload.theString ]) == ["B1","A10","B2","A5","A15","X3","X4","X5","X6"]
    and ([ $s.steps[3:12][] | .payload.intPrimitive ]) == [1,10,2,5,15,3,4,5,6]
    and ([ $s.steps[3:12][] | .payload.doublePrimitive ]) == [-1,-1,-1,-1,-1,-1,-1,-1,-1]
    and ([ $s.steps[16:21][] | .payload.theString ]) == ["B1","A10","A5","A15","B1000"]
    and ([ $s.steps[16:21][] | .payload.intPrimitive ]) == [1,10,5,15,1000]
    and ([ $s.steps[16:21][] | .payload.doublePrimitive ]) == [-1,-1,-1,-1,-1]
    and ([ $s.steps[24:28][] | .payload.theString ]) == ["B1","A1","B2","A2"]
    and ([ $s.steps[24:28][] | .payload.intPrimitive ]) == [1,100,1,100]
    and ([ $s.steps[24:28][] | .payload.doublePrimitive ]) == [10,2,4,3]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-filter-named-parameter-sorted-join replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-filter-named-parameter-sorted-join.XXXXXX")
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
    "$script_root/ResultSetAggregateFilterNamedParameterSortedJoinScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateFilterNamedParameterSortedJoinScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-aggregate-filter-named-parameter-sorted-join"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | startswith("17."))
    and (.records | type == "array" and length == 18)
    and ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array"))] | length) == 18
    and ([.records[] | select(has("old"))] | length) == 0
    and ([.records[] | select(.case == "sorted-join-bound")] | length) == 9
    and ([.records[] | select(.case == "sorted-join-unbound")] | length) == 5
    and ([.records[] | select(.case == "sorted-multicriteria")] | length) == 4
    and ([.records[] | select(.case != "sorted-join-bound" and .case != "sorted-join-unbound" and .case != "sorted-multicriteria")] | length) == 0
    and ([.records[] | select(.case == "sorted-join-bound") | .sequence]) == [range(1; 10)]
    and ([.records[] | select(.case == "sorted-join-unbound") | .sequence]) == [range(1; 6)]
    and ([.records[] | select(.case == "sorted-multicriteria") | .sequence]) == [range(1; 5)]
    and ([.records[] | select(.time != "1970-01-01T00:00:00Z")] | length) == 0
    and (.records[0].new[0].fields | keys_unsorted | sort) == ["aMaxby","aMinby","aSorted","bMaxby","bMinby","bSorted"]
    and (.records[9].new[0].fields | keys_unsorted | sort) == ["aMaxby","aMaxbyever","aMinby","aMinbyever"]
    and (.records[14].new[0].fields | keys_unsorted | sort) == ["aSorted","bSorted"]
    and .records[0].new[0].fields == {"aMaxby":{"state":"null"},"aMinby":{"state":"null"},"aSorted":{"state":"null"},"bMaxby":"B1","bMinby":"B1","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":1,"theString":"B1"}}]}
    and .records[1].new[0].fields == {"aMaxby":"A10","aMinby":"A10","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":10,"theString":"A10"}}],"bMaxby":"B1","bMinby":"B1","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":1,"theString":"B1"}}]}
    and .records[2].new[0].fields == {"aMaxby":"A10","aMinby":"A10","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":10,"theString":"A10"}}],"bMaxby":"B2","bMinby":"B1","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":1,"theString":"B1"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":2,"theString":"B2"}}]}
    and .records[3].new[0].fields == {"aMaxby":"A10","aMinby":"A5","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":5,"theString":"A5"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":10,"theString":"A10"}}],"bMaxby":"B2","bMinby":"B1","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":1,"theString":"B1"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":2,"theString":"B2"}}]}
    and .records[4].new[0].fields == {"aMaxby":"A15","aMinby":"A5","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":5,"theString":"A5"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":10,"theString":"A10"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":15,"theString":"A15"}}],"bMaxby":"B2","bMinby":"B2","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":2,"theString":"B2"}}]}
    and .records[5].new[0].fields == {"aMaxby":"A15","aMinby":"A5","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":5,"theString":"A5"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":15,"theString":"A15"}}],"bMaxby":"B2","bMinby":"B2","bSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":2,"theString":"B2"}}]}
    and .records[6].new[0].fields == {"aMaxby":"A15","aMinby":"A5","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":5,"theString":"A5"}},{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":15,"theString":"A15"}}],"bMaxby":{"state":"null"},"bMinby":{"state":"null"},"bSorted":{"state":"null"}}
    and .records[7].new[0].fields == {"aMaxby":"A15","aMinby":"A15","aSorted":[{"kind":"row","fields":{"doublePrimitive":-1,"intPrimitive":15,"theString":"A15"}}],"bMaxby":{"state":"null"},"bMinby":{"state":"null"},"bSorted":{"state":"null"}}
    and .records[8].new[0].fields == {"aMaxby":{"state":"null"},"aMinby":{"state":"null"},"aSorted":{"state":"null"},"bMaxby":{"state":"null"},"bMinby":{"state":"null"},"bSorted":{"state":"null"}}
    and .records[9].new[0].fields == {"aMaxby":{"state":"null"},"aMaxbyever":{"state":"null"},"aMinby":{"state":"null"},"aMinbyever":{"state":"null"}}
    and .records[10].new[0].fields == {"aMaxby":"A10","aMaxbyever":"A10","aMinby":"A10","aMinbyever":"A10"}
    and .records[11].new[0].fields == {"aMaxby":"A10","aMaxbyever":"A10","aMinby":"A5","aMinbyever":"A5"}
    and .records[12].new[0].fields == {"aMaxby":"A15","aMaxbyever":"A15","aMinby":"A5","aMinbyever":"A5"}
    and .records[13].new[0].fields == {"aMaxby":"A15","aMaxbyever":"A15","aMinby":"A5","aMinbyever":"A5"}
    and .records[14].new[0].fields == {"aSorted":{"state":"null"},"bSorted":[{"kind":"row","fields":{"doublePrimitive":10,"intPrimitive":1,"theString":"B1"}}]}
    and .records[15].new[0].fields == {"aSorted":[{"kind":"row","fields":{"doublePrimitive":2,"intPrimitive":100,"theString":"A1"}}],"bSorted":[{"kind":"row","fields":{"doublePrimitive":10,"intPrimitive":1,"theString":"B1"}}]}
    and .records[16].new[0].fields == {"aSorted":[{"kind":"row","fields":{"doublePrimitive":2,"intPrimitive":100,"theString":"A1"}}],"bSorted":[{"kind":"row","fields":{"doublePrimitive":4,"intPrimitive":1,"theString":"B2"}},{"kind":"row","fields":{"doublePrimitive":10,"intPrimitive":1,"theString":"B1"}}]}
    and .records[17].new[0].fields == {"aSorted":[{"kind":"row","fields":{"doublePrimitive":2,"intPrimitive":100,"theString":"A1"}},{"kind":"row","fields":{"doublePrimitive":3,"intPrimitive":100,"theString":"A2"}}],"bSorted":[{"kind":"row","fields":{"doublePrimitive":4,"intPrimitive":1,"theString":"B2"}},{"kind":"row","fields":{"doublePrimitive":10,"intPrimitive":1,"theString":"B1"}}]}
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
