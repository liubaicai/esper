param(
    [Parameter(Mandatory = $true)][string]$EsperRoot,
    [Parameter(Mandatory = $true)][string]$Scenario,
    [Parameter(Mandatory = $true)][string]$Output,
    [string]$JavaHome = $env:JAVA_HOME,
    [string]$MavenHome = $env:MAVEN_HOME,
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$expectedCommit = '9e1b9f1cc9117fea4bf33ab043762c045d73839c'
$scriptRoot = (Resolve-Path $PSScriptRoot).Path

if (-not (Test-Path -LiteralPath (Join-Path $EsperRoot '.git'))) {
    throw "Esper root is not a Git checkout: $EsperRoot"
}
if (-not (Test-Path -LiteralPath $Scenario)) {
    throw "Scenario was not found: $Scenario"
}
$actualCommit = (& git -C $EsperRoot rev-parse HEAD).Trim()
if ($actualCommit -ne $expectedCommit) {
    throw "Esper checkout is $actualCommit; expected $expectedCommit"
}

$java = if ([string]::IsNullOrWhiteSpace($JavaHome)) { 'java' } else { Join-Path $JavaHome 'bin\java.exe' }
$javac = if ([string]::IsNullOrWhiteSpace($JavaHome)) { 'javac' } else { Join-Path $JavaHome 'bin\javac.exe' }
$mvn = if ([string]::IsNullOrWhiteSpace($MavenHome)) { 'mvn' } else { Join-Path $MavenHome 'bin\mvn.cmd' }
foreach ($tool in @($java, $javac, $mvn)) {
    if ($tool -ne 'java' -and $tool -ne 'javac' -and $tool -ne 'mvn' -and -not (Test-Path -LiteralPath $tool)) {
        throw "Required tool was not found: $tool"
    }
}
$javaVersion = (& $java -version 2>&1 | Select-String 'version').ToString()
if ($javaVersion -notmatch '"17\.') {
    throw "Java 17 is required: $javaVersion"
}

if (-not $SkipBuild) {
    & $mvn -f (Join-Path $EsperRoot 'pom.xml') -pl compiler,runtime -am test-compile `
        '-DskipTests=true' '-Dcheckstyle.skip=true' '-Dgpg.skip=true' `
        '-Dfile.encoding=UTF-8' '-Dproject.build.sourceEncoding=UTF-8' `
        '-Dproject.reporting.outputEncoding=UTF-8' '-Duser.timezone=UTC'
    if ($LASTEXITCODE -ne 0) { throw 'Maven test-compile failed' }
}

$work = Join-Path ([IO.Path]::GetTempPath()) ('esper-context-hash-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $work | Out-Null
try {
    & $mvn -q -f (Join-Path $EsperRoot 'compiler\pom.xml') dependency:build-classpath `
        "-Dmdep.outputFile=$(Join-Path $work 'compiler-cp.txt')" '-Dmdep.includeScope=runtime' `
        '-Dgpg.skip=true' '-Dfile.encoding=UTF-8' '-Duser.timezone=UTC'
    if ($LASTEXITCODE -ne 0) { throw 'Maven compiler classpath failed' }
    & $mvn -q -f (Join-Path $EsperRoot 'runtime\pom.xml') dependency:build-classpath `
        "-Dmdep.outputFile=$(Join-Path $work 'runtime-cp.txt')" '-Dmdep.includeScope=runtime' `
        '-Dgpg.skip=true' '-Dfile.encoding=UTF-8' '-Duser.timezone=UTC'
    if ($LASTEXITCODE -ne 0) { throw 'Maven runtime classpath failed' }

    $classes = Join-Path $work 'classes'
    New-Item -ItemType Directory -Path $classes | Out-Null
    $entries = @(
        $classes,
        (Join-Path $EsperRoot 'common\target\classes'),
        (Join-Path $EsperRoot 'compiler\target\classes'),
        (Join-Path $EsperRoot 'runtime\target\classes'),
        (Join-Path $EsperRoot 'common-avro\target\classes'),
        (Join-Path $EsperRoot 'common-xmlxsd\target\classes')
    )
    $entries += (Get-Content -Raw -LiteralPath (Join-Path $work 'compiler-cp.txt')).Trim().Split(';')
    $entries += (Get-Content -Raw -LiteralPath (Join-Path $work 'runtime-cp.txt')).Trim().Split(';')
    $classpath = ($entries | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) -join ';'
    & $javac '-encoding' 'UTF-8' '-cp' $classpath '-d' $classes (Join-Path $scriptRoot 'ContextHashScenarioOracle.java')
    if ($LASTEXITCODE -ne 0) { throw 'javac failed for ContextHashScenarioOracle.java' }
    $outputParent = Split-Path -Parent $Output
    if (-not (Test-Path -LiteralPath $outputParent)) { New-Item -ItemType Directory -Path $outputParent -Force | Out-Null }
    & $java '-Dfile.encoding=UTF-8' '-Duser.timezone=UTC' '-Duser.language=en' '-Duser.country=US' `
        '-Duser.variant=' '-cp' $classpath 'ContextHashScenarioOracle' $Scenario | Set-Content -LiteralPath $Output -Encoding utf8
    if ($LASTEXITCODE -ne 0) { throw 'Java ContextHash oracle failed' }
    $trace = Get-Content -Raw -LiteralPath $Output | ConvertFrom-Json
    if ($trace.version -ne 'esper-parity/v1' -or [string]::IsNullOrWhiteSpace($trace.id) -or $null -eq $trace.records) {
        throw "Java oracle produced an invalid trace: $Output"
    }
    "javaCommit=$actualCommit output=$Output"
}
finally {
    if (Test-Path -LiteralPath $work) { Remove-Item -LiteralPath $work -Recurse -Force }
}
