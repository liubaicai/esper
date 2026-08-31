#!/usr/bin/env sh
set -eu
usage(){ echo "usage: $0 --esper-root PATH --scenario PATH --output PATH [--skip-build]" >&2; exit 2; }
esper_root=; scenario=; output=; skip_build=0; expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c; root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
while [ "$#" -gt 0 ]; do case "$1" in --esper-root) [ "$#" -ge 2 ]||usage; esper_root=$2;shift 2;; --scenario) [ "$#" -ge 2 ]||usage;scenario=$2;shift 2;; --output) [ "$#" -ge 2 ]||usage;output=$2;shift 2;; --skip-build)skip_build=1;shift;; -h|--help)usage;; *)usage;; esac; done
[ -n "$esper_root" ]&&[ -n "$scenario" ]&&[ -n "$output" ]||usage; [ -d "$esper_root/.git" ]||{ echo "not git checkout" >&2;exit 1;}; [ -f "$scenario" ]||{ echo "scenario missing" >&2;exit 1;}; actual=$(git -C "$esper_root" rev-parse HEAD); [ "$actual" = "$expected_commit" ]||{ echo "wrong Esper commit: $actual" >&2;exit 1;}
java_bin=${JAVA:-java}; javac_bin=${JAVAC:-javac}; mvn_bin=${MAVEN:-mvn}; [ -n "${JAVA_HOME:-}" ]&&java_bin="$JAVA_HOME/bin/java"&&javac_bin="$JAVA_HOME/bin/javac"; [ -n "${MAVEN_HOME:-}" ]&&mvn_bin="$MAVEN_HOME/bin/mvn"; command -v "$java_bin" >/dev/null||exit 1; command -v "$javac_bin" >/dev/null||exit 1; command -v "$mvn_bin" >/dev/null||exit 1; command -v jq >/dev/null||exit 1
ver=$($java_bin -version 2>&1|sed -n 's/.*version "\([0-9][0-9]*\).*/\1/p'|sed -n '1p'); [ "$ver" = 17 ]||{ echo "Java 17 required" >&2;exit 1;}
python3 - "$scenario" <<'PY'
import json,sys

def dup(p):
 d={}
 for k,v in p:
  if k in d: raise ValueError()
  d[k]=v
 return d
try:
 with open(sys.argv[1],encoding='utf8') as f:s=json.load(f,object_pairs_hook=dup,parse_constant=lambda x: (_ for _ in()).throw(ValueError()))
 if not isinstance(s,dict) or set(s)!={'version','id','description','javaCommit','javaSource','javaRuntimes','javaNames','javaStaticIds','javaFlags','cases','steps'}:raise ValueError()
 if s['version']!='esper-parity/v1' or s['id']!='resultset-aggregate-sorted-no-data-window' or len(s['steps'])!=5:raise ValueError()
 if s['javaRuntimes']!=['java-runtime-af551963966a83d26468'] or s['javaNames']!=['ResultSetAggregateNoDataWindow'] or s['javaStaticIds']!=['java-f645b6fb41fd8c63d7f0'] or s['javaFlags']!=[]:raise ValueError()
 if len(s['cases'])!=1 or len(s['cases'][0])!=7 or s['cases'][0]['ordinal']!=6 or s['cases'][0]['observation']!='listener' or s['cases'][0]['iteratorSnapshots']!=0:raise ValueError()
 for x in s['steps'][1:]:
  if set(x)!={'op','eventType','payload'} or x['op']!='send' or x['eventType']!='SupportBean' or set(x['payload'])!={'theString','intPrimitive'}:raise ValueError()
except Exception:sys.exit(1)
PY
[ "$skip_build" -eq 1 ]||"$mvn_bin" -q -f "$esper_root/pom.xml" -pl compiler,runtime -am test-compile -DskipTests=true -Dcheckstyle.skip=true -Dgpg.skip=true -Duser.timezone=UTC
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
"$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/c.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true
"$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath -Dmdep.outputFile="$work/r.txt" -Dmdep.includeScope=runtime -Dgpg.skip=true
mkdir "$work/classes"; cp="$work/classes:$esper_root/common/target/classes:$esper_root/compiler/target/classes:$esper_root/runtime/target/classes:$esper_root/common-avro/target/classes:$esper_root/common-xmlxsd/target/classes:$(tr '\n' ':' < "$work/c.txt"):$(tr '\n' ':' < "$work/r.txt")"; "$javac_bin" -encoding UTF-8 -cp "$cp" -d "$work/classes" "$root/ResultSetAggregateSortedNoDataWindowScenarioOracle.java"; mkdir -p "$(dirname "$output")"; "$java_bin" -Dfile.encoding=UTF-8 -Duser.timezone=UTC -cp "$cp" ResultSetAggregateSortedNoDataWindowScenarioOracle "$scenario" > "$output"; jq -e '.version=="esper-parity/v1" and .id=="resultset-aggregate-sorted-no-data-window" and (.records|type=="array" and length==4)' "$output" >/dev/null; echo "javaCommit=$actual output=$output"
