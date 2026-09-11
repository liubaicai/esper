#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-ext-time-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-ext-time-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews ext-time-slice: the ext_timed(value, 10 sec) sliding window, its projection variant with a separately deployed consumer and the epoch-referenced ext_timed_batch window, all driven purely by event timestamps with the clock pinned at the epoch (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-c960628819cddf06d0bd", "java-runtime-aa7fe5003096f5bf1cf8", "java-runtime-3af75eefff14ce16ae81"]) and
    (.javaNames == ["InfraExtTimeWindow", "InfraExtTimeWindowSceneTwo", "InfraExternallyTimedBatch"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 3) and
    ([.cases[].case] == ["ext-time-window", "ext-time-window-scene-two", "ext-timed-batch"]) and
    ([.cases[].ordinal] == [6, 7, 53]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "ext_timed(value, 10 sec) window over the key/value map schema: the sliding threshold newest-9999 releases expired rows as old data together with the arriving row in one invocation, and a delete releases old-only rows",
        "legacy win:ext_timed(value, 10 sec) projection window over theString/longBoxed with four module deployments and only the window statement listened across the expiry, delete and reinsert waves",
        "epoch-referenced ext_timed_batch(value, 10 sec, 0L) window over the key/value map schema: arrivals stay silent until the 10 second boundary, the flush releases the batch as new data with the replaced batch as old data, and deletes never touch the pending batch"
    ]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.  This slice has no advance-time steps (the
    # ext_timed and ext_timed_batch views expire off the event timestamp
    # property), so the fourth step shape is the bare {"op","case"} pair.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "ordinal", "runtimeId", "s0Epl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "mode", "op", "statement"] or . == ["case", "op"])) and
    # iteratorSnapshots is the execution'"'"'s assertPropsPerRowIterator
    # call-site count: four for ord 6 (J:719, 728, 738, 744), twelve for ord 7
    # (J:791, 798, 801, 806, 813, 818, 821, 825, 828, 832, 835, 839) and six for
    # ord 53 (J:3417, 3425, 3434, 3442, 3447, 3452).  The ord-7 count is the
    # corrected one: the frozen contract pinned 7, which counts only the
    # post-send sites and drops the five milestone re-checks the sibling slice
    # keeps; the Java source and the chain spec now pin 12.
    ([.cases[].iteratorSnapshots] == [4, 12, 6]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "consume", "delete"], ["create", "insert", "s0", "delete"]]) and
    ([.cases[].listened] == [["create", "s0"], ["create"], ["create", "s0"]]) and
    # per-case step counts: ext-time-window = 1 case + 4 deploys + 6 sends + 4
    # snapshots + 1 undeploy-all = 16; ext-time-window-scene-two = 1 + 4 + 7 +
    # 12 + 1 = 25; ext-timed-batch = 1 + 4 + 8 + 6 + 1 = 20 (16+25+20 = 61).
    ([.steps[] | select(.case == "ext-time-window")] | length == 16) and
    ([.steps[] | select(.case == "ext-time-window-scene-two")] | length == 25) and
    ([.steps[] | select(.case == "ext-timed-batch")] | length == 20) and
    (.steps | type == "array" and length == 61) and
    ([.steps[] | select(.op == "case")] | length == 3) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 12) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 3) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 3) and
    ([.steps[] | select(.op == "send")] | length == 21) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 16) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 5) and
    ([.steps[] | select(.op == "snapshot")] | length == 22) and
    # no virtual time in this slice: every delivery is driven by the event
    # timestamp property and the clock stays at the epoch.
    ([.steps[] | select(.op == "advance-time")] | length == 0) and
    # every snapshot targets the window statement "create" and is exact-order.
    ([.steps[] | select(.op == "snapshot") | .statement] | all(. == "create")) and
    ([.steps[] | select(.op == "snapshot") | .mode] | all(. == "ordered")) and
    # per-case op signatures pin the exact wave order (sends, snapshots, the
    # sentinel market deletes and the teardown), independently of the counts.
    ([.steps[] | select(.case == "ext-time-window") | .op] | join(",") == "case,deploy,deploy,deploy,deploy,send,snapshot,send,send,snapshot,send,snapshot,send,snapshot,send,undeploy-all") and
    ([.steps[] | select(.case == "ext-time-window-scene-two") | .op] | join(",") == "case,deploy,deploy,deploy,deploy,send,snapshot,send,snapshot,send,snapshot,snapshot,send,snapshot,snapshot,send,snapshot,snapshot,send,snapshot,snapshot,send,snapshot,snapshot,undeploy-all") and
    ([.steps[] | select(.case == "ext-timed-batch") | .op] | join(",") == "case,deploy,deploy,deploy,deploy,send,send,send,snapshot,send,snapshot,send,snapshot,send,snapshot,send,snapshot,send,snapshot,undeploy-all") and
    # byte-exact EPL pins, including the @public window create of the
    # multi-module ord-7 case and the separately deployed consume consumer.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowETW#ext_timed(value, 10 sec) as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindow.win:ext_timed(value, 10 sec) as select theString as key, longBoxed as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowETB#ext_timed_batch(value, 10 sec, 0L) as MySimpleKeyValueMap"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowETW select theString as key, longBoxed as value from SupportBean", "insert into MyWindow(key, value) select irstream theString, longBoxed from SupportBean", "insert into MyWindowETB select theString as key, longBoxed as value from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowETW", "", "@name('"'"'s0'"'"') select irstream key, value as value from MyWindowETB"]) and
    ([.cases[].consumeEpl] == ["", "@name('"'"'consume'"'"') select irstream key, value as value from MyWindow", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean delete from MyWindowETW where symbol = key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindow as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowETB as s1 where s0.symbol = s1.key"]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "consume") | .epl] == [.cases[1].consumeEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[2].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl, .cases[2].deleteEpl]) and
    # send payload shapes: every SupportBean carries theString plus longBoxed
    # (the window value/timestamp column) and every market delete carries only
    # symbol; the per-case value arrays pin the exact event-time ladder.
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | (.payload | keys)] | all(. == ["symbol"])) and
    ([.steps[] | select(.case == "ext-time-window" and .op == "send" and .eventType == "SupportBean") | [.payload.theString, .payload.longBoxed]] == [["E1", 1000], ["E2", 5000], ["E3", 10000], ["E4", 11000], ["E5", 15000]]) and
    ([.steps[] | select(.case == "ext-time-window" and .op == "send" and .eventType == "SupportMarketDataBean") | .payload.symbol] == ["E2"]) and
    ([.steps[] | select(.case == "ext-time-window-scene-two" and .op == "send" and .eventType == "SupportBean") | [.payload.theString, .payload.longBoxed]] == [["G1", 0], ["G2", 5000], ["G3", 10000], ["G4", 15000], ["G5", 21000]]) and
    ([.steps[] | select(.case == "ext-time-window-scene-two" and .op == "send" and .eventType == "SupportMarketDataBean") | .payload.symbol] == ["G2", "G3"]) and
    ([.steps[] | select(.case == "ext-timed-batch" and .op == "send" and .eventType == "SupportBean") | [.payload.theString, .payload.longBoxed]] == [["E1", 1000], ["E2", 8000], ["E3", 9999], ["E4", 10000], ["E5", 14000], ["E6", 21000]]) and
    ([.steps[] | select(.case == "ext-timed-batch" and .op == "send" and .eventType == "SupportMarketDataBean") | .payload.symbol] == ["E2", "E4"])
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-ext-time-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-ext-time-views.XXXXXX")
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
    "$script_root/InfraNamedWindowExtTimeViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowExtTimeViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def t0: "1970-01-01T00:00:00Z";
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def lis($c; $st; $q; $t; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new, "old": $old};
    def lisn($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def liso($c; $st; $q; $t; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "old": $old};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    # snap0 is the empty-iterator snapshot record: the new key is omitted, not
    # an empty array, exactly like the Go normalizer renders it.
    def snap0($c; $st; $t): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 6 (InfraExtTimeWindow, lines 706-752): one module, create and s0
    # listened, so every wave delivers create first and then s0.  The sliding
    # threshold newest-9999 keeps E1/E2/E3 at the 10000 arrival and releases E1
    # (1000) as old data together with E4 (11000) in ONE invocation; the E2
    # delete is an old-only wave and the E5 (15000) arrival sweeps the E2
    # tombstone silently, so it delivers new data only.  Four iterator
    # snapshots, twelve listener records, sixteen records in total.
    def etw: [
        lisn("ext-time-window"; "create"; 1; t0; [r("E1"; 1000)]),
        lisn("ext-time-window"; "s0"; 1; t0; [r("E1"; 1000)]),
        snapr("ext-time-window"; "create"; t0; [r("E1"; 1000)]),
        lisn("ext-time-window"; "create"; 2; t0; [r("E2"; 5000)]),
        lisn("ext-time-window"; "s0"; 2; t0; [r("E2"; 5000)]),
        lisn("ext-time-window"; "create"; 3; t0; [r("E3"; 10000)]),
        lisn("ext-time-window"; "s0"; 3; t0; [r("E3"; 10000)]),
        snapr("ext-time-window"; "create"; t0; [r("E1"; 1000), r("E2"; 5000), r("E3"; 10000)]),
        lis("ext-time-window"; "create"; 4; t0; [r("E4"; 11000)]; [r("E1"; 1000)]),
        lis("ext-time-window"; "s0"; 4; t0; [r("E4"; 11000)]; [r("E1"; 1000)]),
        snapr("ext-time-window"; "create"; t0; [r("E2"; 5000), r("E3"; 10000), r("E4"; 11000)]),
        liso("ext-time-window"; "create"; 5; t0; [r("E2"; 5000)]),
        liso("ext-time-window"; "s0"; 5; t0; [r("E2"; 5000)]),
        snapr("ext-time-window"; "create"; t0; [r("E3"; 10000), r("E4"; 11000)]),
        lisn("ext-time-window"; "create"; 6; t0; [r("E5"; 15000)]),
        lisn("ext-time-window"; "s0"; 6; t0; [r("E5"; 15000)])
    ];
    # Ord 7 (InfraExtTimeWindowSceneTwo, lines 755-843): four modules on one
    # path and only the window statement listened, so every wave is exactly one
    # create record.  G3 (10000) releases G1 (0) as old data together with the
    # arriving row; the G2 and G3 deletes are old-only waves; the tombstone of
    # G2 is swept silently when G4 arrives and the tombstone of G3 when G5
    # arrives.  Twelve iterator snapshots (the five milestone re-checks after
    # the G2 delete, the G3 expiry, the G4 arrival, the G3 delete and the G5
    # arrival included), seven listener records, nineteen records in total.
    def etw2: [
        lisn("ext-time-window-scene-two"; "create"; 1; t0; [r("G1"; 0)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G1"; 0)]),
        lisn("ext-time-window-scene-two"; "create"; 2; t0; [r("G2"; 5000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G1"; 0), r("G2"; 5000)]),
        liso("ext-time-window-scene-two"; "create"; 3; t0; [r("G2"; 5000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G1"; 0)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G1"; 0)]),
        lis("ext-time-window-scene-two"; "create"; 4; t0; [r("G3"; 10000)]; [r("G1"; 0)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G3"; 10000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G3"; 10000)]),
        lisn("ext-time-window-scene-two"; "create"; 5; t0; [r("G4"; 15000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G3"; 10000), r("G4"; 15000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G3"; 10000), r("G4"; 15000)]),
        liso("ext-time-window-scene-two"; "create"; 6; t0; [r("G3"; 10000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G4"; 15000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G4"; 15000)]),
        lisn("ext-time-window-scene-two"; "create"; 7; t0; [r("G5"; 21000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G4"; 15000), r("G5"; 21000)]),
        snapr("ext-time-window-scene-two"; "create"; t0; [r("G4"; 15000), r("G5"; 21000)])
    ];
    # Ord 53 (InfraExternallyTimedBatch, lines 3399-3456): one module, create
    # and s0 listened, epoch reference 0L.  E1/E2/E3 buffer silently (the 9999
    # arrival stays below the 10000 boundary), the E2 delete releases an
    # old-only row without touching the pending batch, the E4 (10000) arrival
    # flushes the replaced batch {E1, E3} as new data with no old data, the E4
    # delete releases old-only data and leaves the replaced batch intact (empty
    # iterator snapshot follows), E5 (14000) starts a new batch silently and the
    # E6 (21000) flush emits {E5} as new data with the replaced batch {E1, E3}
    # as old data.  Listener records are emitted at send time; the E6 snapshot
    # follows them at its source position (line 3452 before the IR pair on line
    # 3453).  Six iterator snapshots, eight listener records, fourteen records.
    def etb: [
        snapr("ext-timed-batch"; "create"; t0; [r("E1"; 1000), r("E2"; 8000), r("E3"; 9999)]),
        liso("ext-timed-batch"; "create"; 1; t0; [r("E2"; 8000)]),
        liso("ext-timed-batch"; "s0"; 1; t0; [r("E2"; 8000)]),
        snapr("ext-timed-batch"; "create"; t0; [r("E1"; 1000), r("E3"; 9999)]),
        lisn("ext-timed-batch"; "create"; 2; t0; [r("E1"; 1000), r("E3"; 9999)]),
        lisn("ext-timed-batch"; "s0"; 2; t0; [r("E1"; 1000), r("E3"; 9999)]),
        snapr("ext-timed-batch"; "create"; t0; [r("E4"; 10000)]),
        liso("ext-timed-batch"; "create"; 3; t0; [r("E4"; 10000)]),
        liso("ext-timed-batch"; "s0"; 3; t0; [r("E4"; 10000)]),
        snap0("ext-timed-batch"; "create"; t0),
        snapr("ext-timed-batch"; "create"; t0; [r("E5"; 14000)]),
        lis("ext-timed-batch"; "create"; 4; t0; [r("E5"; 14000)]; [r("E1"; 1000), r("E3"; 9999)]),
        lis("ext-timed-batch"; "s0"; 4; t0; [r("E5"; 14000)]; [r("E1"; 1000), r("E3"; 9999)]),
        snapr("ext-timed-batch"; "create"; t0; [r("E6"; 21000)])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-ext-time-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 49) and
    # 16 + 19 + 14 records, 27 listener and 22 snapshot.
    ((crec("ext-time-window") | length) == 16) and
    ((crec("ext-time-window-scene-two") | length) == 19) and
    ((crec("ext-timed-batch") | length) == 14) and
    ([.records[] | select(.operation == "listener")] | length == 27) and
    ([.records[] | select(.operation == "snapshot")] | length == 22) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("ext-time-window"; 16) + repv("ext-time-window-scene-two"; 19) + repv("ext-timed-batch"; 14))) and
    # per-case snapshot record counts: four for ord 6, twelve for ord 7 (the
    # corrected count: twelve assertPropsPerRowIterator call sites) and six for
    # ord 53.
    ((crec("ext-time-window") | map(select(.operation == "snapshot")) | length) == 4) and
    ((crec("ext-time-window-scene-two") | map(select(.operation == "snapshot")) | length) == 12) and
    ((crec("ext-timed-batch") | map(select(.operation == "snapshot")) | length) == 6) and
    # the slice never advances the clock, so every record carries the pinned
    # epoch and never the wall clock.
    ([.records[].time] | all(. == t0)) and
    # listener statement order and per-statement 1-based sequence, snapshots 0.
    # Ord 6 alternates create and s0 across six waves, ord 7 listens to create
    # only and emits seven records, ord 53 alternates create and s0 across four
    # waves.  27 listener records.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 6) + repv("create"; 7) + repa(["create", "s0"]; 4))) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6] + [1, 2, 3, 4, 5, 6, 7] + [1, 1, 2, 2, 3, 3, 4, 4])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    ([.records[] | select(.operation == "snapshot") | .statement] | all(. == "create")) and
    # the single empty-iterator snapshot is the ord-53 E4-delete state.
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 1) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | .case] == ["ext-timed-batch"]) and
    ([.records[] | select(.operation == "snapshot" and has("new")) | (.new | length) > 0] | all) and
    # every listener record carries at least one stream and every row projects
    # exactly the key/value fields of the three cases.
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    (crec("ext-time-window") == etw) and
    (crec("ext-time-window-scene-two") == etw2) and
    (crec("ext-timed-batch") == etb)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-ext-time-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
