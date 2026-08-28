#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
Usage: $0 --esper-root <path> --scenario <json> --output <trace.json>
EOF
  exit 1
}

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
  case "$1" in
    --esper-root) [ "$#" -ge 2 ] || usage; esper_root="$2"; shift 2;;
    --scenario) [ "$#" -ge 2 ] || usage; scenario="$2"; shift 2;;
    --output) [ "$#" -ge 2 ] || usage; output="$2"; shift 2;;
    --skip-build) skip_build=1; shift;;
    *) usage;;
  esac
done

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read HEAD" >&2; exit 1; }
if [ "$actual_commit" != "$expected_commit" ]; then
  echo "Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
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

java_version=$($java_bin -version 2>&1 | awk -F'[\".]' '/version/ {print $2; exit}')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-querytype-row-for-all" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.cases | type == "array") and
    ((.cases | length) == 5) and
    ([.cases[] | select(.ordinal == 4 or .ordinal == 5 or .ordinal == 6 or .ordinal == 7 or .ordinal == 8)] | length) == 5 and
    ([.cases[] | select(.observation == "listener")] | length) == 5 and
    ([.cases[] | select(.runtimeId | startswith("java-runtime-"))] | length) == 5 and
    ([.cases[] | select(.executionName | startswith("ResultSetQueryTypeRowForAll"))] | length) == 5 and
    (.steps | type == "array") and
    ([.steps[] | select(.op == "case")] | length) == 5 and
    ([.steps[] | select(.op == "case")] | map(.case) | unique | length) == 5 and
    ([.steps[] | select(.op == "send")] | length) == 14 and
    ([.steps[] | select(.op == "advance-time")] | length) == 14 and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0" and (.label | length > 0) and (.mode == "fifo" or .mode == "any"))] | length) == 14
' "$scenario" >/dev/null 2>&1; then
  echo "scenario is not a valid resultset-querytype-row-for-all replay: $scenario" >&2
  exit 1
fi

if [ "$skip_build" -eq 0 ]; then
  "$mvn_bin" -q -f "$esper_root/common/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" install -DskipTests -Dgpg.skip=true
  "$mvn_bin" -q -f "$esper_root/regression-lib/pom.xml" install -DskipTests -Dgpg.skip=true
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-querytype-row-for-all.XXXXXX")
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
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/ResultSetQueryTypeRowForAllScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    ResultSetQueryTypeRowForAllScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "resultset-querytype-row-for-all" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array") and
    ((.records | length) == 33) and
    ([.records[] | select(.case == "sum-one-view" and .operation == "listener")] | length) == 6 and
    ([.records[] | select(.case == "sum-one-view" and .operation == "snapshot")] | length) == 7 and
    ([.records[] | select(.case == "sum-join" and .operation == "listener")] | length) == 6 and
    ([.records[] | select(.case == "sum-join" and .operation == "snapshot")] | length) == 7 and
    ([.records[] | select(.case == "avg-per-sym" and .operation == "listener")] | length) == 5 and
    ([.records[] | select(.case == "select-star-std-group-by" and .operation == "listener")] | length) == 1 and
    ([.records[] | select(.case == "select-expr-group-win" and .operation == "listener")] | length) == 1 and
    ([.records[] | select(.time | startswith("1970-01-01T"))] | length) == 33 and
    ([.records[] | select(.case == "select-star-std-group-by" and (.new[0].fields.__type == "SupportMarketDataBean"))] | length) == 1
' "$output" >/dev/null 2>&1; then
  echo "Java oracle produced an invalid trace: $output" >&2
  exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
