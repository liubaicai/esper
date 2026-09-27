#!/usr/bin/env sh
set -eu

usage() {
    cat <<'EOF' >&2
usage: run-infra-nwtable-widening-558.sh --esper-root DIR --scenario FILE --output FILE [--skip-build]

Runs the InfraNWTableWidening558ScenarioOracle against the pinned Esper 9.0.0
checkout (javaCommit 9e1b9f1cc9117fea4bf33ab043762c045d73839c), replaying
InfraNWTableCreateIndex ordinals 2-5 (InfraHashBTreeWidening and InfraWidening,
namedWindow true/false) and writing the normalized Java trace to --output.
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
        --help|-h) usage; exit 0 ;;
        *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
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
    echo "Esper commit drift: expected $expected_commit, got $actual_commit" >&2
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
    .id == "infra-nwtable-widening-558" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableCreateIndex.java" and
    (.javaRuntimes == ["java-runtime-ea8f73c9bd9fc31f1312", "java-runtime-c29b248899a0388e0768", "java-runtime-f4b0d37d40e38914df36", "java-runtime-2bffd7c0683d9195c556"]) and
    (.javaNames == ["InfraHashBTreeWidening{namedWindow=true}", "InfraHashBTreeWidening{namedWindow=false}", "InfraWidening{namedWindow=true}", "InfraWidening{namedWindow=false}"]) and
    (.javaStaticIds == ["java-06a59928c63cc7360d2b", "java-06a59928c63cc7360d2b", "java-06a59928c63cc7360d2b", "java-06a59928c63cc7360d2b"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["hashbtree-window", "hashbtree-table", "widening-window", "widening-table"]) and
    ([.cases[].ordinal] == [2, 3, 4, 5]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 76) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 24) and
    ([.steps[] | select(.op == "deployed")] | length == 24) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 8) and
    ([.steps[] | select(.op == "snapshot" and (.epl | contains("MyInfraHBTW")))] | length == 4) and
    ([.steps[] | select(.op == "snapshot" and (.epl | contains("MyInfraW")))] | length == 4) and
    ([.steps[] | select(.op == "unrepresentable" and .statement == "soda-ix1")] | length == 2) and
    ([.steps[] | select(.op == "unrepresentable" and .statement == "soda-ix2")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "snapshot" and .op != "unrepresentable" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-widening-558 replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime,regression-lib -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-widening-558.XXXXXX")
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
# Windows javac/java need ';' separators and native paths; the Maven
# dependency classpath files already use the platform separator.
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        cp_sep=';'
        native_path() { cygpath -w "$1"; }
        ;;
    *)
        cp_sep=':'
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/regression-lib/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/avro-1.11.3.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-annotations-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-core-2.14.2.jar")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/lib/jackson-databind-2.14.2.jar")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableWidening558ScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableWidening558ScenarioOracle "$scenario" > "$output"
if ! jq -e '
    def proberecords($case):
        ([.records[] | select(.case == $case and .operation == "deployed")] | length == 6) and
        ([.records[] | select(.case == $case and .operation == "deployed") | .statement]
            == ["create-one", "insert-one", "index-one", "create-two", "insert-two", "index-two"]) and
        ([.records[] | select(.case == $case and .operation == "deployed") | .sequence]
            | all(. == 1)) and
        ([.records[] | select(.case == $case and .operation == "snapshot")] | length == 2) and
        ([.records[] | select(.case == $case and .operation == "snapshot") | .new | length == 1
            and .[0].kind == "row" and .[0].fields.f2 == "E1"] | all);
    def legrecords($case; $longop; $longval; $shortop):
        ([.records[] | select(.case == $case and .operation == "snapshot" and .statement == $longop)
            | .new[0].fields.f1] == [$longval]) and
        ([.records[] | select(.case == $case and .operation == "snapshot" and .statement == $shortop)
            | .new[0].fields.f1] == [2]);
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-widening-558" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 36) and
    ([.records[].operation] | all(. == "deployed" or . == "snapshot" or . == "unrepresentable")) and
    ([.records[] | select(.operation != "unrepresentable") | .time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "unrepresentable") | has("time") | not] | all) and
    ([.records[] | select(.operation == "unrepresentable") | .statement]
        == ["soda-ix1", "soda-ix2", "soda-ix1", "soda-ix2"]) and
    (.records[0].case == "hashbtree-window" and .records[9].case == "hashbtree-window") and
    (.records[10].case == "hashbtree-table" and .records[19].case == "hashbtree-table") and
    (.records[20].case == "widening-window" and .records[27].case == "widening-window") and
    (.records[28].case == "widening-table" and .records[35].case == "widening-table") and
    proberecords("hashbtree-window") and proberecords("hashbtree-table") and
    proberecords("widening-window") and proberecords("widening-table") and
    legrecords("hashbtree-window"; "select-gt-long"; 10; "select-gte-short") and
    legrecords("hashbtree-table"; "select-gt-long"; 10; "select-gte-short") and
    legrecords("widening-window"; "select-eq-long"; 10; "select-eq-short") and
    legrecords("widening-table"; "select-eq-long"; 10; "select-eq-short")
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-widening-558 trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
