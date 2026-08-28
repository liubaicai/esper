#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregation-method-sorted-multi-criteria.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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

command -v git >/dev/null 2>&1 || { echo "git executable was not found" >&2; exit 1; }
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
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found; it is required to validate the oracle trace" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "python3 executable was not found; it is required to validate the oracle scenario" >&2; exit 1; }

java_version=$($java_bin -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found ${java_version:-unknown}" >&2; exit 1; }

if ! python3 - "$scenario" <<'PY'
import json
import sys

VERSION = "esper-parity/v1"
SCENARIO_ID = "resultset-aggregate-sorted-multi-criteria"
CASE = "multi-criteria"
SEND_VALUES = [
    ("E1a", 1),
    ("E1b", 1),
    ("E4b", 4),
    ("E6a", 6),
    ("E6b", 6),
    ("E8", 8),
    ("E9", 9),
]


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


def valid_integer(value, lower, upper):
    if not isinstance(value, tuple) or len(value) != 2 or value[0] != "integer":
        return False
    text = value[1]
    if not isinstance(text, str) or not text.isascii():
        return False
    if text.startswith("-"):
        digits = text[1:]
    else:
        digits = text
    if not digits or (len(digits) > 1 and digits[0] == "0") or not digits.isdigit():
        return False
    try:
        parsed = int(text, 10)
    except ValueError:
        return False
    return lower <= parsed <= upper


def exact_object(value, names):
    return isinstance(value, dict) and set(value) == set(names)


try:
    with open(sys.argv[1], encoding="utf-8") as handle:
        scenario = json.load(
            handle,
            parse_int=parse_integer,
            parse_float=parse_float,
            parse_constant=parse_constant,
            object_pairs_hook=reject_duplicate_keys,
        )
    if not exact_object(scenario, ("version", "id", "steps")):
        raise ValueError("scenario has unexpected fields")
    if scenario["version"] != VERSION or scenario["id"] != SCENARIO_ID:
        raise ValueError("unsupported scenario identity")
    steps = scenario["steps"]
    if not isinstance(steps, list) or len(steps) != 9:
        raise ValueError("scenario must contain exactly nine steps")

    marker = steps[0]
    if not exact_object(marker, ("op", "case")) or marker["op"] != "case" or marker["case"] != CASE:
        raise ValueError("scenario must start with the frozen case marker")

    for index, (the_string, int_primitive) in enumerate(SEND_VALUES, start=1):
        step = steps[index]
        if not exact_object(step, ("op", "eventType", "payload")):
            raise ValueError("scenario must contain exact SupportBean send fields")
        if step["op"] != "send" or step["eventType"] != "SupportBean":
            raise ValueError("scenario must contain SupportBean sends in source order")
        payload = step["payload"]
        if not exact_object(payload, ("theString", "intPrimitive")):
            raise ValueError("SupportBean payload has unexpected fields")
        if payload["theString"] != the_string:
            raise ValueError("SupportBean theString sequence mismatch")
        if not valid_integer(payload["intPrimitive"], -(2**31), 2**31 - 1):
            raise ValueError("SupportBean intPrimitive must be a Java int")
        if int(payload["intPrimitive"][1], 10) != int_primitive:
            raise ValueError("SupportBean intPrimitive sequence mismatch")

    trigger = steps[8]
    if not exact_object(trigger, ("op", "eventType", "payload")):
        raise ValueError("scenario must end with exact SupportBean_S0 trigger fields")
    if trigger["op"] != "send" or trigger["eventType"] != "SupportBean_S0":
        raise ValueError("scenario must end with SupportBean_S0 trigger")
    payload = trigger["payload"]
    if not exact_object(payload, ("id",)) or not valid_integer(payload["id"], -(2**31), 2**31 - 1):
        raise ValueError("SupportBean_S0 id must be a Java int")
    if int(payload["id"][1], 10) != -1:
        raise ValueError("SupportBean_S0 trigger id must be -1")
except (OSError, ValueError, TypeError, KeyError, IndexError, AttributeError):
    raise SystemExit(1)
PY
then
    echo "scenario is not a valid resultset-aggregate-sorted-multi-criteria replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregation-method-sorted-multi-criteria.XXXXXX")
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
    "$script_root/ResultSetAggregationMethodSortedMultiCriteriaScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregationMethodSortedMultiCriteriaScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-sorted-multi-criteria" and
    (.records | type == "array" and length == 1 and .[0].operation == "listener") and
    .records[0].case == "multi-criteria" and
    .records[0].statement == "s0" and
    .records[0].sequence == 1
' "$output" >/dev/null 2>&1; then
    echo "Java oracle did not produce exactly one listener trace record: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
