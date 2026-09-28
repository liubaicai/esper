#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-everydistinct-compound-577.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternEveryDistinctCompound577ScenarioOracle against the
pinned Esper 9.0.0 checkout (javaCommit
9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
PatternOperatorEveryDistinct ords 8-12 — the compound-operator quintet
(PatternEveryDistinctOverAnd, PatternEveryDistinctOverOr,
PatternEveryDistinctOverNot, PatternEveryDistinctOverFollowedBy,
PatternEveryDistinctWithinFollowedBy) as ten cases (two expiry legs per
ord) — and writing the normalized Java trace to --output.
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
    echo "Esper commit mismatch: expected $expected_commit, found $actual_commit" >&2
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
    .id == "pattern-everydistinct-compound-577" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternOperatorEveryDistinct.java" and
    (.javaSourceFiles | type == "array" and length == 2) and
    (.javaRuntimes == ["java-runtime-fa2df3aeba01a5eda1d2", "java-runtime-6aab27f9a29d3ad7188f", "java-runtime-c23dddb4d0d001809ffd", "java-runtime-c28f82ea566795672292", "java-runtime-58fcea53e80425896c2d"]) and
    (.javaNames == ["PatternEveryDistinctOverAnd", "PatternEveryDistinctOverOr", "PatternEveryDistinctOverNot", "PatternEveryDistinctOverFollowedBy", "PatternEveryDistinctWithinFollowedBy"]) and
    (.javaStaticIds == ["java-073091a0b42ca8dd974f", "java-073091a0b42ca8dd974f", "java-073091a0b42ca8dd974f", "java-073091a0b42ca8dd974f", "java-073091a0b42ca8dd974f"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 10) and
    ([.cases[].case] == ["over-and", "over-and-expiry", "over-or", "over-or-expiry", "over-not", "over-not-expiry", "over-followed-by", "over-followed-by-expiry", "within-followed-by", "within-followed-by-expiry"]) and
    ([.cases[].ordinal] == [8, 8, 9, 9, 10, 10, 11, 11, 12, 12]) and
    (.steps | type == "array" and length == 138) and
    ([.steps[] | select(.op == "case")] | length == 10) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 10) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 108) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 10) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-everydistinct-compound-577 replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-everydistinct-compound-577.XXXXXX")
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

classes="$work/classes"
mkdir -p "$classes"
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    CYGWIN*|MINGW*|MSYS*) cp_sep=";" ;;
    *) cp_sep=":" ;;
esac
native_path() {
    if [ "$cp_sep" = ";" ]; then cygpath -w "$1"; else printf '%s' "$1"; fi
}

compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/PatternEveryDistinctCompound577ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternEveryDistinctCompound577ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-everydistinct-compound-577" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 34) and
    ([.records[] | select(.operation == "listener")] | length == 34) and
    ([.records[] | select(.case == "over-and")] | length == 4) and
    ([.records[] | select(.case == "over-and-expiry")] | length == 4) and
    ([.records[] | select(.case == "over-or")] | length == 4) and
    ([.records[] | select(.case == "over-or-expiry")] | length == 4) and
    ([.records[] | select(.case == "over-not")] | length == 3) and
    ([.records[] | select(.case == "over-not-expiry")] | length == 3) and
    ([.records[] | select(.case == "over-followed-by")] | length == 2) and
    ([.records[] | select(.case == "over-followed-by-expiry")] | length == 2) and
    ([.records[] | select(.case == "within-followed-by")] | length == 4) and
    ([.records[] | select(.case == "within-followed-by-expiry")] | length == 4)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
