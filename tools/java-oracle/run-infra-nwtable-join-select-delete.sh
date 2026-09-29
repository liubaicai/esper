#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-join-select-delete.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    echo "failed to resolve Esper HEAD commit under $esper_root" >&2
    exit 1
}
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
[ "$java_version" = "17" ] || {
    echo "Java 17 is required (found version '$java_version' from $java_bin)" >&2
    exit 1
}

if ! jq -e '
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-join-select-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableJoin.java" and
    (.description | test("InfraNWTableJoin")) and
    (.description | test("InfraNWTableOnSelectWDelete")) and
    (.javaSourceFiles == ["regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableJoin.java", "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelectWDelete.java"]) and
    (.javaRuntimes == ["java-runtime-9aad0c9a0b81e251f6d4", "java-runtime-333f1a440da03d4b266a", "java-runtime-27d8980edc91c4593d34", "java-runtime-60c74e5e717dc6d9330e"]) and
    (.javaNames == ["InfraNWTableJoinSimple{namedWindow=true}", "InfraNWTableJoinSimple{namedWindow=false}", "InfraNWTableOnSelectWDeleteAssertion{namedWindow=true}", "InfraNWTableOnSelectWDeleteAssertion{namedWindow=false}"]) and
    (.javaStaticIds == ["java-675ca69dbc976b4c4448", "java-675ca69dbc976b4c4448", "java-5a29ff903cb7fd7a100d", "java-5a29ff903cb7fd7a100d"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 4) and
    ([.cases[].case] == ["join-nw", "join-table", "seldel-nw", "seldel-table"]) and
    ([.cases[].ordinal] == [0, 1, 0, 1]) and
    ([.cases[].runtimeId] == ["java-runtime-9aad0c9a0b81e251f6d4", "java-runtime-333f1a440da03d4b266a", "java-runtime-27d8980edc91c4593d34", "java-runtime-60c74e5e717dc6d9330e"]) and
    ([.cases[].executionName] == ["InfraNWTableJoinSimple{namedWindow=true}", "InfraNWTableJoinSimple{namedWindow=false}", "InfraNWTableOnSelectWDeleteAssertion{namedWindow=true}", "InfraNWTableOnSelectWDeleteAssertion{namedWindow=false}"]) and
    # case metadata epl fields pinned byte-exact: the join case lists the
    # source'"'"'s stmtTextCreate (schema line carries its own ";") plus the
    # insert and join statements; the seldel case lists the three deployment
    # texts once each.
    ([.cases[].epl] == ["@public @buseventtype create schema MyEvent(cid string);\n@public create window MyInfra.win:keepall() as MyEvent;\ninsert into MyInfra select * from MyEvent;\n@name('"'"'s0'"'"') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce, SupportBean#keepall() as sb where sb.theString = ce.cid;\n", "@public @buseventtype create schema MyEvent(cid string);\n@public create table MyInfra(cid string primary key);\ninsert into MyInfra select * from MyEvent;\n@name('"'"'s0'"'"') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce, SupportBean#keepall() as sb where sb.theString = ce.cid;\n", "@name('"'"'create'"'"') @public create window MyInfra#keepall as SupportBean;\ninsert into MyInfra select theString, intPrimitive from SupportBean;\n@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString;\n", "@name('"'"'create'"'"') @public create table MyInfra (theString string primary key, intPrimitive int primary key);\ninsert into MyInfra select theString, intPrimitive from SupportBean;\n@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString;\n"]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "op", "statement"] or . == ["case", "fields", "mode", "op", "statement"] or . == ["case", "op"])) and
    # per-case step counts: join = 1 + 3 deploys + 3 deployed + 5 sends +
    # 1 undeploy-all = 13; seldel = 1 + 4 deploys + 4 deployed + 6 sends +
    # 4 snapshots + 1 undeploy-all = 20 (13 + 13 + 20 + 20 = 66 total).
    ([.steps[] | select(.case == "join-nw")] | length == 13) and
    ([.steps[] | select(.case == "join-table")] | length == 13) and
    ([.steps[] | select(.case == "seldel-nw")] | length == 20) and
    ([.steps[] | select(.case == "seldel-table")] | length == 20) and
    ([.steps[] | select(.op == "case")] | length == 4) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 4) and
    ([.steps[] | select(.op == "deploy")] | length == 14) and
    ([.steps[] | select(.op == "deployed")] | length == 14) and
    ([.steps[] | select(.op == "snapshot")] | length == 8) and
    ([.steps[] | select(.op == "send")] | length == 22) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent")] | length == 6) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 12) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0")] | length == 4) and
    # byte-exact EPL pins: the join cases'"'"' first deploy carries the schema
    # and infra statements of the source'"'"'s stmtTextCreate as one ";\n"-joined
    # module; the seldel s0 deploy repeats for the SODA eplToModelCompileDeploy.
    ([.steps[] | select(.op == "deploy" and .case == "join-nw") | .epl] == ["@public @buseventtype create schema MyEvent(cid string);\n@public create window MyInfra.win:keepall() as MyEvent", "insert into MyInfra select * from MyEvent", "@name('"'"'s0'"'"') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce, SupportBean#keepall() as sb where sb.theString = ce.cid"]) and
    ([.steps[] | select(.op == "deploy" and .case == "join-table") | .epl] == ["@public @buseventtype create schema MyEvent(cid string);\n@public create table MyInfra(cid string primary key)", "insert into MyInfra select * from MyEvent", "@name('"'"'s0'"'"') select ce.cid as c0, sb.intPrimitive as c1 from MyInfra as ce, SupportBean#keepall() as sb where sb.theString = ce.cid"]) and
    ([.steps[] | select(.op == "deploy" and .case == "seldel-nw") | .epl] == ["@name('"'"'create'"'"') @public create window MyInfra#keepall as SupportBean", "insert into MyInfra select theString, intPrimitive from SupportBean", "@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString", "@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString"]) and
    ([.steps[] | select(.op == "deploy" and .case == "seldel-table") | .epl] == ["@name('"'"'create'"'"') @public create table MyInfra (theString string primary key, intPrimitive int primary key)", "insert into MyInfra select theString, intPrimitive from SupportBean", "@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString", "@name('"'"'s0'"'"') on SupportBean_S0 as s0 select and delete window(win.*).aggregate(0,(result,value) => result+value.intPrimitive) as c0 from MyInfra as win where s0.p00=win.theString"]) and
    # snapshot probes pin the create statement, the field projection and the
    # mode each Java iterator assertion used: the first window probe is
    # ordered (assertPropsPerRowIterator), every other probe any-order
    # (assertPropsPerRowIteratorAnyOrder).
    ([.steps[] | select(.op == "snapshot" and .case == "seldel-nw") | .mode] == ["ordered", "any", "any", "any"]) and
    ([.steps[] | select(.op == "snapshot" and .case == "seldel-table") | .mode] == ["any", "any", "any", "any"]) and
    ([.steps[] | select(.op == "snapshot") | .fields] | all(. == ["theString", "intPrimitive"])) and
    ([.steps[] | select(.op == "snapshot") | .statement] | all(. == "create")) and
    # send payloads pinned value-for-value: MyEvent carries cid only,
    # SupportBean carries theString+intPrimitive, SupportBean_S0 carries id+p00.
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent") | .payload] | all(keys == ["cid"])) and
    ([.steps[] | select(.op == "send" and .eventType == "MyEvent") | .payload.cid] == ["C1", "C2", "C3", "C1", "C2", "C3"]) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean") | .payload] | all(keys | sort == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .case == "join-nw" and .eventType == "SupportBean") | .payload] == [{"theString": "C2", "intPrimitive": 1}, {"theString": "C1", "intPrimitive": 4}]) and
    ([.steps[] | select(.op == "send" and .case == "join-table" and .eventType == "SupportBean") | .payload] == [{"theString": "C2", "intPrimitive": 1}, {"theString": "C1", "intPrimitive": 4}]) and
    ([.steps[] | select(.op == "send" and .case == "seldel-nw" and .eventType == "SupportBean") | .payload] == [{"theString": "E1", "intPrimitive": 1}, {"theString": "E2", "intPrimitive": 2}, {"theString": "E2", "intPrimitive": 3}, {"theString": "E2", "intPrimitive": 4}]) and
    ([.steps[] | select(.op == "send" and .case == "seldel-table" and .eventType == "SupportBean") | .payload] == [{"theString": "E1", "intPrimitive": 1}, {"theString": "E2", "intPrimitive": 2}, {"theString": "E2", "intPrimitive": 3}, {"theString": "E2", "intPrimitive": 4}]) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | .payload] | all(keys | sort == ["id", "p00"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_S0") | .payload] == [{"id": 100, "p00": "E1"}, {"id": 101, "p00": "E2"}, {"id": 100, "p00": "E1"}, {"id": 101, "p00": "E2"}]) and
    (.steps | type == "array" and length == 66)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-join-select-delete replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-join-select-delete.XXXXXX")
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
classpath="$classpath$cp_sep$compiler_cp$cp_sep$runtime_cp"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$(native_path "$classes")" \
    "$script_root/InfraNWTableJoinSelectDeleteScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableJoinSelectDeleteScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def dep($c; $st; $q): {"case": $c, "operation": "deployed", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z"};
    def lis($c; $st; $q; $rows): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def snap($c; $rows): {"case": $c, "operation": "snapshot", "statement": "create", "sequence": 0, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def row($fields): {"kind": "row", "fields": $fields};
    def crec($c): [.records[] | select(.case == $c)];
    # Snapshot rows canonicalize like the differential comparator: the
    # scenario any-mode probes (every seldel-table probe, seldel-nw probes
    # 2-4) compare the new array order-insensitively, so the raw iterator
    # order sorts by the canonical field encoding before the
    # record-for-record pin. seldel-nw probe 1 stays positional: the Java
    # source asserts it with assertPropsPerRowIterator.
    def canonSnap($c; $firstAny):
        reduce (crec($c)[]) as $rec ({nth: 0, out: []};
            if $rec.operation == "snapshot" then
                .out += [if .nth >= $firstAny
                         then $rec | .new |= sort_by(.fields | tojson)
                         else $rec end]
                | .nth += 1
            else .out += [$rec] end)
        | .out;
    # join: three deployed markers (create covers the schema+infra module)
    # and the two c0/c1 join rows; the MyEvent sends into the store are
    # silent because the SupportBean side is still empty.
    def joinRec($c): [
        dep($c; "create"; 1),
        dep($c; "insert"; 1),
        dep($c; "s0"; 1),
        lis($c; "s0"; 1; [row({"c0": "C2", "c1": 1})]),
        lis($c; "s0"; 2; [row({"c0": "C1", "c1": 4})])
    ];
    # seldel: three deployed markers, the four store-iterator snapshots
    # (empty store renders "new": []), the two select-delete sums and the
    # second s0 deployed marker from the SODA redeploy.
    def seldelRec($c): [
        dep($c; "create"; 1),
        dep($c; "insert"; 1),
        dep($c; "s0"; 1),
        snap($c; [row({"theString": "E1", "intPrimitive": 1}), row({"theString": "E2", "intPrimitive": 2})]),
        lis($c; "s0"; 1; [row({"c0": 1})]),
        snap($c; [row({"theString": "E2", "intPrimitive": 2})]),
        snap($c; [row({"theString": "E2", "intPrimitive": 2}), row({"theString": "E2", "intPrimitive": 3}), row({"theString": "E2", "intPrimitive": 4})]),
        lis($c; "s0"; 2; [row({"c0": 9})]),
        snap($c; []),
        dep($c; "s0"; 2)
    ];
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-join-select-delete" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 30) and
    ((crec("join-nw") | length) == 5) and
    ((crec("join-table") | length) == 5) and
    ((crec("seldel-nw") | length) == 10) and
    ((crec("seldel-table") | length) == 10) and
    ([.records[].case] == (["join-nw", "join-nw", "join-nw", "join-nw", "join-nw"]
        + ["join-table", "join-table", "join-table", "join-table", "join-table"]
        + ["seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw", "seldel-nw"]
        + ["seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table", "seldel-table"])) and
    (crec("join-nw") == joinRec("join-nw")) and
    (crec("join-table") == joinRec("join-table")) and
    (canonSnap("seldel-nw"; 1) == seldelRec("seldel-nw")) and
    (canonSnap("seldel-table"; 0) == seldelRec("seldel-table"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-join-select-delete trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
