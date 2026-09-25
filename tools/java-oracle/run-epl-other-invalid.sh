#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
usage: run-epl-other-invalid.sh [-e ESPER_ROOT] [-s SCENARIO] [-o OUTPUT] [-j JAVA_HOME] [-m MVN_HOME]
Defaults:
  --esper-root /root/app/esper
  --scenario   testdata/parity/epl-other-invalid.json
  --output     testdata/parity/epl-other-invalid.trace.json
EOF
}

script_root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_root/../.." && pwd)
esper_root=/root/app/esper
scenario=
output=
java_home=
mvn_home=
expected_commit=9e1b9f1cc9117fea4bf33ab043762c045d73839c

while [ "$#" -ge 2 ]; do
  case "$1" in
    -e|--esper-root) esper_root=$2; shift 2 ;;
    -s|--scenario)   scenario=$2; shift 2 ;;
    -o|--output)     output=$2; shift 2 ;;
    -j|--java-home)  java_home=$2; shift 2 ;;
    -m|--mvn-home)   mvn_home=$2; shift 2 ;;
    *) usage; exit 2 ;;
  esac
done
[ "$#" -eq 0 ] || { usage; exit 2; }

[ -n "$scenario" ] || scenario="$repo_root/testdata/parity/epl-other-invalid.json"
[ -n "$output" ] || output="$repo_root/testdata/parity/epl-other-invalid.trace.json"

[ -d "$esper_root/.git" ] || { echo "Esper root is not a Git checkout: $esper_root" >&2; exit 1; }
[ -f "$scenario" ] || { echo "scenario was not found: $scenario" >&2; exit 1; }

actual_commit=$(git -C "$esper_root" rev-parse HEAD 2>/dev/null) || {
  echo "failed to resolve Esper commit in $esper_root" >&2
  exit 1
}
if [ "$actual_commit" != "$expected_commit" ]; then
  echo "Esper commit mismatch: expected $expected_commit, got $actual_commit" >&2
  exit 1
fi

java_bin=${JAVA:-java}
javac_bin=${JAVAC:-javac}
mvn_bin=${MVN:-mvn}
if [ -n "${JAVA_HOME:-}" ]; then
  java_bin="$JAVA_HOME/bin/java"
  javac_bin="$JAVA_HOME/bin/javac"
fi
if [ -n "$java_home" ]; then
  java_bin="$java_home/bin/java"
  javac_bin="$java_home/bin/javac"
fi
if [ -n "${MVN_HOME:-}" ]; then
  mvn_bin="$MVN_HOME/bin/mvn"
fi
if [ -n "$mvn_home" ]; then
  mvn_bin="$mvn_home/bin/mvn"
fi

oracle_src="$script_root/EPLOtherInvalidScenarioOracle.java"
[ -f "$oracle_src" ] || { echo "oracle source was not found: $oracle_src" >&2; exit 1; }

classpath_file=$(mktemp)
trap 'rm -f "$classpath_file"' EXIT HUP INT TERM

(
  cd "$esper_root"
  "$mvn_bin" -q -pl regression-lib -am -DskipTests dependency:build-classpath \
    -Dmdep.outputFile="$classpath_file" >/dev/null
)

classes_dir="$esper_root/regression-lib/target/classes"
[ -d "$classes_dir" ] || {
  echo "regression-lib classes were not found: $classes_dir (run mvn -pl regression-lib -am -DskipTests package first)" >&2
  exit 1
}

classpath="$classes_dir:$(cat "$classpath_file")"
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir" "$classpath_file"' EXIT HUP INT TERM

"$javac_bin" -encoding UTF-8 -cp "$classpath" -d "$work_dir" "$oracle_src"
"$java_bin" -cp "$work_dir:$classpath" EPLOtherInvalidScenarioOracle "$scenario" > "$output"
echo "wrote $output"
