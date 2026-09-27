#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-comparative-557.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]
The Esper checkout must be exactly the pinned Java 9.0.0 oracle commit. Java 17
and Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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
        --esper-root=*)
            esper_root=${1#*=}
            shift
            ;;
        --scenario)
            [ "$#" -ge 2 ] || usage
            scenario=$2
            shift 2
            ;;
        --scenario=*)
            scenario=${1#*=}
            shift
            ;;
        --output)
            [ "$#" -ge 2 ] || usage
            output=$2
            shift 2
            ;;
        --output=*)
            output=${1#*=}
            shift
            ;;
        --skip-build)
            skip_build=1
            shift
            ;;
        *)
            usage
            ;;
    esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "could not resolve Esper commit under $esper_root" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper commit drift: expected $expected_commit, found $actual_commit" >&2
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

java_version=$("$java_bin" -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-comparative-557" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableComparative.java" and
    (.javaRuntimes == ["java-runtime-4c1261f263736d23046c", "java-runtime-5c1fe99c70aaea089285"]) and
    (.javaNames == ["InfraNWTableComparativeGroupByTopLevelSingleAgg{caseName='"'"'named window'"'"'}'"'"'", "InfraNWTableComparativeGroupByTopLevelSingleAgg{caseName='"'"'table'"'"'}'"'"'"]) and
    (.javaStaticIds == ["java-c261ec96749884cbf192", "java-c261ec96749884cbf192"]) and
    (.javaFlags == ["EXCLUDEWHENINSTRUMENTED"]) and
    (.cases | type == "array" and length == 2) and
    ([.cases[].case] == ["named-window", "table"]) and
    ([.cases[].ordinal] == [0, 1]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].observation] == ["listener", "listener"]) and
    ([.cases[].iteratorSnapshots] == [0, 0]) and
    (.steps | type == "array" and length == 4008) and
    ([.steps[] | select(.op == "case")] | length == 2) and
    ([.steps[] | select(.op == "deploy")] | length == 2) and
    ([.steps[] | select(.op == "deployed")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 2000) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 2000) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "module" and .epl == "create window TotalsWindow#unique(theString) as (theString string, total int);insert into TotalsWindow select theString, sum(intPrimitive) as total from SupportBean group by theString;@Name('"'"'s0'"'"') select p00 as c0,     (select total from TotalsWindow tw where tw.theString = s0.p00) as c1 from SupportBean_S0 as s0;")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "module" and .epl == "create table varTotal (key string primary key, total sum(int));\ninto table varTotal select theString, sum(intPrimitive) as total from SupportBean group by theString;\n@Name('"'"'s0'"'"') select p00 as c0, varTotal[p00].total as c1 from SupportBean_S0;\n")] | length == 1) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-comparative-557 replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-comparative-557.XXXXXX")
cleanup() {
    status=$?
    rm -rf "$work"
    exit $status
}
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/avro-1.11.3.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-annotations-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-core-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-databind-2.14.2.jar")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableComparative557ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableComparative557ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def proberecords($case):
        ([.records[] | select(.case == $case and .operation == "deployed")] | length == 1) and
        (.records[] | select(.case == $case and .operation == "deployed")
            | .statement == "module" and .sequence == 1) and
        ([.records[] | select(.case == $case and .operation == "listener")] | length == 1000) and
        ([.records[] | select(.case == $case and .operation == "listener") | .sequence]
            == [range(1; 1001)]) and
        ([.records[] | select(.case == $case and .operation == "listener") | .statement == "s0"]
            | all) and
        ([.records[] | select(.case == $case and .operation == "listener")
            | .new | length == 1 and .[0].kind == "row" and (.[0].fields | length == 2)] | all) and
        ([.records[] | select(.case == $case and .operation == "listener") | .new[0].fields.c0]
            == [range(0; 1000) | "E" + (. | tostring)]) and
        ([.records[] | select(.case == $case and .operation == "listener") | .new[0].fields.c1]
            == [range(0; 1000)]) and
        ([.records[] | select(.case == $case and .operation == "listener") | has("old") | not] | all);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-comparative-557" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 2002) and
    ([.records[].operation] | all(. == "deployed" or . == "listener")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[0], .records[1001]] | all(.operation == "deployed")) and
    (.records[0].case == "named-window" and .records[1000].case == "named-window") and
    (.records[1001].case == "table" and .records[2001].case == "table") and
    proberecords("named-window") and
    proberecords("table")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-comparative-557 trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
