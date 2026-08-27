#!/usr/bin/env sh
set -eu

usage() {
    cat >&2 <<'EOF'
usage: run-epl-insert-into-eventcol-rest.sh --esper-root PATH --scenario PATH --output PATH [--skip-build]

The Esper checkout must be exactly the pinned Java oracle commit. Java 17 and
Maven are selected from PATH unless MAVEN is provided. The base esper modules
(common, compiler, runtime) must be built first (`mvn -pl common,compiler,
runtime -am -DskipTests package` or an earlier eventcol runner's build); the
script additionally appends the compiler/runtime module dependency classpaths
from the local Maven repository when needed.
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

oracle_java="$script_root/EPLInsertIntoPopulateEventTypeColRestScenarioOracle.java"
oracle_class="$script_root/EPLInsertIntoPopulateEventTypeColRestScenarioOracle.class"

# Classpath: base esper module classes plus compiler/runtime dependency jars.
# Cache lives beside the sibling runners' caches but under distinct names so a
# parallel invocation cannot clobber them (/tmp/esper-cp-eventcol-rest.txt and
# /tmp/oracle-classes-eventcol-rest).
classpath_file="/tmp/esper-cp-eventcol-rest.txt"
if [ ! -f "$classpath_file" ]; then
    work_cp="/tmp/esper-eventcol-rest-cp"
    mkdir -p "$work_cp"
    mvn_bin=${MAVEN:-mvn}
    "$mvn_bin" -q -f "$esper_root/compiler/pom.xml" dependency:build-classpath \
        -Dmdep.outputFile="$work_cp/compiler-cp.txt" -Dmdep.includeScope=runtime \
        -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    "$mvn_bin" -q -f "$esper_root/runtime/pom.xml" dependency:build-classpath \
        -Dmdep.outputFile="$work_cp/runtime-cp.txt" -Dmdep.includeScope=runtime \
        -Dgpg.skip=true -Dfile.encoding=UTF-8 -Duser.timezone=UTC
    {
        printf '%s\n' "$esper_root/common/target/classes"
        printf '%s\n' "$esper_root/compiler/target/classes"
        printf '%s\n' "$esper_root/runtime/target/classes"
        printf '%s\n' "$(tr ':' '\n' < "$work_cp/compiler-cp.txt")"
        printf '%s\n' "$(tr ':' '\n' < "$work_cp/runtime-cp.txt")"
    } | grep -v '^$' | tr '\n' ':' | sed 's/:$//' > "$classpath_file"
fi

classes_dir="/tmp/oracle-classes-eventcol-rest"

if [ "$skip_build" -eq 0 ] || [ ! -f "$oracle_class" ]; then
    # Compile oracle (javac runs with the Esper checkout as its cwd, like the
    # sibling eventcol runners)
    echo "Compiling oracle..." >&2
    mkdir -p "$classes_dir"
    (cd "$esper_root" && javac -encoding UTF-8 -cp "$(cat "$classpath_file")" -d "$classes_dir" "$oracle_java")
fi

# Run oracle
echo "Running oracle..." >&2
(cd "$esper_root" && java -Duser.timezone=UTC \
    -cp "$(cat "$classpath_file"):$classes_dir" \
    EPLInsertIntoPopulateEventTypeColRestScenarioOracle "$scenario" > "$output")

echo "Trace written to: $output" >&2
