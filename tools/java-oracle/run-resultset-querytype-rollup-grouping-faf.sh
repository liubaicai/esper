#!/usr/bin/env sh
set -eu
usage() { echo "usage: $0 --esper-root DIR --scenario FILE --output FILE [--skip-build]" >&2; exit 2; }
esper_root=; scenario=; output=; skip_build=0; expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c; script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
while [ "$#" -gt 0 ]; do
  case "$1" in
    --esper-root) [ "$#" -ge 2 ] || usage; esper_root=$2; shift 2;;
    --scenario) [ "$#" -ge 2 ] || usage; scenario=$2; shift 2;;
    --output) [ "$#" -ge 2 ] || usage; output=$2; shift 2;;
    --skip-build) skip_build=1; shift;;
    --expected-commit) echo "unknown argument: $1" >&2; usage;;
    -h|--help) usage;; *) echo "unknown argument: $1" >&2; usage;;
  esac
done
[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read Esper HEAD" >&2; exit 1; }
[ "$actual_commit" = "$expected_commit" ] || { echo "Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2; exit 1; }
java_bin=${JAVA:-java}; javac_bin=${JAVAC:-javac}; mvn_bin=${MAVEN:-mvn}
[ -z "${JAVA_HOME:-}" ] || { java_bin="$JAVA_HOME/bin/java"; javac_bin="$JAVA_HOME/bin/javac"; }
[ -z "${MAVEN_HOME:-}" ] || mvn_bin="$MAVEN_HOME/bin/mvn"
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable was not found: $java_bin" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac executable was not found: $javac_bin" >&2; exit 1; }
command -v "$mvn_bin" >/dev/null 2>&1 || { echo "Maven executable was not found: $mvn_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found" >&2; exit 1; }
java_version=$($java_bin -version 2>&1 | awk -F'[\".]' '/version/ {print $2; exit}')
[ "$java_version" = 17 ] || { echo "Java 17 is required; found $java_version" >&2; exit 1; }
if [ "$skip_build" -eq 0 ]; then
  "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi
work=$(mktemp -d "${TMPDIR:-/tmp}/esper-rollup-grouping-faf.XXXXXX"); cleanup(){ rm -rf "$work"; }; trap cleanup EXIT HUP INT TERM
"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
classes="$work/classes"; mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" "$script_root/ResultSetQueryTypeRollupGroupingFAFScenarioOracle.java"
mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en -Duser.country=US -Duser.variant= -cp "$classpath" ResultSetQueryTypeRollupGroupingFAFScenarioOracle "$scenario" > "$output"
jq -e '.version == "esper-parity/v1" and .id == "rollup-grouping-funcs-faf-dedicated" and (.records | type == "array") and ((.records | length) == 1)' "$output" >/dev/null
echo "javaCommit=$actual_commit java=$java_version output=$output"
