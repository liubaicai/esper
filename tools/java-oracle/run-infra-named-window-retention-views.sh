#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-retention-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-retention-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews retention-view slice: the legacy win:keepall() window over the SupportBean bean type, and the MySimpleKeyValueMap lastevent and firstevent windows with insert-into projections, on-delete triggers and irstream consumers, captured from listener callbacks and ordered window iterator snapshots (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-26c44410c8018a34d696", "java-runtime-b85cc831b5c23a570cf7", "java-runtime-59100403b3c6affd0729", "java-runtime-c2f54d9eb104d061950c"]) and
    (.javaNames == ["InfraKeepAllSimple", "InfraLastEvent", "InfraLastEventSceneTwo", "InfraFirstEvent"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["keepall-simple", "lastevent", "lastevent-scene-two", "firstevent"]) and
    ([.cases[].ordinal] == [0, 29, 30, 31]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "legacy win:keepall() window over SupportBean with a separate insert-into module, two window-statement inserts and the two-statement module teardown",
        "lastevent window over the key/value map schema: replacement delivers one invocation carrying the new row and the replaced row, three delete waves each followed by an empty iterator, and a no-match delete that stays silent",
        "projection lastevent window over theString/intBoxed with three module deployments: replacement IR pair, delete to empty, re-insert after becoming empty",
        "firstevent window over the key/value map schema: a dropped insert emits nothing anywhere, deletes drain the window to empty, and re-inserts are admitted once empty"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"] or . == ["case", "op", "statement"])) and
    ([.cases[].iteratorSnapshots] == [0, 6, 3, 6]) and
    ([.cases[].deploys] == [["create", "insert"], ["create", "insert", "s0", "delete"], ["create", "insert", "delete"], ["create", "insert", "s0", "delete"]]) and
    ([.cases[].listened] == [["create"], ["create", "s0"], ["create"], ["create", "s0"]]) and
    # per-case step counts: keepall-simple = 1 case + 2 deploys + 2 sends + 2
    # statement-targeted undeploys = 7; lastevent = 1 case + 4 deploys + 7 sends
    # + 6 snapshots + 1 undeploy-all = 19; lastevent-scene-two = 1 case + 3
    # deploys + 4 sends + 3 snapshots + 1 undeploy-all = 12; firstevent = 1 case
    # + 4 deploys + 8 sends + 6 snapshots + 1 undeploy-all = 20 (7+19+12+20 = 58).
    ([.steps[] | select(.case == "keepall-simple")] | length == 7) and
    ([.steps[] | select(.case == "lastevent")] | length == 19) and
    ([.steps[] | select(.case == "lastevent-scene-two")] | length == 12) and
    ([.steps[] | select(.case == "firstevent")] | length == 20) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "undeploy")] | length == 2) and
    ([.steps[] | select(.op == "deploy")] | length == 13) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 3) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 13) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 8) and
    ([.steps[] | select(.op == "snapshot")] | length == 15) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create" and .mode == "ordered")] | length == 15) and
    # keepall-simple has no consumer and no iterator snapshot: its teardown is
    # the source-s two undeployModuleContaining calls in insert-then-create
    # order (InfraNamedWindowViews lines 183-184).
    ([.steps[] | select(.case == "keepall-simple" and .op == "snapshot")] | length == 0) and
    ([.steps[] | select(.op == "undeploy")] == [{"op": "undeploy", "case": "keepall-simple", "statement": "insert"}, {"op": "undeploy", "case": "keepall-simple", "statement": "create"}]) and
    # byte-exact EPL pins, including the capitalised @Name and @public of ord 0
    # and the @public of the ord-30 window create.
    ([.cases[].createEpl] == ["@Name('"'"'create'"'"') @public create window MyWindow.win:keepall() as SupportBean", "@name('"'"'create'"'"') create window MyWindowLE#lastevent as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.std:lastevent() as select theString as key, intBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowFE#firstevent as MySimpleKeyValueMap"]) and
    ([.cases[].insertEpl] == ["@Name('"'"'insert'"'"') insert into MyWindow select * from SupportBean", "insert into MyWindowLE select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, intBoxed from SupportBean", "insert into MyWindowFE select theString as key, longBoxed as value from SupportBean"]) and
    ([.cases[].s0Epl] == ["", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowLE", "", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowFE"]) and
    ([.cases[].deleteEpl] == ["", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowLE as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowFE as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[1].s0Epl, .cases[3].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[1].deleteEpl, .cases[2].deleteEpl, .cases[3].deleteEpl]) and
    # send payload shapes: theString alone for ord 0, theString plus exactly the
    # one value field the case window projects otherwise, or symbol.
    ([.steps[] | select(.op == "send" and .case == "keepall-simple" and .eventType == "SupportBean") | (.payload | keys)] | all(. == ["theString"])) and
    ([.steps[] | select(.op == "send" and .case == "lastevent" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "lastevent-scene-two" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "firstevent" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    (.steps | type == "array" and length == 58)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-retention-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-retention-views.XXXXXX")
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
    "$script_root/InfraNamedWindowRetentionViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowRetentionViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def nul: {"state": "null"};
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def b($s): {"kind": "row", "fields": {"theString": $s}};
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
    # Ord 0 (InfraKeepAllSimple, lines 165-186): the window create statement is
    # the only listener; the bean-typed window rows project theString only, the
    # insert module carries no listener, and the case has no consumer and no
    # iterator snapshot.
    def ks: [
        lisn("keepall-simple"; "create"; 1; [b("E1")]),
        lisn("keepall-simple"; "create"; 2; [b("E2")])
    ];
    # Ord 29 (InfraLastEvent, lines 2645-2682): every insert and delete wave
    # delivers the window statement itself first and then the s0 consumer (the
    # window statement registers its consumer view at window creation, before
    # s0 is deployed - the order chain 4.381 measured and reported per contract
    # section 6).  Replacement delivers one invocation carrying the new row and
    # the replaced row; deletes deliver old only; the no-match delete E1 at the
    # end emits nothing at all.
    def le: [
        lisn("lastevent"; "create"; 1; [r("E1"; 1)]),
        lisn("lastevent"; "s0"; 1; [r("E1"; 1)]),
        snapr("lastevent"; "create"; [r("E1"; 1)]),
        lis("lastevent"; "create"; 2; [r("E2"; 2)]; [r("E1"; 1)]),
        lis("lastevent"; "s0"; 2; [r("E2"; 2)]; [r("E1"; 1)]),
        snapr("lastevent"; "create"; [r("E2"; 2)]),
        liso("lastevent"; "create"; 3; [r("E2"; 2)]),
        liso("lastevent"; "s0"; 3; [r("E2"; 2)]),
        snap0("lastevent"; "create"),
        lisn("lastevent"; "create"; 4; [r("E3"; 3)]),
        lisn("lastevent"; "s0"; 4; [r("E3"; 3)]),
        snapr("lastevent"; "create"; [r("E3"; 3)]),
        liso("lastevent"; "create"; 5; [r("E3"; 3)]),
        liso("lastevent"; "s0"; 5; [r("E3"; 3)]),
        snap0("lastevent"; "create"),
        lisn("lastevent"; "create"; 6; [r("E4"; 4)]),
        lisn("lastevent"; "s0"; 6; [r("E4"; 4)]),
        snapr("lastevent"; "create"; [r("E4"; 4)])
    ];
    # Ord 30 (InfraLastEventSceneTwo, lines 2691-2738): three modules, create
    # listener only; replacement IR pair, delete to an empty iterator, then a
    # re-insert that is admitted because the window is empty again.
    def ls: [
        lisn("lastevent-scene-two"; "create"; 1; [r("G1"; 1)]),
        snapr("lastevent-scene-two"; "create"; [r("G1"; 1)]),
        lis("lastevent-scene-two"; "create"; 2; [r("G2"; 2)]; [r("G1"; 1)]),
        snapr("lastevent-scene-two"; "create"; [r("G2"; 2)]),
        liso("lastevent-scene-two"; "create"; 3; [r("G2"; 2)]),
        snap0("lastevent-scene-two"; "create"),
        lisn("lastevent-scene-two"; "create"; 4; [r("G3"; 3)])
    ];
    # Ord 31 (InfraFirstEvent, lines 2748-2789): the dropped insert E2 emits
    # nothing anywhere (the iterator still holds E1), delete E1 drains the
    # window, the dropped key E2 delete is silent, and E4 is admitted once the
    # window is empty again.
    def fe: [
        lisn("firstevent"; "create"; 1; [r("E1"; 1)]),
        lisn("firstevent"; "s0"; 1; [r("E1"; 1)]),
        snapr("firstevent"; "create"; [r("E1"; 1)]),
        snapr("firstevent"; "create"; [r("E1"; 1)]),
        liso("firstevent"; "create"; 2; [r("E1"; 1)]),
        liso("firstevent"; "s0"; 2; [r("E1"; 1)]),
        snap0("firstevent"; "create"),
        lisn("firstevent"; "create"; 3; [r("E3"; 3)]),
        lisn("firstevent"; "s0"; 3; [r("E3"; 3)]),
        snapr("firstevent"; "create"; [r("E3"; 3)]),
        liso("firstevent"; "create"; 4; [r("E3"; 3)]),
        liso("firstevent"; "s0"; 4; [r("E3"; 3)]),
        snap0("firstevent"; "create"),
        lisn("firstevent"; "create"; 5; [r("E4"; 4)]),
        lisn("firstevent"; "s0"; 5; [r("E4"; 4)]),
        snapr("firstevent"; "create"; [r("E4"; 4)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-retention-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 43) and
    # 2 + 18 + 7 + 16 records, 28 listener and 15 snapshot.
    ((crec("keepall-simple") | length) == 2) and
    ((crec("lastevent") | length) == 18) and
    ((crec("lastevent-scene-two") | length) == 7) and
    ((crec("firstevent") | length) == 16) and
    ([.records[] | select(.operation == "listener")] | length == 28) and
    ([.records[] | select(.operation == "snapshot")] | length == 15) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("keepall-simple"; 2) + repv("lastevent"; 18) + repv("lastevent-scene-two"; 7) + repv("firstevent"; 16))) and
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # The lastevent and firstevent blocks are create, s0 per wave (see the le()
    # note above); keepall-simple and lastevent-scene-two listen to create only.
    ([.records[] | select(.operation == "listener") | .statement] == (repv("create"; 2) + repa(["create", "s0"]; 6) + repv("create"; 4) + repa(["create", "s0"]; 5))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 2] + [1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6] + [1, 2, 3, 4] + [1, 1, 2, 2, 3, 3, 4, 4, 5, 5])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] == repv(0; 15)) and
    ([.records[] | select(.operation == "snapshot") | .statement] == repv("create"; 15)) and
    # every listener record carries at least one stream; ord 0 rows project
    # theString only and ords 29-31 rows project exactly key and value.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | select(.case == "keepall-simple") | ((.new // []) + (.old // []))[] | .kind == "row" and ((.fields | keys) == ["theString"])] | all) and
    ([.records[] | select(.case != "keepall-simple") | ((.new // []) + (.old // []))[] | .kind == "row" and ((.fields | keys) == ["key", "value"])] | all) and
    (crec("keepall-simple") == ks) and
    (crec("lastevent") == le) and
    (crec("lastevent-scene-two") == ls) and
    (crec("firstevent") == fe)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-retention-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
