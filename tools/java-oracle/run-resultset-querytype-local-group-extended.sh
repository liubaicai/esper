#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-extended.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-extended"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 8-9: ungrouped local aggregation over a unidirectional join and over multi-level window access."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-a1b494ebb2cb69d4c90a","java-runtime-344d65c309eddd141cc5"]
    and $s.javaNames == ["ResultSetLocalUngroupedUnidirectionalJoin","ResultSetLocalUngroupedThreeLevelWTop"]
    and $s.javaStaticIds == ["java-d4c45f0591f6b53f8f74","java-3e936d8e8328dbd8a2ed"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"ungrouped-unidirectional-join","ordinal":8,"runtimeId":"java-runtime-a1b494ebb2cb69d4c90a","executionName":"ResultSetLocalUngroupedUnidirectionalJoin","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select theString, sum(intPrimitive, group_by:theString) as c0 from SupportBean#keepall, SupportBean_S0 unidirectional"},
        {"case":"ungrouped-three-level-wtop","ordinal":9,"runtimeId":"java-runtime-344d65c309eddd141cc5","executionName":"ResultSetLocalUngroupedThreeLevelWTop","observation":"listener","iteratorSnapshots":0,"epl":"@Name(\u0027s0\u0027) select sum(longPrimitive, group_by:theString) as c0,count(*, group_by:theString) as c1,window(*, group_by:theString) as c2,sum(longPrimitive, group_by:intPrimitive) as c3,count(*, group_by:intPrimitive) as c4,window(*, group_by:intPrimitive) as c5,sum(longPrimitive, group_by:(theString, intPrimitive)) as c6,count(*, group_by:(theString, intPrimitive)) as c7,window(*, group_by:(theString, intPrimitive)) as c8,sum(longPrimitive) as c9 from SupportBean#length(4)"}
    ]
    and $s.steps == [
        {"op":"case","case":"ungrouped-unidirectional-join"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":20,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":30,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":40,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean_S0","payload":{"id":1}},
        {"op":"case","case":"ungrouped-three-level-wtop"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":10,"longPrimitive":101}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":20,"longPrimitive":102}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":103}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":104}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-extended replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-extended.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupByExtendedScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupByExtendedScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-extended"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and ([.records[] | .case] == [
        "ungrouped-unidirectional-join","ungrouped-unidirectional-join",
        "ungrouped-three-level-wtop","ungrouped-three-level-wtop","ungrouped-three-level-wtop",
        "ungrouped-three-level-wtop","ungrouped-three-level-wtop"])
    and ([.records[] | .operation] == ["listener","listener","listener","listener","listener","listener","listener"])
    and ([.records[] | .statement] == ["s0","s0","s0","s0","s0","s0","s0"])
    and ([.records[] | .sequence] == [1,2,1,2,3,4,5])
    and ([.records[] | .time] == [
        "1970-01-01T00:00:00Z","1970-01-01T00:00:00Z","1970-01-01T00:00:00Z","1970-01-01T00:00:00Z",
        "1970-01-01T00:00:00Z","1970-01-01T00:00:00Z","1970-01-01T00:00:00Z"])
    and ([.records[] | (.new | length)] == [3,4,1,1,1,1,1])
    and ([.records[0:2][] | [(.new[].fields | [.theString, .c0])]] == [
        [["E1",40],["E2",20],["E1",40]],
        [["E1",80],["E2",20],["E1",80],["E1",80]]])
    and ([.records[0:2][] | (.new[].fields | keys_unsorted)] | unique == [["c0","theString"]])
    and ([.records[2:][] | (.new[0].fields | keys_unsorted)] | unique == [["c0","c1","c2","c3","c4","c5","c6","c7","c8","c9"]])
    and ([.records[2:][] | (.new[0].fields | [.c0, .c1, .c3, .c4, .c6, .c7, .c9])] == [
        [100,1,100,1,100,1,100],
        [101,1,201,2,101,1,201],
        [202,2,102,1,102,1,303],
        [305,3,304,3,203,2,406],
        [309,3,308,3,207,2,410]])
    and ([.records[2:][] | (.new[0].fields.c2 | [.[] | [.fields.theString, .fields.intPrimitive, .fields.longPrimitive]])] == [
        [["E1",10,100]],
        [["E2",10,101]],
        [["E1",10,100],["E1",20,102]],
        [["E1",10,100],["E1",20,102],["E1",10,103]],
        [["E1",20,102],["E1",10,103],["E1",10,104]]])
    and ([.records[2:][] | (.new[0].fields.c5 | [.[] | [.fields.theString, .fields.intPrimitive, .fields.longPrimitive]])] == [
        [["E1",10,100]],
        [["E1",10,100],["E2",10,101]],
        [["E1",20,102]],
        [["E1",10,100],["E2",10,101],["E1",10,103]],
        [["E2",10,101],["E1",10,103],["E1",10,104]]])
    and ([.records[2:][] | (.new[0].fields.c8 | [.[] | [.fields.theString, .fields.intPrimitive, .fields.longPrimitive]])] == [
        [["E1",10,100]],
        [["E2",10,101]],
        [["E1",20,102]],
        [["E1",10,100],["E1",10,103]],
        [["E1",10,103],["E1",10,104]]])
    and ([.records[2:][] | (.new[0].fields.c2 | length)] == [1,1,2,3,3])
    and ([.records[2:][] | (.new[0].fields.c5 | length)] == [1,2,1,3,3])
    and ([.records[2:][] | (.new[0].fields.c8 | length)] == [1,1,1,2,2])
    and ([.records[2:][] | (.new[0].fields.c2[].fields | length)] | unique == [20])
    and ([.records[2:][] | (.new[0].fields.c2[].fields | keys_unsorted)] | unique == [["bigDecimal","bigInteger","boolBoxed","boolPrimitive","byteBoxed","bytePrimitive","charBoxed","charPrimitive","doubleBoxed","doublePrimitive","enumValue","floatBoxed","floatPrimitive","intBoxed","intPrimitive","longBoxed","longPrimitive","shortBoxed","shortPrimitive","theString"]])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
