#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-keepall-delete.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-keepall-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    (.description | test("InfraNamedWindowViews")) and
    (.javaRuntimes == ["java-runtime-14f3c7199cf587fea29a", "java-runtime-6d4a6e1168f100add522", "java-runtime-8fe8d92928db5eba5fc3", "java-runtime-b47bdbdf2ac26de6cb0d", "java-runtime-594ea469769e1990e60f"]) and
    (.javaNames == ["InfraKeepAllSceneTwo", "InfraWithDeleteUseAs", "InfraWithDeleteFirstAs", "InfraWithDeleteSecondAs", "InfraWithDeleteNoAs"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["keepall-scene-two", "with-delete-use-as", "with-delete-first-as", "with-delete-second-as", "with-delete-no-as"]) and
    ([.cases[].ordinal] == [1, 39, 40, 41, 42]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "createEpl", "deleteEpl", "deploys", "description", "executionName", "iteratorSnapshots", "listened", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"])) and
    ([.cases[].iteratorSnapshots] == [11, 9, 9, 9, 9]) and
    ([.cases[].deploys] == [["create", "insert", "delete"], ["create", "insert", "s0", "s2", "s3", "delete"], ["create", "insert", "s0", "s2", "s3", "delete"], ["create", "insert", "s0", "s2", "s3", "delete"], ["create", "insert", "s0", "s2", "s3", "delete"]]) and
    ([.cases[].listened] == [["create"], ["create", "s0", "s2", "s3"], ["create", "s0", "s2", "s3"], ["create", "s0", "s2", "s3"], ["create", "s0", "s2", "s3"]]) and
    # per-case step counts: ordinal 1 = 1 case + 3 deploys + 9 sends + 11
    # snapshots + 1 undeploy-all = 25; each with-delete case = 1 case + 6
    # deploys + 7 sends + 9 snapshots + 1 undeploy-all = 24 (25 + 4*24 = 121).
    ([.steps[] | select(.case == "keepall-scene-two")] | length == 25) and
    ([.steps[] | select(.case == "with-delete-use-as")] | length == 24) and
    ([.steps[] | select(.case == "with-delete-first-as")] | length == 24) and
    ([.steps[] | select(.case == "with-delete-second-as")] | length == 24) and
    ([.steps[] | select(.case == "with-delete-no-as")] | length == 24) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 5) and
    ([.steps[] | select(.op == "deploy")] | length == 27) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s2")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s3")] | length == 4) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 18) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 19) and
    ([.steps[] | select(.op == "snapshot")] | length == 47) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create" and .mode == "ordered")] | length == 39) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0" and .mode == "ordered")] | length == 8) and
    # byte-exact EPL pins, including the @name('create') @public and
    # @name('delete') prefixes the Java source writes.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') @public create window MyWindow.win:keepall() as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') @public create window MyWindow#keepall as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow#keepall as select key, value from MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow#keepall as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow#keepall as select key as key, value as value from MySimpleKeyValueMap"]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindow as s1 where symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow where s0.symbol = key", "@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindow where symbol = key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[].deleteEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] | all(. == "@name('"'"'s0'"'"') select irstream key, value*2 as value from MyWindow")) and
    ([.steps[] | select(.op == "deploy" and .statement == "s2") | .epl] | all(. == "@name('"'"'s2'"'"') select irstream key, sum(value) as value from MyWindow group by key")) and
    ([.steps[] | select(.op == "deploy" and .statement == "s3") | .epl] | all(. == "@name('"'"'s3'"'"') select irstream key, value from MyWindow where value >= 10")) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] | all(. == "@name('"'"'insert'"'"') insert into MyWindow select theString as key, longBoxed as value from SupportBean" or . == "@name('"'"'insert'"'"') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean")) and
    # send payload shapes: theString plus exactly one value field, or symbol.
    ([.steps[] | select(.op == "send" and .case == "keepall-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case != "keepall-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "keepall-scene-two" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    ([.steps[] | select(.op == "send" and .case != "keepall-scene-two" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    (.steps | type == "array" and length == 121)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-keepall-delete replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-keepall-delete.XXXXXX")
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
    "$script_root/InfraNamedWindowKeepAllDeleteScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowKeepAllDeleteScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nul: {"state": "null"};
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def snapr($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def snap0($c; $st): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z"};
    def lis($c; $st; $q; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def liso($c; $st; $q; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "old": $old};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ordinal 1: the window create statement is the only listener; nine
    # deliveries across six inserts and three deletes with the eleven ordered
    # iterator snapshots the source asserts between them (InfraNamedWindowViews
    # lines 195-266).
    def scene: [
        lisn("keepall-scene-two"; "create"; 1; [r("G1"; 10)]),
        snapr("keepall-scene-two"; "create"; [r("G1"; 10)]),
        lisn("keepall-scene-two"; "create"; 2; [r("G2"; 20)]),
        snapr("keepall-scene-two"; "create"; [r("G1"; 10), r("G2"; 20)]),
        liso("keepall-scene-two"; "create"; 3; [r("G2"; 20)]),
        snapr("keepall-scene-two"; "create"; [r("G1"; 10)]),
        snapr("keepall-scene-two"; "create"; [r("G1"; 10)]),
        lisn("keepall-scene-two"; "create"; 4; [r("G3"; 30)]),
        snapr("keepall-scene-two"; "create"; [r("G1"; 10), r("G3"; 30)]),
        liso("keepall-scene-two"; "create"; 5; [r("G1"; 10)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30)]),
        lisn("keepall-scene-two"; "create"; 6; [r("G4"; 40)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30), r("G4"; 40)]),
        lisn("keepall-scene-two"; "create"; 7; [r("G5"; 50)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30), r("G4"; 40), r("G5"; 50)]),
        lisn("keepall-scene-two"; "create"; 8; [r("G6"; 60)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30), r("G4"; 40), r("G5"; 50), r("G6"; 60)]),
        liso("keepall-scene-two"; "create"; 9; [r("G6"; 60)]),
        snapr("keepall-scene-two"; "create"; [r("G3"; 30), r("G4"; 40), r("G5"; 50)])
    ];
    # The four tryCreateWindow cases are identical: each insert and delete wave
    # delivers the window statement itself first and then the consumers in
    # registration order (s0, s2, s3), as verified against the running Java
    # engine: the window statement registers its consumer view at window
    # creation, before s0/s2/s3 are deployed. Contract section 5.1 predicted
    # the reverse (s0, s2, s3, create) from the sibling-chain precedent; the
    # Java runtime contradicts that prediction and this pin follows the Java
    # runtime (contract section 6 requires reporting the mismatch, not tuning
    # the oracle). The payloads are exactly the frozen-contract payloads.
    def wd($c): [
        lisn($c; "create"; 1; [r("E1"; 10)]),
        lisn($c; "s0"; 1; [r("E1"; 20)]),
        lis($c; "s2"; 1; [r("E1"; 10)]; [r("E1"; nul)]),
        lisn($c; "s3"; 1; [r("E1"; 10)]),
        snapr($c; "create"; [r("E1"; 10)]),
        snapr($c; "s0"; [r("E1"; 20)]),
        lisn($c; "create"; 2; [r("E2"; 20)]),
        lisn($c; "s0"; 2; [r("E2"; 40)]),
        lis($c; "s2"; 2; [r("E2"; 20)]; [r("E2"; nul)]),
        lisn($c; "s3"; 2; [r("E2"; 20)]),
        snapr($c; "create"; [r("E1"; 10), r("E2"; 20)]),
        snapr($c; "s0"; [r("E1"; 20), r("E2"; 40)]),
        lisn($c; "create"; 3; [r("E3"; 5)]),
        lisn($c; "s0"; 3; [r("E3"; 10)]),
        lis($c; "s2"; 3; [r("E3"; 5)]; [r("E3"; nul)]),
        snapr($c; "create"; [r("E1"; 10), r("E2"; 20), r("E3"; 5)]),
        liso($c; "create"; 4; [r("E1"; 10)]),
        liso($c; "s0"; 4; [r("E1"; 20)]),
        lis($c; "s2"; 4; [r("E1"; nul)]; [r("E1"; 10)]),
        liso($c; "s3"; 3; [r("E1"; 10)]),
        snapr($c; "create"; [r("E2"; 20), r("E3"; 5)]),
        snapr($c; "create"; [r("E2"; 20), r("E3"; 5)]),
        liso($c; "create"; 5; [r("E2"; 20)]),
        liso($c; "s0"; 5; [r("E2"; 40)]),
        lis($c; "s2"; 5; [r("E2"; nul)]; [r("E2"; 20)]),
        liso($c; "s3"; 4; [r("E2"; 20)]),
        snapr($c; "create"; [r("E3"; 5)]),
        liso($c; "create"; 6; [r("E3"; 5)]),
        liso($c; "s0"; 6; [r("E3"; 10)]),
        lis($c; "s2"; 6; [r("E3"; nul)]; [r("E3"; 5)]),
        snap0($c; "create")
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-keepall-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 144) and
    # per-case counts: 20 for ordinal 1 and 31 for each tryCreateWindow case.
    ((crec("keepall-scene-two") | length) == 20) and
    ((crec("with-delete-use-as") | length) == 31) and
    ((crec("with-delete-first-as") | length) == 31) and
    ((crec("with-delete-second-as") | length) == 31) and
    ((crec("with-delete-no-as") | length) == 31) and
    ([.records[].case] == (repv("keepall-scene-two"; 20) + repv("with-delete-use-as"; 31) + repv("with-delete-first-as"; 31) + repv("with-delete-second-as"; 31) + repv("with-delete-no-as"; 31))) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # The with-delete block is create, s0, s2, s3 per insert/delete wave (see
    # the wd() note above).
    ([.records[] | select(.operation == "listener") | .statement] == (repv("create"; 9) + repa(["create", "s0", "s2", "s3", "create", "s0", "s2", "s3", "create", "s0", "s2", "create", "s0", "s2", "s3", "create", "s0", "s2", "s3", "create", "s0", "s2"]; 4))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 2, 3, 4, 5, 6, 7, 8, 9] + repa([1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 4, 4, 4, 3, 5, 5, 5, 4, 6, 6, 6]; 4))) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == repv(0; 47)) and
    ([.records[] | select(.operation == "snapshot") | .statement] == (repv("create"; 11) + repa(["create", "s0", "create", "s0", "create", "create", "create", "create", "create"]; 4))) and
    # every listener record carries at least one stream, every row projects
    # exactly key and value.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | select(.operation == "listener") | ((.new // []) + (.old // []))[] | .kind == "row" and ((.fields | keys) == ["key", "value"])] | all) and
    ([.records[] | select(.operation == "snapshot") | (.new // [])[] | .kind == "row" and ((.fields | keys) == ["key", "value"])] | all) and
    (crec("keepall-scene-two") == scene) and
    (crec("with-delete-use-as") == wd("with-delete-use-as")) and
    (crec("with-delete-first-as") == wd("with-delete-first-as")) and
    (crec("with-delete-second-as") == wd("with-delete-second-as")) and
    (crec("with-delete-no-as") == wd("with-delete-no-as"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-keepall-delete trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
