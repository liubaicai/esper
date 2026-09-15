#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-ungrouped.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-ungrouped"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 3-6: ungrouped local-group aggregates across iterator, listener and statement-metadata observations."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-2116fa46dfb43cd6ec83","java-runtime-1309ac2ab21826013abf","java-runtime-e9dbed674201a7324216","java-runtime-748fbcf754462901d0e0"]
    and $s.javaNames == ["ResultSetLocalUngroupedAggIterator","ResultSetLocalUngroupedParenSODA{soda=false}","ResultSetLocalUngroupedParenSODA{soda=true}","ResultSetLocalUngroupedColNameRendering"]
    and $s.javaStaticIds == ["java-15d21d1c89c691d1be52","java-dac296ac610fa9db8d71","java-4d4e4747a49e26787a4d"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"ungrouped-agg-iterator","ordinal":3,"runtimeId":"java-runtime-2116fa46dfb43cd6ec83","executionName":"ResultSetLocalUngroupedAggIterator","observation":"iterator","iteratorSnapshots":3,"epl":"@name(\u0027s0\u0027) select intPrimitive as c0, sum(intPrimitive, group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1 from SupportBean#keepall"},
        {"case":"ungrouped-paren-soda-text","ordinal":4,"runtimeId":"java-runtime-1309ac2ab21826013abf","executionName":"ResultSetLocalUngroupedParenSODA{soda=false}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select longPrimitive, sum(longPrimitive) as c0, sum(group_by:(),longPrimitive) as c1, sum(longPrimitive,group_by:()) as c2, sum(longPrimitive,group_by:theString) as c3, sum(longPrimitive,group_by:(theString,intPrimitive)) as c4 from SupportBean"},
        {"case":"ungrouped-paren-soda-model","ordinal":5,"runtimeId":"java-runtime-e9dbed674201a7324216","executionName":"ResultSetLocalUngroupedParenSODA{soda=true}","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select longPrimitive, sum(longPrimitive) as c0, sum(group_by:(),longPrimitive) as c1, sum(longPrimitive,group_by:()) as c2, sum(longPrimitive,group_by:theString) as c3, sum(longPrimitive,group_by:(theString,intPrimitive)) as c4 from SupportBean"},
        {"case":"ungrouped-colname-rendering","ordinal":6,"runtimeId":"java-runtime-748fbcf754462901d0e0","executionName":"ResultSetLocalUngroupedColNameRendering","observation":"statement-metadata","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select count(*, group_by:(theString, intPrimitive)), count(group_by:theString, *) from SupportBean"}
    ]
    and $s.steps == [
        {"op":"case","case":"ungrouped-agg-iterator"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"snapshot","statement":"s0","mode":"any"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":0}},
        {"op":"snapshot","statement":"s0","mode":"any"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":30,"longPrimitive":0}},
        {"op":"snapshot","statement":"s0","mode":"any"},
        {"op":"case","case":"ungrouped-paren-soda-text"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":1,"longPrimitive":10}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":2,"longPrimitive":11}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":1,"longPrimitive":12}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":2,"longPrimitive":13}},
        {"op":"case","case":"ungrouped-paren-soda-model"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":1,"longPrimitive":10}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":2,"longPrimitive":11}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":1,"longPrimitive":12}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":2,"longPrimitive":13}},
        {"op":"case","case":"ungrouped-colname-rendering"},
        {"op":"deployed","statement":"s0"},
        {"op":"types","statement":"s0"}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-ungrouped replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-ungrouped.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupByUngroupedScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupByUngroupedScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-ungrouped"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records == [
        {"case":"ungrouped-agg-iterator","operation":"snapshot","statement":"s0","sequence":1,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":10,"sum0":10,"sum1":10}}]},
        {"case":"ungrouped-agg-iterator","operation":"snapshot","statement":"s0","sequence":2,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":10,"sum0":30,"sum1":10}},{"kind":"row","fields":{"c0":20,"sum0":30,"sum1":20}}]},
        {"case":"ungrouped-agg-iterator","operation":"snapshot","statement":"s0","sequence":3,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":10,"sum0":60,"sum1":40}},{"kind":"row","fields":{"c0":20,"sum0":60,"sum1":20}},{"kind":"row","fields":{"c0":30,"sum0":60,"sum1":40}}]},
        {"case":"ungrouped-paren-soda-text","operation":"listener","statement":"s0","sequence":1,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":10,"c1":10,"c2":10,"c3":10,"c4":10,"longPrimitive":10}}]},
        {"case":"ungrouped-paren-soda-text","operation":"listener","statement":"s0","sequence":2,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":21,"c1":21,"c2":21,"c3":21,"c4":11,"longPrimitive":11}}]},
        {"case":"ungrouped-paren-soda-text","operation":"listener","statement":"s0","sequence":3,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":33,"c1":33,"c2":33,"c3":12,"c4":12,"longPrimitive":12}}]},
        {"case":"ungrouped-paren-soda-text","operation":"listener","statement":"s0","sequence":4,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":46,"c1":46,"c2":46,"c3":25,"c4":13,"longPrimitive":13}}]},
        {"case":"ungrouped-paren-soda-model","operation":"listener","statement":"s0","sequence":1,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":10,"c1":10,"c2":10,"c3":10,"c4":10,"longPrimitive":10}}]},
        {"case":"ungrouped-paren-soda-model","operation":"listener","statement":"s0","sequence":2,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":21,"c1":21,"c2":21,"c3":21,"c4":11,"longPrimitive":11}}]},
        {"case":"ungrouped-paren-soda-model","operation":"listener","statement":"s0","sequence":3,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":33,"c1":33,"c2":33,"c3":12,"c4":12,"longPrimitive":12}}]},
        {"case":"ungrouped-paren-soda-model","operation":"listener","statement":"s0","sequence":4,"time":"1970-01-01T00:00:00Z",
         "new":[{"kind":"row","fields":{"c0":46,"c1":46,"c2":46,"c3":25,"c4":13,"longPrimitive":13}}]},
        {"case":"ungrouped-colname-rendering","operation":"deployed","statement":"s0","sequence":0},
        {"case":"ungrouped-colname-rendering","operation":"types","statement":"s0","sequence":0,
         "value":[{"name":"count(*,group_by:(theString,intPrimitive))","type":"Long"},{"name":"count(group_by:theString,*)","type":"Long"}]}
    ])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
