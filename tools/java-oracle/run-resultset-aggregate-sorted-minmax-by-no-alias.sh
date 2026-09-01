#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-sorted-minmax-by-no-alias.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "could not determine the Esper commit at $esper_root" >&2
    exit 1
}
[ "$actual_commit" = "$expected_commit" ] || {
    echo "Esper root is at $actual_commit, expected $expected_commit" >&2
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
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the oracle trace" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "python3 executable was not found; it is required to validate the oracle scenario" >&2; exit 1; }

java_version=$($java_bin -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required for the Java 9.0.0 oracle; found ${java_version:-unknown}" >&2; exit 1; }

if ! python3 - "$scenario" <<'PY'
import json
import sys

VERSION = "esper-parity/v1"
SCENARIO_ID = "resultset-aggregate-sorted-minmax-by-no-alias"
DESCRIPTION = "ResultSetAggregateSortedMinMaxBy ordinal 3: unaliased min-by/max-by and sorted projections auto-name their output columns; the deployment is acknowledged and the ordered property names/types are recorded without any events."
JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
JAVA_SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregateSortedMinMaxBy.java"
RUNTIME_ID = "java-runtime-bb6969a66cad8464ae18"
STATIC_ID = "java-553516b9d01c12a13172"
EXECUTION = "ResultSetAggregateNoAlias"
CASE = "no-alias"
EPL = "@name('s0') select maxby(intPrimitive).theString, minby(intPrimitive),maxbyever(intPrimitive).theString, minbyever(intPrimitive),sorted(intPrimitive asc, theString desc) from SupportBean#time(10)"


def parse_integer(value):
    return ("integer", value)


def parse_float(value):
    return ("non-integer", value)


def parse_constant(value):
    raise ValueError("non-JSON numeric constant")


def reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON object key")
        result[key] = value
    return result


def exact_object(value, names):
    return isinstance(value, dict) and set(value) == set(names)


def valid_integer(value):
    return (
        isinstance(value, tuple)
        and len(value) == 2
        and value[0] == "integer"
        and isinstance(value[1], str)
        and value[1].isascii()
        and value[1].lstrip("-").isdigit()
        and not (len(value[1].lstrip("-")) > 1 and value[1].lstrip("-")[0] == "0")
    )


try:
    with open(sys.argv[1], encoding="utf-8") as handle:
        scenario = json.load(
            handle,
            object_pairs_hook=reject_duplicate_keys,
            parse_int=parse_integer,
            parse_float=parse_float,
            parse_constant=parse_constant,
        )
    required = (
        "version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
        "javaNames", "javaStaticIds", "javaFlags", "cases", "steps",
    )
    if not exact_object(scenario, required):
        raise ValueError("scenario has unexpected fields")
    if (scenario["version"] != VERSION or scenario["id"] != SCENARIO_ID
            or scenario["description"] != DESCRIPTION
            or scenario["javaCommit"] != JAVA_COMMIT
            or scenario["javaSource"] != JAVA_SOURCE):
        raise ValueError("scenario metadata is not pinned")
    if scenario["javaRuntimes"] != [RUNTIME_ID] or scenario["javaNames"] != [EXECUTION]:
        raise ValueError("Java runtime/name metadata is not pinned")
    if scenario["javaStaticIds"] != [STATIC_ID] or scenario["javaFlags"] != []:
        raise ValueError("Java static/flag metadata is not pinned")

    cases = scenario["cases"]
    if not isinstance(cases, list) or len(cases) != 1:
        raise ValueError("scenario must contain exactly one case")
    case = cases[0]
    if not exact_object(case, ("case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl")):
        raise ValueError("case metadata has unexpected fields")
    if (case["case"] != CASE or case["ordinal"] != ("integer", "3")
            or case["runtimeId"] != RUNTIME_ID or case["executionName"] != EXECUTION
            or case["observation"] != "statement-metadata"
            or case["iteratorSnapshots"] != ("integer", "0") or case["epl"] != EPL):
        raise ValueError("case metadata is not pinned")

    steps = scenario["steps"]
    if not isinstance(steps, list) or len(steps) != 3:
        raise ValueError("scenario must contain exactly three steps")
    marker = steps[0]
    if not exact_object(marker, ("op", "case")) or marker["op"] != "case" or marker["case"] != CASE:
        raise ValueError("case marker is not pinned")
    deployed = steps[1]
    if not exact_object(deployed, ("op", "statement")) or deployed["op"] != "deployed" or deployed["statement"] != "s0":
        raise ValueError("deployed step is not pinned")
    types = steps[2]
    if not exact_object(types, ("op", "statement")) or types["op"] != "types" or types["statement"] != "s0":
        raise ValueError("types step is not pinned")
except (OSError, ValueError, TypeError, KeyError, IndexError, AttributeError):
    raise SystemExit(1)
PY
then
    echo "scenario is not a valid resultset-aggregate-sorted-minmax-by-no-alias replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    for module in common compiler runtime common-avro regression-lib; do
        "$mvn_bin" -q -f "$esper_root/pom.xml" -pl "$module" -am package \
            -DskipTests -Dmaven.javadoc.skip=true -Dgpg.skip=true -Dcheckstyle.skip=true \
            -Dforbiddenapis.skip=true -Dmaven.source.skip=true -Denforcer.skip=true \
            -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    done
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-sorted-minmax-by-no-alias.XXXXXX")
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
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ResultSetAggregateSortedMinMaxByNoAliasScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregateSortedMinMaxByNoAliasScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-sorted-minmax-by-no-alias" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.javaRuntimes == ["java-runtime-bb6969a66cad8464ae18"]) and
    (.javaNames == ["ResultSetAggregateNoAlias"]) and
    (.javaStaticIds == ["java-553516b9d01c12a13172"]) and
    (.javaFlags == []) and
    (.records | type == "array" and length == 2) and
    ([.records[].case] == ["no-alias", "no-alias"]) and
    ([.records[].operation] == ["deployed", "types"]) and
    ([.records[].statement] == ["s0", "s0"]) and
    ([.records[].sequence] == [0, 0]) and
    ([.records[] | has("time")] | all(. == false)) and
    (.records[0] | (keys | sort) == ["case", "operation", "sequence", "statement"]) and
    (.records[1].value | type == "array" and length == 2) and
    (.records[1].value[0] == {"name": "maxby(intPrimitive).theString", "type": "String"}) and
    (.records[1].value[1] == {"name": "maxbyever(intPrimitive).theString", "type": "String"})
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid resultset-aggregate-sorted-minmax-by-no-alias trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
