#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-infra-nwtable-on-select-aggregation.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

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
    .id == "infra-nwtable-on-select-aggregation" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    .javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/infra/nwtable/InfraNWTableOnSelect.java" and
    (.description | test("InfraNWTableOnSelect")) and
    (.javaRuntimes == ["java-runtime-a83c289aeff7f22c5d10", "java-runtime-113be8d12970feaf2fd0", "java-runtime-192de61f3c60d848e6c2", "java-runtime-99d89e9beadeae52ce8b", "java-runtime-fa42452055bf6fa1e4fd", "java-runtime-485188699c51be24f2cb", "java-runtime-36a62fff425f4c9b8cb6", "java-runtime-964dbe543e45c3f14f00", "java-runtime-d10a8674ccc6ecc5c70d", "java-runtime-5b8dddb5496cfbefa7d7"]) and
    (.javaNames == ["InfraSelectAggregationHavingStreamWildcard{namedWindow=true}", "InfraSelectAggregationHavingStreamWildcard{namedWindow=false}", "InfraSelectAggregation{namedWindow=true}", "InfraSelectAggregation{namedWindow=false}", "InfraSelectAggregationCorrelated{namedWindow=true}", "InfraSelectAggregationCorrelated{namedWindow=false}", "InfraSelectAggregationGrouping{namedWindow=true}", "InfraSelectAggregationGrouping{namedWindow=false}", "InfraOnSelectMultikeyWArray{namedWindow=true}", "InfraOnSelectMultikeyWArray{namedWindow=false}"]) and
    (.javaStaticIds == ["java-2ec871e1e4d7e32c39a8", "java-2ec871e1e4d7e32c39a8", "java-36dad03f6eadeaa780c1", "java-36dad03f6eadeaa780c1", "java-46aadb56f6ef28b6b2bd", "java-46aadb56f6ef28b6b2bd", "java-acc31838f7d1ff0ce7a6", "java-acc31838f7d1ff0ce7a6", "java-81de7615b15d1c9671eb", "java-81de7615b15d1c9671eb"]) and
    (.javaFlags == []) and
    (.cases | type == "array" and length == 10) and
    ([.cases[].case] == ["having-wildcard-nw", "having-wildcard-table", "select-agg-nw", "select-agg-table", "correlated-nw", "correlated-table", "grouping-nw", "grouping-table", "multikey-w-array-nw", "multikey-w-array-table"]) and
    ([.cases[].ordinal] == [6, 7, 16, 17, 18, 19, 20, 21, 26, 27]) and
    ([.cases[].runtimeId] == ["java-runtime-a83c289aeff7f22c5d10", "java-runtime-113be8d12970feaf2fd0", "java-runtime-192de61f3c60d848e6c2", "java-runtime-99d89e9beadeae52ce8b", "java-runtime-fa42452055bf6fa1e4fd", "java-runtime-485188699c51be24f2cb", "java-runtime-36a62fff425f4c9b8cb6", "java-runtime-964dbe543e45c3f14f00", "java-runtime-d10a8674ccc6ecc5c70d", "java-runtime-5b8dddb5496cfbefa7d7"]) and
    ([.cases[].executionName] == ["InfraSelectAggregationHavingStreamWildcard{namedWindow=true}", "InfraSelectAggregationHavingStreamWildcard{namedWindow=false}", "InfraSelectAggregation{namedWindow=true}", "InfraSelectAggregation{namedWindow=false}", "InfraSelectAggregationCorrelated{namedWindow=true}", "InfraSelectAggregationCorrelated{namedWindow=false}", "InfraSelectAggregationGrouping{namedWindow=true}", "InfraSelectAggregationGrouping{namedWindow=false}", "InfraOnSelectMultikeyWArray{namedWindow=true}", "InfraOnSelectMultikeyWArray{namedWindow=false}"]) and
    # the case key set and the step key set are pinned exactly: the Go loader
    # rejects any other field set.
    ([.cases[] | (keys | sort)] | all(. == ["case", "epl", "executionName", "observation", "ordinal", "runtimeId"])) and
    ([.steps[] | (keys | sort)] | all(. == ["case", "epl", "op", "statement"] or . == ["case", "eventType", "op", "payload"] or . == ["case", "op", "statement"] or . == ["case", "op"])) and
    # per-case step counts: having-wildcard = 1 + 3 deploys + 3 deployed +
    # 4 sends + 1 undeploy-all = 12; select-agg = 1 + 4 deploys + 4 deployed +
    # 8 sends + 1 types + 1 undeploy-all = 19; correlated = 1 + 3 deploys +
    # 3 deployed + 7 sends + 1 types + 1 undeploy-all = 16; grouping =
    # 1 + 5 deploys + 5 deployed + 11 sends + 1 types + 1 undeploy-all = 24;
    # multikey = 1 + 6 deploys + 2 deployed + 2 sends + 1 undeploy-all = 12
    # (12 + 12 + 19 + 19 + 16 + 16 + 24 + 24 + 12 + 12 = 166 total).
    ([.steps[] | select(.case == "having-wildcard-nw")] | length == 12) and
    ([.steps[] | select(.case == "having-wildcard-table")] | length == 12) and
    ([.steps[] | select(.case == "select-agg-nw")] | length == 19) and
    ([.steps[] | select(.case == "select-agg-table")] | length == 19) and
    ([.steps[] | select(.case == "correlated-nw")] | length == 16) and
    ([.steps[] | select(.case == "correlated-table")] | length == 16) and
    ([.steps[] | select(.case == "grouping-nw")] | length == 24) and
    ([.steps[] | select(.case == "grouping-table")] | length == 24) and
    ([.steps[] | select(.case == "multikey-w-array-nw")] | length == 12) and
    ([.steps[] | select(.case == "multikey-w-array-table")] | length == 12) and
    ([.steps[] | select(.op == "case")] | length == 10) and
    ([.steps[] | select(.op == "undeploy-all")] | length == 10) and
    ([.steps[] | select(.op == "deploy")] | length == 42) and
    ([.steps[] | select(.op == "deployed")] | length == 34) and
    ([.steps[] | select(.op == "deploy" and .statement == "FafInsert")] | length == 8) and
    ([.steps[] | select(.op == "types")] | length == 6) and
    ([.steps[] | select(.op == "send")] | length == 64) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean")] | length == 38) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_A")] | length == 22) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean_B")] | length == 4) and
    # byte-exact EPL pins, including the @name/@public annotations and the
    # verbatim group-by/having/values clauses the Java source writes.
    ([.steps[] | select(.op == "deploy" and .case == "having-wildcard-nw") | .epl] == ["@public create window MyInfraSHS#keepall as (a string, b int)", "insert into MyInfraSHS select theString as a, intPrimitive as b from SupportBean", "@name('"'"'select'"'"') on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc where id = a group by a having sum(b) = 20"]) and
    ([.steps[] | select(.op == "deploy" and .case == "having-wildcard-table") | .epl] == ["@public create table MyInfraSHS as (a string primary key, b int primary key)", "insert into MyInfraSHS select theString as a, intPrimitive as b from SupportBean", "@name('"'"'select'"'"') on SupportBean_A select mwc.* as mwcwin from MyInfraSHS mwc where id = a group by a having sum(b) = 20"]) and
    ([.steps[] | select(.op == "deploy" and .case == "select-agg-nw") | .epl] == ["@name('"'"'create'"'"') @public create window MyInfraSA#keepall as select theString as a, intPrimitive as b from SupportBean", "@name('"'"'select'"'"') on SupportBean_A select sum(b) as sumb from MyInfraSA", "insert into MyInfraSA select theString as a, intPrimitive as b from SupportBean", "on SupportBean_B delete from MyInfraSA where id = a"]) and
    ([.steps[] | select(.op == "deploy" and .case == "select-agg-table") | .epl] == ["@name('"'"'create'"'"') @public create table MyInfraSA (a string primary key, b int primary key)", "@name('"'"'select'"'"') on SupportBean_A select sum(b) as sumb from MyInfraSA", "insert into MyInfraSA select theString as a, intPrimitive as b from SupportBean", "on SupportBean_B delete from MyInfraSA where id = a"]) and
    ([.steps[] | select(.op == "deploy" and .case == "correlated-nw") | .epl] == ["@name('"'"'create'"'"') @public create window MyInfraSAC#keepall as select theString as a, intPrimitive as b from SupportBean", "@name('"'"'select'"'"') on SupportBean_A select sum(b) as sumb from MyInfraSAC where a = id", "insert into MyInfraSAC select theString as a, intPrimitive as b from SupportBean"]) and
    ([.steps[] | select(.op == "deploy" and .case == "correlated-table") | .epl] == ["@name('"'"'create'"'"') @public create table MyInfraSAC(a string primary key, b int primary key)", "@name('"'"'select'"'"') on SupportBean_A select sum(b) as sumb from MyInfraSAC where a = id", "insert into MyInfraSAC select theString as a, intPrimitive as b from SupportBean"]) and
    ([.steps[] | select(.op == "deploy" and .case == "grouping-nw") | .epl] == ["@name('"'"'create'"'"') @public create window MyInfraSAG#keepall as select theString as a, intPrimitive as b from SupportBean", "@name('"'"'select'"'"') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a order by a desc", "@name('"'"'selectTwo'"'"') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a having sum(b) > 5 order by a desc", "@name('"'"'insert'"'"') insert into MyInfraSAG select theString as a, intPrimitive as b from SupportBean", "on SupportBean_B delete from MyInfraSAG where id = a"]) and
    ([.steps[] | select(.op == "deploy" and .case == "grouping-table") | .epl] == ["@name('"'"'create'"'"') @public create table MyInfraSAG(a string primary key, b int primary key)", "@name('"'"'select'"'"') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a order by a desc", "@name('"'"'selectTwo'"'"') on SupportBean_A select a, sum(b) as sumb from MyInfraSAG group by a having sum(b) > 5 order by a desc", "@name('"'"'insert'"'"') insert into MyInfraSAG select theString as a, intPrimitive as b from SupportBean", "on SupportBean_B delete from MyInfraSAG where id = a"]) and
    ([.steps[] | select(.op == "deploy" and .case == "multikey-w-array-nw") | .epl] == ["@name('"'"'create'"'"') @public create window MyInfraPC#keepall as (id string, array int[], value int)", "@name('"'"'s0'"'"') on SupportBean select array, sum(value) as thesum from MyInfraPC group by array", "insert into MyInfraPC values('"'"'E1'"'"', {1, 2}, 10)", "insert into MyInfraPC values('"'"'E2'"'"', {1, 2}, 11)", "insert into MyInfraPC values('"'"'E3'"'"', {1, 2}, 21)", "insert into MyInfraPC values('"'"'E4'"'"', {1}, 22)"]) and
    ([.steps[] | select(.op == "deploy" and .case == "multikey-w-array-table") | .epl] == ["@name('"'"'create'"'"') @public create table MyInfraPC(id string primary key, array int[], value int)", "@name('"'"'s0'"'"') on SupportBean select array, sum(value) as thesum from MyInfraPC group by array", "insert into MyInfraPC values('"'"'E1'"'"', {1, 2}, 10)", "insert into MyInfraPC values('"'"'E2'"'"', {1, 2}, 11)", "insert into MyInfraPC values('"'"'E3'"'"', {1, 2}, 21)", "insert into MyInfraPC values('"'"'E4'"'"', {1}, 22)"]) and
    # send payload shapes: SupportBean carries theString+intPrimitive except
    # the two bare multikey triggers (empty payload); A/B sends carry id.
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean" and (.payload | length) > 0) | (.payload | keys | sort)] | all(. == ["intPrimitive", "theString"])) and
    ([.steps[] | select(.op == "send" and .eventType == "SupportBean" and (.payload | length) == 0)] | length == 4) and
    ([.steps[] | select(.op == "send" and (.eventType == "SupportBean_A" or .eventType == "SupportBean_B")) | (.payload | keys | sort)] | all(. == ["id"])) and
    (.steps | type == "array" and length == 166)
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid infra-nwtable-on-select-aggregation replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile \
        -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Dproject.build.sourceEncoding=UTF-8 \
        -Dproject.reporting.outputEncoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-infra-nwtable-on-select-aggregation.XXXXXX")
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
    "$script_root/InfraNWTableOnSelectAggregationScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    InfraNWTableOnSelectAggregationScenarioOracle "$scenario" > "$output"

if ! jq -e '
    def dep($c; $st; $q): {"case": $c, "operation": "deployed", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z"};
    def lis($c; $st; $q; $rows): {"case": $c, "operation": "listener", "statement": $st, "sequence": $q, "time": "1970-01-01T00:00:00Z", "new": $rows};
    def typ($c; $st; $entries): {"case": $c, "operation": "types", "statement": $st, "sequence": 0, "time": "1970-01-01T00:00:00Z", "value": $entries};
    def row($fields): {"kind": "row", "fields": $fields};
    def crec($c): [.records[] | select(.case == $c)];
    # having-wildcard: three deployed markers and one listener record whose
    # two new rows carry the mwcwin fragment values — the named-window
    # fragment is a Map (Java toString "{a=E1, b=16}"), the table fragment
    # an Object[] (["E1", 16]).
    def havingNw: [
        dep("having-wildcard-nw"; "create"; 1),
        dep("having-wildcard-nw"; "insert"; 1),
        dep("having-wildcard-nw"; "select"; 1),
        lis("having-wildcard-nw"; "select"; 1; [
            row({"mwcwin": "{a=E1, b=16}"}),
            row({"mwcwin": "{a=E1, b=4}"})
        ])
    ];
    def havingTbl: [
        dep("having-wildcard-table"; "create"; 1),
        dep("having-wildcard-table"; "insert"; 1),
        dep("having-wildcard-table"; "select"; 1),
        lis("having-wildcard-table"; "select"; 1; [
            row({"mwcwin": ["E1", 16]}),
            row({"mwcwin": ["E1", 4]})
        ])
    ];
    # select-agg: three deployed markers, the three sum rows, the delete
    # deployed marker and the types record (sumb=Integer).
    def selagg($c): [
        dep($c; "create"; 1),
        dep($c; "select"; 1),
        dep($c; "insert"; 1),
        lis($c; "select"; 1; [row({"sumb": 6})]),
        dep($c; "delete"; 1),
        lis($c; "select"; 2; [row({"sumb": 4})]),
        lis($c; "select"; 3; [row({"sumb": 14})]),
        typ($c; "select"; [{"name": "sumb", "type": "Integer"}])
    ];
    # correlated-nw: three deployed markers, four create listener rows
    # (the window statement fires on every insert), three select listener
    # rows (null, 2, 12) and the types record.
    def correlatedNw: [
        dep("correlated-nw"; "create"; 1),
        dep("correlated-nw"; "select"; 1),
        dep("correlated-nw"; "insert"; 1),
        lis("correlated-nw"; "create"; 1; [row({"a": "E1", "b": 1})]),
        lis("correlated-nw"; "create"; 2; [row({"a": "E2", "b": 2})]),
        lis("correlated-nw"; "create"; 3; [row({"a": "E3", "b": 3})]),
        lis("correlated-nw"; "select"; 1; [row({"sumb": {"state": "null"}})]),
        lis("correlated-nw"; "select"; 2; [row({"sumb": 2})]),
        lis("correlated-nw"; "create"; 4; [row({"a": "E2", "b": 10})]),
        lis("correlated-nw"; "select"; 3; [row({"sumb": 12})]),
        typ("correlated-nw"; "select"; [{"name": "sumb", "type": "Integer"}])
    ];
    # correlated-table: the create table statement listener never fires;
    # only the select rows land.
    def correlatedTbl: [
        dep("correlated-table"; "create"; 1),
        dep("correlated-table"; "select"; 1),
        dep("correlated-table"; "insert"; 1),
        lis("correlated-table"; "select"; 1; [row({"sumb": {"state": "null"}})]),
        lis("correlated-table"; "select"; 2; [row({"sumb": 2})]),
        lis("correlated-table"; "select"; 3; [row({"sumb": 12})]),
        typ("correlated-table"; "select"; [{"name": "sumb", "type": "Integer"}])
    ];
    # grouping: four deployed markers in EPL order; on each trigger Esper
    # dispatches selectTwo before select, so the listener records interleave
    # selectTwo-then-select per firing; then the delete deployed marker and
    # the types record (a=String, sumb=Integer).
    def grouping($c): [
        dep($c; "create"; 1),
        dep($c; "select"; 1),
        dep($c; "selectTwo"; 1),
        dep($c; "insert"; 1),
        lis($c; "selectTwo"; 1; [row({"a": "E1", "sumb": 6})]),
        lis($c; "select"; 1; [row({"a": "E2", "sumb": 2}), row({"a": "E1", "sumb": 6})]),
        lis($c; "selectTwo"; 2; [row({"a": "E2", "sumb": 12}), row({"a": "E1", "sumb": 106})]),
        lis($c; "select"; 2; [row({"a": "E4", "sumb": -1}), row({"a": "E2", "sumb": 12}), row({"a": "E1", "sumb": 106})]),
        dep($c; "delete"; 1),
        lis($c; "selectTwo"; 3; [row({"a": "E1", "sumb": 106})]),
        lis($c; "select"; 3; [row({"a": "E4", "sumb": -1}), row({"a": "E1", "sumb": 106})]),
        typ($c; "select"; [{"name": "a", "type": "String"}, {"name": "sumb", "type": "Integer"}])
    ];
    # multikey: two deployed markers and the two grouped-sum listener
    # records ({thesum=21} then the two rows for keys {1} and {1,2} in
    # the group order Esper emits).
    def multikey($c): [
        dep($c; "create"; 1),
        dep($c; "s0"; 1),
        lis($c; "s0"; 1; [row({"array": [1, 2], "thesum": 21})]),
        lis($c; "s0"; 2; [row({"array": [1], "thesum": 22}), row({"array": [1, 2], "thesum": 42})])
    ];
    .version == "esper-parity/v1" and
    .id == "infra-nwtable-on-select-aggregation" and
    .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c" and
    (.java | startswith("17")) and
    (.records | type == "array" and length == 74) and
    ((crec("having-wildcard-nw") | length) == 4) and
    ((crec("having-wildcard-table") | length) == 4) and
    ((crec("select-agg-nw") | length) == 8) and
    ((crec("select-agg-table") | length) == 8) and
    ((crec("correlated-nw") | length) == 11) and
    ((crec("correlated-table") | length) == 7) and
    ((crec("grouping-nw") | length) == 12) and
    ((crec("grouping-table") | length) == 12) and
    ((crec("multikey-w-array-nw") | length) == 4) and
    ((crec("multikey-w-array-table") | length) == 4) and
    ([.records[].case] == (["having-wildcard-nw", "having-wildcard-nw", "having-wildcard-nw", "having-wildcard-nw"]
        + ["having-wildcard-table", "having-wildcard-table", "having-wildcard-table", "having-wildcard-table"]
        + ["select-agg-nw", "select-agg-nw", "select-agg-nw", "select-agg-nw", "select-agg-nw", "select-agg-nw", "select-agg-nw", "select-agg-nw"]
        + ["select-agg-table", "select-agg-table", "select-agg-table", "select-agg-table", "select-agg-table", "select-agg-table", "select-agg-table", "select-agg-table"]
        + ["correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw", "correlated-nw"]
        + ["correlated-table", "correlated-table", "correlated-table", "correlated-table", "correlated-table", "correlated-table", "correlated-table"]
        + ["grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw", "grouping-nw"]
        + ["grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table", "grouping-table"]
        + ["multikey-w-array-nw", "multikey-w-array-nw", "multikey-w-array-nw", "multikey-w-array-nw"]
        + ["multikey-w-array-table", "multikey-w-array-table", "multikey-w-array-table", "multikey-w-array-table"])) and
    (crec("having-wildcard-nw") == havingNw) and
    (crec("having-wildcard-table") == havingTbl) and
    (crec("select-agg-nw") == selagg("select-agg-nw")) and
    (crec("select-agg-table") == selagg("select-agg-table")) and
    (crec("correlated-nw") == correlatedNw) and
    (crec("correlated-table") == correlatedTbl) and
    (crec("grouping-nw") == grouping("grouping-nw")) and
    (crec("grouping-table") == grouping("grouping-table")) and
    (crec("multikey-w-array-nw") == multikey("multikey-w-array-nw")) and
    (crec("multikey-w-array-table") == multikey("multikey-w-array-table"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid infra-nwtable-on-select-aggregation trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
