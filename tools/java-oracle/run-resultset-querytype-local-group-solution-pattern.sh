#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-solution-pattern.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-solution-pattern"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinal 12: the grouped solution-pattern ratio, count(*) divided by the statement-wide count(*, group_by:()) over a 30-second time window with an output snapshot every 10 seconds, driven entirely by virtual time (0/10/20/30s) so the final boundary expires the batch sent at zero and the third snapshot\u0027s denominator is 12."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-ae96db5ed464e562e6d7"]
    and $s.javaNames == ["ResultSetLocalGroupedSolutionPattern"]
    and $s.javaStaticIds == ["java-13f0da7834ee65870fb0"]
    and $s.javaFlags == []
    and $s.cases == [
        {"case":"grouped-solution-pattern","ordinal":12,"runtimeId":"java-runtime-ae96db5ed464e562e6d7","executionName":"ResultSetLocalGroupedSolutionPattern","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select theString, count(*) / count(*, group_by:()) as pct from SupportBean#time(30 sec) group by theString output snapshot every 10 seconds"}
    ]
    and $s.steps == [
        {"op":"case","case":"grouped-solution-pattern"},
        {"op":"advance-time","at":"1970-01-01T00:00:00Z"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"C","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"C","intPrimitive":0,"longPrimitive":0}},
        {"op":"advance-time","at":"1970-01-01T00:00:10Z"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"advance-time","at":"1970-01-01T00:00:20Z"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"C","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"B","intPrimitive":0,"longPrimitive":0}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"A","intPrimitive":0,"longPrimitive":0}},
        {"op":"advance-time","at":"1970-01-01T00:00:30Z"}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-solution-pattern replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-solution-pattern.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupSolutionPatternScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupSolutionPatternScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-solution-pattern"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and ((keys_unsorted | sort) == ["id","java","javaCommit","records","version"])
    and (.records | length) == 3
    and ([.records[] | .case] == ["grouped-solution-pattern","grouped-solution-pattern","grouped-solution-pattern"])
    and ([.records[] | .operation] | unique == ["listener"])
    and ([.records[] | .statement] | unique == ["s0"])
    and ([.records[] | .sequence] == [1,2,3])
    and ([.records[] | .time] == ["1970-01-01T00:00:10Z","1970-01-01T00:00:20Z","1970-01-01T00:00:30Z"])
    and ([.records[] | has("old")] | any | not)
    and ([.records[] | (.new | length)] == [3,3,3])
    and ([.records[] | .new[] | .kind] | unique == ["row"])
    and ([.records[] | .new[] | .fields | keys_unsorted] | unique == [["pct","theString"]])
    and ([.records[] | .new[].fields.theString | type] | unique == ["string"])
    and ([.records[] | .new[].fields.pct | type] | unique == ["number"])
    and ([.records[] | [.new[] | [.fields.theString, .fields.pct]]] == [
        [["A",0.16666666666666666],["B",0.5],["C",0.3333333333333333]],
        [["A",0.25],["B",0.5833333333333334],["C",0.16666666666666666]],
        [["A",0.5],["B",0.4166666666666667],["C",0.08333333333333333]]])
    and (.records[0].new[0].fields.theString == "A" and .records[0].new[0].fields.pct == (1/6))
    and (.records[0].new[2].fields.theString == "C" and .records[0].new[2].fields.pct == (2/6))
    and (.records[1].new[0].fields.theString == "A" and .records[1].new[0].fields.pct == (3/12))
    and (.records[1].new[2].fields.theString == "C" and .records[1].new[2].fields.pct == (2/12))
    and (.records[2].new[0].fields.theString == "A" and .records[2].new[0].fields.pct == (6/12))
    and (.records[2].new[1].fields.theString == "B" and .records[2].new[1].fields.pct == (5/12))
    and (.records[2].new[2].fields.theString == "C" and .records[2].new[2].fields.pct == (1/12))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
