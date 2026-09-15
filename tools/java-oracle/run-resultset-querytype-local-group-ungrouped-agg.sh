#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-ungrouped-agg.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-ungrouped-agg"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 0/1/2/7: ungrouped local-group sums, SQL-standard aggregate selections, event-valued local aggregates and local-group HAVING."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-a40ad8ec1c03959b5f33","java-runtime-6bec44d03e954b1cd52c","java-runtime-9ad9539a7e8f5581b192","java-runtime-1dc3599a7c07036be603"]
    and $s.javaNames == ["ResultSetLocalUngroupedSumSimple","ResultSetLocalUngroupedAggSQLStandard","ResultSetLocalUngroupedAggEvent","ResultSetLocalUngroupedHaving"]
    and $s.javaStaticIds == ["java-3feeb6f7d6769e71aad1","java-d2a1aaee0aaf06dee915","java-79a9c055e0f90284c89e","java-ee7ae61064ad0a2279ed"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"ungrouped-sum-simple","ordinal":0,"runtimeId":"java-runtime-a40ad8ec1c03959b5f33","executionName":"ResultSetLocalUngroupedSumSimple","observation":"listener","iteratorSnapshots":0,"epl":"@Name(\u0027s0\u0027) select sum(longPrimitive, group_by:(theString, intPrimitive)) as c0, sum(longPrimitive, group_by:(theString)) as c1, sum(longPrimitive, group_by:(intPrimitive)) as c2, sum(longPrimitive) as c3 from SupportBean"},
        {"case":"ungrouped-agg-sql-standard","ordinal":1,"runtimeId":"java-runtime-6bec44d03e954b1cd52c","executionName":"ResultSetLocalUngroupedAggSQLStandard","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select intPrimitive as c0, sum(intPrimitive, group_by:()) as sum0, sum(intPrimitive, group_by:(theString)) as sum1,avedev(intPrimitive, group_by:(theString)) as avedev0,avg(intPrimitive, group_by:(theString)) as avg0,max(intPrimitive, group_by:(theString)) as max0,fmax(intPrimitive, intPrimitive>0, group_by:(theString)) as fmax0,min(intPrimitive, group_by:(theString)) as min0,fmin(intPrimitive, intPrimitive>0, group_by:(theString)) as fmin0,maxever(intPrimitive, group_by:(theString)) as maxever0,fmaxever(intPrimitive, intPrimitive>0, group_by:(theString)) as fmaxever0,minever(intPrimitive, group_by:(theString)) as minever0,fminever(intPrimitive, intPrimitive>0, group_by:(theString)) as fminever0,median(intPrimitive, group_by:(theString)) as median0,Math.round(coalesce(stddev(intPrimitive, group_by:(theString)), 0)) as stddev0 from SupportBean#keepall"},
        {"case":"ungrouped-agg-event","ordinal":2,"runtimeId":"java-runtime-9ad9539a7e8f5581b192","executionName":"ResultSetLocalUngroupedAggEvent","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select intPrimitive as c0, first(sb, group_by:(theString)) as first0, first(sb, group_by:()) as first1, last(sb, group_by:(theString)) as last0, last(sb, group_by:()) as last1, window(sb, group_by:(theString)) as window0, window(sb, group_by:()) as window1, maxby(intPrimitive, group_by:(theString)) as maxby0, maxby(intPrimitive, group_by:()) as maxby1, minby(intPrimitive, group_by:(theString)) as minby0, minby(intPrimitive, group_by:()) as minby1, sorted(intPrimitive, group_by:(theString)) as sorted0, sorted(intPrimitive, group_by:()) as sorted1, maxbyever(intPrimitive, group_by:(theString)) as maxbyever0, maxbyever(intPrimitive, group_by:()) as maxbyever1, minbyever(intPrimitive, group_by:(theString)) as minbyever0, minbyever(intPrimitive, group_by:()) as minbyever1, firstever(sb, group_by:(theString)) as firstever0, firstever(sb, group_by:()) as firstever1, lastever(sb, group_by:(theString)) as lastever0, lastever(sb, group_by:()) as lastever1 from SupportBean#length(3) as sb"},
        {"case":"ungrouped-having","ordinal":7,"runtimeId":"java-runtime-1dc3599a7c07036be603","executionName":"ResultSetLocalUngroupedHaving","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select * from SupportBean having sum(intPrimitive, group_by:theString) > 100"}
    ]
    and $s.steps == [
        {"op":"case","case":"ungrouped-sum-simple"},
        {"op":"advance-time","at":"1970-01-01T00:00:00Z"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":1,"longPrimitive":10}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":2,"longPrimitive":11}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":2,"longPrimitive":12}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":1,"longPrimitive":13}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":1,"longPrimitive":14}},
        {"op":"case","case":"ungrouped-agg-sql-standard"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":30,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":40,"longPrimitive":0}},
        {"op":"case","case":"ungrouped-agg-event"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":15,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E3","intPrimitive":16,"longPrimitive":0}},
        {"op":"case","case":"ungrouped-having"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":95,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":10,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-ungrouped-agg replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-ungrouped-agg.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupUngroupedAggScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupUngroupedAggScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-ungrouped-agg"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records | length) == 14
    and ([.records[] | .case] == [
        "ungrouped-sum-simple","ungrouped-sum-simple","ungrouped-sum-simple","ungrouped-sum-simple",
        "ungrouped-sum-simple",
        "ungrouped-agg-sql-standard","ungrouped-agg-sql-standard","ungrouped-agg-sql-standard",
        "ungrouped-agg-sql-standard",
        "ungrouped-agg-event","ungrouped-agg-event","ungrouped-agg-event","ungrouped-agg-event",
        "ungrouped-having"])
    and ([.records[] | .operation] | unique == ["listener"])
    and ([.records[] | .statement] | unique == ["s0"])
    and ([.records[] | .sequence] == [1,2,3,4,5,1,2,3,4,1,2,3,4,1])
    and ([.records[] | .time] | unique == ["1970-01-01T00:00:00Z"])
    and ([.records[] | (.new | length)] | unique == [1])
    and ([.records[] | has("old")] | any | not)
    and ([.records[0:5][] | (.new[0].fields | keys_unsorted)] | unique == [["c0","c1","c2","c3"]])
    and ([.records[0:5][] | [.new[0].fields.c0, .new[0].fields.c1, .new[0].fields.c2, .new[0].fields.c3]] == [
        [10,10,10,10],
        [11,11,11,21],
        [12,22,23,33],
        [23,35,23,46],
        [14,25,37,60]])
    and ([.records[5:9][] | (.new[0].fields | keys_unsorted)] | unique == [
        ["avedev0","avg0","c0","fmax0","fmaxever0","fmin0","fminever0","max0","maxever0","median0","min0",
         "minever0","stddev0","sum0","sum1"]])
    and ([.records[5:9][] | [.new[0].fields.c0, .new[0].fields.sum0, .new[0].fields.sum1, .new[0].fields.avedev0,
        .new[0].fields.avg0, .new[0].fields.max0, .new[0].fields.fmax0, .new[0].fields.min0, .new[0].fields.fmin0,
        .new[0].fields.maxever0, .new[0].fields.fmaxever0, .new[0].fields.minever0, .new[0].fields.fminever0,
        .new[0].fields.median0, .new[0].fields.stddev0]] == [
        [10,10,10,0.0,10.0,10,10,10,10,10,10,10,10,10.0,0],
        [20,30,20,0.0,20.0,20,20,20,20,20,20,20,20,20.0,0],
        [30,60,40,10.0,20.0,30,30,10,10,30,30,10,10,20.0,14],
        [40,100,60,10.0,30.0,40,40,20,20,40,40,20,20,30.0,14]])
    and ([.records[9:13][] | (.new[0].fields | keys_unsorted)] | unique == [
        ["c0","first0","first1","firstever0","firstever1","last0","last1","lastever0","lastever1","maxby0","maxby1",
         "maxbyever0","maxbyever1","minby0","minby1","minbyever0","minbyever1","sorted0","sorted1","window0",
         "window1"]])
    and ([.records[9:13][] | .new[0].fields.c0] == [10,20,15,16])
    and ([.records[9:13][] | .new[0].fields as $f | [$f.first0, $f.first1, $f.last0, $f.last1, $f.maxby0, $f.maxby1,
        $f.minby0, $f.minby1, $f.maxbyever0, $f.maxbyever1, $f.minbyever0, $f.minbyever1, $f.firstever0, $f.firstever1,
        $f.lastever0, $f.lastever1] | map([.fields.theString, .fields.intPrimitive])] == [
        [["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],
         ["E1",10],["E1",10],["E1",10],["E1",10],["E1",10],["E1",10]],
        [["E2",20],["E1",10],["E2",20],["E2",20],["E2",20],["E2",20],["E2",20],["E1",10],["E2",20],["E2",20],
         ["E2",20],["E1",10],["E2",20],["E1",10],["E2",20],["E2",20]],
        [["E1",10],["E1",10],["E1",15],["E1",15],["E1",15],["E2",20],["E1",10],["E1",10],["E1",15],["E2",20],
         ["E1",10],["E1",10],["E1",10],["E1",10],["E1",15],["E1",15]],
        [["E3",16],["E2",20],["E3",16],["E3",16],["E3",16],["E2",20],["E3",16],["E1",15],["E3",16],["E2",20],
         ["E3",16],["E1",10],["E3",16],["E1",10],["E3",16],["E3",16]]])
    and ([.records[9:13][] | .new[0].fields as $f | [$f.sorted0, $f.sorted1, $f.window0, $f.window1]
        | map(map([.fields.theString, .fields.intPrimitive]))] == [
        [[["E1",10]],[["E1",10]],[["E1",10]],[["E1",10]]],
        [[["E2",20]],[["E1",10],["E2",20]],[["E2",20]],[["E1",10],["E2",20]]],
        [[["E1",10],["E1",15]],[["E1",10],["E1",15],["E2",20]],[["E1",10],["E1",15]],
         [["E1",10],["E2",20],["E1",15]]],
        [[["E3",16]],[["E1",15],["E3",16],["E2",20]],[["E3",16]],[["E2",20],["E1",15],["E3",16]]]])
    and ([.records[9:13][] | .new[0].fields as $f
        | ([$f.first0, $f.first1, $f.last0, $f.last1, $f.maxby0, $f.maxby1, $f.minby0, $f.minby1, $f.maxbyever0,
        $f.maxbyever1, $f.minbyever0, $f.minbyever1, $f.firstever0, $f.firstever1, $f.lastever0, $f.lastever1]
        + $f.sorted0 + $f.sorted1 + $f.window0 + $f.window1)
        | .[] | .fields | length] | unique == [20])
    and ([.records[9:13][] | .new[0].fields as $f
        | ([$f.first0, $f.first1, $f.last0, $f.last1, $f.maxby0, $f.maxby1, $f.minby0, $f.minby1, $f.maxbyever0,
        $f.maxbyever1, $f.minbyever0, $f.minbyever1, $f.firstever0, $f.firstever1, $f.lastever0, $f.lastever1]
        + $f.sorted0 + $f.sorted1 + $f.window0 + $f.window1)
        | .[] | .fields | keys_unsorted] | unique == [
        ["bigDecimal","bigInteger","boolBoxed","boolPrimitive","byteBoxed","bytePrimitive","charBoxed",
         "charPrimitive","doubleBoxed","doublePrimitive","enumValue","floatBoxed","floatPrimitive","intBoxed",
         "intPrimitive","longBoxed","longPrimitive","shortBoxed","shortPrimitive","theString"]])
    and ([.records[13] | .new[0].fields | keys_unsorted] == [
        ["bigDecimal","bigInteger","boolBoxed","boolPrimitive","byteBoxed","bytePrimitive","charBoxed",
         "charPrimitive","doubleBoxed","doublePrimitive","enumValue","floatBoxed","floatPrimitive","intBoxed",
         "intPrimitive","longBoxed","longPrimitive","shortBoxed","shortPrimitive","theString"]])
    and (.records[13].new[0].fields == {
        "bigDecimal":{"state":"null"},"bigInteger":{"state":"null"},"boolBoxed":{"state":"null"},
        "boolPrimitive":false,"byteBoxed":{"state":"null"},"bytePrimitive":0,"charBoxed":{"state":"null"},
        "charPrimitive":"\u0000","doubleBoxed":{"state":"null"},"doublePrimitive":0,"enumValue":{"state":"null"},
        "floatBoxed":{"state":"null"},"floatPrimitive":0,"intBoxed":{"state":"null"},"intPrimitive":10,
        "longBoxed":{"state":"null"},"longPrimitive":0,"shortBoxed":{"state":"null"},"shortPrimitive":0,
        "theString":"E1"})
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
