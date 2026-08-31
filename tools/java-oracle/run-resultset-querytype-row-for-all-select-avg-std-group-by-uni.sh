#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-querytype-row-for-all-select-avg-std-group-by-uni.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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

# Slurp exactly one JSON text so trailing JSON values are rejected here. The
# Java oracle additionally rejects duplicate keys and validates every field.
if ! jq -e -s '
    if length != 1 then false else .[0] as $s |
    (($s | keys_unsorted | sort) == ["cases","description","id","javaCommit","javaFlags","javaNames","javaRuntimes","javaSource","javaStaticIds","steps","version"])
    and $s.version == "esper-parity/v1"
    and $s.id == "resultset-querytype-row-for-all-select-avg-std-group-by-uni"
    and $s.description == "ResultSetQueryTypeRowForAll ordinal 10: average with standard group-by and unique price view."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/querytype/ResultSetQueryTypeRowForAll.java"
    and $s.javaRuntimes == ["java-runtime-de3a01d12a22bb33ee43"]
    and $s.javaNames == ["ResultSetQueryTypeRowForAllSelectAvgStdGroupByUni"]
    and $s.javaStaticIds == ["java-7ba8b4893d30071e2fa5"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 1)
    and $s.cases[0] == {"case":"avg-std-group-by-uni","ordinal":10,"runtimeId":"java-runtime-de3a01d12a22bb33ee43","executionName":"ResultSetQueryTypeRowForAllSelectAvgStdGroupByUni","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select istream average as aprice from SupportMarketDataBean#groupwin(symbol)#length(2)#uni(price)"}
    and $s.steps == [
        {"op":"case","case":"avg-std-group-by-uni"},
        {"op":"send","eventType":"SupportMarketDataBean","payload":{"symbol":"A","id":null,"price":1,"volume":null,"feed":null}},
        {"op":"send","eventType":"SupportMarketDataBean","payload":{"symbol":"B","id":null,"price":3,"volume":null,"feed":null}},
        {"op":"send","eventType":"SupportMarketDataBean","payload":{"symbol":"A","id":null,"price":3,"volume":null,"feed":null}},
        {"op":"send","eventType":"SupportMarketDataBean","payload":{"symbol":"A","id":null,"price":10,"volume":null,"feed":null}},
        {"op":"send","eventType":"SupportMarketDataBean","payload":{"symbol":"A","id":null,"price":20,"volume":null,"feed":null}}
    ]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid resultset-querytype-row-for-all-select-avg-std-group-by-uni replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-row-for-all-select-avg-std-group-by-uni.XXXXXX")
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
    "$script_root/ResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeRowForAllSelectAvgStdGroupByUniScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "resultset-querytype-row-for-all-select-avg-std-group-by-uni"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | type == "string" and startswith("17."))
    and (.records | type == "array" and length == 4)
    and .records == [
        {"case":"avg-std-group-by-uni","operation":"listener","statement":"s0","sequence":1,"time":"1970-01-01T00:00:00Z","new":[{"kind":"row","fields":{"aprice":1.0}}]},
        {"case":"avg-std-group-by-uni","operation":"listener","statement":"s0","sequence":2,"time":"1970-01-01T00:00:00Z","new":[{"kind":"row","fields":{"aprice":3.0}}]},
        {"case":"avg-std-group-by-uni","operation":"listener","statement":"s0","sequence":3,"time":"1970-01-01T00:00:00Z","new":[{"kind":"row","fields":{"aprice":2.0}}]},
        {"case":"avg-std-group-by-uni","operation":"listener","statement":"s0","sequence":4,"time":"1970-01-01T00:00:00Z","new":[{"kind":"row","fields":{"aprice":15.0}}]}
    ]
    and (all(.records[]; keys_unsorted == ["case","operation","statement","sequence","time","new"]
        and (.new | type == "array" and length == 1)
        and (.new[0] | keys_unsorted == ["kind","fields"] and .kind == "row" and (.fields | keys_unsorted == ["aprice"]))))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
