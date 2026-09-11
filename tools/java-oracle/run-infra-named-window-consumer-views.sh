#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-named-window-consumer-views.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-named-window-consumer-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java" and
    # the top-level and per-case description strings are pinned here byte
    # exactly, independently of the oracle constant, because the Go loader
    # compares them verbatim.
    .description == "InfraNamedWindowViews consumer slice: the unique-key window with its filtered consumer and the keepall window with a late filtered aggregate, plus the prior-value, late univariate-statistics and late left-outer-join consumers whose preload and iterator shapes differ from their listener deltas (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/namedwindow/InfraNamedWindowViews.java)." and
    (.javaRuntimes == ["java-runtime-a7827f22ee0c135e84d2", "java-runtime-502dd5b0e84f28fb2c68", "java-runtime-5ccc9535c9241efda4cc", "java-runtime-5118f72a4d8684d593d0", "java-runtime-c49a6a43a1efd3fa0729"]) and
    (.javaNames == ["InfraFilteringConsumer", "InfraFilteringConsumerLateStart", "InfraPriorStats", "InfraLateConsumer", "InfraLateConsumerJoin"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 5) and
    ([.cases[].case] == ["filtering-consumer", "filtering-consumer-late-start", "prior-stats", "late-consumer", "late-consumer-join"]) and
    ([.cases[].ordinal] == [43, 45, 49, 50, 51]) and
    ([.cases[].runtimeId] == .javaRuntimes) and
    ([.cases[].executionName] == .javaNames) and
    ([.cases[].description] == [
        "unique(key) window over the theString/intPrimitive projection: a same-key arrival replaces the stored row in one callback carrying the new and the replaced row, and the filtered consumer evaluates its filter on both streams so the replaced row still passes while the filtered-out new row is dropped",
        "keepall window over the theString/intPrimitive projection with a late filtered aggregate consumer: the preload pushes the two matching window rows as one update so the sum starts at 7, and a filtered-out arrival never enters the aggregation, not even for its later delete",
        "keepall map window with a prior-values consumer and a univariate-statistics consumer: prior(1|2, key) reads the statement'"'"'s own arrival history while the uni(value) view exposes the running average as a single-row iterator on every update",
        "keepall map window whose univariate-statistics consumer and count(*) consumer are both deployed late: the two independent preloads start the average at 1.5 and the count at 4, and the statistics consumer is irstream so the previous average is delivered as old data",
        "keepall map window whose left-outer join consumer against the market-data window is deployed late: the replayed window arrives in the join before the first iterator read, the market send matches both rows in either order, and the join iterator recomputes the full result so an unmatched row vanishes without a remove-stream record"
    ]) and
    # the case key set and the step key sets are pinned exactly: the Go loader
    # rejects any other field set.  The s2/s3 slots pin the extra consumer
    # statements this chain deploys; consume/var/onSet stay empty because no
    # case uses them.  The ord-51 case marker carries the any-order mode.
    ([.cases[] | (keys | sort)] | all(. == ["case", "consumeEpl", "createEpl", "deleteEpl", "deploys", "description", "executionName", "insertEpl", "iteratorSnapshots", "listened", "onSetEpl", "ordinal", "runtimeId", "s0Epl", "s2Epl", "s3Epl", "varEpl"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "fields", "mode", "op", "statement"] or . == ["case", "op", "statement"] or . == ["case", "op"] or . == ["case", "mode", "op"])) and
    # iteratorSnapshots counts the snapshot steps: five for ord 43
    # (INV:2969/2970/2981/2982/2991), six for ord 45 (INV:3136/3140/3144/3148/
    # 3156/3160), four for ord 49 (INV:3241/3246/3251/3256), seven for ord 50
    # (INV:3289/3293/3297/3301/3302/3306/3307) and four for ord 51
    # (INV:3343/3357/3361/3366).
    ([.cases[].iteratorSnapshots] == [5, 6, 4, 7, 4]) and
    ([.cases[].deploys] == [["create", "insert", "s0", "delete"], ["create", "insert", "s0", "delete"], ["create", "insert", "s0", "s3"], ["create", "insert", "s0", "s2"], ["create", "insert", "s2"]]) and
    ([.cases[].listened] == [["create", "s0"], ["s0"], ["s0", "s3"], ["create", "s0"], ["create", "s2"]]) and
    # per-case step counts: filtering-consumer = 1 case + 4 deploys + 8 sends +
    # 5 snapshots + 1 undeploy-all = 19; filtering-consumer-late-start =
    # 1 + 4 + 8 + 6 + 3 undeploys = 22; prior-stats = 1 + 4 + 4 + 4 + 1 = 14;
    # late-consumer = 1 + 4 + 5 + 7 + 1 = 18; late-consumer-join =
    # 1 + 3 + 5 + 4 + 1 = 14 (19+22+14+18+14 = 87).
    ([.steps[] | select(.case == "filtering-consumer")] | length == 19) and
    ([.steps[] | select(.case == "filtering-consumer-late-start")] | length == 22) and
    ([.steps[] | select(.case == "prior-stats")] | length == 14) and
    ([.steps[] | select(.case == "late-consumer")] | length == 18) and
    ([.steps[] | select(.case == "late-consumer-join")] | length == 14) and
    (.steps | type == "array" and length == 87) and
    ([.steps[] | select(.op == "case")] | length == 5) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "undeploy")] | length == 3) and
    ([.steps[] | select(.op == "deploy")] | length == 19) and
    ([.steps[] | select(.op == "deploy" and .statement == "create")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert")] | length == 5) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0")] | length == 4) and
    ([.steps[] | select(.op == "deploy" and .statement == "s2")] | length == 2) and
    ([.steps[] | select(.op == "deploy" and .statement == "s3")] | length == 1) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete")] | length == 2) and
    ([.steps[] | select(.op == "send")] | length == 30) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 24) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean")] | length == 6) and
    ([.steps[] | select(.op == "snapshot")] | length == 26) and
    ([.steps[] | select(.op == "advance-time")] | length == 0) and
    # snapshot statements, modes and projections in source order: the ord-43
    # unique-view window and its filtered consumer (any/ordered), the ord-45
    # sum states, the ord-49 statistics states, the ord-50 statistics and count
    # states and the ord-51 join states (ordered while only unmatched rows
    # exist, any afterwards).
    ([.steps[] | select(.op == "snapshot") | .statement] == ["create", "s0", "create", "s0", "s0", "s0", "s0", "s0", "s0", "s0", "s0", "s3", "s3", "s3", "s3", "s0", "s0", "s0", "s2", "s0", "s0", "s2", "s2", "s2", "s2", "s2"]) and
    ([.steps[] | select(.op == "snapshot") | .mode] == ["any", "ordered", "any", "ordered", "any", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "ordered", "any", "any", "any"]) and
    ([.steps[] | select(.op == "snapshot" and .statement == "create") | .fields] | all(. == ["key", "value"])) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0" and .case == "filtering-consumer") | .fields] | all(. == ["key", "value"])) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0" and .case == "filtering-consumer-late-start") | .fields] | all(. == ["sumvalue"])) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s0" and .case == "late-consumer") | .fields] | all(. == ["average"])) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s3") | .fields] | all(. == ["average"])) and
    ([.steps[] | select(.op == "snapshot" and .statement == "s2") | .fields] | all(. == ["cnt"] or . == ["key", "value", "symbol"])) and
    # byte-exact EPL pins: the unique-key window of ord 43 and its filtered
    # consumer, the late filtered aggregate of ord 45, the prior/statistics
    # pair of ord 49, the late statistics and count consumers of ord 50 and
    # the late left-outer join of ord 51.
    ([.cases[].createEpl] == ["@name('"'"'create'"'"') create window MyWindowFC#unique(key) as select theString as key, intPrimitive as value from SupportBean", "@name('"'"'create'"'"') @public create window MyWindowFCLS#keepall as select theString as key, intPrimitive as value from SupportBean", "@name('"'"'create'"'"') create window MyWindowPS#keepall as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindowLCL#keepall as MySimpleKeyValueMap", "@name('"'"'create'"'"') @public create window MyWindowLCJ#keepall as MySimpleKeyValueMap"]) and
    ([.cases[].insertEpl] == ["insert into MyWindowFC select theString as key, intPrimitive as value from SupportBean", "insert into MyWindowFCLS select theString as key, intPrimitive as value from SupportBean", "insert into MyWindowPS select theString as key, longBoxed as value from SupportBean", "insert into MyWindowLCL select theString as key, longBoxed as value from SupportBean", "insert into MyWindowLCJ select theString as key, longBoxed as value from SupportBean"]) and
    ([.cases[].s0Epl] == ["@name('"'"'s0'"'"') select irstream key, value as value from MyWindowFC(value > 0, value < 10)", "@name('"'"'s0'"'"') select irstream sum(value) as sumvalue from MyWindowFCLS(value > 0, value < 10)", "@name('"'"'s0'"'"') select prior(1, key) as priorKeyOne, prior(2, key) as priorKeyTwo from MyWindowPS", "@name('"'"'s0'"'"') select irstream average from MyWindowLCL#uni(value)", ""]) and
    ([.cases[].s2Epl] == ["", "", "", "@name('"'"'s2'"'"') select count(*) as cnt from MyWindowLCL", "@name('"'"'s2'"'"') select key, value, symbol from MyWindowLCJ as s0 left outer join SupportMarketDataBean#keepall as s1 on s0.value = s1.volume"]) and
    ([.cases[].s3Epl] == ["", "", "@name('"'"'s3'"'"') select average from MyWindowPS#uni(value)", "", ""]) and
    ([.cases[].consumeEpl] == ["", "", "", "", ""]) and
    ([.cases[].deleteEpl] == ["@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowFC as s1 where s0.symbol = s1.key", "@name('"'"'delete'"'"') on SupportMarketDataBean as s0 delete from MyWindowFCLS as s1 where s0.symbol = s1.key", "", "", ""]) and
    ([.cases[].varEpl] == ["", "", "", "", ""]) and
    ([.cases[].onSetEpl] == ["", "", "", "", ""]) and
    ([.steps[] | select(.op == "deploy" and .statement == "create") | .epl] == [.cases[0].createEpl, .cases[1].createEpl, .cases[2].createEpl, .cases[3].createEpl, .cases[4].createEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "insert") | .epl] == [.cases[0].insertEpl, .cases[1].insertEpl, .cases[2].insertEpl, .cases[3].insertEpl, .cases[4].insertEpl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s0") | .epl] == [.cases[0].s0Epl, .cases[1].s0Epl, .cases[2].s0Epl, .cases[3].s0Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s2") | .epl] == [.cases[3].s2Epl, .cases[4].s2Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "s3") | .epl] == [.cases[2].s3Epl]) and
    ([.steps[] | select(.op == "deploy" and .statement == "delete") | .epl] == [.cases[0].deleteEpl, .cases[1].deleteEpl]) and
    # send payload shapes: theString plus intPrimitive for the two intPrimitive
    # projections of ords 43/45, theString plus longBoxed for the map windows of
    # ords 49/50/51, the market trigger of ords 43/45 carries symbol alone and
    # the join trigger of ord 51 carries symbol plus the boxed Long volume.
    ([.steps[] | select(.op == "send" and (.case == "filtering-consumer" or .case == "filtering-consumer-late-start") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and (.case == "prior-stats" or .case == "late-consumer" or .case == "late-consumer-join") and .eventType == "SupportBean") | (.payload | keys | sort)] | all(. == ["longBoxed", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean" and (.case == "filtering-consumer" or .case == "filtering-consumer-late-start")) | (.payload | keys)] | all(. == ["symbol"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean" and .case == "late-consumer-join") | (.payload | keys | sort)] | all(. == ["symbol", "volume"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean" and .case == "late-consumer-join") | .payload.volume] == [1, 2]) and
    # placement pins: every case opens with its module(s) in source order; the
    # three late modules sit at their source positions (ord 45'"'"'s consumer after
    # the three fills and its delete after the four consumer snapshots, ord 50'"'"'s
    # statistics consumer after two fills and its count consumer after the two
    # updates, ord 51'"'"'s join consumer after the two fills); the undeploy order
    # of ord 45 is s0, delete, create and every case ends with its teardown.
    ([.steps[] | select(.case == "filtering-consumer")] | .[0] == {"op": "case", "case": "filtering-consumer"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "delete" and .[5].op == "send" and .[18].op == "undeploy-all") and
    ([.steps[] | select(.case == "filtering-consumer-late-start")] | .[0] == {"op": "case", "case": "filtering-consumer-late-start"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].op == "send" and .[6].op == "deploy" and .[6].statement == "s0" and .[7].op == "snapshot" and .[14].op == "deploy" and .[14].statement == "delete" and .[19].op == "undeploy" and .[19].statement == "s0" and .[20].statement == "delete" and .[21].statement == "create") and
    ([.steps[] | select(.case == "prior-stats")] | .[0] == {"op": "case", "case": "prior-stats"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].statement == "s0" and .[4].statement == "s3" and .[5].op == "send" and .[13].op == "undeploy-all") and
    ([.steps[] | select(.case == "late-consumer")] | .[0] == {"op": "case", "case": "late-consumer"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].op == "send" and .[4].op == "send" and .[5].op == "deploy" and .[5].statement == "s0" and .[11].op == "deploy" and .[11].statement == "s2" and .[17].op == "undeploy-all") and
    ([.steps[] | select(.case == "late-consumer-join")] | .[0] == {"op": "case", "case": "late-consumer-join", "mode": "any"} and .[1].statement == "create" and .[2].statement == "insert" and .[3].op == "send" and .[4].op == "send" and .[5].op == "deploy" and .[5].statement == "s2" and .[6].op == "snapshot" and .[6].mode == "ordered" and .[13].op == "undeploy-all")
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-named-window-consumer-views replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-named-window-consumer-views.XXXXXX")
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
    "$script_root/InfraNamedWindowConsumerViewsScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNamedWindowConsumerViewsScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def r($k; $v): {"kind": "row", "fields": {"key": $k, "value": $v}};
    def sv($n): {"kind": "row", "fields": {"sumvalue": $n}};
    def av($n): {"kind": "row", "fields": {"average": $n}};
    def cv($n): {"kind": "row", "fields": {"cnt": $n}};
    def pr($one; $two): {"kind": "row", "fields": {"priorKeyOne": $one, "priorKeyTwo": $two}};
    def jr($k; $v; $s): {"kind": "row", "fields": {"key": $k, "value": $v, "symbol": $s}};
    def nul: {"state": "null"};
    def lis($c; $st; $q; $t; $new): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new};
    def lis2($c; $st; $q; $t; $new; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "new": $new, "old": $old};
    def liso($c; $st; $q; $t; $old): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": $t, "old": $old};
    def snapr($c; $st; $t; $rows): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t, "new": $rows};
    # snap0 is the exhausted-iterator snapshot of INV:2982: the new key is
    # omitted, not an empty array, exactly like the Go normalizer renders it.
    def snap0($c; $st; $t): {"case": $c, "operation": "snapshot", "statement": $st, "sequence": 0, "time": $t};
    def crec($c): [.records[] | select(.case == $c)];
    # repv repeats a scalar n times and repa repeats the elements of an array n
    # times (jq does not support array * number).
    def repv($v; $n): [range($n) | $v];
    def repa($a; $n): [range($n) | $a[]];
    # Ord 43 (InfraFilteringConsumer, lines 2951-2993): the create window is a
    # unique-key window whose replacement arrives as ONE new-plus-old delta
    # (records 2 and 14), the delete waves are old-only, and the filtered
    # consumer drops the non-matching new row while still delivering the
    # passing replaced row (record 3) or staying silent for a fully filtered
    # pair (G3).  Its iterator states are the HashMap-ordered window (any) and
    # the filtered consumer (ordered, including the exhausted state of
    # INV:2982); the suite leaves the two closing waves unasserted.
    def fc: [
        lis("filtering-consumer"; "create"; 1; "1970-01-01T00:00:00Z"; [r("G1"; 5)]),
        lis("filtering-consumer"; "s0"; 1; "1970-01-01T00:00:00Z"; [r("G1"; 5)]),
        lis2("filtering-consumer"; "create"; 2; "1970-01-01T00:00:00Z"; [r("G1"; 15)]; [r("G1"; 5)]),
        liso("filtering-consumer"; "s0"; 2; "1970-01-01T00:00:00Z"; [r("G1"; 5)]),
        lis("filtering-consumer"; "create"; 3; "1970-01-01T00:00:00Z"; [r("G2"; 8)]),
        lis("filtering-consumer"; "s0"; 3; "1970-01-01T00:00:00Z"; [r("G2"; 8)]),
        snapr("filtering-consumer"; "create"; "1970-01-01T00:00:00Z"; [r("G1"; 15), r("G2"; 8)]),
        snapr("filtering-consumer"; "s0"; "1970-01-01T00:00:00Z"; [r("G2"; 8)]),
        liso("filtering-consumer"; "create"; 4; "1970-01-01T00:00:00Z"; [r("G2"; 8)]),
        liso("filtering-consumer"; "s0"; 4; "1970-01-01T00:00:00Z"; [r("G2"; 8)]),
        lis("filtering-consumer"; "create"; 5; "1970-01-01T00:00:00Z"; [r("G3"; -1)]),
        snapr("filtering-consumer"; "create"; "1970-01-01T00:00:00Z"; [r("G1"; 15), r("G3"; -1)]),
        snap0("filtering-consumer"; "s0"; "1970-01-01T00:00:00Z"),
        liso("filtering-consumer"; "create"; 6; "1970-01-01T00:00:00Z"; [r("G3"; -1)]),
        lis2("filtering-consumer"; "create"; 7; "1970-01-01T00:00:00Z"; [r("G1"; 6)]; [r("G1"; 15)]),
        lis("filtering-consumer"; "s0"; 5; "1970-01-01T00:00:00Z"; [r("G1"; 6)]),
        lis("filtering-consumer"; "create"; 8; "1970-01-01T00:00:00Z"; [r("G2"; 7)]),
        lis("filtering-consumer"; "s0"; 6; "1970-01-01T00:00:00Z"; [r("G2"; 7)]),
        snapr("filtering-consumer"; "s0"; "1970-01-01T00:00:00Z"; [r("G1"; 6), r("G2"; 7)])
    ];
    # Ord 45 (InfraFilteringConsumerLateStart, lines 3125-3164): the late
    # consumer preloads the two matching window rows as one update, so its
    # first iterator state is 7; each later aggregate change is one IR pair and
    # the two filtered-out market deletes leave the state untouched.
    def fcls: [
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(7)]),
        lis2("filtering-consumer-late-start"; "s0"; 1; "1970-01-01T00:00:00Z"; [sv(8)]; [sv(7)]),
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(8)]),
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(8)]),
        lis2("filtering-consumer-late-start"; "s0"; 2; "1970-01-01T00:00:00Z"; [sv(17)]; [sv(8)]),
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(17)]),
        lis2("filtering-consumer-late-start"; "s0"; 3; "1970-01-01T00:00:00Z"; [sv(16)]; [sv(17)]),
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(16)]),
        snapr("filtering-consumer-late-start"; "s0"; "1970-01-01T00:00:00Z"; [sv(16)])
    ];
    # Ord 49 (InfraPriorStats, lines 3226-3258): prior(1|2, key) walks the
    # statement own arrival history (null/null, E1/null, E2/E1, E3/E2) while
    # the uni(value) view reports the running average as a single-row iterator;
    # neither listener carries old data because s3 is not irstream.
    def ps: [
        lis("prior-stats"; "s0"; 1; "1970-01-01T00:00:00Z"; [pr(nul; nul)]),
        lis("prior-stats"; "s3"; 1; "1970-01-01T00:00:00Z"; [av(1)]),
        snapr("prior-stats"; "s3"; "1970-01-01T00:00:00Z"; [av(1)]),
        lis("prior-stats"; "s0"; 2; "1970-01-01T00:00:00Z"; [pr("E1"; nul)]),
        lis("prior-stats"; "s3"; 2; "1970-01-01T00:00:00Z"; [av(1.5)]),
        snapr("prior-stats"; "s3"; "1970-01-01T00:00:00Z"; [av(1.5)]),
        lis("prior-stats"; "s0"; 3; "1970-01-01T00:00:00Z"; [pr("E2"; "E1")]),
        lis("prior-stats"; "s3"; 3; "1970-01-01T00:00:00Z"; [av(1.6666666666666667)]),
        snapr("prior-stats"; "s3"; "1970-01-01T00:00:00Z"; [av(1.6666666666666667)]),
        lis("prior-stats"; "s0"; 4; "1970-01-01T00:00:00Z"; [pr("E3"; "E2")]),
        lis("prior-stats"; "s3"; 4; "1970-01-01T00:00:00Z"; [av(1.75)]),
        snapr("prior-stats"; "s3"; "1970-01-01T00:00:00Z"; [av(1.75)])
    ];
    # Ord 50 (InfraLateConsumer, lines 3269-3309): the statistics consumer
    # preloads the two filled rows (1.5) and is irstream, so every update is one
    # IR pair; the count consumer preloads the four rows that exist at its own
    # deploy time.  The create listener keeps firing on the three arrivals that
    # follow the consumer deploy even though the suite asserts it only twice.
    def lc: [
        lis("late-consumer"; "create"; 1; "1970-01-01T00:00:00Z"; [r("E1"; 1)]),
        lis("late-consumer"; "create"; 2; "1970-01-01T00:00:00Z"; [r("E2"; 2)]),
        snapr("late-consumer"; "s0"; "1970-01-01T00:00:00Z"; [av(1.5)]),
        lis("late-consumer"; "create"; 3; "1970-01-01T00:00:00Z"; [r("E3"; 2)]),
        lis2("late-consumer"; "s0"; 1; "1970-01-01T00:00:00Z"; [av(1.6666666666666667)]; [av(1.5)]),
        snapr("late-consumer"; "s0"; "1970-01-01T00:00:00Z"; [av(1.6666666666666667)]),
        lis("late-consumer"; "create"; 4; "1970-01-01T00:00:00Z"; [r("E4"; 2)]),
        lis2("late-consumer"; "s0"; 2; "1970-01-01T00:00:00Z"; [av(1.75)]; [av(1.6666666666666667)]),
        snapr("late-consumer"; "s0"; "1970-01-01T00:00:00Z"; [av(1.75)]),
        snapr("late-consumer"; "s2"; "1970-01-01T00:00:00Z"; [cv(4)]),
        snapr("late-consumer"; "s0"; "1970-01-01T00:00:00Z"; [av(1.75)]),
        lis("late-consumer"; "create"; 5; "1970-01-01T00:00:00Z"; [r("E5"; 3)]),
        lis2("late-consumer"; "s0"; 3; "1970-01-01T00:00:00Z"; [av(2)]; [av(1.75)]),
        snapr("late-consumer"; "s0"; "1970-01-01T00:00:00Z"; [av(2)]),
        snapr("late-consumer"; "s2"; "1970-01-01T00:00:00Z"; [cv(5)])
    ];
    # Ord 51 (InfraLateConsumerJoin, lines 3319-3368): the replayed window shows
    # up in the ordered null-padded iterator state before any market event
    # exists, the S1 send matches both rows in one new-only invocation (the
    # suite leaves that order open, hence the case mode any), S2 matches nothing
    # and E3 joins S2 in the recomputed result.
    def lcj: [
        lis("late-consumer-join"; "create"; 1; "1970-01-01T00:00:00Z"; [r("E1"; 1)]),
        lis("late-consumer-join"; "create"; 2; "1970-01-01T00:00:00Z"; [r("E2"; 1)]),
        snapr("late-consumer-join"; "s2"; "1970-01-01T00:00:00Z"; [jr("E1"; 1; nul), jr("E2"; 1; nul)]),
        lis("late-consumer-join"; "s2"; 1; "1970-01-01T00:00:00Z"; [jr("E1"; 1; "S1"), jr("E2"; 1; "S1")]),
        snapr("late-consumer-join"; "s2"; "1970-01-01T00:00:00Z"; [jr("E1"; 1; "S1"), jr("E2"; 1; "S1")]),
        snapr("late-consumer-join"; "s2"; "1970-01-01T00:00:00Z"; [jr("E1"; 1; "S1"), jr("E2"; 1; "S1")]),
        lis("late-consumer-join"; "create"; 3; "1970-01-01T00:00:00Z"; [r("E3"; 2)]),
        lis("late-consumer-join"; "s2"; 2; "1970-01-01T00:00:00Z"; [jr("E3"; 2; "S2")]),
        snapr("late-consumer-join"; "s2"; "1970-01-01T00:00:00Z"; [jr("E1"; 1; "S1"), jr("E2"; 1; "S1"), jr("E3"; 2; "S2")])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-named-window-consumer-views" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 64) and
    # 19 + 9 + 12 + 15 + 9 records; 38 listener and 26 snapshot records.
    ((crec("filtering-consumer") | length) == 19) and
    ((crec("filtering-consumer-late-start") | length) == 9) and
    ((crec("prior-stats") | length) == 12) and
    ((crec("late-consumer") | length) == 15) and
    ((crec("late-consumer-join") | length) == 9) and
    ([.records[] | select(.operation == "listener")] | length == 38) and
    ([.records[] | select(.operation == "snapshot")] | length == 26) and
    ([.records[].operation] | all(. == "listener" or . == "snapshot")) and
    ([.records[].case] == (repv("filtering-consumer"; 19) + repv("filtering-consumer-late-start"; 9) + repv("prior-stats"; 12) + repv("late-consumer"; 15) + repv("late-consumer-join"; 9))) and
    # per-case listener/snapshot split: 14+5, 3+6, 8+4, 8+7 and 5+4.
    ((crec("filtering-consumer") | map(select(.operation == "listener")) | length) == 14) and
    ((crec("filtering-consumer") | map(select(.operation == "snapshot")) | length) == 5) and
    ((crec("filtering-consumer-late-start") | map(select(.operation == "listener")) | length) == 3) and
    ((crec("filtering-consumer-late-start") | map(select(.operation == "snapshot")) | length) == 6) and
    ((crec("prior-stats") | map(select(.operation == "listener")) | length) == 8) and
    ((crec("prior-stats") | map(select(.operation == "snapshot")) | length) == 4) and
    ((crec("late-consumer") | map(select(.operation == "listener")) | length) == 8) and
    ((crec("late-consumer") | map(select(.operation == "snapshot")) | length) == 7) and
    ((crec("late-consumer-join") | map(select(.operation == "listener")) | length) == 5) and
    ((crec("late-consumer-join") | map(select(.operation == "snapshot")) | length) == 4) and
    # none of the five executions moves the clock, so every record carries the
    # pinned start instant.
    ([.records[].time] | all(. == "1970-01-01T00:00:00Z")) and
    # listener statements and per-statement 1-based sequences; snapshots carry
    # sequence 0.
    ([.records[] | select(.operation == "listener") | .statement] == (repa(["create", "s0"]; 4) + repv("create"; 3) + ["s0", "create", "s0"] + repv("s0"; 3) + repa(["s0", "s3"]; 4) + ["create", "create", "create", "s0", "create", "s0", "create", "s0"] + ["create", "create", "s2", "create", "s2"])) and
    ([.records[] | select(.operation == "listener") | .sequence] == ([1, 1, 2, 2, 3, 3, 4, 4, 5, 6, 7, 5, 8, 6] + [1, 2, 3] + [1, 1, 2, 2, 3, 3, 4, 4] + [1, 2, 3, 1, 4, 2, 5, 3] + [1, 2, 1, 3, 2])) and
    ([.records[] | select(.operation == "snapshot") | .sequence] | all(. == 0)) and
    # exactly one snapshot is the exhausted consumer iterator of INV:2982 and
    # no other snapshot omits its rows.
    ([.records[] | select(.operation == "snapshot" and (has("new") | not))] | length == 1) and
    ([.records[] | select(.operation == "snapshot" and (has("new") | not)) | [.case, .statement]] == [["filtering-consumer", "s0"]]) and
    # stream shapes: eight listener records carry both streams (the two unique
    # replacements of ord 43 and the three statistics pairs of ords 45 and 50),
    # four are old-only (the two delete waves of ord 43 seen by both listeners)
    # and no record carries more than three rows.
    ([.records[] | select(has("new") and has("old"))] | length == 8) and
    ([.records[] | select(has("old") and (has("new") | not)) | [.case, .statement, .sequence]] == [["filtering-consumer", "s0", 2], ["filtering-consumer", "create", 4], ["filtering-consumer", "s0", 4], ["filtering-consumer", "create", 6]]) and
    ([.records[] | ((.new // []) + (.old // []))[] | .kind == "row"] | all) and
    ([.records[] | select(.operation == "listener") | (has("new") or has("old"))] | all) and
    ([.records[] | select(.case == "filtering-consumer" and .operation == "listener") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["key", "value"]] | all) and
    ([.records[] | select(.case == "filtering-consumer-late-start") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["sumvalue"]] | all) and
    ([.records[] | select(.case == "prior-stats" and .operation == "listener" and .statement == "s0") | .new[] | (.fields | keys) == ["priorKeyOne", "priorKeyTwo"]] | all) and
    ([.records[] | select(.case == "prior-stats" and .statement == "s3") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["average"]] | all) and
    ([.records[] | select(.case == "late-consumer" and .statement == "s0") | ((.new // []) + (.old // []))[] | (.fields | keys) == ["average"]] | all) and
    ([.records[] | select(.case == "late-consumer" and .statement == "s2") | .new[] | (.fields | keys) == ["cnt"]] | all) and
    ([.records[] | select(.case == "late-consumer-join" and .operation == "listener" and .statement == "s2") | .new[] | (.fields | keys) == ["key", "symbol", "value"]] | all) and
    (crec("filtering-consumer") == fc) and
    (crec("filtering-consumer-late-start") == fcls) and
    (crec("prior-stats") == ps) and
    (crec("late-consumer") == lc) and
    (crec("late-consumer-join") == lcj)
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-named-window-consumer-views trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
