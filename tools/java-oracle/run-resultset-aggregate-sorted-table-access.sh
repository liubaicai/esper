#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-sorted-table-access.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java 9.0.0 oracle commit.
Java 17 and Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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
[ "$java_version" = "17" ] || { echo "Java 17 is required for the Java 9.0.0 oracle; found ${java_version:-unknown}" >&2; exit 1; }

if ! python3 - "$scenario" <<'PY'
import json
import sys

VERSION = "esper-parity/v1"
SCENARIO_ID = "resultset-aggregate-sorted-table-access"
DESCRIPTION = "ResultSetAggregationMethodSorted ordinals 7-9: table-backed sorted get/contains/counts, inclusive submap/eventsBetween ranges, and detached navigable-map projections over duplicate-key buckets."
JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
JAVA_SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java"
RUNTIMES = [
    "java-runtime-d7d60fec056d6eb16659",
    "java-runtime-0c0e45c751e3cafeb454",
    "java-runtime-2117ba8232651a61a058",
]
EXECUTIONS = [
    "ResultSetAggregateSortedGetContainsCounts",
    "ResultSetAggregateSortedSubmapEventsBetween",
    "ResultSetAggregateSortedNavigableMapReference",
]
STATIC_IDS = [
    "java-ad219f9a27aebc7885fc",
    "java-4b989f8ba297c8e8d426",
    "java-db7b9e752ef9e4000cc5",
]
CASES = ["get-contains-counts", "submap-events-between", "navigable-map-reference"]
SEND_VALUES = [("E1a", 1), ("E1b", 1), ("E4b", 4), ("E6a", 6), ("E6b", 6), ("E8", 8), ("E9", 9)]
GET_EPL = (
    "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
    "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
    "@name('s0') select "
    "MyTable.sortcol.getEvent(id) as ge,"
    "MyTable.sortcol.getEvents(id) as ges,"
    "MyTable.sortcol.containsKey(id) as ck,"
    "MyTable.sortcol.countEvents() as cnte,"
    "MyTable.sortcol.countKeys() as cntk,"
    "MyTable.sortcol.getEvent(id).theString as geid,"
    "MyTable.sortcol.getEvent(id).firstOf() as gefo,"
    "MyTable.sortcol.getEvents(id).lastOf() as geslo"
    " from SupportBean_S0"
)
SUBMAP_EPL = (
    "@public @buseventtype create schema MySubmapEvent as ResultSetAggregationMethodSortedTableAccessScenarioOracle$MySubmapEvent;\n"
    "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
    "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
    "@name('s0') select "
    "MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive) as eb,"
    "MyTable.sortcol.eventsBetween(fromKey, fromInclusive, toKey, toInclusive).lastOf() as eblastof,"
    "MyTable.sortcol.subMap(fromKey, fromInclusive, toKey, toInclusive) as sm"
    " from MySubmapEvent"
)
NAVIGABLE_EPL = (
    "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
    "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
    "@name('s0') select MyTable.sortcol.navigableMapReference() as nmr from SupportBean_S0"
)
EPLS = [GET_EPL, SUBMAP_EPL, NAVIGABLE_EPL]


def reject_duplicate_keys(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON object key")
        result[key] = value
    return result


def parse_integer(value):
    return ("integer", value)


def parse_float(value):
    return ("non-integer", value)


def parse_constant(value):
    raise ValueError("non-JSON numeric constant")


def exact_object(value, names):
    return isinstance(value, dict) and set(value) == set(names)


def valid_integer(value, lower, upper):
    if not isinstance(value, tuple) or len(value) != 2 or value[0] != "integer":
        return False
    text = value[1]
    if not isinstance(text, str) or not text.isascii():
        return False
    digits = text[1:] if text.startswith("-") else text
    if not digits or (len(digits) > 1 and digits[0] == "0") or not digits.isdigit():
        return False
    try:
        parsed = int(text, 10)
    except ValueError:
        return False
    return lower <= parsed <= upper


def valid_boolean(value):
    return isinstance(value, bool)


def expected_case(case_index):
    return {
        "case": CASES[case_index],
        "ordinal": ("integer", str(7 + case_index)),
        "runtimeId": RUNTIMES[case_index],
        "executionName": EXECUTIONS[case_index],
        "observation": "listener",
        "iteratorSnapshots": ("integer", "0"),
        "epl": EPLS[case_index],
    }


def validate_case_metadata(case_index, value):
    if not exact_object(value, ("case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl")):
        raise ValueError("case metadata has unexpected fields")
    expected = expected_case(case_index)
    if value != expected:
        raise ValueError("case metadata is not pinned")


def validate_marker(value, case_name):
    if not exact_object(value, ("op", "case")) or value["op"] != "case" or value["case"] != case_name:
        raise ValueError("case marker is not pinned")


def validate_bean(value, expected):
    if not exact_object(value, ("op", "eventType", "payload")) or value["op"] != "send" or value["eventType"] != "SupportBean":
        raise ValueError("SupportBean step is not pinned")
    payload = value["payload"]
    if not exact_object(payload, ("theString", "intPrimitive")) or payload["theString"] != expected[0]:
        raise ValueError("SupportBean payload is not pinned")
    if not valid_integer(payload["intPrimitive"], -(2**31), 2**31 - 1) or int(payload["intPrimitive"][1], 10) != expected[1]:
        raise ValueError("SupportBean intPrimitive is not pinned")


def validate_trigger(value, expected_id):
    if not exact_object(value, ("op", "eventType", "payload")) or value["op"] != "send" or value["eventType"] != "SupportBean_S0":
        raise ValueError("SupportBean_S0 step is not pinned")
    payload = value["payload"]
    if not exact_object(payload, ("id",)) or not valid_integer(payload["id"], -(2**31), 2**31 - 1):
        raise ValueError("SupportBean_S0 payload is not pinned")
    if int(payload["id"][1], 10) != expected_id:
        raise ValueError("SupportBean_S0 id is not pinned")


def validate_submap(value, from_key, from_inclusive, to_key, to_inclusive):
    if not exact_object(value, ("op", "eventType", "payload")) or value["op"] != "send" or value["eventType"] != "MySubmapEvent":
        raise ValueError("MySubmapEvent step is not pinned")
    payload = value["payload"]
    if not exact_object(payload, ("fromKey", "fromInclusive", "toKey", "toInclusive")):
        raise ValueError("MySubmapEvent payload has unexpected fields")
    if (not valid_integer(payload["fromKey"], -(2**31), 2**31 - 1)
            or int(payload["fromKey"][1], 10) != from_key
            or not valid_boolean(payload["fromInclusive"])
            or payload["fromInclusive"] != from_inclusive
            or not valid_integer(payload["toKey"], -(2**31), 2**31 - 1)
            or int(payload["toKey"][1], 10) != to_key
            or not valid_boolean(payload["toInclusive"])
            or payload["toInclusive"] != to_inclusive):
        raise ValueError("MySubmapEvent payload is not pinned")


try:
    with open(sys.argv[1], encoding="utf-8") as handle:
        scenario = json.load(handle, object_pairs_hook=reject_duplicate_keys,
                             parse_int=parse_integer, parse_float=parse_float,
                             parse_constant=parse_constant)
    required = ("version", "id", "description", "javaCommit", "javaSource", "javaRuntimes",
                "javaNames", "javaStaticIds", "javaFlags", "cases", "steps")
    if not exact_object(scenario, required):
        raise ValueError("scenario has unexpected fields")
    if scenario["version"] != VERSION or scenario["id"] != SCENARIO_ID or scenario["description"] != DESCRIPTION:
        raise ValueError("scenario identity or description is not pinned")
    if scenario["javaCommit"] != JAVA_COMMIT or scenario["javaSource"] != JAVA_SOURCE:
        raise ValueError("Java source metadata is not pinned")
    if scenario["javaRuntimes"] != RUNTIMES or scenario["javaNames"] != EXECUTIONS or scenario["javaStaticIds"] != STATIC_IDS or scenario["javaFlags"] != []:
        raise ValueError("Java references are not pinned")
    if not isinstance(scenario["cases"], list) or len(scenario["cases"]) != 3:
        raise ValueError("scenario must contain exactly three cases")
    for case_index, value in enumerate(scenario["cases"]):
        validate_case_metadata(case_index, value)

    steps = scenario["steps"]
    submap_count = sum(4 for start in range(12) for end in range(start, 12))
    expected_steps = (1 + len(SEND_VALUES) + 12) + (1 + len(SEND_VALUES) + submap_count) + (1 + len(SEND_VALUES) + 1)
    if not isinstance(steps, list) or len(steps) != expected_steps:
        raise ValueError("scenario step count is not pinned")
    index = 0
    validate_marker(steps[index], CASES[0]); index += 1
    for expected in SEND_VALUES:
        validate_bean(steps[index], expected); index += 1
    for expected_id in range(12):
        validate_trigger(steps[index], expected_id); index += 1

    validate_marker(steps[index], CASES[1]); index += 1
    for expected in SEND_VALUES:
        validate_bean(steps[index], expected); index += 1
    for from_key in range(12):
        for to_key in range(from_key, 12):
            for from_inclusive in (False, True):
                for to_inclusive in (False, True):
                    validate_submap(steps[index], from_key, from_inclusive, to_key, to_inclusive)
                    index += 1

    validate_marker(steps[index], CASES[2]); index += 1
    for expected in SEND_VALUES:
        validate_bean(steps[index], expected); index += 1
    validate_trigger(steps[index], -1); index += 1
    if index != len(steps):
        raise ValueError("scenario has trailing steps")
except (OSError, ValueError, TypeError, KeyError, IndexError, AttributeError):
    raise SystemExit(1)
PY
then
    echo "scenario is not a valid resultset-aggregate-sorted-table-access replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-sorted-table-access.XXXXXX")
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
    "$script_root/ResultSetAggregationMethodSortedTableAccessScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregationMethodSortedTableAccessScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-sorted-table-access" and
    (.records | type == "array" and length == 325) and
    ([.records[].operation] | all(. == "listener"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid sorted table-access trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
