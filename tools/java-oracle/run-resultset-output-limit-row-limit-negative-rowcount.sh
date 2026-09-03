#!/usr/bin/env sh
set -eu
usage() { cat >&2 <<'EOF'
usage: run-resultset-output-limit-row-limit-negative-rowcount.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]
EOF
 exit 2; }
esper_root=; scenario=; output=; skip_build=0
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c
script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
while [ "$#" -gt 0 ]; do
 case "$1" in
 --esper-root) [ "$#" -ge 2 ] || usage; esper_root=$2; shift 2;;
 --scenario) [ "$#" -ge 2 ] || usage; scenario=$2; shift 2;;
 --output) [ "$#" -ge 2 ] || usage; output=$2; shift 2;;
 --skip-build) skip_build=1; shift;; -h|--help) usage;; *) echo "unknown argument: $1" >&2; usage;; esac
done
[ -n "$esper_root" ] || { echo "--esper-root is required" >&2; exit 2; }
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }
actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || { echo "cannot read Esper Git commit" >&2; exit 1; }
[ "$actual_commit" = "$expected_commit" ] || { echo "Esper checkout is $actual_commit; expected $expected_commit" >&2; exit 1; }
java_bin=${JAVA:-java}; javac_bin=${JAVAC:-javac}; mvn_bin=${MAVEN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then java_bin="$JAVA_HOME/bin/java"; javac_bin="$JAVA_HOME/bin/javac"; fi
if [ -n "${MAVEN_HOME:-}" ]; then mvn_bin="$MAVEN_HOME/bin/mvn"; fi
command -v "$java_bin" >/dev/null 2>&1 || { echo "Java executable was not found: $java_bin" >&2; exit 1; }
command -v "$javac_bin" >/dev/null 2>&1 || { echo "javac executable was not found: $javac_bin" >&2; exit 1; }
command -v "$mvn_bin" >/dev/null 2>&1 || { echo "Maven executable was not found: $mvn_bin" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq executable was not found" >&2; exit 1; }
java_version=$($java_bin -version 2>&1 | sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p' | sed -n '1p')
[ "$java_version" = "17" ] || { echo "Java 17 is required; found ${java_version:-unknown}" >&2; exit 1; }
if ! jq -e '
 (type=="object") and ((keys_unsorted|sort)==["cases","description","id","javaCommit","javaFlags","javaNames","javaRuntimes","javaSource","javaStaticIds","steps","version"])
 and .version=="esper-parity/v1" and .id=="resultset-output-limit-row-limit-negative-rowcount"
 and .description=="ResultSetOutputLimitRowLimit ordinal 7: grouped snapshot negative rowcount with offset."
 and .javaCommit=="9e1b9f1cc9117fea4bf33ab043762c045d73839c" and .javaSource=="regression-lib/src/main/java/com/espertech/esper/regressionlib/suite/resultset/outputlimit/ResultSetOutputLimitRowLimit.java"
 and .javaRuntimes==["java-runtime-6bf4cf3ded03c9ff57fa"] and .javaNames==["ResultSetGroupedSnapshotNegativeRowcount"] and .javaStaticIds==["java-0109d52ee4e8b36575b9"] and .javaFlags==[]
 and (.cases|length==1) and .cases[0]=={"case":"grouped-snapshot-negative-rowcount","ordinal":7,"runtimeId":"java-runtime-6bf4cf3ded03c9ff57fa","executionName":"ResultSetGroupedSnapshotNegativeRowcount","observation":"listener+iterator","iteratorSnapshots":1,"epl":"@name(\u0027s0\u0027) select theString, sum(intPrimitive) as mysum from SupportBean#length(5) group by theString output snapshot every 10 seconds order by sum(intPrimitive) desc limit -1 offset 1"}
 and (.steps|length==8) and .steps[0]=={"op":"case","case":"grouped-snapshot-negative-rowcount"} and .steps[1]=={"op":"advance-time","at":"1970-01-01T00:00:01Z"}
 and .steps[2]=={"op":"snapshot","case":"grouped-snapshot-negative-rowcount","statement":"s0","mode":"ordered"}
 and ([.steps[3:7][]|.payload.theString]==["E1","E2","E3","E1"]) and ([.steps[3:7][]|.payload.intPrimitive]==[10,5,20,30])
 and .steps[7]=={"op":"advance-time","at":"1970-01-01T00:00:11Z"}
' "$scenario" >/dev/null 2>&1; then echo "scenario is not a valid negative rowcount replay: $scenario" >&2; exit 1; fi
if [ "$skip_build" -eq 0 ]; then "$mvn_bin" -f "$esper_root/pom.xml" -pl common,common-avro,compiler,runtime,regression-lib -am test-compile -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC; fi
work=$(mktemp -d "${TMPDIR:-/tmp}/esper-resultset-negative-rowcount.XXXXXX"); cleanup(){ rm -rf "$work"; }; trap cleanup EXIT HUP INT TERM
"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/compiler-cp.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/runtime-cp.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
classes="$work/classes"; mkdir -p "$classes"
classpath="$classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes:$esper_root/regression-lib/target/classes:$(tr '\n' ':' < "$work/compiler-cp.txt"):$(tr '\n' ':' < "$work/runtime-cp.txt")"
"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$classes" "$script_root/ResultSetOutputLimitRowLimitNegativeRowcountScenarioOracle.java"
mkdir -p "$(dirname "$output")"
"$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -Duser.language=en -Duser.country=US -Duser.variant= -cp "$classpath" ResultSetOutputLimitRowLimitNegativeRowcountScenarioOracle "$scenario" > "$output"
if ! jq -e '.version=="esper-parity/v1" and .id=="resultset-output-limit-row-limit-negative-rowcount" and .javaCommit=="9e1b9f1cc9117fea4bf33ab043762c045d73839c" and (.records|length==2) and ([.records[]|select(.operation=="snapshot")]|length)==1 and ([.records[]|select(.operation=="listener" and (.new|length==2) and ((.old//[])|length)==0)]|length)==1 and .records[1].time=="1970-01-01T00:00:11Z"' "$output" >/dev/null 2>&1; then echo "Java oracle produced an invalid trace: $output" >&2; exit 1; fi
echo "javaCommit=$actual_commit java=$java_version output=$output"
