#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-resultset-aggregate-sorted-first-last.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
SCENARIO_ID = "resultset-aggregate-sorted-first-last"
DESCRIPTION = "ResultSetAggregationMethodSorted ordinals 5-6: table-backed sorted first/last event, bucket, and key access with enumeration projections."
JAVA_COMMIT = "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
JAVA_SOURCE = "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/aggregate/ResultSetAggregationMethodSorted.java"
RUNTIME_IDS = ["java-runtime-2ec322fbd681b83590f1", "java-runtime-b7e36a11fb9b9c982249"]
EXECUTION_NAMES = ["ResultSetAggregateSortedFirstLast", "ResultSetAggregateSortedFirstLastEnumerationAndDot"]
STATIC_IDS = ["java-0ded2b32c81677a9d36f", "java-5eb997e432653280b46b"]
CASES = ["first-last", "first-last-dot"]
ORDINALS = [5, 6]
EPLS = [
    "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
    "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
    "@name('s0') select MyTable.sortcol.firstEvent() as fe,MyTable.sortcol.minBy() as minb,"
    "MyTable.sortcol.firstEvents() as fes,MyTable.sortcol.firstKey() as fk,"
    "MyTable.sortcol.lastEvent() as le,MyTable.sortcol.maxBy() as maxb,"
    "MyTable.sortcol.lastEvents() as les,MyTable.sortcol.lastKey() as lk from SupportBean_S0",
    "create table MyTable(sortcol sorted(intPrimitive) @type('SupportBean'));\n"
    "into table MyTable select sorted(*) as sortcol from SupportBean;\n"
    "@name('s0') select MyTable.sortcol.firstEvent().theString as feid,"
    "MyTable.sortcol.firstEvent().firstOf() as fefo,"
    "MyTable.sortcol.firstEvents().lastOf() as feslo,"
    "MyTable.sortcol.lastEvent().theString() as leid,"
    "MyTable.sortcol.lastEvent().firstOf() as lefo,"
    "MyTable.sortcol.lastEvents().lastOf as leslo from SupportBean_S0",
]
SEEDS = [("E1a", 1), ("E1b", 1), ("E4b", 4), ("E6a", 6), ("E6b", 6), ("E8", 8), ("E9", 9)]


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


def validate_bean(value, expected_string, expected_int):
    if not exact_object(value, ("op", "eventType", "payload")):
        raise ValueError("SupportBean step has unexpected fields")
    if value["op"] != "send" or value["eventType"] != "SupportBean":
        raise ValueError("SupportBean step is not pinned")
    payload = value["payload"]
    if not exact_object(payload, ("theString", "intPrimitive")):
        raise ValueError("SupportBean payload has unexpected fields")
    if payload["theString"] != expected_string:
        raise ValueError("SupportBean theString sequence mismatch")
    if not valid_integer(payload["intPrimitive"], -(2**31), 2**31 - 1):
        raise ValueError("SupportBean intPrimitive must be a Java int")
    if int(payload["intPrimitive"][1], 10) != expected_int:
        raise ValueError("SupportBean intPrimitive sequence mismatch")


def validate_trigger(value):
    if not exact_object(value, ("op", "eventType", "payload")):
        raise ValueError("SupportBean_S0 step has unexpected fields")
    if value["op"] != "send" or value["eventType"] != "SupportBean_S0":
        raise ValueError("SupportBean_S0 step is not pinned")
    payload = value["payload"]
    if not exact_object(payload, ("id",)):
        raise ValueError("SupportBean_S0 payload has unexpected fields")
    if not valid_integer(payload["id"], -(2**31), 2**31 - 1) or int(payload["id"][1], 10) != -1:
        raise ValueError("SupportBean_S0 id must be the Java int -1")


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
    if scenario["javaRuntimes"] != RUNTIME_IDS or scenario["javaNames"] != EXECUTION_NAMES:
        raise ValueError("Java runtime/name metadata is not pinned")
    if scenario["javaStaticIds"] != STATIC_IDS or scenario["javaFlags"] != []:
        raise ValueError("Java static/flag metadata is not pinned")

    cases = scenario["cases"]
    if not isinstance(cases, list) or len(cases) != 2:
        raise ValueError("scenario must contain exactly two cases")
    for index, case in enumerate(cases):
        if not exact_object(case, ("case", "ordinal", "runtimeId", "executionName", "observation", "iteratorSnapshots", "epl")):
            raise ValueError("case metadata has unexpected fields")
        if (case["case"] != CASES[index] or case["ordinal"] != ("integer", str(ORDINALS[index]))
                or case["runtimeId"] != RUNTIME_IDS[index] or case["executionName"] != EXECUTION_NAMES[index]
                or case["observation"] != "listener" or case["iteratorSnapshots"] != ("integer", "0")
                or case["epl"] != EPLS[index]):
            raise ValueError("case metadata is not pinned")

    steps = scenario["steps"]
    if not isinstance(steps, list) or len(steps) != 18:
        raise ValueError("scenario must contain exactly eighteen steps")
    for case_index in range(2):
        base = case_index * 9
        marker = steps[base]
        if not exact_object(marker, ("op", "case")) or marker["op"] != "case" or marker["case"] != CASES[case_index]:
            raise ValueError("case marker is not pinned")
        for seed_index, seed in enumerate(SEEDS):
            validate_bean(steps[base + 1 + seed_index], seed[0], seed[1])
        validate_trigger(steps[base + 8])
except (OSError, ValueError, TypeError, KeyError, IndexError, AttributeError):
    raise SystemExit(1)
PY
then
    echo "scenario is not a valid resultset-aggregate-sorted-first-last replay: $scenario" >&2
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

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-aggregate-sorted-first-last.XXXXXX")
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
    "$script_root/ResultSetAggregationMethodSortedFirstLastScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetAggregationMethodSortedFirstLastScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-aggregate-sorted-first-last" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 2) and
    ([.records[].case] == ["first-last", "first-last-dot"]) and
    ([.records[].operation] | all(. == "listener")) and
    ([.records[].statement] | all(. == "s0")) and
    ([.records[].sequence] == [1, 1]) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.old != null)] | length == 0) and
    ([.records[].new] | all(type == "array" and length == 1)) and
    ([.records[].new[0].kind] | all(. == "row")) and
    .records[0].new[0].fields.fe == {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1a"}} and
    .records[0].new[0].fields.minb == {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1a"}} and
    .records[0].new[0].fields.fes == [
        {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1a"}},
        {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1b"}}
    ] and
    .records[0].new[0].fields.fk == 1 and
    .records[0].new[0].fields.le == {"kind": "row", "fields": {"intPrimitive": 9, "theString": "E9"}} and
    .records[0].new[0].fields.maxb == {"kind": "row", "fields": {"intPrimitive": 9, "theString": "E9"}} and
    .records[0].new[0].fields.les == [{"kind": "row", "fields": {"intPrimitive": 9, "theString": "E9"}}] and
    .records[0].new[0].fields.lk == 9 and
    .records[1].new[0].fields.feid == "E1a" and
    .records[1].new[0].fields.fefo == {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1a"}} and
    .records[1].new[0].fields.feslo == {"kind": "row", "fields": {"intPrimitive": 1, "theString": "E1b"}} and
    .records[1].new[0].fields.leid == "E9" and
    .records[1].new[0].fields.lefo == {"kind": "row", "fields": {"intPrimitive": 9, "theString": "E9"}} and
    .records[1].new[0].fields.leslo == {"kind": "row", "fields": {"intPrimitive": 9, "theString": "E9"}}
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid resultset-aggregate-sorted-first-last trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
