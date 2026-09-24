#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-other-stream-expr.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided.
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

[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 1; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 1; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 1; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
    echo "cannot read Esper Git commit" >&2
    exit 1
}
[ "$actual_commit" = "$expected_commit" ] || {
    echo "Esper checkout is $actual_commit; expected $expected_commit" >&2
    exit 1
}

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

java_version=$($java_bin -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found ${java_version:-unknown}" >&2; exit 1; }

if ! jq -e -s '
    if length != 1 then false else .[0] as $s |
    (($s | keys_unsorted | sort) == ["cases","description","id","javaCommit","javaFlags","javaNames","javaRuntimes","javaSource","javaStaticIds","steps","version"])
    and $s.version == "esper-parity/v1"
    and $s.id == "epl-other-stream-expr"
    and $s.description == "EPLOtherStreamExpr stream method expressions: chained parameterized event-method calls with verbatim expression-text output names plus a SODA eplToModel round trip, static methods resolved through instance-property navigation, static-method where filters with stream-name, wildcard, and EventBean arguments, instance-method projections over left outer joins returning null on the absent join side, static-method-via-alias join columns, aliased and verbatim no-alias expression-text output names with Long/Double/String value rendering, stream-as-object join columns, a followed-by pattern with a static UDF filter referencing the prior tag, and invalid-select compile probes (Java source regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherStreamExpr.java)."
    and $s.javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and $s.javaSource == "regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/epl/other/EPLOtherStreamExpr.java"
    and $s.javaRuntimes == ["java-runtime-9277aaf4733e12e28d28","java-runtime-67e9ea0d239585623711","java-runtime-cdb6409058fe3e44770f","java-runtime-f33922181fbbb9c2e19b","java-runtime-cc45d135a75bb01736f0","java-runtime-469a37a746e59d25a115","java-runtime-a59b12bbe5788257c37b","java-runtime-827ea8daeea9baec40cf","java-runtime-027a02a57250c1d497cb"]
    and $s.javaNames == ["EPLOtherChainedParameterized","EPLOtherStreamFunction","EPLOtherInstanceMethodOuterJoin","EPLOtherInstanceMethodStatic","EPLOtherStreamInstanceMethodAliased","EPLOtherStreamInstanceMethodNoAlias","EPLOtherJoinStreamSelectNoWildcard","EPLOtherPatternStreamSelectNoWildcard","EPLOtherInvalidSelect"]
    and $s.javaStaticIds == ["java-6828e4919d88389a5b0c","java-e571ee83c24576b8aba7","java-bd1bca1463bae6798d0a","java-ae28c394fd146baf994e","java-b448cd11a74aaf56dbab","java-572869619d74ba3c95be","java-534aeb16b8d33707670a","java-dfb3493bb3eb08a778db","java-f1f45018481fb57e3382"]
    and $s.javaFlags == []
    and ($s.cases | type == "array" and length == 9)
    and $s.cases[0] == {"case":"chained-parameterized","ordinal":0,"runtimeId":"java-runtime-9277aaf4733e12e28d28","executionName":"EPLOtherChainedParameterized","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select top.getChildOne(\"abc\",10).getChildTwo(\"append\") from SupportChainTop as top"}
    and $s.cases[1] == {"case":"stream-function","ordinal":1,"runtimeId":"java-runtime-67e9ea0d239585623711","executionName":"EPLOtherStreamFunction","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZero(s0)"}
    and $s.cases[2] == {"case":"instance-method-outer-join","ordinal":2,"runtimeId":"java-runtime-cdb6409058fe3e44770f","executionName":"EPLOtherInstanceMethodOuterJoin","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select symbol, s1.getTheString() as theString from SupportMarketDataBean#keepall as s0 left outer join SupportBean#keepall as s1 on s0.symbol=s1.theString"}
    and $s.cases[3] == {"case":"instance-method-static","ordinal":3,"runtimeId":"java-runtime-f33922181fbbb9c2e19b","executionName":"EPLOtherInstanceMethodStatic","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select symbol, s1.getSimpleProperty() as simpleprop, s1.makeDefaultBean() as def from SupportMarketDataBean#keepall as s0 left outer join SupportBeanComplexProps#keepall as s1 on s0.symbol=s1.simpleProperty"}
    and $s.cases[4] == {"case":"stream-instance-method-aliased","ordinal":4,"runtimeId":"java-runtime-cc45d135a75bb01736f0","executionName":"EPLOtherStreamInstanceMethodAliased","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select s0.getVolume() as volume, s0.getSymbol() as symbol, s0.getPriceTimesVolume(2) as pvf from SupportMarketDataBean as s0 "}
    and $s.cases[5] == {"case":"stream-instance-method-no-alias","ordinal":5,"runtimeId":"java-runtime-469a37a746e59d25a115","executionName":"EPLOtherStreamInstanceMethodNoAlias","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select s0.getVolume(), s0.getPriceTimesVolume(3) from SupportMarketDataBean as s0 "}
    and $s.cases[6] == {"case":"join-stream-select","ordinal":6,"runtimeId":"java-runtime-a59b12bbe5788257c37b","executionName":"EPLOtherJoinStreamSelectNoWildcard","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select s0 as s0stream, s1 as s1stream from SupportMarketDataBean#keepall as s0, SupportBean#keepall as s1"}
    and $s.cases[7] == {"case":"pattern-stream-select","ordinal":7,"runtimeId":"java-runtime-827ea8daeea9baec40cf","executionName":"EPLOtherPatternStreamSelectNoWildcard","observation":"listener","iteratorSnapshots":0,"epl":"@name(\u0027s0\u0027) select * from pattern [every e1=SupportMarketDataBean -> e2=SupportBean(com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.compareEvents(e1, e2))]"}
    and $s.cases[8] == {"case":"invalid-select","ordinal":8,"runtimeId":"java-runtime-027a02a57250c1d497cb","executionName":"EPLOtherInvalidSelect","observation":"compile-error","iteratorSnapshots":0,"epl":"select s0.getString(1,2,3) from SupportBean as s0\nselect s0.abc() from SupportBean as s0\nselect s.theString from pattern [every [2] s=SupportBean] ee\n"}
    and ($s.steps | type == "array" and length == 54)
    and $s.steps[0] == {"op":"case","case":"chained-parameterized"}
    and $s.steps[1] == {"op":"deploy","case":"chained-parameterized","statement":"s0","epl":"@name(\u0027s0\u0027) select top.getChildOne(\"abc\",10).getChildTwo(\"append\") from SupportChainTop as top"}
    and $s.steps[2] == {"op":"send","case":"chained-parameterized","eventType":"SupportChainTop","payload":{}}
    and $s.steps[3] == {"op":"undeploy-all","case":"chained-parameterized"}
    and $s.steps[4] == {"op":"deploy","case":"chained-parameterized","statement":"s0-soda","epl":"@name(\u0027s0\u0027) select top.getChildOne(\"abc\",10).getChildTwo(\"append\") from SupportChainTop as top","mode":"soda"}
    and $s.steps[5] == {"op":"send","case":"chained-parameterized","eventType":"SupportChainTop","payload":{}}
    and $s.steps[6] == {"op":"undeploy-all","case":"chained-parameterized"}
    and $s.steps[7] == {"op":"deploy","case":"chained-parameterized","statement":"s0-static","epl":"@name(\u0027s0\u0027) select inside.getMyString() as val,inside.insideTwo.getMyOtherString() as val2 from SupportBeanStaticOuter"}
    and $s.steps[8] == {"op":"send","case":"chained-parameterized","eventType":"SupportBeanStaticOuter","payload":{}}
    and $s.steps[9] == {"op":"undeploy-all","case":"chained-parameterized"}
    and $s.steps[10] == {"op":"case","case":"stream-function"}
    and $s.steps[11] == {"op":"deploy","case":"stream-function","statement":"s0a","epl":"@name(\u0027s0\u0027) select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZero(s0)"}
    and $s.steps[12] == {"op":"deploy","case":"stream-function","statement":"s0b","epl":"@name(\u0027s0\u0027) select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZero(*)"}
    and $s.steps[13] == {"op":"deploy","case":"stream-function","statement":"s0c","epl":"@name(\u0027s0\u0027) select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZeroEventBean(s0)"}
    and $s.steps[14] == {"op":"deploy","case":"stream-function","statement":"s0d","epl":"@name(\u0027s0\u0027) select * from SupportMarketDataBean as s0 where com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.volumeGreaterZeroEventBean(*)"}
    and $s.steps[15] == {"op":"send","case":"stream-function","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":0,"feed":null}}
    and $s.steps[16] == {"op":"send","case":"stream-function","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":100,"feed":null}}
    and $s.steps[17] == {"op":"undeploy-all","case":"stream-function"}
    and $s.steps[18] == {"op":"case","case":"instance-method-outer-join"}
    and $s.steps[19] == {"op":"deploy","case":"instance-method-outer-join","statement":"s0","epl":"@name(\u0027s0\u0027) select symbol, s1.getTheString() as theString from SupportMarketDataBean#keepall as s0 left outer join SupportBean#keepall as s1 on s0.symbol=s1.theString"}
    and $s.steps[20] == {"op":"send","case":"instance-method-outer-join","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":0,"feed":null}}
    and $s.steps[21] == {"op":"undeploy-all","case":"instance-method-outer-join"}
    and $s.steps[22] == {"op":"case","case":"instance-method-static"}
    and $s.steps[23] == {"op":"deploy","case":"instance-method-static","statement":"s0","epl":"@name(\u0027s0\u0027) select symbol, s1.getSimpleProperty() as simpleprop, s1.makeDefaultBean() as def from SupportMarketDataBean#keepall as s0 left outer join SupportBeanComplexProps#keepall as s1 on s0.symbol=s1.simpleProperty"}
    and $s.steps[24] == {"op":"send","case":"instance-method-static","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":0,"feed":null}}
    and $s.steps[25] == {"op":"send","case":"instance-method-static","eventType":"SupportBeanComplexProps","payload":{"simpleProperty":"ACME"}}
    and $s.steps[26] == {"op":"undeploy-all","case":"instance-method-static"}
    and $s.steps[27] == {"op":"case","case":"stream-instance-method-aliased"}
    and $s.steps[28] == {"op":"deploy","case":"stream-instance-method-aliased","statement":"s0","epl":"@name(\u0027s0\u0027) select s0.getVolume() as volume, s0.getSymbol() as symbol, s0.getPriceTimesVolume(2) as pvf from SupportMarketDataBean as s0 "}
    and $s.steps[29] == {"op":"send","case":"stream-instance-method-aliased","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":4,"volume":99,"feed":null}}
    and $s.steps[30] == {"op":"undeploy-all","case":"stream-instance-method-aliased"}
    and $s.steps[31] == {"op":"case","case":"stream-instance-method-no-alias"}
    and $s.steps[32] == {"op":"deploy","case":"stream-instance-method-no-alias","statement":"s0a","epl":"@name(\u0027s0\u0027) select s0.getVolume(), s0.getPriceTimesVolume(3) from SupportMarketDataBean as s0 "}
    and $s.steps[33] == {"op":"send","case":"stream-instance-method-no-alias","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":4,"volume":2,"feed":null}}
    and $s.steps[34] == {"op":"deploy","case":"stream-instance-method-no-alias","statement":"s0b","epl":"@public @buseventtype create schema MyTestEvent as com.espertech.esper.regressionlib.suite.epl.other.EPLOtherStreamExpr$MyTestEvent"}
    and $s.steps[35] == {"op":"deploy","case":"stream-instance-method-no-alias","statement":"s0c","epl":"@name(\u0027s0\u0027) select s0.getValueAsInt(s0, \u0027id\u0027) as c0,s0.getValueAsInt(*, \u0027id\u0027) as c1 from MyTestEvent as s0"}
    and $s.steps[36] == {"op":"send","case":"stream-instance-method-no-alias","eventType":"MyTestEvent","payload":{"id":10}}
    and $s.steps[37] == {"op":"undeploy-all","case":"stream-instance-method-no-alias"}
    and $s.steps[38] == {"op":"case","case":"join-stream-select"}
    and $s.steps[39] == {"op":"deploy","case":"join-stream-select","statement":"s0a","epl":"@name(\u0027s0\u0027) select s0 as s0stream, s1 as s1stream from SupportMarketDataBean#keepall as s0, SupportBean#keepall as s1"}
    and $s.steps[40] == {"op":"deploy","case":"join-stream-select","statement":"s0b","epl":"@name(\u0027s0\u0027) select s0, s1 from SupportMarketDataBean#keepall as s0, SupportBean#keepall as s1"}
    and $s.steps[41] == {"op":"send","case":"join-stream-select","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":0,"feed":null}}
    and $s.steps[42] == {"op":"send","case":"join-stream-select","eventType":"SupportBean","payload":{"theString":null,"intPrimitive":0}}
    and $s.steps[43] == {"op":"undeploy-all","case":"join-stream-select"}
    and $s.steps[44] == {"op":"case","case":"pattern-stream-select"}
    and $s.steps[45] == {"op":"deploy","case":"pattern-stream-select","statement":"s0","epl":"@name(\u0027s0\u0027) select * from pattern [every e1=SupportMarketDataBean -> e2=SupportBean(com.espertech.esper.regressionlib.support.epl.SupportStaticMethodLib.compareEvents(e1, e2))]"}
    and $s.steps[46] == {"op":"send","case":"pattern-stream-select","eventType":"SupportMarketDataBean","payload":{"symbol":"ACME","price":0,"volume":0,"feed":null}}
    and $s.steps[47] == {"op":"send","case":"pattern-stream-select","eventType":"SupportBean","payload":{"theString":"ACME","intPrimitive":1}}
    and $s.steps[48] == {"op":"undeploy-all","case":"pattern-stream-select"}
    and $s.steps[49] == {"op":"case","case":"invalid-select"}
    and $s.steps[50] == {"op":"build-error","case":"invalid-select","statement":"getstring-args","epl":"select s0.getString(1,2,3) from SupportBean as s0"}
    and $s.steps[51] == {"op":"build-error","case":"invalid-select","statement":"abc-method","epl":"select s0.abc() from SupportBean as s0","expectError":"Failed to validate select-clause expression \u0027s0.abc()\u0027: Failed to solve \u0027abc\u0027 to either an date-time or enumeration method, an event property or a method on the event underlying object: Failed to resolve method \u0027abc\u0027: Could not find enumeration method, date-time method, instance method or property named \u0027abc\u0027 in class \u0027com.espertech.esper.common.internal.support.SupportBean\u0027 taking no parameters ["}
    and $s.steps[52] == {"op":"build-error","case":"invalid-select","statement":"pattern-indexed","epl":"select s.theString from pattern [every [2] s=SupportBean] ee","expectError":"Failed to validate select-clause expression \u0027s.theString\u0027: Failed to resolve property \u0027s.theString\u0027 (property \u0027s\u0027 is an indexed property and requires an index or enumeration method to access values)"}
    and $s.steps[53] == {"op":"undeploy-all","case":"invalid-select"}
    and ([ $s.steps[] | select(.op != "case" and .op != "deploy" and .op != "send" and .op != "build-error" and .op != "undeploy-all") ] | length) == 0
    and ([ $s.steps[] | select(.op == "undeploy-all") ] | length) == 11
    and ([ $s.steps[] | select(.op == "deploy") ] | length) == 16
    and ([ $s.steps[] | select(.op == "send") ] | length) == 15
    and ([ $s.steps[] | select(.op == "build-error") ] | length) == 3
    and ([ $s.steps[0:10][] | .case ] | unique) == ["chained-parameterized"]
    and ([ $s.steps[10:18][] | .case ] | unique) == ["stream-function"]
    and ([ $s.steps[18:22][] | .case ] | unique) == ["instance-method-outer-join"]
    and ([ $s.steps[22:27][] | .case ] | unique) == ["instance-method-static"]
    and ([ $s.steps[27:31][] | .case ] | unique) == ["stream-instance-method-aliased"]
    and ([ $s.steps[31:38][] | .case ] | unique) == ["stream-instance-method-no-alias"]
    and ([ $s.steps[38:44][] | .case ] | unique) == ["join-stream-select"]
    and ([ $s.steps[44:49][] | .case ] | unique) == ["pattern-stream-select"]
    and ([ $s.steps[49:54][] | .case ] | unique) == ["invalid-select"]
    and ([ $s.steps[] | select(.op == "send") | .eventType ]) == ["SupportChainTop","SupportChainTop","SupportBeanStaticOuter","SupportMarketDataBean","SupportMarketDataBean","SupportMarketDataBean","SupportMarketDataBean","SupportBeanComplexProps","SupportMarketDataBean","SupportMarketDataBean","MyTestEvent","SupportMarketDataBean","SupportBean","SupportMarketDataBean","SupportBean"]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | .payload.symbol ] | unique) == ["ACME"]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | [.payload.price, .payload.volume] ]) == [[0,0],[0,100],[0,0],[0,0],[4,99],[4,2],[0,0],[0,0]]
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportMarketDataBean") | .payload.feed ] | all(. == null))
    and ([ $s.steps[] | select(.op == "send" and .eventType == "SupportBean") | [.payload.theString, .payload.intPrimitive] ]) == [[null,0],["ACME",1]]
    and ([ $s.steps[] | select(.op == "deploy") | .statement ]) == ["s0","s0-soda","s0-static","s0a","s0b","s0c","s0d","s0","s0","s0","s0a","s0b","s0c","s0a","s0b","s0"]
    end
' "$scenario" >/dev/null 2>&1; then
    echo "scenario is not a valid epl-other-stream-expr replay: $scenario" >&2
    exit 1
fi

if [ "$skip_build" -eq 0 ]; then
    "$mvn_bin" -q -f "$esper_root/common/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/regression-lib/pom.xml" install -DskipTests -Dgpg.skip=true \
        -Dfile.encoding=UTF-8 -Duser.timezone=UTC
fi

work=$(mktemp -d "${TMPDIR:-/tmp}/esper-epl-other-stream-expr.XXXXXX")
cleanup() { rm -rf "$work"; }
trap cleanup EXIT HUP INT TERM

"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/common-avro/pom.xml" dependency:build-classpath \
    -Dmdep.outputFile="$work/avro-cp.txt" -Dmdep.includeScope=runtime \
    -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC

classes="$work/classes"
mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes"
classpath="$classpath:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes"
classpath="$classpath:$esper_root/regression-lib/target/classes"
classpath="$classpath:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt"):$(tr '\n' ':' < "$work/avro-cp.txt")"

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" \
    "$script_root/EPLOtherStreamExprScenarioOracle.java"

mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en \
    -Duser.country=US -Duser.variant= -cp "$classpath" \
    EPLOtherStreamExprScenarioOracle "$scenario" > "$output"

if ! jq -e '
    .version == "esper-parity/v1"
    and .id == "epl-other-stream-expr"
    and .javaCommit == "9e1b9f1cc9117fea4bf33ab043762c045d73839c"
    and (.java | startswith("17."))
    and (.records | type == "array" and length == 19)
    and ([.records[] | select(.operation == "listener" and .statement == "s0" and (.new | type == "array"))] | length) == 16
    and ([.records[] | select(.operation == "compile-error")] | length) == 3
    and ([.records[] | select(has("old"))] | length) == 0
    and ([.records[] | select(.case == "chained-parameterized")] | length) == 3
    and ([.records[] | select(.case == "stream-function")] | length) == 4
    and ([.records[] | select(.case == "instance-method-outer-join")] | length) == 1
    and ([.records[] | select(.case == "instance-method-static")] | length) == 2
    and ([.records[] | select(.case == "stream-instance-method-aliased")] | length) == 1
    and ([.records[] | select(.case == "stream-instance-method-no-alias")] | length) == 2
    and ([.records[] | select(.case == "join-stream-select")] | length) == 2
    and ([.records[] | select(.case == "pattern-stream-select")] | length) == 1
    and ([.records[] | select(.case == "invalid-select")] | length) == 3
    and ([.records[] | select(.case == "chained-parameterized") | .sequence]) == [1, 2, 3]
    and ([.records[] | select(.case == "stream-function") | .sequence]) == [range(1; 5)]
    and ([.records[] | select(.case == "instance-method-outer-join") | .sequence]) == [1]
    and ([.records[] | select(.case == "instance-method-static") | .sequence]) == [1, 2]
    and ([.records[] | select(.case == "stream-instance-method-aliased") | .sequence]) == [1]
    and ([.records[] | select(.case == "stream-instance-method-no-alias") | .sequence]) == [1, 2]
    and ([.records[] | select(.case == "join-stream-select") | .sequence]) == [1, 2]
    and ([.records[] | select(.case == "pattern-stream-select") | .sequence]) == [1]
    and ([.records[] | select(.case == "invalid-select") | .sequence]) == [0, 0, 0]
    and ([.records[] | select(.operation == "listener" and .time != "1970-01-01T00:00:00Z")] | length) == 0
    and (.records[0].new[0].fields | keys_unsorted | sort) == ["top.getChildOne(\"abc\",10).getChildTwo(\"append\")"]
    and .records[0].new[0].fields == {"top.getChildOne(\"abc\",10).getChildTwo(\"append\")":{"text":"abcappend"}}
    and .records[1].new[0].fields == {"top.getChildOne(\"abc\",10).getChildTwo(\"append\")":{"text":"abcappend"}}
    and .records[2].new[0].fields == {"val":"hello","val2":"hello2"}
    and .records[3].new[0].fields == {"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":100}
    and .records[4].new[0].fields == {"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":100}
    and .records[5].new[0].fields == {"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":100}
    and .records[6].new[0].fields == {"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":100}
    and .records[7].new[0].fields == {"symbol":"ACME","theString":{"state":"null"}}
    and .records[8].new[0].fields == {"def":{"state":"null"},"simpleprop":{"state":"null"},"symbol":"ACME"}
    and .records[9].new[0].fields == {"def":{"simpleProperty":"simple"},"simpleprop":"ACME","symbol":"ACME"}
    and .records[10].new[0].fields == {"pvf":792,"symbol":"ACME","volume":99}
    and .records[11].new[0].fields == {"s0.getPriceTimesVolume(3)":24,"s0.getVolume()":2}
    and .records[12].new[0].fields == {"c0":10,"c1":10}
    and .records[13].new[0].fields == {"s0stream":{"kind":"row","fields":{"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":0}},"s1stream":{"kind":"row","fields":{"intPrimitive":0,"theString":{"state":"null"}}}}
    and .records[14].new[0].fields == {"s0":{"kind":"row","fields":{"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":0}},"s1":{"kind":"row","fields":{"intPrimitive":0,"theString":{"state":"null"}}}}
    and .records[15].new[0].fields == {"e1":{"kind":"row","fields":{"feed":{"state":"null"},"price":0,"symbol":"ACME","volume":0}},"e2":{"kind":"row","fields":{"intPrimitive":1,"theString":"ACME"}}}
    and .records[16] == {"case":"invalid-select","operation":"compile-error","statement":"getstring-args","sequence":0}
    and .records[17].operation == "compile-error" and .records[17].statement == "abc-method" and .records[17].sequence == 0 and (.records[17].value | startswith("Failed to validate select-clause expression"))
    and .records[18].operation == "compile-error" and .records[18].statement == "pattern-indexed" and .records[18].sequence == 0 and (.records[18].value | startswith("Failed to validate select-clause expression"))
' "$output" >/dev/null 2>&1; then
    echo "Java oracle produced an invalid trace: $output" >&2
    exit 1
fi

echo "javaCommit=$actual_commit java=$java_version output=$output"
