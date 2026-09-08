#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-other-select-expr-stream-selector.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 1; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 1; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 1; }
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
    and $s.id == "epl-other-select-expr-stream-selector"
    and $s.description == "EPLOtherSelectExprStreamSelector alias-with-properties executions: stream-dot notation with alias (theString.* as s0/s1) beside plain property aliases (intPrimitive as a/b) over a length window, and mixed join select with stream-as-object columns (s0stream/s1stream), plain properties (intPrimitive, theString), and aliased columns (symbol as sym) over a length-keepall inner join (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java)."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherSelectExprStreamSelector.java"
    and $s.javaRuntimes == ["java-runtime-89123cf55af0a7f5987e","java-runtime-b53494cb36a6b54c2c6f"]
    and $s.javaNames == ["EPLOtherNoJoinWithAliasWithProperties","EPLOtherJoinWithAliasWithProperties"]
    and $s.javaStaticIds == ["java-b5352434faea014585a8","java-9682b92ad00f7295162d"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 2)
    and $s.cases[0] == {"case":"no-join-alias-props","ordinal":8,"runtimeId":"java-runtime-89123cf55af0a7f5987e","executionName":"EPLOtherNoJoinWithAliasWithProperties","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select theString.* as s0, intPrimitive as a, theString.* as s1, intPrimitive as b from SupportBean#length(3) as theString"}
    and $s.cases[1] == {"case":"join-alias-props","ordinal":9,"runtimeId":"java-runtime-b53494cb36a6b54c2c6f","executionName":"EPLOtherJoinWithAliasWithProperties","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select intPrimitive, s1.* as s1stream, theString, symbol as sym, s0.* as s0stream from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1"}
    and ($s.steps | type == "array" and length == 9)
    and $s.steps[0] == {"op":"case","case":"no-join-alias-props"}
    and $s.steps[1] == {"op":"deploy","case":"no-join-alias-props","statement":"s0","epl":"@name(\u0027s0\u0027) select theString.* as s0, intPrimitive as a, theString.* as s1, intPrimitive as b from SupportBean#length(3) as theString"}
    and $s.steps[2] == {"op":"send","case":"no-join-alias-props","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":12}}
    and $s.steps[3] == {"op":"undeploy-all","case":"no-join-alias-props"}
    and $s.steps[4] == {"op":"case","case":"join-alias-props"}
    and $s.steps[5] == {"op":"deploy","case":"join-alias-props","statement":"s0","epl":"@name(\u0027s0\u0027) select intPrimitive, s1.* as s1stream, theString, symbol as sym, s0.* as s0stream from SupportBean#length(3) as s0, SupportMarketDataBean#keepall as s1"}
    and $s.steps[6] == {"op":"send","case":"join-alias-props","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":13}}
    and $s.steps[7] == {"op":"send","case":"join-alias-props","eventType":"SupportMarketDataBean","payload":{"symbol":"E2","price":0,"volume":0,"feed":""}}
    and $s.steps[8] == {"op":"undeploy-all","case":"join-alias-props"}
    and ([ $s.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all") ] | length) == 0
    and ([ $s.steps[] | select(.op == "undeploy-all") ] | length) == 2
    and ([ $s.steps[] | select(.op == "deploy") ] | length) == 2
    and ([ $s.steps[] | select(.op == "send") ] | length) == 3
    and ([ $s.steps[0:4][] | .case ] | unique) == ["no-join-alias-props"]
    and ([ $s.steps[4:9][] | .case ] | unique) == ["join-alias-props"]
    and ([ $s.steps[] | select(.op == "send") | .eventType ]) == ["SupportBean","SupportBean","SupportMarketDataBean"]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportBean") | [.payload.theString, .payload.intPrimitive] ]) == [["E1",12],["E1",13]]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | .payload.symbol ] | unique) == ["E2"]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | [.payload.price, .payload.volume] ]) == [[0,0]]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | .payload.feed ] | all(. == ""))
    and ([ $s.steps[] | select(.op == "deploy") | .statement ]) == ["s0","s0"]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid epl-other-select-expr-stream-selector replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/common/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/regression-lib/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-other-select-expr-stream-selector.XXXXXX")
cleanup() { rm -rf "$work"; }
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/common-avro/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/avro-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt"):$(tr '\n' ':' < "$work/avro-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/EPLOtherSelectExprStreamSelectorScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLOtherSelectExprStreamSelectorScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "epl-other-select-expr-stream-selector"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | startswith("17."))
    and (.records | type == "array" and length == 2)
    and ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array"))] | length) == 2
    and ([.records[] | select(has("old"))] | length) == 0
    and ([.records[] | select(.case == "no-join-alias-props")] | length) == 1
    and ([.records[] | select(.case == "join-alias-props")] | length) == 1
    and ([.records[] | select(.case != "no-join-alias-props" and .case != "join-alias-props")] | length) == 0
    and ([.records[] | select(.case == "no-join-alias-props") | .sequence]) == [1]
    and ([.records[] | select(.case == "join-alias-props") | .sequence]) == [1]
    and ([.records[] | select(.time != "1970-01-01T00:00:00Z")] | length) == 0
    and .records[0].new[0].fields == {"a":12,"b":12,"s0":{"kind":"row","fields":{"intPrimitive":12,"theString":"E1"}},"s1":{"kind":"row","fields":{"intPrimitive":12,"theString":"E1"}}}
    and .records[1].new[0].fields == {"intPrimitive":13,"s0stream":{"kind":"row","fields":{"intPrimitive":13,"theString":"E1"}},"s1stream":{"kind":"row","fields":{"feed":"","price":0,"symbol":"E2","volume":0}},"sym":"E2","theString":"E1"}
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
