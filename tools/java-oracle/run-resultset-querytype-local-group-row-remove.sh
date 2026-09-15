#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-row-remove.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

Replays the resultset-querytype-local-group-row-remove scenario
(ResultSetQueryTypeLocalGroupBy ordinals 20/21: named-window row removal with
ungrouped and grouped local-group aggregates) against the pinned Java oracle in
the fixed Esper 9.0.0 checkout. Java 17 and Maven are selected from PATH unless
JAVA_HOME/MAVEN_HOME are provided.
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
    and $s.id == "resultset-querytype-local-group-row-remove"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinals 20/21: named-window row removal with ungrouped and grouped local-group aggregates."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-3a47ac428a21e66d8614","java-runtime-07d68fdd9a8dffc8c7f4"]
    and $s.javaNames == ["ResultSetLocalUngroupedRowRemove","ResultSetLocalGroupedRowRemove"]
    and $s.javaStaticIds == ["java-3da6ea3da5df0d53578a","java-ce99d7bbb48728947c59"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case": "ungrouped-row-remove", "ordinal": 20, "runtimeId": "java-runtime-3a47ac428a21e66d8614", "executionName": "ResultSetLocalUngroupedRowRemove", "observation": "listener", "iteratorSnapshots": 0, "epl": "create window MyWindow#keepall as SupportBean;\ninsert into MyWindow select * from SupportBean;\non SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\non SupportBean_S1 delete from MyWindow;\n@name(\u0027s0\u0027) select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive, group_by:theString) as c1 from MyWindow;\n"},
        {"case": "grouped-row-remove", "ordinal": 21, "runtimeId": "java-runtime-07d68fdd9a8dffc8c7f4", "executionName": "ResultSetLocalGroupedRowRemove", "observation": "listener", "iteratorSnapshots": 0, "epl": "create window MyWindow#keepall as SupportBean;\ninsert into MyWindow select * from SupportBean;\non SupportBean_S0 delete from MyWindow where p00 = theString and id = intPrimitive;\non SupportBean_S1 delete from MyWindow;\n@name(\u0027s0\u0027) select theString, intPrimitive, sum(longPrimitive) as c0,   sum(longPrimitive, group_by:theString) as c1   from MyWindow group by theString, intPrimitive;\n"}
    ]
    and $s.steps == [
        {"op": "case", "case": "ungrouped-row-remove"},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 10, "longPrimitive": 101}},
        {"op": "send", "eventType": "SupportBean_S0", "payload": {"id": 10, "p00": "E1"}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 20, "longPrimitive": 102}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E2", "intPrimitive": 30, "longPrimitive": 103}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 40, "longPrimitive": 104}},
        {"op": "send", "eventType": "SupportBean_S0", "payload": {"id": 40, "p00": "E1"}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 50, "longPrimitive": 105}},
        {"op": "send", "eventType": "SupportBean_S1", "payload": {"id": -1}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 60, "longPrimitive": 106}},
        {"op": "case", "case": "grouped-row-remove"},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 10, "longPrimitive": 101}},
        {"op": "send", "eventType": "SupportBean_S0", "payload": {"id": 10, "p00": "E1"}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 20, "longPrimitive": 102}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E2", "intPrimitive": 30, "longPrimitive": 103}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 40, "longPrimitive": 104}},
        {"op": "send", "eventType": "SupportBean_S0", "payload": {"id": 40, "p00": "E1"}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 50, "longPrimitive": 105}},
        {"op": "send", "eventType": "SupportBean_S1", "payload": {"id": -1}},
        {"op": "send", "eventType": "SupportBean", "payload": {"theString": "E1", "intPrimitive": 60, "longPrimitive": 106}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-row-remove replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-row-remove.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupRowRemoveScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupRowRemoveScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-row-remove"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records | length) == 15
    and ([.records[] | .case] == [
        "ungrouped-row-remove","ungrouped-row-remove","ungrouped-row-remove","ungrouped-row-remove",
        "ungrouped-row-remove","ungrouped-row-remove",
        "grouped-row-remove","grouped-row-remove","grouped-row-remove","grouped-row-remove",
        "grouped-row-remove","grouped-row-remove","grouped-row-remove","grouped-row-remove",
        "grouped-row-remove"])
    and ([.records[] | .operation] | unique == ["listener"])
    and ([.records[] | .statement] | unique == ["s0"])
    and ([.records[0:6][] | .sequence] == [1,2,3,4,5,6])
    and ([.records[6:15][] | .sequence] == [1,2,3,4,5,6,7,8,9])
    and ([.records[] | .time] | unique == ["1970-01-01T00:00:00Z"])
    and ([.records[] | has("old")] | any | not)
    and ([.records[] | .new[] | .kind] | unique == ["row"])
    and ([.records[] | .new[] | .fields | keys_unsorted] | unique == [["c0","c1","intPrimitive","theString"]])
    and ([.records[] | .new[] | .fields | length] | unique == [4])
    and ([.records[] | (.new | length)] == [1,1,1,1,1,1,1,1,1,1,1,1,1,3,1])
    and ([.records[0:6][] | [.new[0].fields.theString, .new[0].fields.intPrimitive, .new[0].fields.c0, .new[0].fields.c1]] == [
        ["E1",10,101,101],
        ["E1",20,102,102],
        ["E2",30,205,103],
        ["E1",40,309,206],
        ["E1",50,310,207],
        ["E1",60,106,106]])
    and ([.records[6:15][] | .new | map([.fields.theString, .fields.intPrimitive, .fields.c0, .fields.c1])] == [
        [["E1",10,101,101]],
        [["E1",10,{"state":"null"},{"state":"null"}]],
        [["E1",20,102,102]],
        [["E2",30,103,103]],
        [["E1",40,104,206]],
        [["E1",40,{"state":"null"},102]],
        [["E1",50,105,207]],
        [["E1",20,{"state":"null"},{"state":"null"}],["E1",50,{"state":"null"},{"state":"null"}],
         ["E2",30,{"state":"null"},{"state":"null"}]],
        [["E1",60,106,106]]])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
