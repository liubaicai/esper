#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-insert-into-populate-und-stream-select.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless JAVA_HOME/MAVEN_HOME are provided. The
avro representation additionally requires the esper common-avro module
classes, the Avro 1.11.3 jar and Jackson jars on the classpath; the script
derives them from the local Maven repository when present.
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
[ -n "$scenario" ] || { echo "--scenario is required" >&2; exit 2; }
[ -n "$output" ] || { echo "--output is required" >&2; exit 2; }
[ -f "$scenario" ] || { echo "scenario file not found: $scenario" >&2; exit 2; }

# Verify Java 17
java_version=$(java -version 2>&1 | head -1)
case "$java_version" in
    *17*) ;;
    *) echo "Java 17 required, got: $java_version" >&2; exit 2 ;;
esac

# Verify commit
actual_commit=$(cd "$esper_root" && git rev-parse HEAD)
if [ "$actual_commit" != "$expected_commit" ]; then
    echo "WARNING: Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
fi

# Resolve scenario/output to absolute paths; the oracle runs with the Esper
# checkout as its working directory so relative repo paths would not resolve.
scenario=$(CDPATH= cd -- "$(dirname -- "$scenario")" && pwd)/$(basename -- "$scenario")
output_dir=$(CDPATH= cd -- "$(dirname -- "$output")" && pwd)
output="$output_dir/$(basename -- "$output")"

oracle_java="$script_root/EPLInsertIntoPopulateUndStreamSelectScenarioOracle.java"
oracle_class="$script_root/EPLInsertIntoPopulateUndStreamSelectScenarioOracle.class"

# Classpath: base esper modules plus Avro/Jackson dependencies for the avro reps.
classpath_file="/tmp/esper-cp-avro-iups.txt"
if [ ! -f "$classpath_file" ]; then
    work_cp="/tmp/esper-iups-cp"
    mkdir -p "$work_cp"
    mvn_bin=${MAVEN:-mvn}
    "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
        -Dmdep.outputFile="$work_cp/compiler-cp.txt" -Dmdep.includeScope=runtime \
        -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
        -Dmdep.outputFile="$work_cp/runtime-cp.txt" -Dmdep.includeScope=runtime \
        -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    avro_jar=$(find "$HOME/.m2/repository/org/apache/avro/avro" -name 'avro-*.jar' ! -name '*sources*' ! -name '*javadoc*' 2>/dev/null | sort -V | tail -1)
    jackson_databind=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-databind" -name 'jackson-databind-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    jackson_core=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-core" -name 'jackson-core-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    jackson_ann=$(find "$HOME/.m2/repository/com/fasterxml/jackson/core/jackson-annotations" -name 'jackson-annotations-*.jar' ! -name '*sources*' 2>/dev/null | sort -V | tail -1)
    {
        printf '%s\n' "$esper_root/common/target/classes"
        printf '%s\n' "$esper_root/compiler/target/classes"
        printf '%s\n' "$esper_root/runtime/target/classes"
        printf '%s\n' "$esper_root/common-avro/target/classes"
        printf '%s\n' "$(tr ':' '\n' < "$work_cp/compiler-cp.txt")"
        printf '%s\n' "$(tr ':' '\n' < "$work_cp/runtime-cp.txt")"
        [ -n "$avro_jar" ] && printf '%s\n' "$avro_jar"
        [ -n "$jackson_databind" ] && printf '%s\n' "$jackson_databind"
        [ -n "$jackson_core" ] && printf '%s\n' "$jackson_core"
        [ -n "$jackson_ann" ] && printf '%s\n' "$jackson_ann"
    } | grep -v '^$' | tr '\n' ':' | sed 's/:$//' > "$classpath_file"
fi

if [ "$skip_build" -eq 0 ] || [ ! -f "$oracle_class" ]; then
    # Compile oracle
    echo "Compiling oracle..." >&2
    (cd "$esper_root" && javac -encoding UTF-8 -cp "$(cat "$classpath_file")" -d /tmp/oracle-classes-iups "$oracle_java")
fi

# Run oracle: one JVM per case/phase invocation. Each invocation needs a
# fresh runtime because a runtime that has deployed @public path schemas
# cannot re-register those names within its lifetime (undeployAll does not
# release path types); a fresh JVM per invocation is the simplest way to
# guarantee that. A single-JVM whole-suite run truncates named-window-rep
# to 5 rows for exactly this reason.
seq_offset=0
run_dir=$(mktemp -d "${TMPDIR:-/tmp}/esper-iups-run.XXXXXX")
run_case () { # case_name phase_filter(empty=all)
    (cd "$esper_root" && java -Duser.timezone=UTC \
        -cp "$(cat "$classpath_file"):/tmp/oracle-classes-iups" \
        EPLInsertIntoPopulateUndStreamSelectScenarioOracle "$scenario" "$1" "$seq_offset" "$2" \
        > "$run_dir/$1$2.json")
    count=$(jq '.records | length' "$run_dir/$1$2.json")
    seq_offset=$((seq_offset + count))
}
run_case named-window-inherits-map ""
# json+inheritance schemas compile once per JVM: exec1 runs one phase per
# invocation, keeping the observable record stream identical.
for phase in a b; do run_case named-window-rep "$phase"; done
run_case stream-insert-w-widen ""

# Merge per-invocation traces in order (python: jq -s would reorder keys).
python3 - "$run_dir" "$output" <<'PYEOF'
import json, sys, os
rd, out = sys.argv[1], sys.argv[2]
files = ["named-window-inherits-map.json", "named-window-repa.json",
         "named-window-repb.json", "stream-insert-w-widen.json"]
records = []
trace = None
for f in files:
    d = json.load(open(os.path.join(rd, f)))
    trace = d
    records.extend(d["records"])
for i, rec in enumerate(records):
    rec["sequence"] = i + 1
trace["records"] = records
json.dump(trace, open(out, "w"))
PYEOF
rm -rf "$run_dir"

echo "Trace written to: $output" >&2
