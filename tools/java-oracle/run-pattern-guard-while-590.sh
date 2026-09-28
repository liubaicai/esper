#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-pattern-guard-while-590.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the PatternGuardWhile590ScenarioOracle against the pinned
Esper 9.0.0 checkout (javaCommit 9e1b9f1cc9117fea4bf33ab043762c045d73839c),
replaying PatternGuardWhile ords 0-3 — PatternGuardWhileSimple's
deploy-plus-send falsify sequence (E1/E2 fire, the first X match quits
the while-guard permanently so E3 and the trailing X stay silent),
PatternOp's five-leg while-guard PatternTestHarness over
EventCollectionFactory.getEventSetOne(0,1000) (all five legs deploy
inside ONE deploy-all step — legs S0/S1/S2/S4 as `@name("S<i>")
select * from pattern [<atom>]` plus S3 through the SODA model path
whose toEPL keeps `b.id!="B3"` — then twelve advance-before-send
steps and the undeploy-all kill-resend), PatternVariable's
create-variable + runtimeSetVariable('var','myVariable',false)
reconfiguration (B1 fans out one row per live every-a branch, the
flip falsifies every live branch) — and PatternInvalid's two
compile-error probes — and writing the normalized Java trace to
--output.
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
    echo "Esper checkout is at $actual_commit, expected $expected_commit" >&2
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
    .id == "pattern-guard-while-590" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/pattern/PatternGuardWhile.java" and
    (.javaSourceFiles | type == "array" and length == 12) and
    (.javaRuntimes == ["java-runtime-aa6c31e987ec865a8cf2", "java-runtime-bf4b4c7d66f189e40a73", "java-runtime-8808f29a3bfdd475589f", "java-runtime-4416cc3367e38623077a"]) and
    (.javaNames == ["PatternGuardWhileSimple", "PatternOp", "PatternVariable", "PatternInvalid"]) and
    (.javaStaticIds == ["java-2be6b5626499eeff834e", "java-2be6b5626499eeff834e", "java-2be6b5626499eeff834e", "java-2be6b5626499eeff834e"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["simple", "pattern-op", "pattern-variable", "pattern-invalid"]) and
    ([.cases[].ordinal] == [0, 1, 2, 3]) and
    ([.cases[].epls | length] == [1, 5, 2, 2]) and
    (.steps | type == "array" and length == 37) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "all")] | length == 1) and
    ([.steps[] | select(.op == "send")] | length == 23) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 11) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_C")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_D")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_E")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_F")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_G")] | length == 1) and
    ([.steps[] | select(.op == "send" and has("at"))] | length == 12) and
    ([.steps[] | select(.op == "set-variable" and .statement == "var" and .name == "myVariable" and .payload == false)] | length == 1) and
    ([.steps[] | select(.op == "build-error")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "set-variable" and .op != "build-error" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid pattern-guard-while-590 replay: $scenario" >&2
    exit 1
fi
if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/pom.xml" -pl common,compiler,runtime,regression-lib \
        -am install -DskipTests -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-pattern-guard-while-590.XXXXXX")
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
    "$script_root/PatternGuardWhile590ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    PatternGuardWhile590ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "pattern-guard-while-590" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 12) and
    ([.records[] | select(.case == "simple" and .operation == "listener" and .statement == "s0")] | length == 2) and
    ([.records[] | select(.case == "pattern-op" and .operation == "listener")] | length == 7) and
    ([.records[] | select(.case == "pattern-op" and .statement == "S0")] | length == 1) and
    ([.records[] | select(.case == "pattern-op" and .statement == "S1")] | length == 2) and
    ([.records[] | select(.case == "pattern-op" and .statement == "S2")] | length == 2) and
    ([.records[] | select(.case == "pattern-op" and .statement == "S3")] | length == 2) and
    ([.records[] | select(.case == "pattern-op" and .statement == "S4")] | length == 0) and
    ([.records[] | select(.case == "pattern-op" and .time == "1970-01-01T00:00:02Z")] | length == 4) and
    ([.records[] | select(.case == "pattern-op" and .time == "1970-01-01T00:00:04Z")] | length == 3) and
    ([.records[] | select(.case == "pattern-variable" and .operation == "listener" and .statement == "s0")] | length == 1) and
    ([.records[] | select(.case == "pattern-variable" and (.new | length == 2))] | length == 1) and
    ([.records[] | select(.case == "pattern-invalid" and .operation == "compile-error")] | length == 2)
' "$output" >/dev/null 2>&1; then
    echo "oracle trace failed validation: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
