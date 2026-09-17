#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-namedwindow-on-delete-silent.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "cannot read Esper Git commit" >&2
    exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required for the Java 9.0.0 oracle; found ${java_version:-unknown}" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-namedwindow-on-delete-silent" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnDelete.java" and
    (.javaRuntimes | type == "array" and length == 2) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames == ["InfraNamedWindowSilentDeleteOnDelete", "InfraNamedWindowSilentDeleteOnDeleteMany"]) and
    (.javaStaticIds == ["java-06bf0eb71230b3119293", "java-06bf0eb71230b3119293"]) and
    (.javaFlags | type == "array" and length == 0) and
    (.cases | type == "array" and length == 2) and
    ([.cases[].case] == ["silent-delete", "silent-delete-many"]) and
    ([.cases[].ordinal] == [5, 6]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    (.steps | type == "array" and length == 27) and
    ([.steps[] | select(.op == "case")] | length == 2) and
    ([.steps[] | select(.op == "deploy")] | length == 2) and
    ([.steps[] | select(.op == "deployed")] | length == 8) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 8) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 5) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 2) and
    ([.steps[] | select(.op != "case" and .op != "deploy" and .op != "deployed" and .op != "send" and .op != "undeploy-all")] | length == 0)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-namedwindow-on-delete-silent replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-namedwindow-on-delete-silent.XXXXXX")
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
    MINGW* | MSYS* | CYGWIN*)
        cp_sep=";"
        native_path() { cygpath -m "$1"; }
        ;;
    *)
        cp_sep=":"
        native_path() { printf '%s' "$1"; }
        ;;
esac
compiler_cp=$(tr -d '\r\n' < "$work/compiler-cp.txt")
runtime_cp=$(tr -d '\r\n' < "$work/runtime-cp.txt")
classpath="$(native_path "$classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/compiler/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/runtime/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-avro/target/classes")"
classpath="$classpath$cp_sep$(native_path "$esper_root/common-xmlxsd/target/classes")"
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNamedWindowOnDeleteSilentScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowOnDeleteSilentScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def deployed_at($idx; $name):
        .records[$idx].operation == "deployed" and .records[$idx].statement == $name
            and .records[$idx].sequence == 1;
    .version == "esper-parity/v1" and
    .id == "infra-namedwindow-on-delete-silent" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 32) and
    ([.records[].operation] | all(. == "listener" or . == "deployed")) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # silent-delete: 4 deployed + 14 listener = 18.
    ([.records[0:18][].case] | all(. == "silent-delete")) and
    deployed_at(0; "create") and deployed_at(1; "insert") and deployed_at(2; "delete") and deployed_at(3; "count") and
    ([.records[4:18][].operation] | all(. == "listener")) and
    # E1 insert: create then count.
    (.records[4].statement == "create" and .records[4].new[0].fields.theString == "E1") and
    (.records[5].statement == "count" and .records[5].new[0].fields.cnt == 1) and
    # S0(E1): delete fires (new=E1), count cnt=0, create silent.
    (.records[6].statement == "delete" and .records[6].new[0].fields.theString == "E1") and
    (.records[7].statement == "count" and .records[7].new[0].fields.cnt == 0) and
    # E2/E3 inserts.
    (.records[8].statement == "create" and .records[8].new[0].fields.theString == "E2") and
    (.records[9].statement == "count" and .records[9].new[0].fields.cnt == 1) and
    (.records[10].statement == "create" and .records[10].new[0].fields.theString == "E3") and
    (.records[11].statement == "count" and .records[11].new[0].fields.cnt == 2) and
    # E4 insert: length-2 expiry IR pair on create.
    (.records[12].statement == "create" and .records[12].new[0].fields.theString == "E4"
        and .records[12].old[0].fields.theString == "E2") and
    (.records[13].statement == "count" and .records[13].new[0].fields.cnt == 2) and
    # S0(E4): delete fires, count cnt=1, create silent.
    (.records[14].statement == "delete" and .records[14].new[0].fields.theString == "E4") and
    (.records[15].statement == "count" and .records[15].new[0].fields.cnt == 1) and
    # S0(E3): delete fires, count cnt=0.
    (.records[16].statement == "delete" and .records[16].new[0].fields.theString == "E3") and
    (.records[17].statement == "count" and .records[17].new[0].fields.cnt == 0) and
    # S0(EX): nothing fires (no records between 17 and the next case).
    # silent-delete-many: 4 deployed + 10 listener = 14.
    ([.records[18:32][].case] | all(. == "silent-delete-many")) and
    deployed_at(18; "create") and deployed_at(19; "insert") and deployed_at(20; "delete") and deployed_at(21; "count") and
    ([.records[22:32][].operation] | all(. == "listener")) and
    ([.records[22:30][] | select(.statement == "create") | .new[0].fields.theString] == ["A", "A", "B", "B"]) and
    ([.records[22:30][] | select(.statement == "count") | .new[0].fields.cnt] == [1, 2, 3, 4]) and
    (.records[30].statement == "delete" and ([.records[30].new[].fields.theString] == ["A", "A", "B", "B"])
        and ([.records[30].new[].fields.intPrimitive] == [1, 2, 3, 4])) and
    (.records[31].statement == "count" and .records[31].new[0].fields.cnt == 0)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-namedwindow-on-delete-silent trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
