#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-unique-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-unique-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews unique-slice: the MySimpleKeyValueMap unique and firstunique windows and the projection unique window with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered or canonical any-mode window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-63daa6cfa27a786c0480", "java-runtime-1ee31df7428f7c8e85eb", "java-runtime-bb670bc21e9d4333cefd"]) and
    (.javaNames == ["InfraUnique", "InfraUniqueSceneTwo", "InfraFirstUnique"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["unique", "unique-scene-two", "firstunique"]) and
    ([.cases[].ordinal] == [32, 33, 34]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "unique window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, re-admitted keys after delete, and any-mode two-row snapshots",
        "projection unique window over theString/intBoxed with four module deployments and only the window statement listened; every snapshot is any-mode",
        "firstunique window over the key/value map schema: duplicate keys are swallowed without any callback, deletes free the key for re-admission, and any-mode snapshots cover the two-row phases"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  Ord 33 carries its consumer under
    # consumeEpl (the statement is named "consume", not "s0") and the other two
    # cases carry an empty consumeEpl.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"])) and
    ([.cases[].iteratorSnapshots] == [6, 5, 7]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "consume", "delete"], ["create", "insert", "s0", "delete"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["create", "s0"]]) and
    # per-case step counts: unique = 1 case + 4 deploys + 7 sends + 6 snapshots
    # + 1 undeploy-all = 19; unique-scene-two = 1 case + 4 deploys + 4 sends + 5
    # snapshots + 1 undeploy-all = 15; firstunique = 1 case + 4 deploys + 7 sends
    # + 7 snapshots + 1 undeploy-all = 20 (19+15+20 = 54).
    ([.steps[] | select(.case == "unique")] | length == 19) and
    ([.steps[] | select(.case == "unique-scene-two")] | length == 15) and
    ([.steps[] | select(.case == "firstunique")] | length == 20) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 6) and
    ([.steps[] | select(.op == "snapshot")] | length == 18) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create")] | length == 18) and
    ([.steps[] | select(.op == "snapshot" and .mode == "ordered")] | length == 7) and
    ([.steps[] | select(.op == "snapshot" and .mode == "any")] | length == 11) and
    # the per-case snapshot mode sequence in step order: unique has three
    # exact-order (single-row) and three any-mode (two-row) snapshots,
    # unique-scene-two is any-mode throughout, and firstunique alternates with
    # the swallowed-duplicate phases staying ordered single-row.
    ([.steps[] | select(.op == "snapshot") | .mode] == ["ordered", "any", "ordered", "any", "any", "ordered", "any", "any", "any", "any", "any", "ordered", "any", "ordered", "ordered", "any", "any", "ordered"]) and
    # byte-exact EPL pins, including the @public of the ord-33 window create
    # and the named insert/consume statements of the ord-33 four-module case.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowUN#unique(key) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow#unique(key) as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowFU#firstunique(key) as MySimpleKeyValueMap"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowUN select theString as key, longBoxed as value from SupportBean", "@name('"'"'insert'"'"') insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowFU select theString as key, longBoxed as value from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowUN", "", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowFU"]) and
    ([.cases[].consumeEpl] == ["", "@name('"'"'consume'"'"') select irstream key, value as value from MyWindow", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowUN as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowFU as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume") | .epl] == [.cases[1].consumeEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl]) and
    # send payload shapes: theString plus exactly the one value field the case
    # window projects, or symbol for the on-delete trigger.
    ([.steps[] | select(.op == "send" and .case == "unique" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "unique-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "firstunique" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    (.steps | type == "array" and length == 54)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-unique-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-unique-views.XXXXXX")
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
    "$script_root/InfraNamedWindowUniqueViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowUniqueViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def snapr($c; $st; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def lis($c; $st; $q; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new, "old": $old};
    def lisn($c; $st; $q; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $new};
    def liso($c; $st; $q; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "old": $old};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 32 (InfraUnique, lines 2793-2828): every insert and delete wave
    # delivers the window statement itself first and then the s0 consumer (the
    # window statement registers its consumer view at window creation, before
    # s0 is deployed - the order chain 4.381 measured and reported per contract
    # section 6).  The replacement waves deliver ONE invocation carrying the
    # new row and the replaced row; deletes deliver old only.  The any-mode
    # snapshots are emitted/sorted canonically, which for G1/G2 coincides with
    # the declared order.
    def un: [
        lisn("unique"; "create"; 1; [r("G1"; 1)]),
        lisn("unique"; "s0"; 1; [r("G1"; 1)]),
        snapr("unique"; "create"; [r("G1"; 1)]),
        lisn("unique"; "create"; 2; [r("G2"; 20)]),
        lisn("unique"; "s0"; 2; [r("G2"; 20)]),
        snapr("unique"; "create"; [r("G1"; 1), r("G2"; 20)]),
        liso("unique"; "create"; 3; [r("G2"; 20)]),
        liso("unique"; "s0"; 3; [r("G2"; 20)]),
        lis("unique"; "create"; 4; [r("G1"; 2)]; [r("G1"; 1)]),
        lis("unique"; "s0"; 4; [r("G1"; 2)]; [r("G1"; 1)]),
        snapr("unique"; "create"; [r("G1"; 2)]),
        lisn("unique"; "create"; 5; [r("G2"; 21)]),
        lisn("unique"; "s0"; 5; [r("G2"; 21)]),
        snapr("unique"; "create"; [r("G1"; 2), r("G2"; 21)]),
        lis("unique"; "create"; 6; [r("G2"; 22)]; [r("G2"; 21)]),
        lis("unique"; "s0"; 6; [r("G2"; 22)]; [r("G2"; 21)]),
        snapr("unique"; "create"; [r("G1"; 2), r("G2"; 22)]),
        liso("unique"; "create"; 7; [r("G1"; 2)]),
        liso("unique"; "s0"; 7; [r("G1"; 2)]),
        snapr("unique"; "create"; [r("G2"; 22)])
    ];
    # Ord 33 (InfraUniqueSceneTwo, lines 2837-2887): four modules, create
    # listener only; every snapshot is any-mode and the consume statement is
    # deployed but unlistened, so it emits nothing.  Note the source comment at
    # :2874 says "delete event G2" while the market send carries G1.
    def us: [
        lisn("unique-scene-two"; "create"; 1; [r("G1"; 10)]),
        snapr("unique-scene-two"; "create"; [r("G1"; 10)]),
        lisn("unique-scene-two"; "create"; 2; [r("G2"; 20)]),
        snapr("unique-scene-two"; "create"; [r("G1"; 10), r("G2"; 20)]),
        snapr("unique-scene-two"; "create"; [r("G1"; 10), r("G2"; 20)]),
        liso("unique-scene-two"; "create"; 3; [r("G1"; 10)]),
        snapr("unique-scene-two"; "create"; [r("G2"; 20)]),
        snapr("unique-scene-two"; "create"; [r("G2"; 20)]),
        liso("unique-scene-two"; "create"; 4; [r("G2"; 20)])
    ];
    # Ord 34 (InfraFirstUnique, lines 2908-2943): the swallowed duplicates at
    # :2927 (G1 with value 2) and :2935 (G2 with value 22) emit nothing
    # anywhere - the stored rows keep values 1 and 21 - and the deletes free
    # the keys for re-admission.
    def fu: [
        lisn("firstunique"; "create"; 1; [r("G1"; 1)]),
        lisn("firstunique"; "s0"; 1; [r("G1"; 1)]),
        snapr("firstunique"; "create"; [r("G1"; 1)]),
        lisn("firstunique"; "create"; 2; [r("G2"; 20)]),
        lisn("firstunique"; "s0"; 2; [r("G2"; 20)]),
        snapr("firstunique"; "create"; [r("G1"; 1), r("G2"; 20)]),
        liso("firstunique"; "create"; 3; [r("G2"; 20)]),
        liso("firstunique"; "s0"; 3; [r("G2"; 20)]),
        snapr("firstunique"; "create"; [r("G1"; 1)]),
        snapr("firstunique"; "create"; [r("G1"; 1)]),
        lisn("firstunique"; "create"; 4; [r("G2"; 21)]),
        lisn("firstunique"; "s0"; 4; [r("G2"; 21)]),
        snapr("firstunique"; "create"; [r("G1"; 1), r("G2"; 21)]),
        snapr("firstunique"; "create"; [r("G1"; 1), r("G2"; 21)]),
        liso("firstunique"; "create"; 5; [r("G1"; 1)]),
        liso("firstunique"; "s0"; 5; [r("G1"; 1)]),
        snapr("firstunique"; "create"; [r("G2"; 21)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-unique-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 46) and
    # 20 + 9 + 17 records, 28 listener and 18 snapshot.
    ((crec("unique") | length) == 20) and
    ((crec("unique-scene-two") | length) == 9) and
    ((crec("firstunique") | length) == 17) and
    ([.records[] | select(.operation == "listener")] | length == 28) and
    ([.records[] | select(.operation == "snapshot")] | length == 18) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("unique"; 20) + repv("unique-scene-two"; 9) + repv("firstunique"; 17))) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # The unique and firstunique blocks are create, s0 per wave (see the un()
    # and fu() notes above); unique-scene-two listens to create only.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 7) + repv("create"; 4) + repa(["create", "s0"]; 5))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7] + [1, 2, 3, 4] + [1, 1, 2, 2, 3, 3, 4, 4, 5, 5])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == repv(0; 18)) and
    ([.records[] | select(.operation == "snapshot") | .statement] == repv("create"; 18)) and
    # every listener record carries at least one stream and every row projects
    # exactly the key/value map fields.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row" and ((.fields | keys) == ["key", "value"])] | all) and
    (crec("unique") == un) and
    (crec("unique-scene-two") == us) and
    (crec("firstunique") == fu)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-unique-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
