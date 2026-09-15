#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-keys.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-keys"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 18/19/24/26/27: local-group key representation and readback - repeated scalar keys on object-array events, local groups shared across outer groups, the empty group_by:() statement-wide level, array-typed (int[]/long[]/double[]) keys compared by deep content, and the window(*)/first(*) accessor methods resolved per local group."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-890001d4334d5de6c50a","java-runtime-a2b77511196632040e51","java-runtime-b6a938fde543383eb73c","java-runtime-81855e4095ee0ca7cadd","java-runtime-e1253cd2c17a180c243a"]
    and $s.javaNames == ["ResultSetLocalUngroupedSameKey","ResultSetLocalGroupedSameKey","ResultSetLocalEnumMethods","ResultSetLocalMultikeyWArray","ResultSetLocalUngroupedOnlyWGroupBy"]
    and $s.javaStaticIds == ["java-2c2e1d80b0046f88b67e","java-b0aa55f10cfa580ccd4f","java-98ac70ee0f434579c8c8","java-6f7f7c3ba440787d3117","java-e3b7ff9f0f3bf5872d7b"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"ungrouped-same-key","ordinal":18,"runtimeId":"java-runtime-890001d4334d5de6c50a","executionName":"ResultSetLocalUngroupedSameKey","observation":"listener","iteratorSnapshots":0,"epl":"@public @buseventtype create objectarray schema MyEventOne (d1 String, d2 String, val int);\n@name(\u0027s0\u0027) select sum(val, group_by: d1) as c0, sum(val, group_by: d2) as c1 from MyEventOne"},
        {"case":"grouped-same-key","ordinal":19,"runtimeId":"java-runtime-a2b77511196632040e51","executionName":"ResultSetLocalGroupedSameKey","observation":"listener","iteratorSnapshots":0,"epl":"@public @buseventtype create objectarray schema MyEventTwo (g1 String, d1 String, d2 String, val int);\n@name(\u0027s0\u0027) select sum(val) as c0, sum(val, group_by: d1) as c1, sum(val, group_by: d2) as c2 from MyEventTwo group by g1"},
        {"case":"enum-methods-grouped","ordinal":24,"runtimeId":"java-runtime-b6a938fde543383eb73c","executionName":"ResultSetLocalEnumMethods","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select window(*, group_by:()).firstOf() as c0, window(*, group_by:theString).firstOf() as c1, window(intPrimitive, group_by:()).firstOf() as c2, window(intPrimitive, group_by:theString).firstOf() as c3, first(*, group_by:()).intPrimitive as c4, first(*, group_by:theString).intPrimitive as c5  from SupportBean#keepall group by theString, intPrimitive"},
        {"case":"multikey-w-array","ordinal":26,"runtimeId":"java-runtime-81855e4095ee0ca7cadd","executionName":"ResultSetLocalMultikeyWArray","observation":"listener","iteratorSnapshots":0,"epl":"@Name(\u0027s0\u0027) select sum(value, group_by:(intArray)) as c0, sum(value, group_by:(longArray)) as c1, sum(value, group_by:(doubleArray)) as c2, sum(value, group_by:(intArray, longArray, doubleArray)) as c3, sum(value) as c4 from SupportThreeArrayEvent"},
        {"case":"ungrouped-only-w-group-by","ordinal":27,"runtimeId":"java-runtime-e1253cd2c17a180c243a","executionName":"ResultSetLocalUngroupedOnlyWGroupBy","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select first(*, group_by:()).intPrimitive as c0  from SupportBean#keepall group by theString, intPrimitive"}
    ]
    and $s.steps == [
        {"op":"case","case":"ungrouped-same-key"},
        {"op":"send","eventType":"MyEventOne","payload":["E1","E1",10]},
        {"op":"send","eventType":"MyEventOne","payload":["E1","E2",11]},
        {"op":"send","eventType":"MyEventOne","payload":["E2","E1",12]},
        {"op":"send","eventType":"MyEventOne","payload":["E3","E1",13]},
        {"op":"send","eventType":"MyEventOne","payload":["E3","E3",14]},
        {"op":"case","case":"grouped-same-key"},
        {"op":"send","eventType":"MyEventTwo","payload":["E1","E1","E1",10]},
        {"op":"send","eventType":"MyEventTwo","payload":["E1","E1","E2",11]},
        {"op":"send","eventType":"MyEventTwo","payload":["E1","E2","E1",12]},
        {"op":"send","eventType":"MyEventTwo","payload":["X","E1","E1",13]},
        {"op":"send","eventType":"MyEventTwo","payload":["E1","E2","E3",14]},
        {"op":"case","case":"enum-methods-grouped"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"case","case":"multikey-w-array"},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E1","value":10,"intArray":[1],"longArray":[10],"doubleArray":[100]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E2","value":11,"intArray":[2],"longArray":[20],"doubleArray":[200]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E3","value":12,"intArray":[3],"longArray":[10],"doubleArray":[300]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E4","value":13,"intArray":[1],"longArray":[20],"doubleArray":[200]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E5","value":14,"intArray":[1],"longArray":[10],"doubleArray":[100]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E6","value":15,"intArray":[3],"longArray":[20],"doubleArray":[300]}},
        {"op":"send","eventType":"SupportThreeArrayEvent","payload":{"id":"E7","value":16,"intArray":[2],"longArray":[20],"doubleArray":[200]}},
        {"op":"case","case":"ungrouped-only-w-group-by"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":1,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":2,"longPrimitive":0}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-keys replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-keys.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupKeysScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupKeysScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def beanrow: {"kind":"row","fields":{"bigDecimal":{"state":"null"},"bigInteger":{"state":"null"},"boolBoxed":{"state":"null"},"boolPrimitive":false,"byteBoxed":{"state":"null"},"bytePrimitive":0,"charBoxed":{"state":"null"},"charPrimitive":"\u0000","doubleBoxed":{"state":"null"},"doublePrimitive":0,"enumValue":{"state":"null"},"floatBoxed":{"state":"null"},"floatPrimitive":0,"intBoxed":{"state":"null"},"intPrimitive":10,"longBoxed":{"state":"null"},"longPrimitive":0,"shortBoxed":{"state":"null"},"shortPrimitive":0,"theString":"E1"}};
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-keys"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records | length) == 20
    and ([.records[] | .case] == [
        "ungrouped-same-key","ungrouped-same-key","ungrouped-same-key","ungrouped-same-key","ungrouped-same-key",
        "grouped-same-key","grouped-same-key","grouped-same-key","grouped-same-key","grouped-same-key",
        "enum-methods-grouped",
        "multikey-w-array","multikey-w-array","multikey-w-array","multikey-w-array","multikey-w-array","multikey-w-array","multikey-w-array",
        "ungrouped-only-w-group-by","ungrouped-only-w-group-by"])
    and ([.records[] | .operation] | unique == ["listener"])
    and ([.records[] | .statement] | unique == ["s0"])
    and ([.records[] | .sequence] == [1,2,3,4,5,1,2,3,4,5,1,1,2,3,4,5,6,7,1,2])
    and ([.records[] | .time] | unique == ["1970-01-01T00:00:00Z"])
    and ([.records[] | has("old")] | any | not)
    and ([.records[] | (.new | length)] | unique == [1])
    and ([.records[] | .new[0].fields | keys_unsorted] == [
        ["c0","c1"],["c0","c1"],["c0","c1"],["c0","c1"],["c0","c1"],
        ["c0","c1","c2"],["c0","c1","c2"],["c0","c1","c2"],["c0","c1","c2"],["c0","c1","c2"],
        ["c0","c1","c2","c3","c4","c5"],
        ["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],["c0","c1","c2","c3","c4"],
        ["c0"],["c0"]])
    and ([.records[0:5][] | [.new[0].fields.c0, .new[0].fields.c1]] == [[10,10],[21,11],[12,22],[13,35],[27,14]])
    and ([.records[5:10][] | [.new[0].fields.c0, .new[0].fields.c1, .new[0].fields.c2]] == [
        [10,10,10],[21,21,11],[33,12,22],[13,34,35],[47,26,14]])
    and (.records[10].new[0].fields.c0 == beanrow)
    and (.records[10].new[0].fields.c1 == beanrow)
    and ((.records[10].new[0].fields.c0.fields | keys_unsorted) == [
        "bigDecimal","bigInteger","boolBoxed","boolPrimitive","byteBoxed","bytePrimitive","charBoxed","charPrimitive",
        "doubleBoxed","doublePrimitive","enumValue","floatBoxed","floatPrimitive","intBoxed","intPrimitive",
        "longBoxed","longPrimitive","shortBoxed","shortPrimitive","theString"])
    and ([.records[10].new[0].fields | .c2, .c3, .c4, .c5] == [10,10,10,10])
    and ([.records[11:18][] | [.new[0].fields.c0, .new[0].fields.c1, .new[0].fields.c2, .new[0].fields.c3, .new[0].fields.c4]] == [
        [10,10,10,10,10],[11,11,11,11,21],[12,22,12,12,33],[23,24,24,13,46],
        [37,36,24,24,60],[27,39,27,15,75],[27,55,40,27,91]])
    and ([.records[18:20][] | .new[0].fields.c0] == [1,1])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
