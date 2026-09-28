#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-matchuntil-untimed-585.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternMatchUntilUntimed585ScenarioOracle against the pinned
Esper 9.0.0 checkout (javaCommit 9e1b9f1cc9117fea4bf33ab043762c045d73839c),
replaying PatternOperatorMatchUntil ords 0/2/3/5 — the untimed
until-array quad: PatternMatchUntilSimple (capital-@Name until with the
permanent-false tail), PatternSelectArray (explicit a/b/a[0..2]+id leg
plus the select * wildcard leg), PatternUseFilter (five select-* legs
correlating the follow-on c against a[i] via concat/equals/in/not-in/
between) and PatternArrayFunctionRepeat ([1:] bound, untagged
terminator, arrayLength + Array.getLength columns) — with no clock ops,
writing the normalized Java trace to --output. The trace carries nine
records: one per firing leg (1+2+5+1).
EOF
}

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
        -h|--help) usage; exit 0 ;;
        *) usage; exit 2 ;;
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
    echo "Esper checkout is $actual_commit, want $expected_commit" >&2; exit 1
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
    .id == "pattern-matchuntil-untimed-585" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorMatchUntil.java" and
    (.javaSourceFiles | type == "array" and length == 5) and
    (.javaRuntimes == ["java-runtime-93ea4ae0a2ca85d18a0d", "java-runtime-bd06e4f21e0083fb261d", "java-runtime-4b8c99341f4a3af06ea9", "java-runtime-602d438d756709deb50e"]) and
    (.javaNames == ["PatternMatchUntilSimple", "PatternSelectArray", "PatternUseFilter", "PatternArrayFunctionRepeat"]) and
    (.javaStaticIds == ["java-2169accb9d43755e87e4", "java-c393b406de406e3ffd1f", "java-bf3211b9b2822d0cbbf4", "java-7a5adcaeaa460c4641cb"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["simple", "select-array", "use-filter", "array-function-repeat"]) and
    ([.cases[].ordinal] == [0, 2, 3, 5]) and
    (.steps | type == "array" and length == 66) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 9) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 17) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 18) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_C")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 9) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-matchuntil-untimed-585 replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-matchuntil-untimed-585.XXXXXX")
cleanup() {
    rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
# common-avro is required at COMPILE time: the ord 5 EPL imports
# SupportStaticMethodLib whose public methods reference Avro types, so
# resolveMethodOverloadChecked needs the avro jar + module classes.
"$mvn_bin" -q -f "$esper_root/common-avro/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/avro-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    CYGWIN*|MINGW*|MSYS*) cp_sep=';' ;;
    *) cp_sep=':' ;;
esac
native_path() {
    if [ "$cp_sep" = ";" ]; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
avro_cp=$(tr -d '\r\n' < "$work/avro-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp$cp_sep$avro_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/PatternMatchUntilUntimed585ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternMatchUntilUntimed585ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-matchuntil-untimed-585" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 9) and
    ([.records[] | select(.operation == "listener")] | length == 9) and
    ([.records[] | select(.case == "simple")] | length == 1) and
    ([.records[] | select(.case == "select-array")] | length == 2) and
    ([.records[] | select(.case == "use-filter")] | length == 5) and
    ([.records[] | select(.case == "array-function-repeat")] | length == 1) and
    ([.records[] | select((.new | length) == 1)] | length == 9)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
