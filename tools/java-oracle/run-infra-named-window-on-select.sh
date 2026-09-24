#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-on-select.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-on-select" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowOnSelect.java" and
    (.description | test("InfraNamedWindowOnSelect")) and
    (.javaRuntimes == ["java-runtime-8a348801e82d24a6c755", "java-runtime-1bbd8705a7bbea1db0cd", "java-runtime-caeff76ee56490d832ea"]) and
    (.javaNames == ["InfraNamedWindowOnSelectSimple", "InfraNamedWindowOnSelectSceneTwo", "InfraNamedWindowOnSelectWPattern"]) and
    (.javaStaticIds == ["java-763e7086c0fdad5f4c49", "java-0f1cbcf0f43473325b2b", "java-4201486cef18d7d66843"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["simple", "scene-two", "wpattern"]) and
    ([.cases[].ordinal] == [0, 1, 2]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op", "statement"] or . == ["case", "op"])) and
    # per-case step counts: simple = 1 case + 3 deploys + 3 deployed + 2 sends
    # + 1 undeploy-all = 10; scene-two = 1 + 6 deploys + 6 deployed + 7 sends +
    # 3 snapshots + 1 undeploy-all = 24; wpattern = 1 + 3 deploys + 3 deployed +
    # 3 sends + 1 undeploy-all = 11 (10 + 24 + 11 = 45... see total below).
    ([.steps[] | select(.case == "simple")] | length == 10) and
    ([.steps[] | select(.case == "scene-two")] | length == 24) and
    ([.steps[] | select(.case == "wpattern")] | length == 11) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deployed")] | length == 12) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "select")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "consumer")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert-i")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 1) and
    ([.steps[] | select(.op == "send")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 7) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 1) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 1) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create" and .mode == "ordered")] | length == 3) and
    # byte-exact EPL pins, including the @Name/@name and @public annotations
    # the Java source writes.
    ([.steps[] | select(.op == "deploy" and .case == "simple") | .epl] == ["@Name('"'"'create'"'"') @public create window MyWindow.win:keepall() as SupportBean", "@Name('"'"'insert'"'"') insert into MyWindow select * from SupportBean", "@Name('"'"'delete'"'"') on SupportBean_S0 delete from MyWindow where intPrimitive = id"]) and
    ([.steps[] | select(.op == "deploy" and .case == "scene-two") | .epl] == ["@name('"'"'create'"'"') @public create window MyWindow#keepall as select * from SupportBean", "insert into MyWindow select * from SupportBean(theString like '"'"'E%'"'"')", "@name('"'"'select'"'"') on SupportBean_A insert into MyStream select mywin.* from MyWindow as mywin order by theString asc", "@name('"'"'consumer'"'"') select * from MyStream", "insert into MyStream select * from SupportBean(theString like '"'"'I%'"'"')", "@name('"'"'delete'"'"') on SupportBean_B delete from MyWindow"]) and
    ([.steps[] | select(.op == "deploy" and .case == "wpattern") | .epl] == ["@public create window MyWindow.win:keepall() as SupportBean", "insert into MyWindow select * from SupportBean(theString = '"'"'Z'"'"')", "@Name('"'"'s0'"'"') on pattern[every e = SupportBean(theString = '"'"'A'"'"') -> SupportBean(intPrimitive = e.intPrimitive)] select * from MyWindow"]) and
    # send payload shapes: theString + intPrimitive for SupportBean, id only
    # for the trigger types.
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType != "SupportBean") | (.payload | keys)] | all(. == ["id"])) and
    (.steps | type == "array" and length == 45)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-on-select replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-on-select.XXXXXX")
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
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNamedWindowOnSelectScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowOnSelectScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nul: {"state": "null"};
    def bean($s; $i): {"kind": "row", "fields": {
        "bigDecimal": nul, "bigInteger": nul, "boolBoxed": nul,
        "boolPrimitive": false, "byteBoxed": nul, "bytePrimitive": 0,
        "charBoxed": nul, "charPrimitive": "\u0000", "doubleBoxed": nul,
        "doublePrimitive": 0, "enumValue": nul, "floatBoxed": nul,
        "floatPrimitive": 0, "intBoxed": nul, "intPrimitive": $i,
        "longBoxed": nul, "longPrimitive": 0, "shortBoxed": nul,
        "shortPrimitive": 0, "theString": $s}};
    def dep($c; $st; $q): {"case": $c, "operation": "deployed", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z"};
    def snapr($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def liso($c; $st; $q; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "old": $old};
    def crec($c): [.records[] | select(.case == $c)];
    # simple: three deployed markers, the E1 insert as the window statement
    # new row and the S0(1) delete as its old row.
    def simple: [
        dep("simple"; "create"; 1),
        dep("simple"; "insert"; 1),
        dep("simple"; "delete"; 1),
        lisn("simple"; "create"; 1; [bean("E1"; 1)]),
        liso("simple"; "create"; 2; [bean("E1"; 1)])
    ];
    # scene-two: five deployed markers for the module, the E1/A1/I2/E3/A2 wave
    # with the three create snapshots, the delete marker, and nothing after
    # the B1 delete-all.
    def scene: [
        dep("scene-two"; "create"; 1),
        dep("scene-two"; "insert"; 1),
        dep("scene-two"; "select"; 1),
        dep("scene-two"; "consumer"; 1),
        dep("scene-two"; "insert-i"; 1),
        snapr("scene-two"; "create"; [bean("E1"; 1)]),
        lisn("scene-two"; "select"; 1; [bean("E1"; 1)]),
        lisn("scene-two"; "consumer"; 1; [bean("E1"; 1)]),
        lisn("scene-two"; "consumer"; 2; [bean("I2"; 2)]),
        snapr("scene-two"; "create"; [bean("E1"; 1)]),
        snapr("scene-two"; "create"; [bean("E1"; 1), bean("E3"; 3)]),
        lisn("scene-two"; "select"; 2; [bean("E1"; 1), bean("E3"; 3)]),
        lisn("scene-two"; "consumer"; 3; [bean("E1"; 1)]),
        lisn("scene-two"; "consumer"; 4; [bean("E3"; 3)]),
        dep("scene-two"; "delete"; 1)
    ];
    # wpattern: three deployed markers and the single s0 join-shaped row:
    # stream_0 is the retained Z bean toString and stream_1 the pattern
    # match map toString ({e=BeanEventBean ...}).
    def wpat: [
        dep("wpattern"; "create"; 1),
        dep("wpattern"; "insert"; 1),
        dep("wpattern"; "s0"; 1),
        lisn("wpattern"; "s0"; 1; [{"kind": "row", "fields": {
            "stream_0": "SupportBean(Z, 0)",
            "stream_1": "{e=BeanEventBean eventType=BeanEventType name=SupportBean clazz=com.espertech.esper.common.internal.support.SupportBean bean=SupportBean(A, 1)}"}}])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-on-select" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 24) and
    ((crec("simple") | length) == 5) and
    ((crec("scene-two") | length) == 15) and
    ((crec("wpattern") | length) == 4) and
    ([.records[].case] == (["simple", "simple", "simple", "simple", "simple"]
        + ["scene-two", "scene-two", "scene-two", "scene-two", "scene-two",
           "scene-two", "scene-two", "scene-two", "scene-two", "scene-two",
           "scene-two", "scene-two", "scene-two", "scene-two", "scene-two"]
        + ["wpattern", "wpattern", "wpattern", "wpattern"])) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    (crec("simple") == simple) and
    (crec("scene-two") == scene) and
    (crec("wpattern") == wpat)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-on-select trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
