#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-minmax-named-window-wever.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
fi

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

# Slurp exactly one JSON text so trailing JSON values are rejected here. The
# Java oracle additionally rejects duplicate keys and validates every field.
if ! jq -e -s '
    if length != 1 then false else .[0] as $s |
    (($s | keys_unsorted | sort) == ["cases","description","id","javaCommit","javaFlags","javaNames","javaRuntimes","javaSource","javaStaticIds","steps","version"])
    and $s.version == "esper-parity/v1"
    and $s.id == "resultset-aggregate-minmax-named-window-wever"
    and $s.description == "ResultSetAggregateMinMax ordinals 2-3 NamedWindowWEver: public length(2) named window min/max current plus minever/maxever; four SupportBean sends produce new-only rows and length eviction changes current extrema while ever values retain the first event."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateMinMax.java"
    and $s.javaRuntimes == ["java-runtime-7d0a94525b0038b397fd", "java-runtime-840f1ca5610dd00d3ec0"]
    and $s.javaNames == ["ResultSetAggregateMinMaxNamedWindowWEver{soda=false}", "ResultSetAggregateMinMaxNamedWindowWEver{soda=true}"]
    and $s.javaStaticIds == ["java-09115f6e876ce88a42f9"]
    and $s.javaFlags == ["EXCLUDEWHENINSTRUMENTED"]
    and ($s.cases | type == "array" and length == 2)
    and $s.cases[0] == {"case":"named-window-wever-soda-false","ordinal":2,"runtimeId":"java-runtime-7d0a94525b0038b397fd","executionName":"ResultSetAggregateMinMaxNamedWindowWEver{soda=false}","observation":"listener","epl":"@name(\u0027s0\u0027) select min(intPrimitive) as lower, max(intPrimitive) as upper, minever(intPrimitive) as lowerever, maxever(intPrimitive) as upperever from NamedWindow5m"}
    and $s.cases[1] == {"case":"named-window-wever-soda-true","ordinal":3,"runtimeId":"java-runtime-840f1ca5610dd00d3ec0","executionName":"ResultSetAggregateMinMaxNamedWindowWEver{soda=true}","observation":"listener","epl":"@name(\u0027s0\u0027) select min(intPrimitive) as lower, max(intPrimitive) as upper, minever(intPrimitive) as lowerever, maxever(intPrimitive) as upperever from NamedWindow5m"}
    and ($s.steps | type == "array" and length == 10)
    and $s.steps[0] == {"op":"case","case":"named-window-wever-soda-false"}
    and $s.steps[5] == {"op":"case","case":"named-window-wever-soda-true"}
    and $s.steps[1:5] == [
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":1}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":5}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":3}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":6}}
    ]
    and $s.steps[6:10] == [
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":1}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":5}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":3}},
        {"op":"send","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":6}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-aggregate-minmax-named-window-wever replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-minmax-named-window-wever.XXXXXX")
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
    "$script_root/ResultSetAggregateMinMaxNamedWindowWEverScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateMinMaxNamedWindowWEverScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-aggregate-minmax-named-window-wever"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and (.records | type == "array" and length == 8)
    and ([.records[0:4][] | .case] == ["named-window-wever-soda-false","named-window-wever-soda-false","named-window-wever-soda-false","named-window-wever-soda-false"])
    and ([.records[4:8][] | .case] == ["named-window-wever-soda-true","named-window-wever-soda-true","named-window-wever-soda-true","named-window-wever-soda-true"])
    and (all(.records[];
        (keys_unsorted == ["case","operation","statement","sequence","time","new"])
        and .operation == "listener"
        and .statement == "s0"
        and (.sequence | type == "number" and floor == .)
        and (.time == "1970-01-01T00:00:00Z")
        and (.new | type == "array" and length == 1)
        and (.new[0] | keys_unsorted == ["kind","fields"] and .kind == "row" and (.fields | keys_unsorted == ["lower","lowerever","upper","upperever"]))
    ))
    and ([.records[0:4][] | .sequence] == [1,2,3,4])
    and ([.records[4:8][] | .sequence] == [1,2,3,4])
    and ([.records[] | .new[0]] == [
        {"kind":"row","fields":{"lower":1,"lowerever":1,"upper":1,"upperever":1}},
        {"kind":"row","fields":{"lower":1,"lowerever":1,"upper":5,"upperever":5}},
        {"kind":"row","fields":{"lower":3,"lowerever":1,"upper":5,"upperever":5}},
        {"kind":"row","fields":{"lower":3,"lowerever":1,"upper":6,"upperever":6}},
        {"kind":"row","fields":{"lower":1,"lowerever":1,"upper":1,"upperever":1}},
        {"kind":"row","fields":{"lower":1,"lowerever":1,"upper":5,"upperever":5}},
        {"kind":"row","fields":{"lower":3,"lowerever":1,"upper":5,"upperever":5}},
        {"kind":"row","fields":{"lower":3,"lowerever":1,"upper":6,"upperever":6}}
    ])
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
