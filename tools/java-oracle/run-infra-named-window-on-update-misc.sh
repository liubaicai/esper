#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-on-update-misc.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 1; }
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
    .id == "infra-named-window-on-update-misc" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnUpdate.java" and
    (.javaRuntimes | type == "array" and length == 4) and
    ([.javaRuntimes[] | startswith("java-runtime-")] | all) and
    (.javaNames | type == "array" and length == 4) and
    ([.javaNames[] | startswith("Infra")] | all) and
    (.javaStaticIds | type == "array" and length == 4) and
    ([.javaStaticIds[] | startswith("java-")] | all) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["non-property-set", "subclass", "copy-method", "wrapper"]) and
    ([.cases[].ordinal] == [0, 3, 4, 5]) and
    ([.cases[].runtimeId] == ["java-runtime-5d041a3958410a90fa9a", "java-runtime-9ea51cb84d163be79693", "java-runtime-43feff597147c7867ca7", "java-runtime-09f56f49bab53388bb2d"]) and
    ([.cases[].executionName] == ["InfraUpdateNonPropertySet", "InfraSubclass", "InfraUpdateCopyMethodBean", "InfraUpdateWrapper"]) and
    ([.cases[] | select(.observation == "listener")] | length == 4) and
    ([.cases[].iteratorSnapshots] == [0, 0, 1, 1]) and
    ([.steps[] | select(.case == "non-property-set")] | length == 7) and
    ([.steps[] | select(.case == "subclass")] | length == 7) and
    ([.steps[] | select(.case == "copy-method")] | length == 8) and
    ([.steps[] | select(.case == "wrapper")] | length == 7) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    # the wrapper create module bundles the pinned insert statement because the
    # engine cannot deploy a separate insert module against a
    # wildcard-plus-extra create-window type (see the oracle EPL_CREATE_INSERT_WRAPPER)
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "update")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 2) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBeanAbstractSub")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBeanCopyMethod")] | length == 1) and
    ([.steps[] | select(.op == "snapshot")] | length == 2) and
    ([.steps[] | select(.op == "snapshot" and .statement == "window" and .mode == "any")] | length == 2) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    (.steps | type == "array" and length == 29)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-on-update-misc replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-on-update-misc.XXXXXX")
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
    "$script_root/InfraNamedWindowOnUpdateMiscScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowOnUpdateMiscScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def rownp($i; $l): {"kind": "row", "fields": {"intPrimitive": $i, "longPrimitive": $l}};
    def rowsc($a; $b): {"kind": "row", "fields": {"v1": $a, "v2": $b}};
    def rowcm($v): {"kind": "row", "fields": {"valOne": $v}};
    def roww($p; $t): {"kind": "row", "fields": {"p0": $p, "theString": $t}};
    def lis($c; $q; $st; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    # liso is a listener record carrying both update new and old delivery.
    def liso($c; $q; $st; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    # snap($c; $st; $rows) checks a snapshot record; pass .records[N].new as
    # $rows for mode-any snapshots so the engine iteration order is not pinned.
    def snap($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    .version == "esper-parity/v1" and
    .id == "infra-named-window-on-update-misc" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.records | type == "array" and length == 5) and
    ([.records[].case] == ["non-property-set", "subclass", "subclass", "copy-method", "wrapper"]) and
    ([.records[].operation] == ["listener", "listener", "listener", "snapshot", "snapshot"]) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "listener") | .statement] == ["update", "create", "create"]) and
    ([.records[] | select(.operation == "listener") | .sequence] == [1, 1, 2]) and
    ([.records[] | select(.operation == "snapshot") | .statement] == ["window", "window"]) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == [0, 0]) and
    ([.records[] | has("new")] | all) and
    ([.records[] | select(has("old"))] | length == 2) and
    ([.records[] | select(.case == "non-property-set") | (.new[], .old[])
        | .kind == "row" and ((.fields | keys) == ["intPrimitive", "longPrimitive"])] | all) and
    ([.records[] | select(.case == "subclass") | (.new[], (.old // [])[])
        | .kind == "row" and ((.fields | keys) == ["v1", "v2"])] | all) and
    ([.records[] | select(.case == "copy-method") | .new[] | .kind == "row" and ((.fields | keys) == ["valOne"])] | all) and
    ([.records[] | select(.case == "wrapper") | .new[] | .kind == "row" and ((.fields | keys) == ["p0", "theString"])] | all) and
    # non-property-set: the update statement delivery carries the post-image
    # as new (setIntPrimitive(10) plus the plugin single-row longPrimitive 999)
    # and the pre-image as old (intPrimitive 1, longPrimitive 0), mirroring
    # assertPropsPerRowLastNew at InfraNamedWindowOnUpdate line 149.
    (.records[0] == liso("non-property-set"; 1; "update"; [rownp(10; 999)]; [rownp(1; 0)])) and
    # subclass: the create listener sees the insert delivery (inherited v1
    # null, v2 "value2") and then the update delivery (new v1/v2 "E1" plus the
    # engine-provided old pre-image), mirroring assertPropsPerRowLastNew at
    # line 221.
    (.records[1] == lis("subclass"; 1; "create"; [rowsc({"state": "null"}; "value2")])) and
    (.records[2] == liso("subclass"; 2; "create"; [rowsc("E1"; "E1")]; [rowsc({"state": "null"}; "value2")])) and
    # copy-method: single mode-any snapshot with the myCopyMethod-copied row
    # after the valOne update, mirroring the iterator assert at line 126.
    (.records[3] == snap("copy-method"; "window"; [rowcm("x")])) and
    # wrapper: single mode-any snapshot of the wrapper-select window after the
    # theString/p0 update, mirroring the iterator assert at line 112.
    (.records[4] == snap("wrapper"; "window"; [roww(2; "x")]))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-on-update-misc trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
