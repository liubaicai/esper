#!/usr/bin/env sh
set -eu

esper_root=
scenario=
output=
skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

while [ "$#" -gt 0 ]; do
    case "$1" in
        --esper-root) esper_root=$2; shift 2 ;;
        --scenario) scenario=$2; shift 2 ;;
        --output) output=$2; shift 2 ;;
        --skip-build) skip_build=1; shift ;;
        *) echo "unknown arg: $1" >&2; exit 1 ;;
    esac
done
[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null)
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper root commit $actual_commit != expected $expected_commit" >&2
    exit 1
fi

java_bin=${JAVA:-java}
javac_bin=${JAVAC:-javac}
mvn_bin=${MAVEN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then
    java_bin=${JAVA_HOME}/bin/java
    javac_bin=${JAVA_HOME}/bin/javac
    mvn_bin=${MAVEN_HOME:-${JAVA_HOME}}/bin/mvn
fi
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable not found" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "Javac executable not found" >&2; exit 1; }
command -v "$mvn_bin" >/dev/null 2>&1 || { echo "Maven executable not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq required" >&2; exit 1; }

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/common/pom.xml" compile -Dgpg.skip=true -Dfile.encoding=UTF-8
    "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" compile -Dgpg.skip=true -Dfile.encoding=UTF-8
    "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" compile -Dgpg.skip=true -Dfile.encoding=UTF-8
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-other-distinct.XXXXXX")
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
    "$script_root/EPOtherDistinctScenarioOracle.java"

parent=$(dirname "$output")
mkdir -p "$parent"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPOtherDistinctScenarioOracle "$scenario" > "$output"

if ! jq -e '.version == "esper-parity/v1" and (.id | length > 0) and (.records | type == "array")' "$output" >/dev/null 2>&1; then
    echo "Invalid trace output: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit output=$output"
