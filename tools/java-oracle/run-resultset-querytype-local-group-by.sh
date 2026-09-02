#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-local-group-by.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    and $s.id == "resultset-querytype-local-group-by"
    and $s.description == "ResultSetQueryTypeLocalGroupBy ordinal 13: grouped multi-level window access with local group_by dimensions."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeLocalGroupBy.java"
    and $s.javaRuntimes == ["java-runtime-87b3bcfdf164bc939fe5"]
    and $s.javaNames == ["ResultSetLocalGroupedMultiLevelAccess"]
    and $s.javaStaticIds == ["java-b0b3444c9ac5a1297cd0"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 1)
    and $s.cases[0] == {"case":"multi-level-access","ordinal":13,"runtimeId":"java-runtime-87b3bcfdf164bc939fe5","executionName":"ResultSetLocalGroupedMultiLevelAccess","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select   theString, intPrimitive,   window(*, group_by:(intPrimitive, theString)) as c0,   window(*) as c1,   window(*, group_by:theString) as c2,   window(*, group_by:intPrimitive) as c3,   window(*, group_by:()) as c4 from SupportBean#keepall group by theString, intPrimitive output snapshot every 10 seconds order by theString, intPrimitive"}
    and $s.steps == [
        {"op":"case","case":"multi-level-access"},
        {"op":"advance-time","at":"1970-01-01T00:00:00Z"},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":100}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":20,"longPrimitive":202}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":10,"longPrimitive":303}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E1","intPrimitive":10,"longPrimitive":404}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":"E2","intPrimitive":10,"longPrimitive":505}},
        {"op":"advance-time","at":"1970-01-01T00:00:10Z"}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-local-group-by replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-local-group-by.XXXXXX")
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
    "$script_root/ResultSetQueryTypeLocalGroupByScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeLocalGroupByScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-local-group-by"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and (.records | type == "array" and length == 1)
    and .records[0].case == "multi-level-access"
    and .records[0].operation == "listener"
    and .records[0].statement == "s0"
    and .records[0].sequence == 1
    and .records[0].time == "1970-01-01T00:00:10Z"
    and (.records[0].new | type == "array" and length == 3)
    and (.records[0].new[0].fields | keys_unsorted == ["c0","c1","c2","c3","c4","intPrimitive","theString"])
    and (.records[0].new[1].fields | keys_unsorted == ["c0","c1","c2","c3","c4","intPrimitive","theString"])
    and (.records[0].new[2].fields | keys_unsorted == ["c0","c1","c2","c3","c4","intPrimitive","theString"])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
